/**
 * Push Image Tool
 * Pushes Docker images to container registries
 */

import type { Logger } from 'pino';
import type { ToolContext } from '@mcp/context';
import type { DockerClient } from '@services/docker-client';
import { Success, Failure, type Result } from '@types';
import { pushImageSchema, type PushImageParams } from './schema';

export interface SinglePushResult {
  success: boolean;
  imageId: string;
  registry: string;
  digest: string;
  pushedTag: string;
  moduleRoot?: string;
}

export interface PushImageResult {
  images: SinglePushResult[];
  successCount: number;
  failureCount: number;
  warnings?: string[];
  sessionId: string;
}

export interface PushImageDeps {
  docker: DockerClient;
  logger: Logger;
}

/**
 * Retry helper for network operations.
 * Trade-off: Exponential backoff vs immediate retry - prevents registry rate limits
 */
async function withRetry<T>(
  operation: () => Promise<T>,
  maxAttempts = 3,
  logger?: Logger,
): Promise<T> {
  let lastError: Error | undefined;

  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    try {
      return await operation();
    } catch (error) {
      lastError = error instanceof Error ? error : new Error(String(error));

      if (attempt < maxAttempts) {
        const delay = Math.min(1000 * Math.pow(2, attempt - 1), 5000); // Exponential backoff
        logger?.debug(
          { attempt, maxAttempts, delay, error: lastError.message },
          'Retrying after failure',
        );
        await new Promise((resolve) => setTimeout(resolve, delay));
      }
    }
  }

  throw lastError || new Error('Operation failed after retries');
}

/**
 * Push a single Docker image
 */
