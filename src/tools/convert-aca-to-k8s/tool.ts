import { Success, Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { convertAcaToK8sSchema, type ConvertAcaToK8sParams } from './schema';
import type { AIResponse } from '../ai-response-types';

export async function convertAcaToK8s(
  params: ConvertAcaToK8sParams,
  context: ToolContext,
): Promise<Result<AIResponse>> {
  const validatedParams = convertAcaToK8sSchema.parse(params);
  const { acaManifest } = validatedParams;

  // Generate prompt for conversion
  const basePrompt = `Convert the following Azure Container Apps manifest to Kubernetes manifests:

\`\`\`yaml
${acaManifest}
\`\`\`

Generate equivalent Kubernetes manifests including:
1. Deployment with proper resource limits and replica configuration
2. Service for internal communication
3. Ingress if external access is enabled
4. ConfigMaps/Secrets for environment variables

Maintain all configurations and ensure compatibility with standard Kubernetes clusters.`;

  // Build messages using the new prompt engine
  const messages = await buildMessages({
    basePrompt,
    topic: TOPICS.CONVERT_ACA_TO_K8S,
    tool: 'convert-aca-to-k8s',
    environment: 'production', // Default environment
    contract: {
      name: 'aca_to_k8s_v1',
      description: 'Convert Azure Container Apps manifests to Kubernetes',
    },
    knowledgeBudget: 3000, // Character budget for knowledge snippets
  });

  // Execute via AI with structured messages
  const mcpMessages = toMCPMessages(messages);
  const response = await context.sampling.createMessage({
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

export const metadata = {
  name: 'convert-aca-to-k8s',
  description: 'Convert Azure Container Apps manifests to Kubernetes',
  version: '2.1.0',
  aiDriven: true,
  knowledgeEnhanced: true,
};
