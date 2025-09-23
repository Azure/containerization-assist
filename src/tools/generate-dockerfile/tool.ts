import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { promptTemplates, type DockerfilePromptParams } from '@/prompts/templates';
import { buildPolicyConstraints } from '@/config/policy-prompt';
import { enhancePrompt } from '../knowledge-helper';
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
  const basePrompt = promptTemplates.dockerfile(promptParams);

  // Enhance with knowledge base
  const enhancedPrompt = await enhancePrompt(basePrompt, 'generate_dockerfile', {
    environment: validatedParams.environment || 'production',
  });

  // Apply policy constraints
  const constraints = buildPolicyConstraints({
    tool: 'generate-dockerfile',
    environment: validatedParams.environment || 'production',
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
  version: '2.1.0',
  aiDriven: true,
  knowledgeEnhanced: true,
};
