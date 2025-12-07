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
import type { ToolContext } from '@/core/context';
import { tool } from '@/types/tool';
import { tagImageSchema } from './schema';
import { z } from 'zod';
import { summarizeList } from '@/lib/summary-helpers';

export interface TagImageResult {
  /**
   * Natural language summary for user display.
   * 1-3 sentences describing the tagging result.
   * @example "✅ Tagged image. Applied 3 tags: v1.0.0, latest, stable. Ready to push."
   */
  summary?: string;
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
      message: 'Missing required parameter: tag',
      hint: 'Tag name must be specified for the image',
      resolution: 'Add tag parameter with the desired image tag (e.g., myapp:v1.0)',
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
        message: 'Missing required parameter: imageId',
        hint: 'Source image ID or name must be specified to tag',
        resolution: 'Add imageId parameter with the Docker image ID or name to tag',
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

    // Generate summary
    const tagsList = summarizeList(tags);
    const summary = `✅ Tagged image. Applied ${tags.length === 1 ? 'tag' : `${tags.length} tags`}: ${tagsList}. Ready to push.`;

    const result: TagImageResult = {
      summary,
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
