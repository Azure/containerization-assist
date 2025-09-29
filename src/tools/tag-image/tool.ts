/**
 * Tag Image Tool - Modernized Implementation
 *
 * Tags Docker images with version and registry information
 * Follows the new Tool interface pattern
 */

import { ensureSession, updateSession } from '@/mcp/tool-session-helpers';
import { getToolLogger, createToolTimer } from '@/lib/tool-helpers';
import { extractErrorMessage } from '@/lib/error-utils';
import { createDockerClient } from '@/lib/docker';
import { Success, Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { Tool } from '@/types/tool';
import { tagImageSchema } from './schema';
import { z } from 'zod';
import { sampleWithRerank } from '@/mcp/ai/sampling-runner';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages, type MCPMessage } from '@/mcp/ai/message-converter';

// Additional interface for AI tagging suggestions
export interface TaggingStrategySuggestions {
  recommendations: string[];
  bestPractices: string[];
  alternatives: string[];
  confidence: number;
}

export interface TagImageResult {
  success: boolean;
  sessionId: string;
  tags: string[];
  imageId: string;
  taggingSuggestions?: TaggingStrategySuggestions;
  workflowHints?: {
    nextStep: string;
    message: string;
  };
}

/**
 * Score tagging strategy suggestions based on content quality and relevance
 */
function scoreTaggingStrategy(text: string): number {
  let score = 0;

  // Basic content quality (30 points)
  if (text.length > 100) score += 10;
  if (text.includes('\n')) score += 10;
  if (!text.toLowerCase().includes('error')) score += 10;

  // Tagging strategy indicators (40 points)
  if (/semver|semantic|version/i.test(text)) score += 15;
  if (/latest|stable|production|staging/i.test(text)) score += 10;
  if (/registry|repository|namespace/i.test(text)) score += 15;

  // Structure and actionability (30 points)
  if (/\d+\.|-|\*/.test(text)) score += 10; // Has list structure
  if (/tag|label|organize|strategy/i.test(text)) score += 10;
  if (text.split('\n').length >= 3) score += 10; // Multi-line content

  return Math.min(score, 100);
}

/**
 * Build tagging strategy prompt for AI enhancement
 */
async function buildTaggingPrompt(
  imageId: string,
  currentTag: string,
  sessionContext?: unknown,
): Promise<{ messages: MCPMessage[]; maxTokens: number }> {
  const basePrompt = `You are a Docker tagging expert. Analyze tagging strategies and provide specific recommendations.

Focus on:
1. Semantic versioning and tag naming conventions
2. Environment-specific tagging strategies
3. Registry organization and namespace management
4. Tag lifecycle and deprecation strategies
5. Security and compliance considerations

Provide concrete, actionable recommendations.

Analyze this Docker image tagging scenario and provide strategy recommendations:

**Image ID:** ${imageId}
**Current Tag:** ${currentTag}
${sessionContext ? `**Session Context:** ${JSON.stringify(sessionContext, null, 2)}` : ''}

Please provide:
1. **Tag Strategy:** Best practices for this tagging approach
2. **Alternatives:** Other tagging options to consider
3. **Best Practices:** Docker tagging conventions not currently followed
4. **Next Steps:** Recommended follow-up actions

Format your response as clear, actionable recommendations.`;

  const messages = await buildMessages({
    basePrompt,
    topic: TOPICS.DOCKER_TAGGING,
    tool: 'tag-image',
    environment: 'docker',
  });

  return { messages: toMCPMessages(messages).messages, maxTokens: 2048 };
}

/**
 * Generate AI-powered tagging strategy suggestions
 */
async function generateTaggingSuggestions(
  imageId: string,
  currentTag: string,
  ctx: ToolContext,
  sessionContext?: any,
): Promise<Result<TaggingStrategySuggestions>> {
  try {
    const suggestionResult = await sampleWithRerank(
      ctx,
      async () => buildTaggingPrompt(imageId, currentTag, sessionContext),
      scoreTaggingStrategy,
      { count: 2, stopAt: 85 },
    );

    if (!suggestionResult.ok) {
      return Failure(`Failed to generate tagging suggestions: ${suggestionResult.error}`);
    }

    const text = suggestionResult.value.text;

    // Parse the AI response to extract structured suggestions
    const recommendations: string[] = [];
    const bestPractices: string[] = [];
    const alternatives: string[] = [];

    const lines = text
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => line.length > 0);

    let currentSection = '';
    for (const line of lines) {
      if (line.includes('Tag Strategy') || line.includes('strategy')) {
        currentSection = 'strategy';
        continue;
      }
      if (line.includes('Alternatives') || line.includes('alternatives')) {
        currentSection = 'alternatives';
        continue;
      }
      if (line.includes('Best Practices') || line.includes('practices')) {
        currentSection = 'practices';
        continue;
      }

      if (line.startsWith('-') || line.startsWith('*') || line.match(/^\d+\./)) {
        const cleanLine = line.replace(/^[-*\d.]\s*/, '');
        if (cleanLine.length > 10) {
          if (currentSection === 'strategy') {
            recommendations.push(cleanLine);
          } else if (currentSection === 'alternatives') {
            alternatives.push(cleanLine);
          } else if (currentSection === 'practices') {
            bestPractices.push(cleanLine);
          } else {
            recommendations.push(cleanLine);
          }
        }
      }
    }

    // Add general best practices if none found
    if (bestPractices.length === 0) {
      bestPractices.push('Use semantic versioning for release tags');
      bestPractices.push('Avoid using :latest in production environments');
      bestPractices.push('Include environment indicators in tags (dev, staging, prod)');
    }

    return Success({
      recommendations,
      bestPractices,
      alternatives,
      confidence: suggestionResult.value.winner.score,
    });
  } catch (error) {
    return Failure(`Failed to generate tagging suggestions: ${extractErrorMessage(error)}`);
  }
}

