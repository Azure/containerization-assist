/**
 * Push Image Tool - Modernized Implementation
 *
 * Pushes Docker images to a registry with retry logic
 * Follows the new Tool interface pattern
 */

import { createDockerClient, type DockerClient } from '@/infra/docker/client';
import { getToolLogger } from '@/lib/tool-helpers';
import { Success, Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { MCPTool } from '@/types/tool';
import { pushImageSchema } from './schema';
import type { z } from 'zod';
import { sampleWithRerank } from '@/mcp/ai/sampling-runner';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages, type MCPMessage } from '@/mcp/ai/message-converter';
import { extractErrorMessage, createErrorGuidance } from '@/lib/error-utils';

// Additional interface for AI push optimization insights
export interface PushOptimizationInsights {
  recommendations: string[];
  registryOptimizations: string[];
  securityConsiderations: string[];
  confidence: number;
}

export interface PushImageResult {
  success: true;
  registry: string;
  digest: string;
  pushedTag: string;
  pushOptimizationInsights?: PushOptimizationInsights;
}

/**
 * Score push optimization insights based on content quality and relevance
 */
function scorePushOptimization(text: string): number {
  let score = 0;

  // Basic content quality (30 points)
  if (text.length > 100) score += 10;
  if (text.includes('\n')) score += 10;
  if (!text.toLowerCase().includes('error')) score += 10;

  // Push optimization indicators (40 points)
  if (/registry|repository|optimization|performance/i.test(text)) score += 15;
  if (/security|authentication|vulnerability|scan/i.test(text)) score += 15;
  if (/layer|cache|compression|bandwidth/i.test(text)) score += 10;

  // Structure and actionability (30 points)
  if (/\d+\.|-|\*/.test(text)) score += 10; // Has list structure
  if (/optimize|improve|secure|enhance/i.test(text)) score += 10;
  if (text.split('\n').length >= 3) score += 10; // Multi-line content

  return Math.min(score, 100);
}

/**
 * Build push optimization prompt for AI enhancement
 */
async function buildPushOptimizationPrompt(
  pushedTag: string,
  registry: string,
  digest: string,
  pushTime: number,
): Promise<{ messages: MCPMessage[]; maxTokens: number }> {
  const basePrompt = `You are a Docker registry and image push optimization expert. Analyze push operations and provide specific optimization insights.

Focus on:
1. Registry optimization strategies
2. Push performance improvements
3. Security best practices for image distribution
4. Layer optimization and caching
5. Registry authentication and access patterns

Provide concrete, actionable recommendations.

Analyze this Docker image push operation and provide optimization insights:

**Pushed Tag:** ${pushedTag}
**Registry:** ${registry}
**Digest:** ${digest}
**Push Time:** ${pushTime}ms

Please provide:
1. **Registry Optimizations:** Ways to improve registry usage and performance
2. **Security Considerations:** Security best practices for image distribution
3. **Push Performance:** Techniques to optimize push speed and efficiency
4. **Best Practices:** Docker registry best practices not currently followed

Format your response as clear, actionable recommendations.`;

  const messages = await buildMessages({
    basePrompt,
    topic: TOPICS.DOCKER_OPTIMIZATION,
    tool: 'push-image',
    environment: 'docker',
  });

  return { messages: toMCPMessages(messages).messages, maxTokens: 2048 };
}

/**
 * Generate AI-powered push optimization insights
 */
async function generatePushOptimizationInsights(
  pushedTag: string,
  registry: string,
  digest: string,
  pushTime: number,
  ctx: ToolContext,
): Promise<Result<PushOptimizationInsights>> {
  try {
    const insightResult = await sampleWithRerank(
      ctx,
      async () => buildPushOptimizationPrompt(pushedTag, registry, digest, pushTime),
      scorePushOptimization,
      {},
    );

    if (!insightResult.ok) {
      return Failure(`Failed to generate push optimization insights: ${insightResult.error}`);
    }

    const text = insightResult.value.text;

    // Parse the AI response to extract structured insights
    const recommendations: string[] = [];
    const registryOptimizations: string[] = [];
    const securityConsiderations: string[] = [];

    const lines = text
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => line.length > 0);

    let currentSection = '';
    for (const line of lines) {
      if (
        line.includes('Registry Optimizations') ||
        line.includes('registry') ||
        line.includes('Registry')
      ) {
        currentSection = 'registry';
        continue;
      }
      if (
        line.includes('Security Considerations') ||
        line.includes('security') ||
        line.includes('Security')
      ) {
        currentSection = 'security';
        continue;
      }
      if (
        line.includes('Push Performance') ||
        line.includes('performance') ||
        line.includes('Performance')
      ) {
        currentSection = 'performance';
        continue;
      }
      if (line.includes('Best Practices') || line.includes('practices')) {
        currentSection = 'practices';
        continue;
      }

      if (line.startsWith('-') || line.startsWith('*') || line.match(/^\d+\./)) {
        const cleanLine = line.replace(/^[-*\d.]\s*/, '');
        if (cleanLine.length > 10) {
          if (currentSection === 'registry') {
            registryOptimizations.push(cleanLine);
          } else if (currentSection === 'security') {
            securityConsiderations.push(cleanLine);
          } else if (currentSection === 'performance') {
            recommendations.push(`Performance: ${cleanLine}`);
          } else {
            recommendations.push(cleanLine);
          }
        }
      }
    }

    // Add general recommendations if none found
    if (recommendations.length === 0) {
      recommendations.push('Consider using multi-stage builds to reduce image size');
      recommendations.push('Implement layer caching strategies for faster pushes');
    }

    if (securityConsiderations.length === 0) {
      securityConsiderations.push('Scan images for vulnerabilities before pushing');
      securityConsiderations.push('Use registry authentication and access controls');
      securityConsiderations.push('Consider image signing for production workloads');
    }

    return Success({
      recommendations,
      registryOptimizations,
      securityConsiderations,
      confidence: insightResult.value.score ?? 0,
    });
  } catch (error) {
    return Failure(`Failed to generate push optimization insights: ${extractErrorMessage(error)}`);
  }
}

