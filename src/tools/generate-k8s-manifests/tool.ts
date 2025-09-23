import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { promptTemplates, K8sManifestPromptParams } from '@/prompts/templates';
import { buildMessages, toMCPMessages } from '@/ai/prompt-engine';
import { generateK8sManifestsSchema, type GenerateK8sManifestsParams } from './schema';
import type { AIResponse } from '../ai-response-types';

export async function generateK8sManifests(
  params: GenerateK8sManifestsParams,
  context: ToolContext,
): Promise<Result<AIResponse>> {
  const validatedParams = generateK8sManifestsSchema.parse(params);
  const {
    appName,
    imageId,
    namespace = 'default',
    replicas = 3,
    port = 8080,
    serviceType = 'ClusterIP',
    ingressEnabled = false,
    resources,
    healthCheck,
  } = validatedParams;

  // Generate prompt from template
  const promptParams = {
    appName,
    image: imageId,
    replicas,
    port,
    namespace,
    serviceType,
    ingressEnabled,
    healthCheck: healthCheck?.enabled === true,
    resources: resources?.limits
      ? {
          cpu: resources.limits.cpu,
          memory: resources.limits.memory,
        }
      : undefined,
  } as K8sManifestPromptParams;
  const basePrompt = promptTemplates.k8sManifests(promptParams);

  // Build messages using the new prompt engine
  const messages = await buildMessages({
    basePrompt,
    topic: 'generate_k8s_manifests',
    tool: 'generate-k8s-manifests',
    environment: 'production',
    contract: {
      name: 'kubernetes_manifests_v1',
      description: 'Generate Kubernetes YAML manifests',
    },
    knowledgeBudget: 4000, // Larger budget for K8s manifests
  });

  // Execute via AI with structured messages
  const mcpMessages = toMCPMessages(messages);
  const response = await context.sampling.createMessage({
    ...mcpMessages,
    maxTokens: 8192,
    modelPreferences: {
      hints: [{ name: 'kubernetes-manifests' }],
    },
  });

  // Return result
  try {
    const responseText = response.content[0]?.text || '';
    return Success({ manifests: responseText });
  } catch (e) {
    const error = e as Error;
    return Failure(`AI response parsing failed: ${error.message}`);
  }
}

export const metadata = {
  name: 'generate-k8s-manifests',
  description: 'Generate Kubernetes deployment manifests',
  version: '2.1.0',
  aiDriven: true,
  knowledgeEnhanced: true,
};
