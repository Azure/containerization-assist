/**
 * Generate K8s Manifests Tool - Standardized Implementation
 *
 * Generates Kubernetes manifests for application deployment
 * Uses standardized helpers for consistent behavior
 */

import { joinPaths } from '@lib/path-utils';
import { getToolLogger, createToolTimer } from '@lib/tool-helpers';
import { withDefaults, K8S_DEFAULTS } from '@lib/param-defaults';
import { extractErrorMessage } from '@lib/error-utils';
import { promises as fs } from 'node:fs';
import {
  ensureSession,
  defineToolIO,
  useSessionSlice,
  getSessionSlice,
} from '@mcp/tool-session-helpers';
import { aiGenerateWithSampling } from '@mcp/tool-ai-helpers';
import { enhancePromptWithKnowledge } from '@lib/ai-knowledge-enhancer';
import type { SamplingOptions } from '@lib/sampling';
import { createStandardProgress } from '@mcp/progress-helper';
// Moved to tool-helpers
import type { ToolContext } from '@mcp/context';
import { Success, Failure, type Result } from '@types';
import { stripFencesAndNoise, isValidKubernetesContent } from '@lib/text-processing';
import { createKubernetesValidator, getValidationSummary } from '@validation';
import { scoreConfigCandidates } from '@lib/integrated-scoring';
import * as yaml from 'js-yaml';
import { generateK8sManifestsSchema, type GenerateK8sManifestsParams } from './schema';
import { z } from 'zod';
import { buildImageSchema } from '@tools/build-image/schema';
import { analyzeRepoSchema } from '@tools/analyze-repo/schema';
// Note: Tool now uses GenerateK8sManifestsParams from schema for type safety

/**
 * Result from K8s manifest generation
 */
export interface GenerateK8sManifestsResult {
  /** Generated manifests as YAML */
  manifests: string;
  /** Output directory path */
  outputPath: string;
  /** List of generated resources */
  resources: Array<{
    kind: string;
    name: string;
    namespace: string;
  }>;
  /** Warnings about manifest configuration */
  warnings?: string[];
  /** Session ID for reference */
  sessionId?: string;
  /** Sampling metadata if sampling was used */
  samplingMetadata?: {
    stoppedEarly?: boolean;
    candidatesGenerated: number;
    winnerScore: number;
    samplingDuration?: number;
  };
  /** Winner score if sampling was used */
  winnerScore?: number;
  /** Score breakdown if requested */
  scoreBreakdown?: Record<string, number>;
  /** All candidates if requested */
  allCandidates?: Array<{
    id: string;
    content: string;
    score: number;
    scoreBreakdown: Record<string, number>;
    rank?: number;
  }>;
  /** Validation score and report */
  validationScore?: number;
  validationGrade?: string;
  validationReport?: string;
  /** Quality score from config scoring */
  score?: number;
}

// Define the result schema for type safety
const GenerateK8sManifestsResultSchema = z.object({
  manifests: z.string(),
  outputPath: z.string(),
  resources: z.array(
    z.object({
      kind: z.string(),
      name: z.string(),
      namespace: z.string(),
    }),
  ),
  warnings: z.array(z.string()).optional(),
  sessionId: z.string().optional(),
  samplingMetadata: z
    .object({
      stoppedEarly: z.boolean().optional(),
      candidatesGenerated: z.number(),
      winnerScore: z.number(),
      samplingDuration: z.number().optional(),
    })
    .optional(),
  winnerScore: z.number().optional(),
  scoreBreakdown: z.record(z.number()).optional(),
  allCandidates: z
    .array(
      z.object({
        id: z.string(),
        content: z.string(),
        score: z.number(),
        scoreBreakdown: z.record(z.number()),
        rank: z.number().optional(),
      }),
    )
    .optional(),
  validationScore: z.number().optional(),
  validationGrade: z.string().optional(),
  validationReport: z.string().optional(),
  score: z.number().optional(),
});

// Define tool IO for type-safe session operations
const io = defineToolIO(generateK8sManifestsSchema, GenerateK8sManifestsResultSchema);

// Tool-specific state schema
const StateSchema = z.object({
  lastGeneratedAt: z.date().optional(),
  lastAppName: z.string().optional(),
  lastNamespace: z.string().optional(),
  totalManifestsGenerated: z.number().optional(),
  lastManifestCount: z.number().optional(),
  lastValidationScore: z.number().optional(),
  lastUsedAI: z.boolean().optional(),
});

/**
 * Kubernetes resource type definitions
 */
