/**
 * Generate Azure Container Apps Manifests tool using the new Tool pattern
 */

import { Success, Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { MCPTool } from '@/types/tool';
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

  const imageId = input.imageId;
  const appName = input.appName;

  if (!imageId) {
    return Failure('Container image is required. Provide imageId parameter.');
  }
  if (!appName) {
    return Failure('Application name is required. Provide appName parameter.');
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

const tool: MCPTool<typeof generateAcaManifestsSchema, AIResponse> = {
  name,
  description,
  category: 'azure',
  version,
  schema: generateAcaManifestsSchema,
  metadata: {
    knowledgeEnhanced: true,
    samplingStrategy: 'single',
    enhancementCapabilities: ['content-generation', 'manifest-generation', 'azure-optimization'],
  },
  run,
};

export default tool;
