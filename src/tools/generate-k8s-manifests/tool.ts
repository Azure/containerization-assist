/**
 * Generate Kubernetes Manifests tool using the new Tool pattern
 */

import { Success, Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { Tool } from '@/types/tool';
import { storeToolResults } from '@/lib/tool-helpers';
import { promptTemplates, K8sManifestPromptParams } from '@/ai/prompt-templates';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { sampleWithRerank } from '@/mcp/ai/sampling-runner';
import { createKubernetesScoringFunction } from '@/lib/scoring';
import { generateK8sManifestsSchema } from './schema';
import type { AIResponse } from '../ai-response-types';
import { repairK8sManifests, shouldRepairManifests } from '../shared/k8s-repair';
import type { KnowledgeEnhancementResult } from '@/mcp/ai/knowledge-enhancement';
import { extractKubernetesContent } from '@/lib/content-extraction';
import { createKubernetesValidator } from '@/validation/kubernetes-validator';
import { createK8sSchemaValidator } from '@/validation/k8s-schema-validator';
import { createK8sNormalizer } from '@/validation/k8s-normalizer';
import { mergeMultipleReports } from '@/validation/merge-reports';
import type { ModuleInfo } from '@/tools/analyze-repo/schema';
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

/**
 * Generate K8s manifests for a single module or app
 */
async function generateSingleManifest(
  input: z.infer<typeof generateK8sManifestsSchema>,
  ctx: ToolContext,
  targetModule?: ModuleInfo,
): Promise<Result<AIResponse>> {
  const {
    namespace = 'default',
    replicas = 3,
    serviceType = 'ClusterIP',
    ingressEnabled = false,
    resources,
    healthCheck,
  } = input;

  const targetModuleName = targetModule?.name;

  // Retrieve imageId from session if not provided
  let imageId = input.imageId;
  if (!imageId && input.sessionId && ctx.session) {
    const buildResult = ctx.session.getResult<{ tags?: string[] }>('build-image');
    if (buildResult?.tags && buildResult.tags.length > 0) {
      imageId = buildResult.tags[0];
      ctx.logger.info({ imageId }, 'Using image from session (build-image)');
    }
  }

  // Retrieve appName from session if not provided
  let appName = input.appName;
  if (!appName && input.sessionId && ctx.session) {
    // If generating for a specific module, use module name
    if (targetModuleName) {
      appName = targetModuleName;
      ctx.logger.info({ appName, moduleName: targetModuleName }, 'Using module name as app name');
    } else {
      appName = ctx.session.get<string>('appName');
      if (appName) {
        ctx.logger.info({ appName }, 'Using app name from session (analyze-repo)');
      }
    }
  }

  // Retrieve port from session if not explicitly provided
  let port = input.port;
  if (!port && input.sessionId && ctx.session) {
    // If generating for a specific module, use module's port
    if (targetModule?.ports && targetModule.ports.length > 0) {
      port = targetModule.ports[0];
      ctx.logger.info({ port, moduleName: targetModuleName }, 'Using port from module data');
    } else {
      const appPorts = ctx.session.get<number[]>('appPorts');
      if (appPorts && appPorts.length > 0) {
        port = appPorts[0];
        ctx.logger.info({ port }, 'Using port from session (analyze-repo)');
      }
    }
  }
  if (!port) {
    port = 8080; // Default fallback
  }

  // Validate required parameters
  if (!imageId) {
    return Failure(
      'Docker image is required. Either provide imageId parameter or run build-image first with a sessionId.',
    );
  }
  if (!appName) {
    return Failure(
      'Application name is required. Either provide appName parameter or run analyze-repo first with a sessionId.',
    );
  }

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

  // Execute via AI with deterministic sampling
  const samplingResult = await sampleWithRerank(
    ctx,
    async (attemptIndex) => ({
      ...toMCPMessages(messages),
      maxTokens: 8192,
      modelPreferences: {
        hints: [{ name: 'kubernetes-manifests' }],
        intelligencePriority: 0.85,
        speedPriority: attemptIndex > 0 ? 0.7 : 0.4,
        costPriority: 0.3,
      },
    }),
    createKubernetesScoringFunction(),
    {},
  );

  if (!samplingResult.ok) {
    return Failure(`K8s manifest generation failed: ${samplingResult.error}`);
  }

  const responseText = samplingResult.value.text;
  if (!responseText) {
    return Failure('Empty response from AI');
  }

  ctx.logger.info(
    {
      score: samplingResult.value.score,
      scoreBreakdown: samplingResult.value.scoreBreakdown,
    },
    'K8s manifests generated with sampling',
  );

  // Parse and validate the generated manifests using unified extraction
  try {
    const extraction = extractKubernetesContent(responseText);
    if (!extraction.success) {
      return Failure(`Failed to extract Kubernetes manifests: ${extraction.error}`);
    }

    const manifests = extraction.content as KubernetesManifest[];

    // For validation and repair, we need the original YAML content
    let manifestsContent = responseText;

    // Validate each extracted manifest
    for (const manifest of manifests) {
      try {
        if (!manifest || typeof manifest !== 'object') {
          ctx.logger.warn(
            { manifest: JSON.stringify(manifest).substring(0, 100) },
            'Skipping non-object manifest',
          );
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
              'Ingress using outdated API version',
            );
          }
        }
      } catch (parseError) {
        ctx.logger.error(
          {
            error: parseError instanceof Error ? parseError.message : String(parseError),
            manifest: JSON.stringify(manifest).substring(0, 200),
          },
          'Failed to validate manifest',
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

    // Enhanced validation with schema validation, normalization, and repair
    const schemaValidator = createK8sSchemaValidator({
      allowUnknownResources: true,
    });
    const rulesValidator = createKubernetesValidator();
    const normalizer = createK8sNormalizer({
      addSecurityContext: true,
      fixSelectors: true,
      standardizeLabels: true,
    });

    // Perform parallel validation
    const [schemaReport, rulesReport] = await Promise.all([
      schemaValidator.validate(manifestsContent),
      Promise.resolve(rulesValidator.validate(manifestsContent)),
    ]);

    // Merge validation reports
    const combinedReport = mergeMultipleReports([schemaReport, rulesReport]);

    ctx.logger.info(
      {
        schemaScore: schemaReport.score,
        rulesScore: rulesReport.score,
        combinedScore: combinedReport.score,
        totalErrors: combinedReport.errors,
        totalWarnings: combinedReport.warnings,
      },
      'Enhanced K8s manifest validation completed',
    );

    // Apply normalization
    const normalizationResult = normalizer.normalize(manifestsContent);
    if (normalizationResult.changes.length > 0) {
      manifestsContent = normalizationResult.normalized;
      ctx.logger.info(
        {
          changes: normalizationResult.changes.length,
          changeDetails: normalizationResult.changes.map((c) => `${c.resource}: ${c.description}`),
        },
        'Manifest normalization applied',
      );
    }

    // Attempt repair if validation still shows issues
    if (shouldRepairManifests(combinedReport)) {
      ctx.logger.warn(
        {
          errors: combinedReport.errors,
          score: combinedReport.score,
          grade: combinedReport.grade,
        },
        'Attempting self-repair of K8s manifests',
      );

      const originalRequirements = `App: ${input.appName}, Image: ${input.imageId}, Replicas: ${input.replicas || 3}, Port: ${input.port || 8080}`;
      const repairResult = await repairK8sManifests(
        ctx,
        manifestsContent,
        combinedReport.results,
        originalRequirements,
      );

      if (repairResult.ok && repairResult.value.errorsReduced > 0) {
        manifestsContent = repairResult.value.repaired;
        ctx.logger.info(
          {
            improvements: repairResult.value.improvements,
            originalScore: repairResult.value.originalScore,
            repairedScore: repairResult.value.repairedScore,
            errorsReduced: repairResult.value.errorsReduced,
          },
          'Self-repair completed successfully',
        );

        // Re-parse the repaired manifests for validation
        try {
          const repairedDocs = manifestsContent.split(/^---$/m).filter((doc) => doc.trim());
          const repairedManifests: KubernetesManifest[] = [];

          for (const doc of repairedDocs) {
            try {
              const manifest = yaml.load(doc) as KubernetesManifest;
              if (
                manifest &&
                typeof manifest === 'object' &&
                manifest.apiVersion &&
                manifest.kind
              ) {
                repairedManifests.push(manifest);
              }
            } catch (parseError) {
              // If parsing fails after repair, keep original manifests
              ctx.logger.warn(
                { parseError },
                'Failed to parse repaired manifest, keeping original',
              );
              break;
            }
          }

          if (repairedManifests.length > 0) {
            manifests.splice(0, manifests.length, ...repairedManifests);
          }
        } catch (error) {
          ctx.logger.warn({ error }, 'Failed to re-parse repaired manifests, keeping original');
        }
      } else if (repairResult.ok) {
        ctx.logger.info('Self-repair attempted but no improvements made');
      } else {
        ctx.logger.warn(
          { error: repairResult.error },
          'Self-repair failed, keeping original manifests',
        );
      }
    } else {
      ctx.logger.info(
        { score: combinedReport.score },
        'Manifests passed validation, no repair needed',
      );
    }

    // Apply knowledge enhancement if there are validation issues
    let knowledgeEnhancement: KnowledgeEnhancementResult | undefined;
    let finalManifestsContent = manifestsContent;

    if (combinedReport.score < 90) {
      try {
        const { enhanceWithKnowledge, createEnhancementFromValidation } = await import(
          '@/mcp/ai/knowledge-enhancement'
        );

        const enhancementRequest = createEnhancementFromValidation(
          manifestsContent,
          'kubernetes',
          combinedReport.results
            .filter((r) => !r.passed)
            .map((r) => ({
              message: r.message || 'Validation issue',
              severity: r.metadata?.severity === 'error' ? 'error' : 'warning',
              category: r.ruleId?.split('-')[0] || 'general',
            })),
          'security',
        );

        // Add specific enhancement goal for Kubernetes manifests
        enhancementRequest.userQuery = `Original requirements: App: ${input.appName}, Image: ${input.imageId}, Replicas: ${input.replicas || 3}, Port: ${input.port || 8080}`;

        const enhancementResult = await enhanceWithKnowledge(enhancementRequest, ctx);

        if (enhancementResult.ok) {
          knowledgeEnhancement = enhancementResult.value;
          finalManifestsContent = knowledgeEnhancement.enhancedContent;

          ctx.logger.info(
            {
              knowledgeAppliedCount: knowledgeEnhancement.knowledgeApplied.length,
              confidence: knowledgeEnhancement.confidence,
              enhancementAreas: knowledgeEnhancement.analysis.enhancementAreas.length,
            },
            'Knowledge enhancement applied to Kubernetes manifests',
          );
        } else {
          ctx.logger.warn(
            { error: enhancementResult.error },
            'Knowledge enhancement failed, using original manifests',
          );
        }
      } catch (enhancementError) {
        ctx.logger.debug(
          {
            error:
              enhancementError instanceof Error
                ? enhancementError.message
                : String(enhancementError),
          },
          'Knowledge enhancement threw exception, continuing without enhancement',
        );
      }
    }

    // Write manifests to file if path is provided
    let manifestPath = '';
    if (input.path) {
      manifestPath = path.isAbsolute(input.path)
        ? input.path
        : path.resolve(process.cwd(), input.path);

      const filename = `${input.appName}-manifests.yaml`;
      manifestPath = path.join(manifestPath, filename);

      await fs.writeFile(manifestPath, finalManifestsContent, 'utf-8');
      ctx.logger.info({ manifestPath }, 'Kubernetes manifests written to disk');
    }

    // Prepare the result
    const result = {
      manifests: finalManifestsContent,
      manifestPath,
      validatedResources: manifests.map((m) => ({ kind: m.kind, name: m.metadata?.name })),
      sessionId: input.sessionId,
      ...(knowledgeEnhancement && {
        analysis: {
          enhancementAreas: knowledgeEnhancement.analysis.enhancementAreas,
          confidence: knowledgeEnhancement.confidence,
          knowledgeApplied: knowledgeEnhancement.knowledgeApplied,
        },
        confidence: knowledgeEnhancement.confidence,
        suggestions: knowledgeEnhancement.suggestions,
      }),
      workflowHints: {
        nextStep: 'deploy',
        message: `Kubernetes manifests generated and validated successfully. ${knowledgeEnhancement ? `Enhanced with ${knowledgeEnhancement.knowledgeApplied.length} knowledge improvements. ` : ''}${manifestPath ? `Saved to ${manifestPath}. ` : ''}Use "deploy" with sessionId ${input.sessionId || '<sessionId>'} to deploy to your cluster.`,
      },
    };

    // Store in sessionManager for cross-tool persistence using helper
    await storeToolResults(ctx, input.sessionId, 'generate-k8s-manifests', {
      manifests: finalManifestsContent,
      manifestPath,
      validatedResources: manifests.map((m) => ({ kind: m.kind, name: m.metadata?.name })),
    });

    return Success(result);
  } catch (e) {
    const error = e as Error;
    return Failure(`Manifest generation failed: ${error.message}`);
  }
}

/**
 * Main run function - orchestrates single or multi-module generation
 */
async function run(
  input: z.infer<typeof generateK8sManifestsSchema>,
  ctx: ToolContext,
): Promise<Result<AIResponse>> {
  const { sessionId } = input;

  // Check for multi-module/monorepo scenario
  if (sessionId && ctx.session) {
    const isMonorepo = ctx.session.get<boolean>('isMonorepo');
    const modules = ctx.session.get<ModuleInfo[]>('modules');

    if (isMonorepo && modules && modules.length > 0) {
      // User explicitly specified a module
      if (input.moduleName) {
        const targetModule = modules.find((m) => m.name === input.moduleName);
        if (!targetModule) {
          return Failure(
            `Module "${input.moduleName}" not found. Available modules: ${modules.map((m) => m.name).join(', ')}`,
          );
        }
        ctx.logger.info(
          { moduleName: targetModule.name, modulePath: targetModule.path },
          'Generating K8s manifests for specific module',
        );
        return generateSingleManifest(input, ctx, targetModule);
      }

      // No module specified - generate for all modules automatically
      ctx.logger.info(
        { moduleCount: modules.length },
        'Generating K8s manifests for all modules in monorepo',
      );

      const results: Array<{ module: string; success: boolean; path?: string; error?: string }> =
        [];
      const manifests: Array<{ module: string; content: string; path?: string }> = [];

      for (const module of modules) {
        ctx.logger.info({ moduleName: module.name }, 'Generating K8s manifests for module');

        const result = await generateSingleManifest(input, ctx, module);

        if (result.ok) {
          const value = result.value as {
            content?: string;
            workflowHints?: { message?: string };
          };
          const extractedPath = value.workflowHints?.message?.match(/Saved to (.+?)\./)?.[1];
          results.push({
            module: module.name,
            success: true,
            ...(extractedPath ? { path: extractedPath } : {}),
          });
          manifests.push({
            module: module.name,
            content: value.content || '',
            ...(extractedPath ? { path: extractedPath } : {}),
          });
          ctx.logger.info({ moduleName: module.name }, 'K8s manifests generated successfully');
        } else {
          results.push({
            module: module.name,
            success: false,
            error: result.error,
          });
          ctx.logger.warn(
            { moduleName: module.name, error: result.error },
            'Failed to generate K8s manifests for module',
          );
        }
      }

      // Store multi-module results in session
      if (ctx.session) {
        ctx.session.storeResult('generate-k8s-manifests-multi', {
          modules: results,
          manifests,
        });
        ctx.session.set('k8sManifestsGenerated', true);
      }

      const successCount = results.filter((r) => r.success).length;
      const failureCount = results.filter((r) => !r.success).length;

      if (successCount === 0) {
        return Failure(
          `Failed to generate K8s manifests for all ${modules.length} modules:\n${results.map((r) => `- ${r.module}: ${r.error}`).join('\n')}`,
        );
      }

      // Build summary response
      const summary = `Generated Kubernetes manifests for ${successCount}/${modules.length} modules:\n${results
        .filter((r) => r.success)
        .map((r) => `✅ ${r.module}${r.path ? `: ${r.path}` : ''}`)
        .join('\n')}${
        failureCount > 0
          ? `\n\n⚠️  Failed modules (${failureCount}):\n${results
              .filter((r) => !r.success)
              .map((r) => `❌ ${r.module}: ${r.error}`)
              .join('\n')}`
          : ''
      }`;

      return Success({
        content: summary,
        language: 'text',
        confidence: successCount / modules.length,
        suggestions: [
          `Successfully generated ${successCount} K8s manifest(s)`,
          failureCount > 0 ? `${failureCount} module(s) failed` : 'All modules successful',
        ],
        analysis: {
          enhancementAreas: [],
          confidence: successCount / modules.length,
          knowledgeApplied: [],
        },
        workflowHints: {
          nextStep: 'deploy',
          message: `Kubernetes manifests generated for ${successCount} module(s). Use "prepare-cluster" and "deploy" for each module to deploy to your cluster.`,
        },
      });
    }
  }

  // Single-module repository or no session data - generate for single app
  return generateSingleManifest(input, ctx);
}

const tool: Tool<typeof generateK8sManifestsSchema, AIResponse> = {
  name,
  description,
  category: 'kubernetes',
  version,
  schema: generateK8sManifestsSchema,
  metadata: {
    aiDriven: true,
    knowledgeEnhanced: true,
    samplingStrategy: 'single',
    enhancementCapabilities: [
      'content-generation',
      'manifest-generation',
      'kubernetes-optimization',
    ],
  },
  run,
};

export default tool;
