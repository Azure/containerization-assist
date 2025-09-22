import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { applyPolicyConstraints } from '@/config/policy-prompt';
import { convertAcaToK8sSchema, type ConvertAcaToK8sParams } from './schema';
import type { AIResponse } from '../ai-response-types';

export async function convertAcaToK8s(
  params: ConvertAcaToK8sParams,
  context: ToolContext,
): Promise<Result<AIResponse>> {
  const validatedParams = convertAcaToK8sSchema.parse(params);
  const { acaManifest } = validatedParams;

  // Generate prompt for conversion
  const prompt = `Convert the following Azure Container Apps manifest to Kubernetes manifests:

\`\`\`yaml
${acaManifest}
\`\`\`

Generate equivalent Kubernetes manifests including:
1. Deployment with proper resource limits and replica configuration
2. Service for internal communication
3. Ingress if external access is enabled
4. ConfigMaps/Secrets for environment variables

Maintain all configurations and ensure compatibility with standard Kubernetes clusters.`;

  // Apply policy constraints
  const constrained = applyPolicyConstraints(prompt, {
    tool: 'convert-aca-to-k8s',
    environment: 'production',
  });

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
  version: '2.0.0',
  aiDriven: true,
};
