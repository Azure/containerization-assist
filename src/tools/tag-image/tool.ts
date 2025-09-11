/**
 * Tag Image Tool - Standardized Implementation
 *
 * Tags Docker images with version and registry information
 * Uses standardized helpers for consistency
 */

import { getSession, updateSession } from '@mcp/tool-session-helpers';
import { extractErrorMessage } from '../../lib/error-utils';
import type { ToolContext } from '../../mcp/context';
import { createDockerClient } from '../../lib/docker';
import { createTimer, createLogger } from '../../lib/logger';
import { Success, Failure, type Result } from '../../types';
import type { TagImageParams } from './schema';

// Session data type for accessing build results
interface SessionData {
  build_result?: {
    imageId?: string;
    tags?: string[];
  };
}

export interface TagImageResult {
  success: boolean;
  sessionId: string;
  tags: string[];
  imageId: string;
}

/**
 * Tag image implementation - direct execution without wrapper
 */
async function tagImageImpl(
  params: TagImageParams,
  context: ToolContext,
): Promise<Result<TagImageResult>> {
  // Basic parameter validation (essential validation only)
  if (!params || typeof params !== 'object') {
    return Failure('Invalid parameters provided');
  }
  const logger = context.logger || createLogger({ name: 'tag-image' });
  const timer = createTimer(logger, 'tag-image');

  try {
    const { tag } = params;

    if (!tag) {
      return Failure('Tag parameter is required');
    }

    // Resolve session (now always optional)
    const sessionResult = await getSession(params.sessionId, context);

    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }

    const { id: sessionId, state: session } = sessionResult.value;
    logger.info({ sessionId, tag }, 'Starting image tagging');

    const dockerClient = createDockerClient(logger);

    // Check for built image in session or use provided imageId
    const sessionData = session as SessionData;
    const buildResult = sessionData?.build_result;
    const source = params.imageId || buildResult?.imageId;

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

    // Update session with tag information using standardized helper
    const updateResult = await updateSession(
      sessionId,
      {
        build_result: {
          ...(buildResult || {}),
          imageId: source,
          tags,
        },
        completed_steps: [...(session.completed_steps || []), 'tag'],
      },
      context,
    );

    if (!updateResult.ok) {
      logger.warn({ error: updateResult.error }, 'Failed to update session, but tagging succeeded');
    }

    timer.end({ source, tag });
    logger.info({ source, tag }, 'Image tagging completed');

    return Success({
      success: true,
      sessionId,
      tags,
      imageId: source,
      chainHint: 'Next: push_image to registry or generate_k8s_manifests for deployment',
    });
  } catch (error) {
    timer.end({ error });
    logger.error({ error }, 'Image tagging failed');

    return Failure(extractErrorMessage(error));
  }
}

/**
 * Tag image tool
 */
export const tagImage = tagImageImpl;
