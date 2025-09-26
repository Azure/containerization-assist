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
import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { Tool } from '@/types/tool';
import { tagImageSchema } from './schema';
import { z } from 'zod';

export interface TagImageResult {
  success: boolean;
  sessionId: string;
  tags: string[];
  imageId: string;
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
    const result: TagImageResult = {
      success: true,
      sessionId,
      tags,
      imageId: source,
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
  run,
};

export default tool;
