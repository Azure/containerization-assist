/**
 * Generate Azure Container Apps Manifests Tool
 *
 * Generates Azure Container Apps deployment manifests
 * Following the same patterns as generate-k8s-manifests
 */

import { joinPaths } from '@lib/path-utils';
import { extractErrorMessage } from '../../lib/error-utils';
import { promises as fs } from 'node:fs';
import { ensureSession, defineToolIO, useSessionSlice } from '@mcp/tool-session-helpers';
import { aiGenerateWithSampling } from '@mcp/tool-ai-helpers';
import { enhancePromptWithKnowledge } from '@lib/ai-knowledge-enhancer';
import type { SamplingOptions } from '@lib/sampling';
import { createStandardProgress } from '@mcp/progress-helper';
import { createTimer } from '@lib/logger';
import type { ToolContext } from '../../mcp/context';
import type { SessionData } from '../session-types';
import { Success, Failure, type Result } from '../../types';
import { stripFencesAndNoise } from '@lib/text-processing';
import { getSuccessProgression, type SessionContext } from '../../workflows/workflow-progression';
import { TOOL_NAMES } from '../../exports/tool-names.js';
import { generateAcaManifestsSchema, type GenerateAcaManifestsParams } from './schema';
import { z } from 'zod';

// Define the result schema for type safety
const GenerateAcaManifestsResultSchema = z.object({
  manifest: z.string(),
  outputPath: z.string(),
  appName: z.string(),
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
});

// Define tool IO for type-safe session operations
const io = defineToolIO(generateAcaManifestsSchema, GenerateAcaManifestsResultSchema);

// Tool-specific state schema
const StateSchema = z.object({
  lastGeneratedAt: z.date().optional(),
  manifestCount: z.number().optional(),
  lastAppName: z.string().optional(),
  lastLocation: z.string().optional(),
  aiStrategy: z.enum(['ai', 'template', 'hybrid']).optional(),
});

/**
 * Result from ACA manifest generation
 */
export interface GenerateAcaManifestsResult {
  /** Generated manifest as JSON */
  manifest: string;
  /** Output directory path */
  outputPath: string;
  /** Application name */
  appName: string;
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
}

/**
 * Azure Container Apps resource type
 */
interface AcaResource {
  name: string;
  location?: string;
  properties: {
    managedEnvironmentId?: string;
    template: {
      containers: Array<{
        name: string;
        image: string;
        resources?: {
          cpu: number;
          memory: string;
        };
        env?: Array<{
          name: string;
          value?: string;
          secretRef?: string;
        }>;
        probes?: Array<{
          type: 'liveness' | 'readiness' | 'startup';
          httpGet?: {
            path: string;
            port: number;
          };
          initialDelaySeconds?: number;
          periodSeconds?: number;
        }>;
      }>;
      scale?: {
        minReplicas?: number;
        maxReplicas?: number;
        rules?: Array<{
          name: string;
          http?: {
            metadata: {
              concurrentRequests: string;
            };
          };
          custom?: {
            type: string;
            metadata: Record<string, string>;
          };
        }>;
      };
    };
    configuration?: {
      ingress?: {
        external: boolean;
        targetPort: number;
        transport?: 'http' | 'tcp';
        allowInsecure?: boolean;
        traffic?: Array<{
          latestRevision: boolean;
          weight: number;
        }>;
      };
      secrets?: Array<{
        name: string;
        value: string;
      }>;
      dapr?: {
        enabled: boolean;
        appId?: string;
        appPort?: number;
        appProtocol?: 'http' | 'grpc';
      };
    };
  };
}

/**
 * Parse ACA manifest from AI response
 */
function parseAcaManifestFromAI(content: string): AcaResource | null {
  try {
    // Try parsing as JSON
    const parsed = JSON.parse(content);
    if (validateAcaResource(parsed)) {
      return parsed;
    }
  } catch {
    // Try extracting JSON from response
    const jsonMatch = content.match(/\{[\s\S]*\}/);
    if (jsonMatch) {
      try {
        const parsed = JSON.parse(jsonMatch[0]);
        if (validateAcaResource(parsed)) {
          return parsed;
        }
      } catch {
        // Continue to fallback
      }
    }
  }
  return null;
}

