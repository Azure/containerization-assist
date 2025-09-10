/**
 * Push Image Tool - Standardized Implementation
 *
 * Pushes Docker images to container registries
 * Uses standardized helpers for consistency
 */

import { getSession, updateSession } from '@mcp/tools/session-helpers';
import type { ToolContext } from '../../mcp/context/types';
import { createDockerClient } from '../../lib/docker';
import { createTimer, createLogger } from '../../lib/logger';
import type { SessionData } from '../session-types';
import { Success, Failure, type Result } from '../../domain/types';
import type { PushImageParams } from './schema';

export interface PushImageResult {
  success: boolean;
  sessionId: string;
  registry: string;
  digest: string;
  pushedTags: string[];
}

/**
 * Push image implementation - direct execution without wrapper
 */
async function pushImageImpl(
  params: PushImageParams,
  context: ToolContext,
): Promise<Result<PushImageResult>> {
  // Basic parameter validation (essential validation only)
  if (!params || typeof params !== 'object') {
    return Failure('Invalid parameters provided');
  }
  const logger = context.logger || createLogger({ name: 'push-image' });
  const timer = createTimer(logger, 'push-image');

  try {
    const { registry = 'docker.io' } = params;

    // Resolve session (now always optional)
    const sessionResult = await getSession(params.sessionId, context);

    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }

    const { id: sessionId, state: session } = sessionResult.value;
    logger.info({ sessionId, registry }, 'Starting image push');

    const dockerClient = createDockerClient(logger);

    // Check for tagged images in session
    const sessionData = session as SessionData;
    const buildResult = sessionData?.build_result;
    const imageTag = params.imageId || buildResult?.tags?.[0];

    if (!imageTag) {
      return Failure(
        'No image specified. Provide imageId parameter or ensure session has tagged images from tag-image tool.',
      );
    }

    logger.info({ imageTag, registry }, 'Pushing image to registry');

    // Push image using lib docker client
    // Extract repository and tag from imageTag
    const parts = imageTag.split(':');
    const repository = parts[0];
    const tag = parts[1] || 'latest';

    if (!repository) {
      return Failure('Invalid image tag format');
    }

    const pushResult = await dockerClient.pushImage(repository, tag);

    if (!pushResult.ok) {
      return Failure(`Failed to push image: ${pushResult.error ?? 'Unknown error'}`);
    }

    const { digest } = pushResult.value;

    // Update session with push results using standardized helper
    const updateResult = await updateSession(
      sessionId,
      {
        completed_steps: [...(session.completed_steps || []), 'push'],
        metadata: {
          ...session.metadata,
          pushResult: {
            registry,
            digest,
            pushedTags: [imageTag],
            timestamp: new Date().toISOString(),
          },
        },
      },
      context,
    );

    if (!updateResult.ok) {
      logger.warn({ error: updateResult.error }, 'Failed to update session, but push succeeded');
    }

    timer.end({
      imageTag,
      registry,
      digest,
    });

    logger.info(
      {
        imageTag,
        registry,
        digest,
      },
      'Image push completed',
    );

    return Success({
      success: true,
      sessionId,
      registry,
      digest,
      pushedTags: [imageTag],
      _chainHint:
        'Next: generate_k8s_manifests for deployment or deploy_application if manifests exist',
    });
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'Image push failed');

    return Failure(error instanceof Error ? error.message : String(error));
  }
}

/**
 * Push image tool
 */
export const pushImage = pushImageImpl;