interface K8sResource {
  apiVersion: string;
  kind: string;
  metadata: {
    name: string;
    namespace?: string;
    labels?: Record<string, string>;
    annotations?: Record<string, string>;
  };
  spec?: Record<string, unknown>;
  data?: Record<string, string>;
}

/**
 * Parse K8s manifests from AI response
 */
function parseK8sManifestsFromAI(content: string): K8sResource[] {
  const manifests: K8sResource[] = [];
  try {
    // Try parsing as JSON array first
    const parsed = JSON.parse(content);
    if (Array.isArray(parsed)) {
      return parsed.filter(validateK8sResource);
    } else if (validateK8sResource(parsed)) {
      return [parsed];
    }
  } catch {
    // Try YAML-like parsing
    const documents = content.split(/^---$/m);
    for (const doc of documents) {
      if (!doc.trim()) continue;
      try {
        // Simple conversion from YAML-like to JSON
        const jsonStr = doc
          .replace(/^(\s*)(\w+):/gm, '$1"$2":')
          .replace(/:\s*(\w+)$/gm, ': "$1"')
          .replace(/:\s*(\d+)$/gm, ': $1');
        const obj = JSON.parse(`{${jsonStr}}`);
        if (validateK8sResource(obj)) {
          manifests.push(obj);
        }
      } catch {
        // Skip invalid documents
      }
    }
  }
  return manifests;
}

/**
 * Validate a K8s resource object
 */
function validateK8sResource(obj: unknown): obj is K8sResource {
  if (!obj || typeof obj !== 'object') return false;
  const resource = obj as Record<string, unknown>;
  return Boolean(
    typeof resource.apiVersion === 'string' &&
      typeof resource.kind === 'string' &&
      resource.metadata &&
      typeof resource.metadata === 'object' &&
      resource.metadata !== null &&
      typeof (resource.metadata as Record<string, unknown>).name === 'string',
  );
}

/**
 * Generate basic K8s manifests (fallback)
 */
function generateBasicManifests(
  params: GenerateK8sManifestsParams,
  image: string,
): Result<{ manifests: K8sResource[]; aiUsed: boolean }> {
  const {
    appName = 'app',
    namespace = 'default',
    replicas = 1,
    port = 8080,
    serviceType = 'ClusterIP',
    ingressEnabled = false,
    ingressHost,
    resources,
    envVars = [],
    configMapData,
    healthCheck,
    autoscaling,
  } = params;
  const labels = { app: appName };
  // Deployment
  const deployment: K8sResource = {
    apiVersion: 'apps/v1',
    kind: 'Deployment',
    metadata: {
      name: appName,
      namespace,
      labels,
    },
    spec: {
      replicas: autoscaling?.enabled ? undefined : replicas,
      selector: {
        matchLabels: labels,
      },
      template: {
        metadata: {
          labels,
        },
        spec: {
          containers: [
            {
              name: appName,
              image,
              imagePullPolicy: 'Never', // Always use local images, never pull from registry
              ports: [{ containerPort: port }],
              ...(envVars.length > 0 && { env: envVars }),
              ...(resources && { resources }),
              ...(healthCheck?.enabled && {
                livenessProbe: {
                  httpGet: {
                    path: healthCheck.path || '/health',
                    port: healthCheck.port || port,
                  },
                  initialDelaySeconds: healthCheck.initialDelaySeconds || 30,
                  periodSeconds: 10,
                },
                readinessProbe: {
                  httpGet: {
                    path: healthCheck.path || '/health',
                    port: healthCheck.port || port,
                  },
                  initialDelaySeconds: healthCheck.initialDelaySeconds || 5,
                  periodSeconds: 5,
                },
              }),
            },
          ],
        },
      },
    },
  };
  const manifests: K8sResource[] = [];
  manifests.push(deployment);
  // Service
  const service: K8sResource = {
    apiVersion: 'v1',
    kind: 'Service',
    metadata: {
      name: appName,
      namespace,
      labels,
    },
    spec: {
      type: serviceType,
      selector: labels,
      ports: [
        {
          port,
          targetPort: port,
          protocol: 'TCP',
        },
      ],
    },
  };
  manifests.push(service);
  // ConfigMap
  if (configMapData && Object.keys(configMapData).length > 0) {
    const configMap: K8sResource = {
      apiVersion: 'v1',
      kind: 'ConfigMap',
      metadata: {
        name: `${appName}-config`,
        namespace,
        labels,
      },
      data: configMapData,
    };
    manifests.push(configMap);
  }
  // Ingress
  if (ingressEnabled) {
    const ingress: K8sResource = {
      apiVersion: 'networking.k8s.io/v1',
      kind: 'Ingress',
      metadata: {
        name: appName,
        namespace,
        labels,
        annotations: {
          'nginx.ingress.kubernetes.io/rewrite-target': '/',
        },
      },
      spec: {
        rules: [
          {
            host: ingressHost || `${appName}.example.com`,
            http: {
              paths: [
                {
                  path: '/',
                  pathType: 'Prefix',
                  backend: {
                    service: {
                      name: appName,
                      port: {
                        number: port,
                      },
                    },
                  },
                },
              ],
            },
          },
        ],
      },
    };
    manifests.push(ingress);
  }
  // HPA
  if (autoscaling?.enabled) {
    const hpa: K8sResource = {
      apiVersion: 'autoscaling/v2',
      kind: 'HorizontalPodAutoscaler',
      metadata: {
        name: `${appName}-hpa`,
        namespace,
        labels,
      },
      spec: {
        scaleTargetRef: {
          apiVersion: 'apps/v1',
          kind: 'Deployment',
          name: appName,
        },
        minReplicas: autoscaling.minReplicas || 1,
        maxReplicas: autoscaling.maxReplicas || 10,
        metrics: [
          {
            type: 'Resource',
            resource: {
              name: 'cpu',
              target: {
                type: 'Utilization',
                averageUtilization: autoscaling.targetCPUUtilizationPercentage || 70,
              },
            },
          },
        ],
      },
    };
    manifests.push(hpa);
  }
  return Success({ manifests, aiUsed: false });
}