/**
 * Validate an ACA resource object
 */
function validateAcaResource(obj: unknown): obj is AcaResource {
  if (!obj || typeof obj !== 'object') return false;
  const resource = obj as Record<string, unknown>;
  return Boolean(
    typeof resource.name === 'string' &&
      resource.properties &&
      typeof resource.properties === 'object' &&
      (resource.properties as Record<string, unknown>).template &&
      typeof (resource.properties as Record<string, unknown>).template === 'object',
  );
}

/**
 * Generate basic ACA manifest (fallback)
 */
function generateBasicAcaManifest(params: GenerateAcaManifestsParams): AcaResource {
  const {
    appName,
    imageId,
    cpu = 0.5,
    memory = '1Gi',
    minReplicas = 0,
    maxReplicas = 10,
    targetPort = 8080,
    external = true,
    envVars = [],
    location = 'eastus',
    environment = 'production',
  } = params;

  // Build environment variables
  const containerEnv = envVars.map((env) => ({
    name: env.name,
    ...(env.value ? { value: env.value } : {}),
    ...(env.secretRef ? { secretRef: env.secretRef } : {}),
  }));

  // Basic ACA manifest structure
  const manifest: AcaResource = {
    name: appName,
    location,
    properties: {
      template: {
        containers: [
          {
            name: appName,
            image: imageId,
            resources: {
              cpu,
              memory,
            },
            ...(containerEnv.length > 0 && { env: containerEnv }),
            ...(environment === 'production' && {
              probes: [
                {
                  type: 'liveness' as const,
                  httpGet: {
                    path: '/health',
                    port: targetPort,
                  },
                  initialDelaySeconds: 30,
                  periodSeconds: 30,
                },
                {
                  type: 'readiness' as const,
                  httpGet: {
                    path: '/ready',
                    port: targetPort,
                  },
                  initialDelaySeconds: 5,
                  periodSeconds: 10,
                },
              ],
            }),
          },
        ],
        scale: {
          minReplicas,
          maxReplicas,
          ...(minReplicas === 0 && {
            rules: [
              {
                name: 'http-rule',
                http: {
                  metadata: {
                    concurrentRequests: '10',
                  },
                },
              },
            ],
          }),
        },
      },
      ...(params.ingressEnabled && {
        configuration: {
          ingress: {
            external,
            targetPort,
            transport: 'http' as const,
            allowInsecure: false,
            traffic: [
              {
                latestRevision: true,
                weight: 100,
              },
            ],
          },
        },
      }),
    },
  };

  return manifest;
}

/**
 * Build prompt arguments for ACA manifest generation
 */
function buildAcaManifestPromptArgs(params: GenerateAcaManifestsParams): Record<string, unknown> {
  return {
    appName: params.appName,
    imageId: params.imageId,
    cpu: params.cpu || 0.5,
    memory: params.memory || '1Gi',
    minReplicas: params.minReplicas || 0,
    maxReplicas: params.maxReplicas || 10,
    targetPort: params.targetPort || 8080,
    external: params.external !== false,
    ingressEnabled: params.ingressEnabled || false,
    envVars: params.envVars,
    environment: params.environment || 'production',
    location: params.location || 'eastus',
    resourceGroup: params.resourceGroup,
    environmentName: params.environmentName,
  };
}

/**
 * Generate Azure Container Apps manifests implementation
 */