/**
 * Tag image implementation
 */
async function run(
  input: z.infer<typeof tagImageSchema>,
  ctx: ToolContext,
): Promise<Result<TagImageResult>> {
  const logger = getToolLogger(ctx, 'tag-image');
  const timer = createToolTimer(logger, 'tag-image');

  try {
    const { tag } = input;

    if (!tag) {
      return Failure('Tag parameter is required');
    }

    // Ensure session exists and get typed slice operations
    const sessionResult = await ensureSession(ctx, input.sessionId);
    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }

    const { id: sessionId, state: session } = sessionResult.value;

    logger.info({ sessionId, tag }, 'Starting image tagging');

    const dockerClient = createDockerClient(logger);

    // Check for built image in session results or use provided imageId
    const buildResult = session.results?.['build-image'] as
      | { imageId?: string; tags?: string[] }
      | undefined;
    const source = input.imageId || buildResult?.imageId;

    if (!source) {
      return Failure(
        'No image specified. Provide imageId parameter or ensure session has built image from build-image tool.',
      );
    }

    // Tag image using lib docker client
    // Parse repository and tag from the tag parameter
    const parts = tag.split(':');
    const repository = parts[0];
    const tagName = parts[1] || 'latest';

    if (!repository) {
      return Failure('Invalid tag format');
    }

    const tagResult = await dockerClient.tagImage(source, repository, tagName);
    if (!tagResult.ok) {
      return Failure(`Failed to tag image: ${tagResult.error ?? 'Unknown error'}`);
    }

    const tags = [tag];

    // Generate AI-powered tagging suggestions
    let taggingSuggestions: TaggingStrategySuggestions | undefined;
    try {
      const suggestionResult = await generateTaggingSuggestions(source, tag, ctx, session.metadata);

      if (suggestionResult.ok) {
        taggingSuggestions = suggestionResult.value;
        logger.info(
          {
            recommendations: taggingSuggestions.recommendations.length,
            confidence: taggingSuggestions.confidence,
          },
          'Generated AI tagging suggestions',
        );
      } else {
        logger.warn({ error: suggestionResult.error }, 'Failed to generate tagging suggestions');
      }
    } catch (error) {
      logger.warn({ error: extractErrorMessage(error) }, 'Error generating tagging suggestions');
    }

    const result: TagImageResult = {
      success: true,
      sessionId,
      tags,
      imageId: source,
      ...(taggingSuggestions && { taggingSuggestions }),
      workflowHints: {
        nextStep: 'push-image',
        message: `Image tagged successfully as ${tag}. Use "push-image" with sessionId ${sessionId} to push the tagged image to a registry.${taggingSuggestions ? ' Review AI tagging suggestions for strategy improvements.' : ''}`,
      },
    };

    // Store tag result in session
    const currentSteps = sessionResult.ok ? sessionResult.value.state.completed_steps || [] : [];
    await updateSession(
      sessionId,
      {
        results: {
          'tag-image': result,
        },
        completed_steps: [...currentSteps, 'tag-image'],
        current_step: 'tag-image',
      },
      ctx,
    );

    timer.end({ tags, sessionId });
    logger.info({ sessionId, tags }, 'Image tagging completed');

    return Success(result);
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'Image tagging failed');
    return Failure(extractErrorMessage(error));
  }
}

/**
 * Tag image tool conforming to Tool interface
 */
const tool: Tool<typeof tagImageSchema, TagImageResult> = {
  name: 'tag-image',
  description: 'Tag Docker images with version and registry information',
  version: '2.0.0',
  schema: tagImageSchema,
  metadata: {
    aiDriven: true,
    knowledgeEnhanced: true,
    samplingStrategy: 'rerank',
    enhancementCapabilities: ['tagging-strategy', 'best-practices', 'recommendations'],
  },
  run,
};

export default tool;
