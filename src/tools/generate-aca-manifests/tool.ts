import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { promptTemplates, type AcaManifestParams } from '@/prompts/templates';
import { applyPolicyConstraints } from '@/config/policy-prompt';
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
      cpu: cpu?.toString() || '0.5',
      memory: memory || '1Gi',
    },
    scaling: {
      minReplicas: minReplicas || 0,
      maxReplicas: maxReplicas || 10,
    },
  };
  const prompt = promptTemplates.acaManifests(promptParams as AcaManifestParams);

  // Apply policy constraints
  const constrained = applyPolicyConstraints(prompt, {
    tool: 'generate-aca-manifests',
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
      hints: [{ name: 'azure-container-apps' }],
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
  name: 'generate-aca-manifests',
  description: 'Generate Azure Container Apps manifests',
  version: '2.0.0',
  aiDriven: true,
};
