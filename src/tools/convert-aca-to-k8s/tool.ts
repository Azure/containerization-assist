import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { buildPolicyConstraints } from '@/config/policy-prompt';
import { enhancePrompt } from '../knowledge-helper';
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

  // Enhance with knowledge base
  const enhancedPrompt = await enhancePrompt(basePrompt, 'convert_aca_to_k8s', {
    environment: 'production',
  });

  // Apply policy constraints
  const constraints = buildPolicyConstraints({
    tool: 'convert-aca-to-k8s',
    environment: 'production',
  });
  const constrained =
    constraints.length > 0
      ? `${enhancedPrompt}\n\nPolicy Constraints:\n${constraints.join('\n')}`
      : enhancedPrompt;

  // Execute via AI
  const response = await context.sampling.createMessage({
    messages: [
      {
        role: 'user',
        content: [{ type: 'text', text: constrained }],
      },
    ],
    maxTokens: 8192,
    modelPreferences: {
      hints: [{ name: 'kubernetes-conversion' }],
    },
  });

  // Return result
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
