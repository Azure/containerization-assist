/**
 * Generate K8s Manifests Tool - Standardized Implementation
 *
 * Generates Kubernetes manifests for application deployment
 * Uses standardized helpers for consistent behavior
 */

import { joinPaths } from '@lib/path-utils';
import { extractErrorMessage } from '../../lib/error-utils';
import { promises as fs } from 'node:fs';
import { getSession, updateSession } from '@mcp/tool-session-helpers';
import { aiGenerateWithSampling } from '@mcp/tool-ai-helpers';
import { enhancePromptWithKnowledge } from '@lib/ai-knowledge-enhancer';
import type { SamplingOptions } from '@lib/sampling';
import { createStandardProgress } from '@mcp/progress-helper';
import { createTimer } from '@lib/logger';
import type { ToolContext } from '../../mcp/context';
import type { SessionData } from '../session-types';
import { Success, Failure, type Result } from '../../types';
import { stripFencesAndNoise, isValidKubernetesContent } from '@lib/text-processing';
import { getSuccessChainHint, type SessionContext } from '../../lib/chain-hints';
import { TOOL_NAMES } from '../../exports/tools.js';
import { createKubernetesValidator, getValidationSummary } from '../../validation';
import { scoreConfigCandidates } from '@lib/integrated-scoring';
import * as yaml from 'js-yaml';
import type { GenerateK8sManifestsParams } from './schema';
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
  return {
    appName: params.appName || 'app',
    namespace: params.namespace || 'default',
    image,
    replicas: params.replicas || 1,
    port: params.port || 8080,
    serviceType: params.serviceType || 'ClusterIP',
    ingressEnabled: params.ingressEnabled || false,
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
  const logger = context.logger;
  const timer = createTimer(logger, 'generate-k8s-manifests');

  try {
    const { appName = 'app', namespace = 'default' } = params;
    // Progress: Starting validation and analysis
    if (progress) await progress('VALIDATING');
    // Resolve session with optional sessionId
    const sessionResult = await getSession(params.sessionId, context);
    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }
    const session = sessionResult.value;
    const sessionData = session.state as unknown as SessionData;
    // Get build result from session for image tag
    const buildResult = sessionData?.build_result || sessionData?.workflow_state?.build_result;
    const image = params.imageId || buildResult?.tags?.[0] || `${appName}:latest`;
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
          // Get analysis from session for language/framework context
          const analysisResult =
            sessionData?.analysis_result || sessionData?.workflow_state?.analysis_result;

          const knowledgeResult = await enhancePromptWithKnowledge(promptArgs, {
            operation: 'generate_k8s_manifests',
            ...(analysisResult?.language && { language: analysisResult.language }),
            ...(analysisResult?.framework && { framework: analysisResult.framework }),
            environment: params.environment || 'production',
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
    // Write manifests to disk
    const repoPath =
      sessionData?.metadata?.repo_path || sessionData?.workflow_state?.metadata?.repo_path || '.';
    const outputPath = joinPaths(repoPath, 'k8s');
    await fs.mkdir(outputPath, { recursive: true });
    const manifestPath = joinPaths(outputPath, 'manifests.yaml');
    await fs.writeFile(manifestPath, yamlContent, 'utf-8');

    // Score the generated manifests
    let qualityScore: number | undefined;
    try {
      const scoring = await scoreConfigCandidates(
        [yamlContent],
        'yaml',
        params.environment || 'production',
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
    // Update session with K8s result using standardized helper
    const updateResult = await updateSession(
      session.id,
      {
        k8s_result: {
          manifests: [
            {
              kind: 'Multiple',
              namespace,
              content: yamlContent,
              file_path: manifestPath,
            },
          ],
          replicas: params.replicas,
          resources: params.resources,
          output_path: outputPath,
        },
        completed_steps: [...(sessionData?.completed_steps || []), 'k8s'],
        metadata: {
          ...(sessionData?.metadata || {}),
          ai_enhancement_used: result.value.aiUsed || false,
          ai_generation_type: 'k8s-manifests',
          k8s_warnings: warnings,
        },
      },
      context,
    );
    if (!updateResult.ok) {
      logger.warn(
        { error: updateResult.error },
        'Failed to update session, but K8s generation succeeded',
      );
    }

    // Progress: Complete
    if (progress) await progress('COMPLETE');
    timer.end({ outputPath });

    // Prepare session context for dynamic chain hints
    const sessionContext: SessionContext = {
      completed_steps: (session.state as SessionContext).completed_steps || [],
      ...((session.state as SessionContext).analysis_result && {
        analysis_result: (session.state as SessionContext).analysis_result,
      }),
    };

    // Return result with file indicator and chain hint
    const finalResult: GenerateK8sManifestsResult & {
      _fileWritten?: boolean;
      _fileWrittenPath?: string;
      NextStep?: string;
    } = {
      manifests: yamlContent,
      outputPath,
      resources: resourceList,
      ...(warnings.length > 0 && { warnings }),
      sessionId: session.id,
      validationScore: validationReport.score,
      validationGrade: validationReport.grade,
      validationReport: getValidationSummary(validationReport),
      ...(qualityScore !== undefined && { score: qualityScore }),
      _fileWritten: true,
      _fileWrittenPath: outputPath,
      NextStep: getSuccessChainHint(TOOL_NAMES.GENERATE_K8S_MANIFESTS, sessionContext),
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
