/**
 * Push image tool implementation using DockerClient directly
 * Lightweight, testable tool for pushing Docker images
 */

import { createDockerClient, type DockerClient } from '../../services/docker-client';
import type { MCPTool, MCPResponse } from '../../mcp/types';
import type { ToolContext } from '../../mcp/context';
import { Success, Failure, type Result } from '../../types';
import { getSuccessProgression } from '../../workflows/workflow-progression';
import { TOOL_NAMES } from '../../exports/tool-names.js';
import { pushImageSchema, type PushImageParams } from './schema';
import type { z } from 'zod';

export interface PushImageResult {
  success: true;
  registry: string;
  digest: string;
  pushedTag: string;
  NextStep?: string;
}

/**
 * Create push image tool with injected Docker client
 */
export function makePushImage(
  docker: DockerClient,
): MCPTool<typeof pushImageSchema, PushImageResult> {
  return {
    name: 'push_image',
    description: 'Push a Docker image to a registry',
    inputSchema: pushImageSchema,

    async handler(params: z.infer<typeof pushImageSchema>): Promise<MCPResponse<PushImageResult>> {
      // Validate required imageId
      if (!params.imageId) {
        return {
          content: [
            {
              type: 'text',
              text: 'Error: imageId is required to push an image',
            },
          ],
          error: 'Missing required parameter: imageId',
        };
      }

      // Parse repository and tag from imageId
      let repository: string;
      let tag: string;

      const colonIndex = params.imageId.lastIndexOf(':');
      if (colonIndex === -1 || colonIndex < params.imageId.lastIndexOf('/')) {
        // No tag specified, use 'latest'
        repository = params.imageId;
        tag = 'latest';
      } else {
        repository = params.imageId.substring(0, colonIndex);
        tag = params.imageId.substring(colonIndex + 1);
      }

      // Apply registry prefix if provided
      if (params.registry) {
        const registryHost = params.registry.replace(/^https?:\/\//, '').replace(/\/$/, '');
        if (!repository.startsWith(registryHost)) {
          repository = `${registryHost}/${repository}`;
        }
      }

      // Tag image if registry was specified
      if (params.registry) {
        const tagResult = await docker.tagImage(params.imageId, repository, tag);
        if (!tagResult.ok) {
          return {
            content: [
              {
                type: 'text',
                text: `Failed to tag image: ${tagResult.error}`,
              },
            ],
            error: tagResult.error,
          };
        }
      }

      // Push the image
      const pushResult = await docker.pushImage(repository, tag);
      if (!pushResult.ok) {
        return {
          content: [
            {
              type: 'text',
              text: `Failed to push image: ${pushResult.error}`,
            },
          ],
          error: pushResult.error,
        };
      }

      // Return success response
      const result: PushImageResult = {
        success: true,
        registry: params.registry || 'docker.io',
        digest: pushResult.value.digest,
        pushedTag: `${repository}:${tag}`,
        NextStep: getSuccessProgression(TOOL_NAMES.PUSH_IMAGE, { completed_steps: [] }).summary,
      };

      return {
        content: [
          {
            type: 'text',
            text: `Successfully pushed image ${result.pushedTag} with digest ${result.digest}`,
          },
        ],
        value: result,
      };
    },
  };
}

/**
 * Push image function for workflow usage
 * Follows the standard pattern used by other tools
 */
export async function pushImage(
  params: PushImageParams,
  context: ToolContext,
): Promise<Result<PushImageResult>> {
  const logger = context.logger;
  const dockerClient = createDockerClient(logger);
  const tool = makePushImage(dockerClient);

  const result = await tool.handler(params);

  if ('error' in result) {
    return Failure(result.error);
  }

  if (result.value) {
    return Success(result.value);
  }

  return Failure('Unexpected result from push-image tool');
}
