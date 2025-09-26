/**
 * Generate Azure Container Apps Manifests tool using the new Tool pattern
 */

import { Success, Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { Tool } from '@/types/tool';
import { promptTemplates, type AcaManifestParams } from '@/ai/prompt-templates';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
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
  const { appName, imageId, cpu, memory, minReplicas, maxReplicas } = input;

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

  // Execute via AI with structured messages
  const mcpMessages = toMCPMessages(messages);
  const response = await ctx.sampling.createMessage({
    ...mcpMessages, // Spreads the messages array
    maxTokens: 8192,
    modelPreferences: {
      hints: [{ name: 'azure-container-apps' }],
    },
  });

  try {
    const responseText = response.content[0]?.text || '';
    return Success({ manifests: responseText });
  } catch (e) {
    const error = e as Error;
    return Failure(`AI response parsing failed: ${error.message}`);
  }
}

const tool: Tool<typeof generateAcaManifestsSchema, AIResponse> = {
  name,
  description,
  category: 'azure',
  version,
  schema: generateAcaManifestsSchema,
  run,
};

export default tool;

export const metadata = {
  name,
  description,
  version,
  aiDriven: true,
  knowledgeEnhanced: true,
};
