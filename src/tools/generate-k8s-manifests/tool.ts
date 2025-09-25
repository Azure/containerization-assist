/**
 * Generate Kubernetes Manifests tool using the new Tool pattern
 */

import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { Tool } from '@/types/tool';
import { promptTemplates, K8sManifestPromptParams } from '@/ai/prompt-templates';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { generateK8sManifestsSchema } from './schema';
import type { AIResponse } from '../ai-response-types';
import * as yaml from 'js-yaml';
import { promises as fs } from 'node:fs';
import path from 'node:path';
import type { z } from 'zod';

// Type definition for Kubernetes manifests
interface KubernetesManifest extends Record<string, unknown> {
  apiVersion: string;
  kind: string;
  metadata?: {
    name?: string;
    namespace?: string;
    labels?: Record<string, string>;
    [key: string]: unknown;
  };
  spec?: {
    selector?: {
      matchLabels?: Record<string, string>;
      [key: string]: unknown;
    };
    template?: {
      metadata?: {
        labels?: Record<string, string>;
        [key: string]: unknown;
      };
      [key: string]: unknown;
    };
    [key: string]: unknown;
  };
}

const name = 'generate-k8s-manifests';
const description = 'Generate Kubernetes deployment manifests';
const version = '2.1.0';

async function run(
  input: z.infer<typeof generateK8sManifestsSchema>,
  ctx: ToolContext,
): Promise<Result<AIResponse>> {
  const {
    appName,
    imageId,
    namespace = 'default',
    replicas = 3,
    port = 8080,
    serviceType = 'ClusterIP',
    ingressEnabled = false,
    resources,
    healthCheck,
  } = input;

  // Generate prompt from template
  const promptParams = {
    appName,
    image: imageId,
    replicas,
    port,
    namespace,
    serviceType,
    ingressEnabled,
    healthCheck: healthCheck?.enabled === true,
    resources: resources?.limits
      ? {
          cpu: resources.limits.cpu,
          memory: resources.limits.memory,
        }
      : undefined,
  } as K8sManifestPromptParams;
  const basePrompt = promptTemplates.k8sManifests(promptParams);

  // Build messages using the new prompt engine
  const messages = await buildMessages({
    basePrompt,
    topic: 'generate_k8s_manifests',
    tool: 'generate-k8s-manifests',
    environment: 'production',
    contract: {
      name: 'kubernetes_manifests_v1',
      description: 'Generate Kubernetes YAML manifests',
    },
    knowledgeBudget: 4000, // Larger budget for K8s manifests
  });

  // Execute via AI with structured messages
  const mcpMessages = toMCPMessages(messages);
  const response = await ctx.sampling.createMessage({
    ...mcpMessages,
    maxTokens: 8192,
    modelPreferences: {
      hints: [{ name: 'kubernetes-manifests' }],
    },
  });

  // Parse and validate the generated manifests
  try {
    const responseText = response.content[0]?.text || '';
    let manifestsContent = responseText;

    // Clean up the response if needed
    if (manifestsContent.includes('```yaml')) {
      const yamlMatch = manifestsContent.match(/```yaml\n([\s\S]*?)```/);
      if (yamlMatch?.[1]) {
        manifestsContent = yamlMatch[1].trim();
      }
    } else if (manifestsContent.includes('```')) {
      manifestsContent = manifestsContent.replace(/```/g, '').trim();
    }

    // Parse YAML to validate it
    const manifests: KubernetesManifest[] = [];
    const docs = manifestsContent.split(/^---$/m).filter((doc) => doc.trim());

    for (const doc of docs) {
      try {
        const manifest = yaml.load(doc) as KubernetesManifest;
        if (!manifest || typeof manifest !== 'object') {
          ctx.logger.warn({ doc: doc.substring(0, 100) }, 'Skipping non-object manifest');
          continue;
        }

        // Validate essential Kubernetes fields
        if (!manifest.apiVersion || typeof manifest.apiVersion !== 'string') {
          return Failure(
            `Manifest missing apiVersion: ${JSON.stringify(manifest).substring(0, 100)}`,
          );
        }
        if (!manifest.kind || typeof manifest.kind !== 'string') {
          return Failure(`Manifest missing kind: ${JSON.stringify(manifest).substring(0, 100)}`);
        }
        if (!manifest.metadata?.name) {
          return Failure(`Manifest missing metadata.name: ${manifest.kind}`);
        }

        // Validate specific resource types
        if (manifest.kind === 'Deployment') {
          if (typeof manifest.apiVersion === 'string' && manifest.apiVersion !== 'apps/v1') {
            ctx.logger.warn(
              { apiVersion: manifest.apiVersion },
              'Deployment using non-standard API version',
            );
          }
          if (!manifest.spec?.selector?.matchLabels) {
            return Failure('Deployment missing spec.selector.matchLabels');
          }
          if (!manifest.spec?.template?.metadata?.labels) {
            return Failure('Deployment missing spec.template.metadata.labels');
          }
          // Check that selector matches template labels
          const selectorLabels = manifest.spec.selector.matchLabels;
          const templateLabels = manifest.spec.template.metadata.labels;
          for (const key in selectorLabels) {
            if (selectorLabels[key] !== templateLabels[key]) {
              return Failure(`Deployment selector label ${key} doesn't match template label`);
            }
          }
        }

        if (manifest.kind === 'Service') {
          if (typeof manifest.apiVersion === 'string' && manifest.apiVersion !== 'v1') {
            ctx.logger.warn(
              { apiVersion: manifest.apiVersion },
              'Service using non-standard API version',
            );
          }
          if (!manifest.spec?.selector) {
            return Failure('Service missing spec.selector');
          }
        }

        if (manifest.kind === 'Ingress') {
          if (
            typeof manifest.apiVersion === 'string' &&
            !manifest.apiVersion.startsWith('networking.k8s.io/')
          ) {
            ctx.logger.warn(
              { apiVersion: manifest.apiVersion },
              'Ingress using legacy API version',
            );
          }
        }

        manifests.push(manifest);
      } catch (parseError) {
        ctx.logger.error(
          {
            error: parseError instanceof Error ? parseError.message : String(parseError),
            doc: doc.substring(0, 200),
          },
          'Failed to parse YAML document',
        );
        return Failure(
          `Invalid YAML in manifest: ${parseError instanceof Error ? parseError.message : 'Unknown error'}`,
        );
      }
    }

    if (manifests.length === 0) {
      return Failure('No valid Kubernetes manifests were generated');
    }

    // Log what we validated
    ctx.logger.info(
      {
        manifestCount: manifests.length,
        kinds: manifests.map((m) => m.kind),
        names: manifests.map((m) => m.metadata?.name),
      },
      'Validated Kubernetes manifests',
    );

    // Write manifests to file if path is provided
    let manifestPath = '';
    if (input.path) {
      manifestPath = path.isAbsolute(input.path)
        ? input.path
        : path.resolve(process.cwd(), input.path);

      const filename = `${input.appName}-manifests.yaml`;
      manifestPath = path.join(manifestPath, filename);

      await fs.writeFile(manifestPath, manifestsContent, 'utf-8');
      ctx.logger.info({ manifestPath }, 'Kubernetes manifests written to disk');
    }

    return Success({
      manifests: manifestsContent,
      manifestPath,
      validatedResources: manifests.map((m) => ({ kind: m.kind, name: m.metadata?.name })),
      sessionId: input.sessionId,
      workflowHints: {
        nextStep: 'deploy',
        message: `Kubernetes manifests generated and validated successfully. ${manifestPath ? `Saved to ${manifestPath}. ` : ''}Use "deploy" with sessionId ${input.sessionId || '<sessionId>'} to deploy to your cluster.`,
      },
    });
  } catch (e) {
    const error = e as Error;
    return Failure(`Manifest generation failed: ${error.message}`);
  }
}

const tool: Tool<typeof generateK8sManifestsSchema, AIResponse> = {
  name,
  description,
  version,
  schema: generateK8sManifestsSchema,
  run,
};

export default tool;

// Keep legacy export for backward compatibility during migration
export { run as generateK8sManifests };

export const metadata = {
  name,
  description,
  version,
  aiDriven: true,
  knowledgeEnhanced: true,
};