/**
 * Build prompt arguments for K8s manifest generation
 */
function buildK8sManifestPromptArgs(
  params: GenerateK8sManifestsParams,
  image: string,
): Record<string, unknown> {
  const defaults = {
    appName: 'app',
    namespace: K8S_DEFAULTS.namespace,
    replicas: K8S_DEFAULTS.replicas,
    port: K8S_DEFAULTS.port,
    serviceType: K8S_DEFAULTS.serviceType,
    ingressEnabled: false,
    healthCheckEnabled: false,
    autoscalingEnabled: false,
  };

  const merged = withDefaults(params, defaults as any);

  return {
    ...merged,
    image,
    imagePullPolicy: 'Never', // Always use local images, never pull from registry
    ingressHost: params.ingressHost,
    resources: params.resources,
    envVars: params.envVars,
    healthCheckEnabled: params.healthCheck?.enabled || false,
    autoscalingEnabled: params.autoscaling?.enabled || false,
  };
}

// computeHash function removed - was unused after tool wrapper elimination
/**
 * Generate K8s manifests implementation with selective progress reporting
 */
async function generateK8sManifestsImpl(
  params: GenerateK8sManifestsParams,
  context: ToolContext,
): Promise<Result<GenerateK8sManifestsResult>> {
  // Basic parameter validation
  if (!params || typeof params !== 'object') {
    return Failure('Invalid parameters provided');
  }
  // Progress reporting for complex manifest generation
  const progress = context.progress ? createStandardProgress(context.progress) : undefined;
  const logger = getToolLogger(context, 'generate-k8s-manifests');
  const timer = createToolTimer(logger, 'generate-k8s-manifests');

  try {
    const { appName = 'app', namespace = 'default' } = params;
    // Progress: Starting validation and analysis
    if (progress) await progress('VALIDATING');

    // Ensure session exists and get typed slice operations
    const sessionResult = await ensureSession(context, params.sessionId);
    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }

    const { id: sessionId } = sessionResult.value;
    const slice = useSessionSlice('generate-k8s-manifests', io, context, StateSchema);

    if (!slice) {
      return Failure('Session manager not available');
    }

    logger.info({ sessionId }, 'Starting K8s manifest generation with session');

    // Record input in session slice
    await slice.patch(sessionId, { input: params });

    // Get build result from session slice for image tag
    let image = params.imageId || `${appName}:latest`;
    try {
      const BuildImageResultSchema = z.object({
        success: z.boolean(),
        sessionId: z.string(),
        imageId: z.string(),
        tags: z.array(z.string()),
      });
      const buildImageIO = defineToolIO(buildImageSchema, BuildImageResultSchema);
      const buildSliceResult = await getSessionSlice(
        'build-image',
        sessionId,
        buildImageIO,
        context,
      );

      if (buildSliceResult.ok && buildSliceResult.value?.output) {
        const buildResult = buildSliceResult.value.output;
        if (!params.imageId && buildResult.tags?.[0]) {
          image = buildResult.tags[0];
          logger.debug({ image }, 'Using image tag from build-image session');
        }
      }
    } catch (error) {
      logger.debug({ error }, 'Could not get build result from session, using default image');
    }
    // Kubernetes manifest generation from repository analysis and configuration
    if (progress) await progress('EXECUTING');
    // Prepare sampling options
    const samplingOptions: SamplingOptions = {};
    // Sampling is enabled by default unless explicitly disabled
    samplingOptions.enableSampling = !params.disableSampling;
    if (params.maxCandidates !== undefined) samplingOptions.maxCandidates = params.maxCandidates;
    if (params.earlyStopThreshold !== undefined)
      samplingOptions.earlyStopThreshold = params.earlyStopThreshold;
    if (params.includeScoreBreakdown !== undefined)
      samplingOptions.includeScoreBreakdown = params.includeScoreBreakdown;
    if (params.returnAllCandidates !== undefined)
      samplingOptions.returnAllCandidates = params.returnAllCandidates;
    if (params.useCache !== undefined) samplingOptions.useCache = params.useCache;
    // Generate K8s manifests with AI or fallback
    let result: Result<{ manifests: K8sResource[]; aiUsed: boolean }>;
    let samplingMetadata: GenerateK8sManifestsResult['samplingMetadata'];
    let winnerScore: number | undefined;
    let scoreBreakdown: Record<string, number> | undefined;
    let allCandidates: GenerateK8sManifestsResult['allCandidates'];
    try {
      if (!params.disableSampling) {
        // Enhance prompt with knowledge context
        let promptArgs = buildK8sManifestPromptArgs(params, image);
        try {
          // Get analysis from session slice for language/framework context
          let analysisResult:
            | { language?: string; framework?: string; frameworkVersion?: string }
            | undefined;
          try {
            const AnalyzeRepoResultSchema = z.object({
              ok: z.boolean(),
              sessionId: z.string(),
              language: z.string(),
              framework: z.string().optional(),
              frameworkVersion: z.string().optional(),
            });
            const analyzeRepoIO = defineToolIO(analyzeRepoSchema, AnalyzeRepoResultSchema);
            const analysisSliceResult = await getSessionSlice(
              'analyze-repo',
              sessionId,
              analyzeRepoIO,
              context,
            );

            if (analysisSliceResult.ok && analysisSliceResult.value?.output) {
              const output = analysisSliceResult.value.output;
              analysisResult = {
                language: output.language,
                ...(output.framework && { framework: output.framework }),
                ...(output.frameworkVersion && { frameworkVersion: output.frameworkVersion }),
              };
            }
          } catch (error) {
            logger.debug({ error }, 'Could not get analysis result from session');
          }

          const knowledgeResult = await enhancePromptWithKnowledge(promptArgs, {
            operation: 'generate_k8s_manifests',
            ...(analysisResult?.language && { language: analysisResult.language }),
            ...(analysisResult?.framework && { framework: analysisResult.framework }),
            environment: params.environment ?? 'production',
            tags: ['kubernetes', 'deployment', 'manifests', analysisResult?.language].filter(
              Boolean,
            ) as string[],
          });

          if (knowledgeResult.bestPractices && knowledgeResult.bestPractices.length > 0) {
            promptArgs = knowledgeResult;
            logger.info(
              {
                practicesCount: knowledgeResult.bestPractices.length,
              },
              'Enhanced K8s generation with knowledge',
            );
          }
        } catch (error) {
          logger.debug({ error }, 'Knowledge enhancement failed, using base prompt');
        }

        // Use sampling-aware generation
        const aiResult = await aiGenerateWithSampling(logger, context, {
          promptName: 'generate-k8s-manifests',
          promptArgs,
          expectation: 'yaml' as const,
          maxRetries: 2,
          fallbackBehavior: 'default',
          ...samplingOptions,
        });
        if (aiResult.ok) {
          const cleaned = stripFencesAndNoise(aiResult.value.winner.content, 'yaml');
          if (isValidKubernetesContent(cleaned)) {
            const manifests = parseK8sManifestsFromAI(cleaned);
            if (manifests.length > 0) {
              result = Success({
                manifests,
                aiUsed: true,
              });
              // Capture sampling metadata
              samplingMetadata = aiResult.value.samplingMetadata;
              winnerScore = aiResult.value.winner.score;
              scoreBreakdown = aiResult.value.winner.scoreBreakdown;
              allCandidates = aiResult.value.allCandidates;
            } else {
              result = generateBasicManifests(params, image);
            }
          } else {
            result = generateBasicManifests(params, image);
          }
        } else {
          result = generateBasicManifests(params, image);
        }
      } else {
        // Standard generation without sampling
        result = generateBasicManifests(params, image);
      }
    } catch {
      // Fallback to basic generation
      result = generateBasicManifests(params, image);
    }
    if (!result.ok) {
      return Failure('Failed to generate K8s manifests');
    }
    // Progress: Finalizing results
    if (progress) await progress('FINALIZING');
    // Build resource list
    const resourceList: Array<{ kind: string; name: string; namespace: string }> = [];
    const manifests = result.value.manifests || [];
    for (const manifest of manifests) {
      if (manifest.kind && manifest.metadata?.name) {
        resourceList.push({
          kind: manifest.kind,
          name: manifest.metadata.name,
          namespace: manifest.metadata.namespace || namespace,
        });
      }
    }
    // Convert manifests to YAML string
    const yamlContent = manifests
      .map((m: K8sResource) => yaml.dump(m, { noRefs: true, lineWidth: -1 }))
      .join('---\n');
    // Run validation on the generated manifests
    const validator = createKubernetesValidator();
    const validationReport = validator.validate(yamlContent);
    logger.info(
      {
        score: validationReport.score,
        grade: validationReport.grade,
        errors: validationReport.errors,
        warnings: validationReport.warnings,
        info: validationReport.info,
      },
      'Kubernetes manifest validation complete',
    );
    // Write manifests to disk - use provided path as base
    const outputPath = joinPaths(params.path, 'k8s');
    await fs.mkdir(outputPath, { recursive: true });
    const manifestPath = joinPaths(outputPath, 'manifests.yaml');
    await fs.writeFile(manifestPath, yamlContent, 'utf-8');

    // Score the generated manifests
    let qualityScore: number | undefined;
    try {
      const scoring = await scoreConfigCandidates(
        [yamlContent],
        'yaml',
        params.environment ?? 'production',
        logger,
      );
      if (scoring.ok && scoring.value[0]) {
        qualityScore = scoring.value[0].score;
        logger.info({ qualityScore }, 'Scored generated K8s manifests');
      }
    } catch (error) {
      logger.debug({ error }, 'Could not score K8s manifests, continuing without score');
    }

    // Check for warnings
    const warnings: string[] = [];
    if (!params.resources) {
      warnings.push('No resource limits specified - consider adding for production');
    }
    if (!params.healthCheck?.enabled) {
      warnings.push('No health checks configured - consider adding for resilience');
    }
    if (params.serviceType === 'LoadBalancer' && !params.ingressEnabled) {
      warnings.push('LoadBalancer service without Ingress may incur cloud costs');
    }
    // Prepare the main result
    const k8sResult = {
      manifests: yamlContent,
      outputPath,
      resources: resourceList,
      warnings,
      sessionId,
      validationScore: validationReport.score,
      validationGrade: validationReport.grade,
      validationReport: getValidationSummary(validationReport),
      ...(qualityScore !== undefined && { score: qualityScore }),
    };

    // Update typed session slice with output and state
    await slice.patch(sessionId, {
      output: k8sResult,
      state: {
        lastGeneratedAt: new Date(),
        lastAppName: appName,
        lastNamespace: namespace,
        totalManifestsGenerated: 1, // Default generation iteration
        lastManifestCount: resourceList.length,
        lastValidationScore: validationReport.score,
        lastUsedAI: result.value.aiUsed || false,
      },
    });

    // Progress: Complete
    if (progress) await progress('COMPLETE');
    timer.end({ outputPath });

    // Return result with file indicator
    const finalResult: GenerateK8sManifestsResult & {
      _fileWritten?: boolean;
      _fileWrittenPath?: string;
    } = {
      ...k8sResult,
      _fileWritten: true,
      _fileWrittenPath: outputPath,
    };

    // Add sampling metadata if sampling was used
    if (!params.disableSampling) {
      if (samplingMetadata) {
        finalResult.samplingMetadata = samplingMetadata;
      }
      if (winnerScore !== undefined) {
        finalResult.winnerScore = winnerScore;
      }
      if (scoreBreakdown && params.includeScoreBreakdown) {
        finalResult.scoreBreakdown = scoreBreakdown;
      }
      if (allCandidates && params.returnAllCandidates) {
        finalResult.allCandidates = allCandidates;
      }
    }
    return Success(finalResult);
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'K8s manifest generation failed');
    return Failure(extractErrorMessage(error));
  }
}

/**
 * Generate K8s manifests tool with selective progress reporting
 */
export const generateK8sManifests = generateK8sManifestsImpl;
