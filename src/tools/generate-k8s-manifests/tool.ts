import { Success, Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { promptTemplates, K8sManifestPromptParams } from '@/ai/prompt-templates';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { generateK8sManifestsSchema, type GenerateK8sManifestsParams } from './schema';
import type { AIResponse } from '../ai-response-types';
import * as yaml from 'js-yaml';
import { promises as fs } from 'node:fs';
import path from 'node:path';

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

export async function generateK8sManifests(
  params: GenerateK8sManifestsParams,
  context: ToolContext,
): Promise<Result<AIResponse>> {
  const validatedParams = generateK8sManifestsSchema.parse(params);
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
  } = validatedParams;

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
    topic: TOPICS.GENERATE_K8S_MANIFESTS,
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
  const response = await context.sampling.createMessage({
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
          context.logger.warn({ doc: doc.substring(0, 100) }, 'Skipping non-object manifest');
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
            context.logger.warn(
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
            context.logger.warn(
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
            context.logger.warn(
              { apiVersion: manifest.apiVersion },
              'Ingress using legacy API version',
            );
          }
        }

        manifests.push(manifest);
      } catch (parseError) {
        context.logger.error(
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
    context.logger.info(
      {
        manifestCount: manifests.length,
        kinds: manifests.map((m) => m.kind),
        names: manifests.map((m) => m.metadata?.name),
      },
      'Validated Kubernetes manifests',
    );

    // Write manifests to file if path is provided
    let manifestPath = '';
    if (validatedParams.path) {
      manifestPath = path.isAbsolute(validatedParams.path)
        ? validatedParams.path
        : path.resolve(process.cwd(), validatedParams.path);

      const filename = `${validatedParams.appName}-manifests.yaml`;
      manifestPath = path.join(manifestPath, filename);

      await fs.writeFile(manifestPath, manifestsContent, 'utf-8');
      context.logger.info({ manifestPath }, 'Kubernetes manifests written to disk');
    }

    return Success({
      manifests: manifestsContent,
      manifestPath,
      validatedResources: manifests.map((m) => ({ kind: m.kind, name: m.metadata?.name })),
      sessionId: validatedParams.sessionId,
      workflowHints: {
        nextStep: 'deploy',
        message: `Kubernetes manifests generated and validated successfully. ${manifestPath ? `Saved to ${manifestPath}. ` : ''}Use "deploy" with sessionId ${validatedParams.sessionId || '<sessionId>'} to deploy to your cluster.`,
      },
    });
  } catch (e) {
    const error = e as Error;
    return Failure(`Manifest generation failed: ${error.message}`);
  }
}

export const metadata = {
  name: 'generate-k8s-manifests',
  description: 'Generate Kubernetes deployment manifests',
  version: '2.1.0',
  aiDriven: true,
  knowledgeEnhanced: true,
};
