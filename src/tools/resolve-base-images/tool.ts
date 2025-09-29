/**
 * Resolve Base Images Tool
 *
 * Provides AI-powered recommendations for optimal Docker base images based on
 * technology stack analysis. Uses intelligent sampling and scoring algorithms
 * to suggest secure, efficient, and well-maintained base images.
 *
 * @category docker
 * @version 2.1.0
 * @aiDriven true
 * @samplingStrategy rerank
 */

import { Success, Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { Tool } from '@/types/tool';
import { promptTemplates, type BaseImageResolutionParams } from '@/ai/prompt-templates';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { sampleWithRerank } from '@/mcp/ai/sampling-runner';
import { scoreBaseImageRecommendation } from '@/lib/sampling';
import { resolveBaseImagesSchema } from './schema';
import type { AIResponse } from '../ai-response-types';
import type { z } from 'zod';

const name = 'resolve-base-images';
const description = 'Recommend optimal Docker base images';
const version = '2.1.0';

/**
 * Generate AI-powered base image recommendations
 *
 * Analyzes the provided technology stack and uses AI sampling with
 * reranking to suggest optimal Docker base images. Considers security,
 * performance, and maintainability factors.
 *
 * @param input - Technology and context parameters
 * @param ctx - Tool execution context with AI sampling capabilities
 * @returns Result containing ranked base image recommendations
 */
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

  // Execute via AI with sampling and reranking
  const samplingResult = await sampleWithRerank(
    ctx,
    async (attemptIndex) => ({
      ...toMCPMessages(messages),
      maxTokens: 4096,
      modelPreferences: {
        hints: [{ name: 'docker-base-images' }],
        intelligencePriority: 0.9, // Higher priority for accurate recommendations
        speedPriority: attemptIndex > 0 ? 0.7 : 0.4,
        costPriority: 0.4,
      },
    }),
    scoreBaseImageRecommendation,
    { count: 3, stopAt: 85 },
  );

  if (!samplingResult.ok) {
    // Check if failure is due to empty AI responses - be graceful in this case
    if (samplingResult.error?.includes('No valid candidates generated')) {
      ctx.logger.debug('AI returned empty responses, returning empty recommendations');
      return Success({ recommendations: '' });
    }
    return Failure(`Base image recommendation failed: ${samplingResult.error}`);
  }

  const responseText = samplingResult.value.text;
  if (!responseText) {
    // Return empty recommendations rather than failing when AI response is empty
    ctx.logger.debug('AI returned empty response, returning empty recommendations');
    return Success({ recommendations: '' });
  }

  ctx.logger.info(
    {
      score: samplingResult.value.winner.score,
      scoreBreakdown: samplingResult.value.winner.scoreBreakdown,
    },
    'Base image recommendations generated with sampling',
  );

  return Success({ recommendations: responseText });
}

const tool: Tool<typeof resolveBaseImagesSchema, AIResponse> = {
  name,
  description,
  category: 'docker',
  version,
  schema: resolveBaseImagesSchema,
  metadata: {
    aiDriven: true,
    knowledgeEnhanced: true,
    samplingStrategy: 'rerank',
    enhancementCapabilities: ['content-generation', 'image-recommendation', 'security-analysis'],
  },
  run,
};

export default tool;
