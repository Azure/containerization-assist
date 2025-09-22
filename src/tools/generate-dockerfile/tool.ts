import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { promptTemplates, type DockerfilePromptParams } from '@/prompts/templates';
import { applyPolicyConstraints } from '@/config/policy-prompt';
import { generateDockerfileSchema, type GenerateDockerfileParams } from './schema';
import type { AIResponse } from '../ai-response-types';

export async function generateDockerfile(
  params: GenerateDockerfileParams,
  context: ToolContext,
): Promise<Result<AIResponse>> {
  const validatedParams = generateDockerfileSchema.parse(params);
  const { baseImage, multistage, securityHardening, optimization } = validatedParams;

  // Generate prompt from template
  const promptParams = {
    language: 'auto-detect',
    dependencies: [],
    ports: [8080],
    optimization: optimization === 'size' || optimization === 'balanced',
    securityHardening,
    multistage,
    baseImage,
  } as DockerfilePromptParams;
  const prompt = promptTemplates.dockerfile(promptParams);

  // Apply policy constraints
  const constrained = applyPolicyConstraints(prompt, {
    tool: 'generate-dockerfile',
    environment: validatedParams.environment || 'production',
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
      hints: [{ name: 'dockerfile-generation' }],
    },
  });

  // Return result
  try {
    const responseText = response.content[0]?.text || '';
    return Success({ dockerfile: responseText });
  } catch (e) {
    const error = e as Error;
    return Failure(`AI response parsing failed: ${error.message}`);
  }
}

export const metadata = {
  name: 'generate-dockerfile',
  description: 'Generate optimized Dockerfiles for containerization',
  version: '2.0.0',
  aiDriven: true,
};