async function generateAcaManifestsImpl(
  params: GenerateAcaManifestsParams,
  context: ToolContext,
): Promise<Result<GenerateAcaManifestsResult>> {
  // Basic parameter validation
  if (!params || typeof params !== 'object') {
    return Failure('Invalid parameters provided');
  }

  // Progress reporting
  const progress = context.progress ? createStandardProgress(context.progress) : undefined;
  const logger = context.logger;
  const timer = createTimer(logger, 'generate-aca-manifests');

  try {
    const { appName } = params;

    // Progress: Starting validation
    if (progress) await progress('VALIDATING');

    // Ensure session exists and get typed slice operations
    const sessionResult = await ensureSession(context, params.sessionId);
    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }

    const { id: sessionId, state: session } = sessionResult.value;
    const slice = useSessionSlice('generate-aca-manifests', io, context, StateSchema);

    if (!slice) {
      return Failure('Session manager not available');
    }

    // Record input in session slice
    await slice.patch(sessionId, { input: params });

    const sessionData = session as unknown as SessionData;

    // Get build result from session for image tag if not provided
    const buildResult = sessionData?.build_result || sessionData?.workflow_state?.build_result;
    const imageId = params.imageId || buildResult?.tags?.[0] || `${appName}:latest`;

    // Progress: Executing generation
    if (progress) await progress('EXECUTING');

    // Prepare sampling options
    const samplingOptions: SamplingOptions = {};
    samplingOptions.enableSampling = !params.disableSampling;
    if (params.maxCandidates !== undefined) samplingOptions.maxCandidates = params.maxCandidates;
    if (params.earlyStopThreshold !== undefined)
      samplingOptions.earlyStopThreshold = params.earlyStopThreshold;
    if (params.includeScoreBreakdown !== undefined)
      samplingOptions.includeScoreBreakdown = params.includeScoreBreakdown;
    if (params.returnAllCandidates !== undefined)
      samplingOptions.returnAllCandidates = params.returnAllCandidates;
    if (params.useCache !== undefined) samplingOptions.useCache = params.useCache;

    // Generate ACA manifest with AI or fallback
    let manifest: AcaResource;
    let aiUsed = false;
    let samplingMetadata: GenerateAcaManifestsResult['samplingMetadata'];
    let winnerScore: number | undefined;
    let scoreBreakdown: Record<string, number> | undefined;
    let allCandidates: GenerateAcaManifestsResult['allCandidates'];

    try {
      if (!params.disableSampling) {
        // Enhance prompt with knowledge context
        let promptArgs = buildAcaManifestPromptArgs({ ...params, imageId });

        try {
          // Get analysis from session for language/framework context
          const analysisResult =
            sessionData?.analysis_result || sessionData?.workflow_state?.analysis_result;

          const knowledgeResult = await enhancePromptWithKnowledge(promptArgs, {
            operation: 'generate_aca_manifests',
            ...(analysisResult?.language && { language: analysisResult.language }),
            ...(analysisResult?.framework && { framework: analysisResult.framework }),
            environment: params.environment || 'production',
            tags: [
              'azure-container-apps',
              'deployment',
              'manifests',
              analysisResult?.language,
            ].filter((x): x is string => typeof x === 'string'),
          });

          if (knowledgeResult.bestPractices && knowledgeResult.bestPractices.length > 0) {
            promptArgs = knowledgeResult;
            logger.info(
              {
                practicesCount: knowledgeResult.bestPractices.length,
              },
              'Enhanced ACA generation with knowledge',
            );
          }
        } catch (error) {
          logger.debug({ error }, 'Knowledge enhancement failed, using base prompt');
        }

        // Use sampling-aware generation
        const aiResult = await aiGenerateWithSampling(logger, context, {
          promptName: 'generate-aca-manifests',
          promptArgs,
          expectation: 'json' as const,
          maxRetries: 2,
          fallbackBehavior: 'default',
          ...samplingOptions,
        });

        if (aiResult.ok) {
          const cleaned = stripFencesAndNoise(aiResult.value.winner.content, 'json');
          const parsed = parseAcaManifestFromAI(cleaned);
          if (parsed) {
            manifest = parsed;
            aiUsed = true;
            // Capture sampling metadata
            samplingMetadata = aiResult.value.samplingMetadata;
            winnerScore = aiResult.value.winner.score;
            scoreBreakdown = aiResult.value.winner.scoreBreakdown;
            allCandidates = aiResult.value.allCandidates;
          } else {
            manifest = generateBasicAcaManifest({ ...params, imageId });
          }
        } else {
          manifest = generateBasicAcaManifest({ ...params, imageId });
        }
      } else {
        // Standard generation without sampling
        manifest = generateBasicAcaManifest({ ...params, imageId });
      }
    } catch {
      // Fallback to basic generation
      manifest = generateBasicAcaManifest({ ...params, imageId });
    }

    // Progress: Finalizing results
    if (progress) await progress('FINALIZING');

    // Convert manifest to JSON string
    const manifestContent = JSON.stringify(manifest, null, 2);

    // Write manifest to disk
    const repoPath =
      sessionData?.metadata?.repo_path || sessionData?.workflow_state?.metadata?.repo_path || '.';
    const outputPath = joinPaths(repoPath, 'aca');
    await fs.mkdir(outputPath, { recursive: true });
    const manifestPath = joinPaths(outputPath, 'app.json');
    await fs.writeFile(manifestPath, manifestContent, 'utf-8');

    // Check for warnings
    const warnings: string[] = [];
    if (params.minReplicas > 0 && params.environment !== 'production') {
      warnings.push('Consider setting minReplicas to 0 for cost optimization in non-production');
    }
    if (!params.envVars || params.envVars.length === 0) {
      warnings.push('No environment variables configured - consider adding configuration');
    }
    if (params.cpu > 1 && params.environment === 'development') {
      warnings.push('High CPU allocation for development environment - consider reducing');
    }

    // Prepare result
    const result: GenerateAcaManifestsResult = {
      manifest: manifestContent,
      outputPath,
      appName,
      ...(warnings.length > 0 && { warnings }),
      sessionId,
    };

    // Add sampling metadata if sampling was used
    if (!params.disableSampling) {
      if (samplingMetadata) result.samplingMetadata = samplingMetadata;
      if (winnerScore !== undefined) result.winnerScore = winnerScore;
      if (scoreBreakdown && params.includeScoreBreakdown) result.scoreBreakdown = scoreBreakdown;
      if (allCandidates && params.returnAllCandidates) result.allCandidates = allCandidates;
    }

    // Update typed session slice with output and state
    await slice.patch(sessionId, {
      output: result,
      state: {
        lastGeneratedAt: new Date(),
        manifestCount: 1,
        lastAppName: appName,
        lastLocation: params.location || 'eastus',
        aiStrategy: aiUsed ? 'ai' : 'template',
      },
    });

    // Update session metadata for backward compatibility
    const sessionManager = context.sessionManager;
    if (sessionManager) {
      try {
        await sessionManager.update(sessionId, {
          metadata: {
            ...session.metadata,
            aca_result: {
              manifest: manifestContent,
              file_path: manifestPath,
              app_name: appName,
              output_path: outputPath,
            },
            ai_enhancement_used: aiUsed,
            ai_generation_type: 'aca-manifests',
            aca_warnings: warnings,
          },
          completed_steps: [...(session.completed_steps ?? []), 'generate-aca-manifests'],
        });
      } catch (error) {
        logger.warn(
          { error: extractErrorMessage(error) },
          'Failed to update session metadata, but ACA generation succeeded',
        );
      }
    }

    // Progress: Complete
    if (progress) await progress('COMPLETE');
    timer.end({ outputPath });

    // Prepare session context for dynamic chain hints
    const sessionContext: SessionContext = {
      completed_steps: session.completed_steps || [],
      ...(session.analysis_result ? { analysis_result: session.analysis_result } : {}),
    };

    // Return result with file indicator and chain hint
    const enrichedResult = {
      ...result,
      _fileWritten: true,
      _fileWrittenPath: outputPath,
      NextStep: getSuccessProgression(TOOL_NAMES.GENERATE_ACA_MANIFESTS, sessionContext).summary,
    };

    return Success(enrichedResult);
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'ACA manifest generation failed');
    return Failure(extractErrorMessage(error));
  }
}

/**
 * Generate Azure Container Apps manifests tool
 */
export const generateAcaManifests = generateAcaManifestsImpl;
