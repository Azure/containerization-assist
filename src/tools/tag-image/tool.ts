/**
 * Tag Image Tool
 * Tags Docker images with version and registry information
 */

import type { Logger } from 'pino';
import type { ToolContext } from '@mcp/context';
import type { DockerClient } from '@services/docker-client';
import { Success, Failure, type Result } from '@types';
import { tagImageSchema, type TagImageParams } from './schema';

export interface SingleTagResult {
  success: boolean;
  imageId: string;
  sourceTag: string;
  targetTag: string;
  moduleRoot?: string;
}

export interface TagImageResult {
  images: SingleTagResult[];
  successCount: number;
  failureCount: number;
  warnings?: string[];
  sessionId: string;
}

export interface TagImageDeps {
  docker: DockerClient;
  logger: Logger;
}

/**
 * Tag a single Docker image
 */
async function tagSingleImage(
  imageId: string,
  tag: string,
  moduleRoot: string | undefined,
  deps: TagImageDeps,
): Promise<Result<SingleTagResult>> {
  const { docker, logger } = deps;

  try {
    // Parse repository and tag
    const parts = tag.split(':');
    const repository = parts[0];
    const tagName = parts[1] || 'latest';

    if (!repository) {
      return Failure('Invalid tag format. Use repository:tag');
    }

    logger.info({ sourceImage: imageId, repository, tagName, moduleRoot }, 'Tagging Docker image');

    // Perform Docker tag operation
    const tagResult = await docker.tagImage(imageId, repository, tagName);
    if (!tagResult.ok) {
      return Failure(`Failed to tag image: ${tagResult.error}`);
    }

    const result: SingleTagResult = {
      success: true,
      imageId,
      sourceTag: imageId.includes(':') ? imageId : `${imageId}:latest`,
      targetTag: tag,
      ...(moduleRoot && { moduleRoot }),
    };

    return Success(result);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return Failure(`Failed to tag image ${imageId}: ${message}`);
  }
}

/**
 * Create tag image tool with explicit dependencies - supports multi-module
 */
export function createTagImageTool(deps: TagImageDeps) {
  return async (params: TagImageParams, context: ToolContext): Promise<Result<TagImageResult>> => {
    const start = Date.now();
    const { logger } = deps;
    const sessionId = params.sessionId || `tag-${Date.now()}`;

    try {
      // Validate parameters
      const validated = tagImageSchema.parse(params);
      const { tag, imageId } = validated;

      if (!tag) {
        return Failure('Tag parameter is required');
      }

      // Determine images to tag
      let imagesToTag: Array<{ imageId: string; moduleRoot?: string }> = [];

      if (imageId) {
        // Single image specified
        imagesToTag = [{ imageId }];
      } else if (params.sessionId && context.sessionManager) {
        // Try to get multi-module build results from session
        const sessionResult = await context.sessionManager.get(params.sessionId);
        if (sessionResult.ok && sessionResult.value?.state) {
          const state = sessionResult.value.state as Record<string, any>;
          const buildData = state['build-image'];

          if (buildData?.images && Array.isArray(buildData.images)) {
            // Multi-module build: tag all images
            imagesToTag = buildData.images.map((img: any) => ({
              imageId: `${img.image}:${img.tag}`,
              moduleRoot: img.moduleRoot,
            }));
            logger.info(
              { count: imagesToTag.length },
              'Tagging all images from multi-module build',
            );
          } else {
            // Try legacy single image format
            const buildResult = sessionResult.value?.metadata?.buildResult as
              | { imageId?: string }
              | undefined;
            if (buildResult?.imageId) {
              imagesToTag = [{ imageId: buildResult.imageId }];
            }
          }
        }
      }

      if (imagesToTag.length === 0) {
        return Failure(
          'No images to tag. Provide imageId parameter or ensure session has built images.',
        );
      }

      // Tag all images
      const taggedImages: SingleTagResult[] = [];
      const warnings: string[] = [];
      let successCount = 0;
      let failureCount = 0;

      for (const imageToTag of imagesToTag) {
        // Generate tag with module-specific naming for multi-module
        let finalTag = tag;
        if (imagesToTag.length > 1 && imageToTag.moduleRoot) {
          // For multi-module, append module name to tag
          const moduleName = imageToTag.moduleRoot.split('/').pop() || 'module';
          const [repo, tagPart] = tag.split(':');
          finalTag = `${repo}/${moduleName}:${tagPart || 'latest'}`;
        }

        const tagResult = await tagSingleImage(
          imageToTag.imageId,
          finalTag,
          imageToTag.moduleRoot,
          deps,
        );

        if (tagResult.ok) {
          taggedImages.push(tagResult.value);
          successCount++;
        } else {
          const warning = `Failed to tag ${imageToTag.imageId}: ${tagResult.error}`;
          warnings.push(warning);
          failureCount++;
          logger.warn({ imageId: imageToTag.imageId, error: tagResult.error }, warning);
        }
      }

      if (taggedImages.length === 0) {
        return Failure('No images could be tagged successfully');
      }

      // Update session if provided
      if (context.sessionManager) {
        await context.sessionManager.update(sessionId, {
          'tag-image': {
            lastTaggedAt: new Date().toISOString(),
            images: taggedImages.map((img) => ({
              imageId: img.imageId,
              targetTag: img.targetTag,
              moduleRoot: img.moduleRoot,
            })),
            successCount,
            failureCount,
          },
        });
      }

      const result: TagImageResult = {
        images: taggedImages,
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
          tool: 'tag-image',
        },
        'Multi-module tag operation complete',
      );
      return Success(result);
    } catch (error) {
      const duration = Date.now() - start;
      const message = error instanceof Error ? error.message : String(error);
      logger.error({ error: message, duration, tool: 'tag-image' }, 'Tool execution failed');
      return Failure(`Failed to tag image: ${message}`);
    }
  };
}

/**
 * Standard tool export for MCP server integration
 */
export const tool = {
  type: 'standard' as const,
  name: 'tag-image',
  description: 'Tag Docker images with version and registry information',
  inputSchema: tagImageSchema,
  execute: createTagImageTool,
};
