import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { promptTemplates, type DockerfilePromptParams } from '@/prompts/templates';
import { buildMessages, toMCPMessages } from '@/ai/prompt-engine';
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

  // Build messages using the new prompt engine
  const messages = await buildMessages({
    basePrompt,
    topic: 'generate_dockerfile',
    tool: 'generate-dockerfile',
    environment: validatedParams.environment || 'production',
    contract: {
      name: 'dockerfile_v1',
      description: 'Generate an optimized Dockerfile',
    },
    knowledgeBudget: 3000, // Character budget for knowledge snippets
  });

  // Execute via AI with structured messages
  const mcpMessages = toMCPMessages(messages);
  const response = await context.sampling.createMessage({
    ...mcpMessages, // Spreads the MCP-compatible messages
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
