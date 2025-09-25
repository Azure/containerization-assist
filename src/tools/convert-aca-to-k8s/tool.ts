/**
 * Convert ACA to K8s tool using the new Tool pattern
 */

import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { Tool } from '@/types/tool';
import { promptTemplates } from '@/ai/prompt-templates';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { convertAcaToK8sSchema } from './schema';
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

  // Use the prompt template from @/ai/prompt-templates
  const basePrompt = promptTemplates.convertAcaToK8s(acaManifest);

  // Build messages using the prompt engine with knowledge injection
  const messages = await buildMessages({
    basePrompt,
    topic: 'convert_aca_to_k8s',
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
  const response = await ctx.sampling.createMessage({
    ...mcpMessages, // Spreads the messages array
    maxTokens: 8192,
    modelPreferences: {
      hints: [{ name: 'kubernetes-conversion' }],
    },
  });

  try {
    const responseText = response.content[0]?.text || '';
    return Success({ k8sManifests: responseText });
  } catch (e) {
    const error = e as Error;
    return Failure(`AI response parsing failed: ${error.message}`);
  }
}

const tool: Tool<typeof convertAcaToK8sSchema, AIResponse> = {
  name,
  description,
  category: 'azure',
  version,
  schema: convertAcaToK8sSchema,
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
