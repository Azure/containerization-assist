/**
 * Generate Azure Container Apps Manifests tool using the new Tool pattern
 */

import { Success, Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { Tool } from '@/types/tool';
import { promptTemplates, type AcaManifestParams } from '@/ai/prompt-templates';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { sampleWithRerank } from '@/mcp/ai/sampling-runner';
import { scoreACAManifest } from '@/lib/scoring';
import { generateAcaManifestsSchema } from './schema';

import type { AIResponse } from '../ai-response-types';
import type { z } from 'zod';

const name = 'generate-aca-manifests';
const description = 'Generate Azure Container Apps manifests';
const version = '2.1.0';

async function run(
  input: z.infer<typeof generateAcaManifestsSchema>,
  ctx: ToolContext,
): Promise<Result<AIResponse>> {
  const { cpu, memory, minReplicas, maxReplicas } = input;

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
    appName = ctx.session.get<string>('appName');
    if (appName) {
      ctx.logger.info({ appName }, 'Using app name from session (analyze-repo)');
    }
  }

  // Retrieve port from session if not explicitly provided in input
  let targetPort: number | undefined = input.targetPort;
  if (!targetPort || targetPort === 8080) {
    // If port is default or not provided, try to get from session
    if (input.sessionId && ctx.session) {
      const appPorts = ctx.session.get<number[]>('appPorts');
      if (appPorts && appPorts.length > 0) {
        targetPort = appPorts[0];
        ctx.logger.info({ port: targetPort }, 'Using port from session (analyze-repo)');
      }
    }
  }

  // Validate required parameters
  if (!imageId) {
    return Failure(
      'Container image is required. Either provide imageId parameter or run build-image first with a sessionId.',
    );
  }
  if (!appName) {
    return Failure(
      'Application name is required. Either provide appName parameter or run analyze-repo first with a sessionId.',
    );
  }

  try {
    // Generate prompt from template
    const promptParams = {
      appName,
      image: imageId,
      resources: {
        cpu: cpu?.toString() ?? '0.5',
        memory: memory ?? '1Gi',
      },
      scaling: {
        minReplicas: minReplicas ?? 0,
        maxReplicas: maxReplicas ?? 10,
      },
    };
    const basePrompt = promptTemplates.acaManifests(promptParams as AcaManifestParams);

    // Build messages using the new prompt engine
    const messages = await buildMessages({
      basePrompt,
      topic: TOPICS.GENERATE_ACA_MANIFESTS,
      tool: name,
      environment: input.environment || 'production',
      contract: {
        name: 'aca_manifests_v1',
        description: 'Generate Azure Container Apps manifests',
      },
      knowledgeBudget: 3500, // Character budget for knowledge snippets
    });

    // Execute via AI with deterministic sampling
    const samplingResult = await sampleWithRerank(
      ctx,
      async (attemptIndex) => ({
        ...toMCPMessages(messages),
        maxTokens: 8192,
        modelPreferences: {
          hints: [{ name: 'azure-container-apps' }],
          intelligencePriority: 0.85,
          speedPriority: attemptIndex > 0 ? 0.8 : 0.5,
          costPriority: 0.3,
        },
      }),
      scoreACAManifest,
      {},
    );

    if (!samplingResult.ok) {
      return Failure(`ACA manifest generation failed: ${samplingResult.error}`);
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
      'ACA manifest generated with sampling',
    );

    return Success({ manifests: responseText });
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : String(error);
    return Failure(`ACA manifest generation failed: ${errorMessage}`);
  }
}

const tool: Tool<typeof generateAcaManifestsSchema, AIResponse> = {
  name,
  description,
  category: 'azure',
  version,
  schema: generateAcaManifestsSchema,
  metadata: {
    aiDriven: true,
    knowledgeEnhanced: true,
    samplingStrategy: 'single',
    enhancementCapabilities: ['content-generation', 'manifest-generation', 'azure-optimization'],
  },
  run,
};

export default tool;
