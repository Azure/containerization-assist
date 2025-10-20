/**
 * Tag Image Tool - Modernized Implementation
 *
 * Tags Docker images with version and registry information
 * Follows the new Tool interface pattern
 *
 * This is a deterministic operational tool with no AI calls.
 */

import { setupToolContext } from '@/lib/tool-context-helpers';
import { extractErrorMessage } from '@/lib/error-utils';
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
    return Failure('Tag parameter is required');
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
      return Failure('No image specified. Provide imageId parameter.');
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
  },
  handler: handleTagImage,
});
