import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { promptTemplates, type BaseImageResolutionParams } from '@/prompts/templates';
import { buildMessages, toMCPMessages } from '@/ai/prompt-engine';
import { resolveBaseImagesSchema, type ResolveBaseImagesParams } from './schema';
import type { AIResponse } from '../ai-response-types';

export async function resolveBaseImages(
  params: ResolveBaseImagesParams,
  context: ToolContext,
): Promise<Result<AIResponse>> {
  const validatedParams = resolveBaseImagesSchema.parse(params);
  const { technology } = validatedParams;

  // Generate prompt from template - provide context for better recommendations
  let contextPrefix = '';
  if (technology && technology !== 'auto-detect') {
    contextPrefix = `Technology stack: ${technology}\n`;
  } else {
    contextPrefix = `First, examine the repository to identify the technology stack, framework, and dependencies.\n`;
  }

  const promptParams = {
    language: technology || 'auto-detect',
    requirements: [
      contextPrefix,
      'Check Docker Hub and official repositories for the latest stable base images',
      'Consider both Alpine and Debian/Ubuntu variants',
      'Evaluate distroless options for production security',
      'Check for language-specific optimized images (e.g., node:slim, python:slim)',
    ],
  };
  const basePrompt = promptTemplates.baseImageResolution(promptParams as BaseImageResolutionParams);

  // Build messages using the new prompt engine
  const messages = await buildMessages({
    basePrompt,
    topic: 'resolve_base_images',
    tool: 'resolve-base-images',
    environment: 'production',
    contract: {
      name: 'base_images_v1',
      description: 'Recommend optimal Docker base images',
    },
    knowledgeBudget: 2000,
  });

  // Execute via AI with structured messages
  const mcpMessages = toMCPMessages(messages);
  const response = await context.sampling.createMessage({
    ...mcpMessages,
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
  version: '2.1.0',
  aiDriven: true,
  knowledgeEnhanced: true,
};