async function pushSingleImage(
  imageId: string,
  registry: string | undefined,
  moduleRoot: string | undefined,
  deps: PushImageDeps,
): Promise<Result<SinglePushResult>> {
  const { docker, logger } = deps;

  try {
    // Parse repository and tag
    let repository: string;
    let tag: string;

    const colonIndex = imageId.lastIndexOf(':');
    if (colonIndex === -1 || colonIndex < imageId.lastIndexOf('/')) {
      repository = imageId;
      tag = 'latest';
    } else {
      repository = imageId.substring(0, colonIndex);
      tag = imageId.substring(colonIndex + 1);
    }

    // Apply registry prefix if provided
    if (registry) {
      const registryHost = registry.replace(/^https?:\/\//, '').replace(/\/$/, '');
      if (!repository.startsWith(registryHost)) {
        repository = `${registryHost}/${repository}`;
      }
    }

    logger.info(
      { sourceImage: imageId, repository, tag, registry, moduleRoot },
      'Pushing Docker image',
    );

    // Tag image for target registry if needed
    if (registry && !imageId.startsWith(registry)) {
      const tagResult = await docker.tagImage(imageId, repository, tag);
      if (!tagResult.ok) {
        return Failure(`Failed to tag image for registry: ${tagResult.error}`);
      }
    }

    // Push the image with retry for network failures
    const pushResult = await withRetry(
      async () => {
        const result = await docker.pushImage(repository, tag);
        if (!result.ok) {
          throw new Error(result.error);
        }
        return result;
      },
      3,
      logger,
    );

    const result: SinglePushResult = {
      success: true,
      imageId,
      registry: registry || 'docker.io',
      digest: pushResult.value.digest,
      pushedTag: `${repository}:${tag}`,
      ...(moduleRoot && { moduleRoot }),
    };

    return Success(result);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return Failure(`Failed to push image ${imageId}: ${message}`);
  }
}

/**
 * Create push image tool with explicit dependencies - supports multi-module
 */
export function createPushImageTool(deps: PushImageDeps) {
  return async (
    params: PushImageParams,
    context: ToolContext,
  ): Promise<Result<PushImageResult>> => {
    const start = Date.now();
    const { logger } = deps;
    const sessionId = params.sessionId || `push-${Date.now()}`;

    try {
      // Validate parameters
      const validated = pushImageSchema.parse(params);
      const { imageId, registry } = validated;
      /* Auth integration point - see docs/security.md#docker-registry-auth */

      // Determine images to push
      let imagesToPush: Array<{ imageId: string; moduleRoot?: string }> = [];

      if (imageId) {
        // Single image specified
        imagesToPush = [{ imageId }];
      } else if (params.sessionId && context.sessionManager) {
        // Try to get multi-module results from session
        const sessionResult = await context.sessionManager.get(params.sessionId);
        if (sessionResult.ok && sessionResult.value?.state) {
          const state = sessionResult.value.state as Record<string, any>;

          // Check for tag results first (tagged images are preferred)
          const tagData = state['tag-image'];
          if (tagData?.images && Array.isArray(tagData.images)) {
            imagesToPush = tagData.images.map((img: any) => ({
              imageId: img.targetTag,
              moduleRoot: img.moduleRoot,
            }));
            logger.info(
              { count: imagesToPush.length },
              'Pushing all tagged images from multi-module build',
            );
          } else {
            // Fallback to build results
            const buildData = state['build-image'];
            if (buildData?.images && Array.isArray(buildData.images)) {
              imagesToPush = buildData.images.map((img: any) => ({
                imageId: `${img.image}:${img.tag}`,
                moduleRoot: img.moduleRoot,
              }));
              logger.info(
                { count: imagesToPush.length },
                'Pushing all built images from multi-module build',
              );
            } else {
              // Try legacy single image format
              const buildResult = sessionResult.value?.metadata?.buildResult as
                | { imageId?: string }
                | undefined;
              if (buildResult?.imageId) {
                imagesToPush = [{ imageId: buildResult.imageId }];
              }
            }
          }
        }
      }

      if (imagesToPush.length === 0) {
        return Failure(
          'No images to push. Provide imageId parameter or ensure session has built/tagged images.',
        );
      }

      // Push all images
      const pushedImages: SinglePushResult[] = [];
      const warnings: string[] = [];
      let successCount = 0;
      let failureCount = 0;

      for (const imageToPush of imagesToPush) {
        const pushResult = await pushSingleImage(
          imageToPush.imageId,
          registry,
          imageToPush.moduleRoot,
          deps,
        );

        if (pushResult.ok) {
          pushedImages.push(pushResult.value);
          successCount++;
        } else {
          const warning = `Failed to push ${imageToPush.imageId}: ${pushResult.error}`;
          warnings.push(warning);
          failureCount++;
          logger.warn({ imageId: imageToPush.imageId, error: pushResult.error }, warning);
        }
      }

      if (pushedImages.length === 0) {
        return Failure('No images could be pushed successfully');
      }

      // Update session if provided
      if (context.sessionManager) {
        await context.sessionManager.update(sessionId, {
          'push-image': {
            lastPushedAt: new Date().toISOString(),
            images: pushedImages.map((img) => ({
              imageId: img.imageId,
              pushedTag: img.pushedTag,
              digest: img.digest,
              moduleRoot: img.moduleRoot,
            })),
            registry: registry || 'docker.io',
            successCount,
            failureCount,
          },
        });
      }

      const result: PushImageResult = {
        images: pushedImages,
        successCount,
        failureCount,
        ...(warnings.length > 0 && { warnings }),
        sessionId,
      };

      const duration = Date.now() - start;
      logger.info(
        {
          successCount,
          failureCount,
          duration,
          tool: 'push-image',
        },
        'Multi-module push operation complete',
      );
      return Success(result);
    } catch (error) {
      const duration = Date.now() - start;
      const message = error instanceof Error ? error.message : String(error);
      logger.error({ error: message, duration, tool: 'push-image' }, 'Tool execution failed');
      return Failure(`Failed to push image: ${message}`);
    }
  };
}

/**
 * Standard tool export for MCP server integration
 */
export const tool = {
  type: 'standard' as const,
  name: 'push-image',
  description: 'Push Docker images to container registries with retry support',
  inputSchema: pushImageSchema,
  execute: createPushImageTool,
};