/**
 * Push image implementation
 */
async function run(
  input: z.infer<typeof pushImageSchema>,
  ctx: ToolContext,
): Promise<Result<PushImageResult>> {
  const logger = getToolLogger(ctx, 'push-image');
  const startTime = Date.now();

  try {
    // Validate required imageId
    if (!input.imageId) {
      return Failure(
        'Missing required parameter: imageId',
        createErrorGuidance(
          'Missing required parameter: imageId',
          'The imageId parameter is required to push an image',
          'Provide the imageId of the Docker image to push. Use `docker images` to list available images.',
        ),
      );
    }

    // Use docker from context if provided (for testing), otherwise create new client
    // Type guard for test context with docker property
    const dockerClient: DockerClient =
      (ctx && 'docker' in ctx && ((ctx as Record<string, unknown>).docker as DockerClient)) ||
      createDockerClient(logger);

    // Parse repository and tag from imageId
    let repository: string;
    let tag: string;

    const colonIndex = input.imageId.lastIndexOf(':');
    if (colonIndex === -1 || colonIndex < input.imageId.lastIndexOf('/')) {
      // No tag specified, use 'latest'
      repository = input.imageId;
      tag = 'latest';
    } else {
      repository = input.imageId.substring(0, colonIndex);
      tag = input.imageId.substring(colonIndex + 1);
    }

    // Apply registry prefix if provided
    if (input.registry) {
      const registryHost = input.registry.replace(/^https?:\/\//, '').replace(/\/$/, '');
      if (!repository.startsWith(registryHost)) {
        repository = `${registryHost}/${repository}`;
      }
    }

    // Tag image if registry was specified
    if (input.registry) {
      const tagResult = await dockerClient.tagImage(input.imageId, repository, tag);
      if (!tagResult.ok) {
        return Failure(
          `Failed to tag image: ${tagResult.error}`,
          tagResult.guidance ||
            createErrorGuidance(
              tagResult.error,
              'Unable to tag the Docker image',
              'Verify the image exists with `docker images` and the tag format is valid.',
            ),
        );
      }
    }

    // Push the image
    const pushResult = await dockerClient.pushImage(repository, tag);
    if (!pushResult.ok) {
      // Use the guidance from the Docker client if available
      return Failure(`Failed to push image: ${pushResult.error}`, pushResult.guidance);
    }

    const pushTime = Date.now() - startTime;
    const pushedTag = `${repository}:${tag}`;

    // Generate AI-powered push optimization insights
    let pushOptimizationInsights: PushOptimizationInsights | undefined;
    try {
      const insightResult = await generatePushOptimizationInsights(
        pushedTag,
        input.registry ?? 'docker.io',
        pushResult.value.digest,
        pushTime,
        ctx,
      );

      if (insightResult.ok) {
        pushOptimizationInsights = insightResult.value;
        logger.info(
          {
            recommendations: pushOptimizationInsights.recommendations.length,
            confidence: pushOptimizationInsights.confidence,
          },
          'Generated AI push optimization insights',
        );
      } else {
        logger.warn(
          { error: insightResult.error },
          'Failed to generate push optimization insights',
        );
      }
    } catch (error) {
      logger.warn(
        { error: extractErrorMessage(error) },
        'Error generating push optimization insights',
      );
    }

    // Return success response
    const result: PushImageResult = {
      success: true,
      registry: input.registry ?? 'docker.io',
      digest: pushResult.value.digest,
      pushedTag,
      ...(pushOptimizationInsights && { pushOptimizationInsights }),
    };

    return Success(result);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'Unknown error occurred';
    return Failure(`Push image failed: ${message}`);
  }
}

/**
 * Push image tool conforming to Tool interface
 */
const tool: MCPTool<typeof pushImageSchema, PushImageResult> = {
  name: 'push-image',
  description: 'Push a Docker image to a registry',
  version: '2.0.0',
  schema: pushImageSchema,
  metadata: {
    knowledgeEnhanced: true,
    samplingStrategy: 'single',
    enhancementCapabilities: ['push-optimization', 'registry-insights', 'security-recommendations'],
  },
  run,
};

export default tool;
