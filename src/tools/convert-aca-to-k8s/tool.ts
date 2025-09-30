/**
 * Convert ACA to K8s tool using the new Tool pattern
 */

import { Success, Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { Tool } from '@/types/tool';
import { promptTemplates } from '@/ai/prompt-templates';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { sampleWithRerank } from '@/mcp/ai/sampling-runner';
import { scoreACAConversion } from '@/lib/scoring';
import { convertAcaToK8sSchema } from './schema';
import { createStandardizedToolTracker } from '@/lib/tool-helpers';
import type { AIResponse } from '../ai-response-types';
import type { z } from 'zod';

const name = 'convert-aca-to-k8s';
const description = 'Convert Azure Container Apps manifests to Kubernetes';
const version = '2.1.0';

async function run(
  input: z.infer<typeof convertAcaToK8sSchema>,
  ctx: ToolContext,
): Promise<Result<AIResponse>> {
  const { acaManifest } = input;

  const tracker = createStandardizedToolTracker(
    'convert-aca-to-k8s',
    { manifestLength: acaManifest.length },
    ctx.logger,
  );

  try {
    // Use the prompt template from @/ai/prompt-templates
    const basePrompt = promptTemplates.convertAcaToK8s(acaManifest);

    // Build messages using the prompt engine with knowledge injection
    const messages = await buildMessages({
      basePrompt,
      topic: TOPICS.CONVERT_ACA_TO_K8S,
      tool: name,
      environment: 'production', // Default environment
      contract: {
        name: 'aca_to_k8s_v1',
        description: 'Convert Azure Container Apps manifests to Kubernetes',
      },
      knowledgeBudget: 3000, // Character budget for knowledge snippets
    });

    // Execute via AI with structured messages
    const mcpMessages = toMCPMessages(messages);
    const response = await sampleWithRerank(
      ctx,
      async (attempt) => ({
        ...mcpMessages,
        maxTokens: 8192,
        modelPreferences: {
          hints: [{ name: 'kubernetes-conversion' }],
          intelligencePriority: 0.85,
          speedPriority: attempt > 0 ? 0.6 : 0.3,
        },
      }),
      scoreACAConversion,
      {},
    );

    if (!response.ok) {
      tracker.fail(`AI sampling failed: ${response.error}`);
      return Failure(`AI sampling failed: ${response.error}`);
    }

    const responseText = response.value.text;
    tracker.complete({ score: response.value.score ?? 0 });
    return Success({ k8sManifests: responseText });
  } catch (error) {
    tracker.fail(error as Error);
    const errorMessage = error instanceof Error ? error.message : String(error);
    return Failure(`Conversion failed: ${errorMessage}`);
  }
}

const tool: Tool<typeof convertAcaToK8sSchema, AIResponse> = {
  name,
  description,
  category: 'azure',
  version,
  schema: convertAcaToK8sSchema,
  metadata: {
    aiDriven: true,
    knowledgeEnhanced: true,
    samplingStrategy: 'single',
    enhancementCapabilities: ['content-generation', 'manifest-conversion', 'platform-translation'],
  },
  run,
};

export default tool;
