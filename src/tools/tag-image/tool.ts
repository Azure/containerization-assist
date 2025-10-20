/**
 * Tag Image Tool - Modernized Implementation
 *
 * Tags Docker images with version and registry information
 * Follows the new Tool interface pattern
 *
 * This is a deterministic operational tool with no AI calls.
 */

import { setupToolContext } from '@/lib/tool-context-helpers';
import { extractErrorMessage } from '@/lib/errors';
import { createDockerClient } from '@/infra/docker/client';
import { parseImageName } from '@/lib/validation-helpers';
import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { tool } from '@/types/tool';
import { tagImageSchema } from './schema';
import { z } from 'zod';

export interface TagImageResult {
  success: boolean;
  tags: string[];
  imageId: string;
}

/**
 * Tag image handler
 */
async function handleTagImage(
  input: z.infer<typeof tagImageSchema>,
  ctx: ToolContext,
): Promise<Result<TagImageResult>> {
  const { logger, timer } = setupToolContext(ctx, 'tag-image');

  const { tag } = input;

  if (!tag) {
    return Failure('Tag parameter is required', {
      message: 'Tag parameter is required',
      hint: 'The tag parameter is missing',
      resolution: 'Provide a valid tag in the format "repository:tag" or "registry/repository:tag"',
    });
  }

  // Parse and validate image name
  const parsedImage = parseImageName(tag);
  if (!parsedImage.ok) {
    return parsedImage;
  }

  try {
    const dockerClient = createDockerClient(logger);

    const source = input.imageId;

    if (!source) {
      return Failure('No image specified. Provide imageId parameter.', {
        message: 'No image specified. Provide imageId parameter.',
        hint: 'The imageId parameter is required to tag an image',
        resolution: 'Provide the imageId or existing image tag to apply a new tag to',
      });
    }

    // Extract repository and tag from parsed image
    const { repository, tag: tagName } = parsedImage.value;
    // For Docker tag operation, repository includes registry if present
    const fullRepository = parsedImage.value.registry
      ? `${parsedImage.value.registry}/${repository}`
      : repository;

    const tagResult = await dockerClient.tagImage(source, fullRepository, tagName);
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
    };

    timer.end({ tags });

    return Success(result);
  } catch (error) {
    timer.error(error);
    return Failure(extractErrorMessage(error), {
      message: extractErrorMessage(error),
      hint: 'An unexpected error occurred while tagging the image',
      resolution: 'Verify that Docker is running, the source image exists, and the tag format is valid',
    });
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
  },
  handler: handleTagImage,
});
