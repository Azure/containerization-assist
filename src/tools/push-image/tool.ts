/**
 * Push Image Tool - Modernized Implementation
 *
 * Pushes Docker images to a registry with authentication support
 * Follows the new Tool interface pattern
 *
 * This is a deterministic operational tool with no AI calls.
 */

import { createDockerClient, type DockerClient } from '@/infra/docker/client';
import { getRegistryCredentials } from '@/infra/docker/credential-helpers';
import { getToolLogger } from '@/lib/tool-helpers';
import { parseImageName } from '@/lib/validation-helpers';
import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/core/context';
import { tool } from '@/types/tool';
import { pushImageSchema } from './schema';
import type { z } from 'zod';
import { createErrorGuidance } from '@/lib/errors';

export interface PushImageResult {
  /**
   * Natural language summary for user display.
   * 1-3 sentences describing the push result.
   * @example "✅ Pushed image to registry. Image: docker.io/myapp:v1.0.0. Digest: sha256:abc123..."
   */
  summary?: string;
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

    // Parse and validate image name
    const parsedImage = parseImageName(input.imageId);
    if (!parsedImage.ok) {
      return parsedImage;
    }

    // Use docker from context if provided (for testing), otherwise create new client
    // Type guard for test context with docker property
    const dockerClient: DockerClient =
      (ctx && 'docker' in ctx && ((ctx as Record<string, unknown>).docker as DockerClient)) ||
      createDockerClient(logger);

    // Determine the final repository and tag based on registry input
    let repository: string;
    const tag = parsedImage.value.tag;

    if (input.registry) {
      // Registry provided - clean it and determine if we need to prefix
      const registryHost = input.registry.replace(/^https?:\/\//, '').replace(/\/$/, '');

      // Check if the image already contains the registry (avoid double prefixing)
      // Reconstruct the full image path (without tag) to compare with target registry
      const fullImagePath = parsedImage.value.registry ?
        `${parsedImage.value.registry}/${parsedImage.value.repository}` :
        parsedImage.value.repository;

      // Check if the image already starts with the target registry
      if (fullImagePath === registryHost || fullImagePath.startsWith(`${registryHost}/`)) {
        // Image already contains the target registry - use the full path as-is
        repository = fullImagePath;
      } else if (parsedImage.value.repository.includes('/') && !parsedImage.value.repository.startsWith('docker.io/')) {
        // Image has namespace or registry prefix that doesn't match input registry
        // Only strip first part if it looks like a registry hostname (contains '.' or ':')
        const imageParts = parsedImage.value.repository.split('/');
        const firstPart = imageParts[0];

        if (firstPart && (firstPart.includes('.') || firstPart.includes(':'))) {
          // Looks like a registry hostname, replace it
          const pathAfterRegistry = imageParts.slice(1).join('/');
          repository = `${registryHost}/${pathAfterRegistry}`;
        } else {
          // Looks like a namespace/organization, keep the full path
          repository = `${registryHost}/${parsedImage.value.repository}`;
        }
      } else {
        // No registry in image or it's docker.io - prefix with input registry
        repository = `${registryHost}/${parsedImage.value.repository}`;
      }
    } else {
      // No registry provided - use image as-is (defaults to Docker Hub)
      repository = parsedImage.value.repository;
    }

    // Build auth config - try credential helpers first, then manual credentials
    let authConfig: { username: string; password: string; serveraddress: string } | undefined;

    // Try Docker credential helpers first (only if registry is provided)
    if (input.registry) {
      const credResult = await getRegistryCredentials(input.registry, logger);
      if (credResult.ok && credResult.value) {
        authConfig = credResult.value;
        logger.info({
          registry: input.registry,
          username: authConfig.username,
          serveraddress: authConfig.serveraddress,
          passwordProvided: !!authConfig.password,
        }, 'Using credentials from Docker credential helper');
      } else if (credResult.ok) {
        logger.debug({ registry: input.registry }, 'No credentials found in Docker credential helpers');
      } else {
        logger.debug({ registry: input.registry, error: credResult.error }, 'Credential helper lookup failed');
      }
    }

    // Fall back to manual credentials if provided and no credentials found via helpers
    if (!authConfig && input.credentials) {
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

      logger.info({ registry: input.registry }, 'Using provided credentials');

      // Simple serveraddress: use the registry host for most cases, special case Docker Hub
      let serverAddress: string;
      if (input.registry) {
        const registryHost = input.registry.replace(/^https?:\/\//, '').replace(/\/$/, '');
        if (
          registryHost === 'docker.io' ||
          registryHost === 'index.docker.io' ||
          registryHost === 'registry-1.docker.io'
        ) {
          serverAddress = 'https://index.docker.io/v1/';
        } else {
          serverAddress = registryHost;
        }
      } else {
        // Default to Docker Hub if no registry provided
        serverAddress = 'https://index.docker.io/v1/';
      }

      authConfig = {
        username: input.credentials.username,
        password: input.credentials.password,
        serveraddress: serverAddress,
      };
    }

    // Tag image with target registry
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

    // Push the image with auth config if provided
    logger.info({
      repository,
      tag,
      hasAuthConfig: !!authConfig,
      authServerAddress: authConfig?.serveraddress,
      authUsername: authConfig?.username,
    }, 'Pushing image to registry');

    const pushResult = await dockerClient.pushImage(repository, tag, authConfig);
    if (!pushResult.ok) {
      // Use the guidance from the Docker client if available
      return Failure(`Failed to push image: ${pushResult.error}`, pushResult.guidance);
    }

    const pushTime = Date.now() - startTime;
    // Build pushed tag for the response - use original image format if no registry, otherwise use the resolved repository
    const pushedTag = input.registry ? `${repository}:${tag}` : `${parsedImage.value.repository}:${tag}`;

    // Build display tag for summary based on the actual registry used
    let displayTag: string;
    if (input.registry) {
      // Custom registry provided - use the resolved repository format
      displayTag = `${repository}:${tag}`;
    } else {
      // No custom registry - always use docker.io prefix
      displayTag = `docker.io/${repository}:${tag}`;
    }

    logger.info(
      { pushedTag, pushTime, digest: pushResult.value.digest },
      'Image pushed successfully',
    );

    // Generate summary
    const digest = pushResult.value.digest;
    // Truncate digest to algorithm + 6 chars (e.g. "sha256:abcdef...")
    const colonIndex = digest.indexOf(':');
    const digestShort = colonIndex >= 0 && digest.length > colonIndex + 7
      ? `${digest.substring(0, colonIndex + 7)}...`
      : digest;
    const summary = `✅ Pushed image to registry. Image: ${displayTag}. Digest: ${digestShort}`;

    // Return success response
    const result: PushImageResult = {
      summary,
      success: true,
      registry: input.registry || 'docker.io',
      digest: pushResult.value.digest,
      pushedTag,
    };

    return Success(result);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'Unknown error occurred';
    return Failure(`Push image failed: ${message}`, {
      message: `Push image failed: ${message}`,
      hint: 'An unexpected error occurred while pushing the image to the registry',
      resolution: 'Check the error message for details. Common issues include network connectivity, registry authentication, or insufficient permissions',
    });
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
  },
  chainHints: {
    success: 'Image pushed successfully. Review AI optimization insights for push improvements.',
    failure:
      'Image push failed. Check registry credentials, network connectivity, and image tag format.',
  },
  handler: handlePushImage,
});
