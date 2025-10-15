/**
 * Tag Image Tool - Modernized Implementation
 *
 * Tags Docker images with version and registry information
 * Follows the new Tool interface pattern
 *
 * This is a deterministic operational tool with no AI calls.
 */

import { getToolLogger, createToolTimer } from '@/lib/tool-helpers';
import { extractErrorMessage } from '@/lib/error-utils';
import { createDockerClient } from '@/lib/docker';
import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { MCPTool } from '@/types/tool';
import { tagImageSchema } from './schema';
import { z } from 'zod';

export interface TagImageResult {
  success: boolean;
  tags: string[];
  imageId: string;
  workflowHints?: {
    nextStep: string;
    message: string;
  };
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

  const { tag } = input;

  if (!tag) {
    return Failure('Tag parameter is required');
  }

  try {
    const dockerClient = createDockerClient(logger);

    const source = input.imageId;

    if (!source) {
      return Failure('No image specified. Provide imageId parameter.');
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
      return Failure(
        `Failed to tag image: ${tagResult.error ?? 'Unknown error'}`,
        tagResult.guidance,
      );
    }

    const tags = [tag];

    const result: TagImageResult = {
      success: true,
      tags,
      imageId: source,
      workflowHints: {
        nextStep: 'push-image',
        message: `Image tagged successfully as ${tag}. Use "push-image" to push the tagged image to a registry.`,
      },
    };

    timer.end({ tags });

    return Success(result);
  } catch (error) {
    timer.error(error);
    return Failure(extractErrorMessage(error));
  }
}

/**
 * Tag image tool conforming to Tool interface
 */
const tool: MCPTool<typeof tagImageSchema, TagImageResult> = {
  name: 'tag-image',
  description: 'Tag Docker images with version and registry information',
  version: '2.0.0',
  schema: tagImageSchema,
  metadata: {
    knowledgeEnhanced: false,
    samplingStrategy: 'none',
    enhancementCapabilities: [],
  },
  run,
};

export default tool;
