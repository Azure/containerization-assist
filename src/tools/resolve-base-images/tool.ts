import { Success, Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { Tool } from '@/types/tool';
import { promptTemplates, type BaseImageResolutionParams } from '@/ai/prompt-templates';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { resolveBaseImagesSchema } from './schema';
import type { AIResponse } from '../ai-response-types';
import type { z } from 'zod';

const name = 'resolve-base-images';
const description = 'Recommend optimal Docker base images';
const version = '2.1.0';

async function run(
  input: z.infer<typeof resolveBaseImagesSchema>,
  ctx: ToolContext,
): Promise<Result<AIResponse>> {
  const { technology } = input;

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
    topic: TOPICS.RESOLVE_BASE_IMAGES,
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
  const response = await ctx.sampling.createMessage({
    ...mcpMessages,
    maxTokens: 4096,
    modelPreferences: {
      hints: [{ name: 'docker-base-images' }],
    },
  });

  try {
    const responseText = response.content[0]?.text || '';
    return Success({ recommendations: responseText });
  } catch (e) {
    const error = e as Error;
    return Failure(`AI response parsing failed: ${error.message}`);
  }
}

const tool: Tool<typeof resolveBaseImagesSchema, AIResponse> = {
  name,
  description,
  category: 'docker',
  version,
  schema: resolveBaseImagesSchema,
  run,
};

export default tool;

export const metadata = {
  name,
  description,
  version,
  aiDriven: true,
  knowledgeEnhanced: true,
};
