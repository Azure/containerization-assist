/**
 * Push Image Tool - Modernized Implementation
 *
 * Pushes Docker images to a registry with retry logic
 * Follows the new Tool interface pattern
 */

import { createDockerClient } from '@/infra/docker/client';
import { getToolLogger } from '@/lib/tool-helpers';
import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { Tool } from '@/types/tool';
import { pushImageSchema } from './schema';
import type { z } from 'zod';

export interface PushImageResult {
  success: true;
  registry: string;
  digest: string;
  pushedTag: string;
  sessionId?: string;
  workflowHints?: {
    nextStep: string;
    message: string;
  };
}

/**
 * Push image implementation
 */
async function run(
  input: z.infer<typeof pushImageSchema>,
  ctx: ToolContext,
): Promise<Result<PushImageResult>> {
  const logger = getToolLogger(ctx, 'push-image');

  try {
    // Validate required imageId
    if (!input.imageId) {
      return Failure('Missing required parameter: imageId');
    }

    // Use docker from context if provided (for testing), otherwise create new client
    const dockerClient = (ctx as any).docker || createDockerClient(logger);

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

    // Tag image if registry was specified
    if (input.registry) {
      const tagResult = await dockerClient.tagImage(input.imageId, repository, tag);
      if (!tagResult.ok) {
        return Failure(`Failed to tag image: ${tagResult.error}`);
      }
    }

    // Push the image
    const pushResult = await dockerClient.pushImage(repository, tag);
    if (!pushResult.ok) {
      return Failure(`Failed to push image: ${pushResult.error}`);
    }

    // Return success response
    const result: PushImageResult = {
      success: true,
      registry: input.registry ?? 'docker.io',
      digest: pushResult.value.digest,
      pushedTag: `${repository}:${tag}`,
      ...(input.sessionId && { sessionId: input.sessionId }),
      workflowHints: {
        nextStep: 'generate-k8s-manifests',
        message: `Image pushed successfully. Use "generate-k8s-manifests" with sessionId ${input.sessionId || '<sessionId>'} to create Kubernetes deployment manifests.`,
      },
    };

    logger.info(
      { pushedTag: result.pushedTag, digest: result.digest },
      'Image pushed successfully',
    );

    return Success(result);
  } catch (error) {
    logger.error({ error }, 'Failed to push image');
    const message = error instanceof Error ? error.message : 'Unknown error occurred';
    return Failure(`Push image failed: ${message}`);
  }
}

/**
 * Push image tool conforming to Tool interface
 */
const tool: Tool<typeof pushImageSchema, PushImageResult> = {
  name: 'push-image',
  description: 'Push a Docker image to a registry',
  version: '2.0.0',
  schema: pushImageSchema,
  run,
};

export default tool;
