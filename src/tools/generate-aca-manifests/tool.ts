import { Success, Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { promptTemplates, type AcaManifestParams } from '@/ai/prompt-templates';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { generateAcaManifestsSchema, type GenerateAcaManifestsParams } from './schema';
import type { AIResponse } from '../ai-response-types';

export async function generateAcaManifests(
  params: GenerateAcaManifestsParams,
  context: ToolContext,
): Promise<Result<AIResponse>> {
  const validatedParams = generateAcaManifestsSchema.parse(params);
  const { appName, imageId, cpu, memory, minReplicas, maxReplicas } = validatedParams;

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
    tool: 'generate-aca-manifests',
    environment: validatedParams.environment || 'production',
    contract: {
      name: 'aca_manifests_v1',
      description: 'Generate Azure Container Apps manifests',
    },
    knowledgeBudget: 3500, // Character budget for knowledge snippets
  });

  // Execute via AI with structured messages
  const mcpMessages = toMCPMessages(messages);
  const response = await context.sampling.createMessage({
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

export const metadata = {
  name: 'generate-aca-manifests',
  description: 'Generate Azure Container Apps manifests',
  version: '2.1.0',
  aiDriven: true,
  knowledgeEnhanced: true,
};
