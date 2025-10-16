/**
 * Push Image Tool - Modernized Implementation
 *
 * Pushes Docker images to a registry with authentication support
 * Follows the new Tool interface pattern
 *
 * This is a deterministic operational tool with no AI calls.
 */

import { createDockerClient, type DockerClient } from '@/infra/docker/client';
import { getToolLogger } from '@/lib/tool-helpers';
import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { tool } from '@/types/tool';
import { pushImageSchema } from './schema';
import type { z } from 'zod';
import { createErrorGuidance } from '@/lib/error-utils';

export interface PushImageResult {
  success: true;
  registry: string;
  digest: string;
  pushedTag: string;
}

/**
 * Push image handler
 */
async function handlePushImage(
  input: z.infer<typeof pushImageSchema>,
  ctx: ToolContext,
): Promise<Result<PushImageResult>> {
  const logger = getToolLogger(ctx, 'push-image');
  const startTime = Date.now();

  try {
    // Validate required imageId
    if (!input.imageId) {
      return Failure(
        'Missing required parameter: imageId',
        createErrorGuidance(
          'Missing required parameter: imageId',
          'The imageId parameter is required to push an image',
          'Provide the imageId of the Docker image to push. Use `docker images` to list available images.',
        ),
      );
    }

    // Use docker from context if provided (for testing), otherwise create new client
    // Type guard for test context with docker property
    const dockerClient: DockerClient =
      (ctx && 'docker' in ctx && ((ctx as Record<string, unknown>).docker as DockerClient)) ||
      createDockerClient(logger);

    // Parse repository and tag from imageId
    let repository: string;
    let tag: string;

    const colonIndex = input.imageId.lastIndexOf(':');
    if (colonIndex === -1 || colonIndex < input.imageId.lastIndexOf('/')) {
      // No tag specified, use 'latest'
      repository = input.imageId;
      tag = 'latest';
    } else {
      repository = input.imageId.substring(0, colonIndex);
      tag = input.imageId.substring(colonIndex + 1);
    }

    // Apply registry prefix if provided
    if (input.registry) {
      const registryHost = input.registry.replace(/^https?:\/\//, '').replace(/\/$/, '');
      if (!repository.startsWith(registryHost)) {
        repository = `${registryHost}/${repository}`;
      }
    }

    // Build auth config if credentials are provided
    let authConfig: { username: string; password: string; serveraddress: string } | undefined;
    if (input.credentials && input.registry) {
      // Validate that both username and password are present
      if (!input.credentials.username || !input.credentials.password) {
        return Failure(
          'Missing registry credentials',
          createErrorGuidance(
            'Both username and password are required for registry authentication',
            'Registry credentials are incomplete',
            'Provide both username and password in the credentials parameter',
          ),
        );
      }

      logger.info({ registry: input.registry }, 'Preparing registry authentication');

      // Normalize registry URL for auth config
      const registryHost = input.registry.replace(/^https?:\/\//, '').replace(/\/$/, '');

      // Docker Hub requires canonical serveraddress to avoid auth failures
      let serverAddress: string;
      if (
        registryHost === 'docker.io' ||
        registryHost === 'index.docker.io' ||
        registryHost === 'registry-1.docker.io' ||
        registryHost === ''
      ) {
        serverAddress = 'https://index.docker.io/v1/';
      } else {
        serverAddress = registryHost;
      }

      authConfig = {
        username: input.credentials.username,
        password: input.credentials.password,
        serveraddress: serverAddress,
      };
    }

    // Tag image if registry was specified
    if (input.registry) {
      const tagResult = await dockerClient.tagImage(input.imageId, repository, tag);
      if (!tagResult.ok) {
        return Failure(
          `Failed to tag image: ${tagResult.error}`,
          tagResult.guidance ||
            createErrorGuidance(
              tagResult.error,
              'Unable to tag the Docker image',
              'Verify the image exists with `docker images` and the tag format is valid.',
            ),
        );
      }
    }

    // Push the image with auth config if provided
    const pushResult = await dockerClient.pushImage(repository, tag, authConfig);
    if (!pushResult.ok) {
      // Use the guidance from the Docker client if available
      return Failure(`Failed to push image: ${pushResult.error}`, pushResult.guidance);
    }

    const pushTime = Date.now() - startTime;
    const pushedTag = `${repository}:${tag}`;

    logger.info(
      { pushedTag, pushTime, digest: pushResult.value.digest },
      'Image pushed successfully',
    );

    // Return success response
    const result: PushImageResult = {
      success: true,
      registry: input.registry ?? 'docker.io',
      digest: pushResult.value.digest,
      pushedTag,
    };

    return Success(result);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'Unknown error occurred';
    return Failure(`Push image failed: ${message}`);
  }
}

/**
 * Push image tool conforming to Tool interface
 */
export default tool({
  name: 'push-image',
  description: 'Push a Docker image to a registry',
  category: 'docker',
  version: '2.0.0',
  schema: pushImageSchema,
  metadata: {
    knowledgeEnhanced: false,
    enhancementCapabilities: [],
  },
  handler: handlePushImage,
});
