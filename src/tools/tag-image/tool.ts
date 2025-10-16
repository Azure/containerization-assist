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
import { createDockerClient } from '@/infra/docker/client';
import { validateImageName } from '@/lib/validation';
import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { tool } from '@/types/tool';
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
 * Tag image handler
 */
async function handleTagImage(
  input: z.infer<typeof tagImageSchema>,
  ctx: ToolContext,
): Promise<Result<TagImageResult>> {
  const logger = getToolLogger(ctx, 'tag-image');
  const timer = createToolTimer(logger, 'tag-image');

  const { tag } = input;

  if (!tag) {
    return Failure('Tag parameter is required');
  }

  // Validate tag format
  const tagValidation = validateImageName(tag);
  if (!tagValidation.ok) {
    return tagValidation;
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
export default tool({
  name: 'tag-image',
  description: 'Tag Docker images with version and registry information',
  category: 'docker',
  version: '2.0.0',
  schema: tagImageSchema,
  metadata: {
    knowledgeEnhanced: false,
    enhancementCapabilities: [],
  },
  handler: handleTagImage,
});
