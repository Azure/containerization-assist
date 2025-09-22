import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { promptTemplates, type BaseImageResolutionParams } from '@/prompts/templates';
import { applyPolicyConstraints } from '@/config/policy-prompt';
import { resolveBaseImagesSchema, type ResolveBaseImagesParams } from './schema';
import type { AIResponse } from '../ai-response-types';

export async function resolveBaseImages(
  params: ResolveBaseImagesParams,
  context: ToolContext,
): Promise<Result<AIResponse>> {
  const validatedParams = resolveBaseImagesSchema.parse(params);
  const { technology } = validatedParams;

  // Generate prompt from template
  const promptParams = {
    language: technology || 'auto-detect',
    requirements: [],
  };
  const prompt = promptTemplates.baseImageResolution(promptParams as BaseImageResolutionParams);

  // Apply policy constraints
  const constrained = applyPolicyConstraints(prompt, {
    tool: 'resolve-base-images',
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
    maxTokens: 4096,
    modelPreferences: {
      hints: [{ name: 'docker-base-images' }],
    },
  });

  // Return result
  try {
    const responseText = response.content[0]?.text || '';
    return Success({ recommendations: responseText });
  } catch (e) {
    const error = e as Error;
    return Failure(`AI response parsing failed: ${error.message}`);
  }
}

export const metadata = {
  name: 'resolve-base-images',
  description: 'Recommend optimal Docker base images',
  version: '2.0.0',
  aiDriven: true,
};
