/**
 * Compatibility wrapper for push-image tool
 * Provides backward compatibility with old tool interface
 */

import type { ToolContext } from '../../mcp/context/types';
import { createLogger } from '../../lib/logger';
import { Success, Failure, type Result } from '../../types';
import type { PushImageParams } from './schema';
import { makePushImage } from './tool';
import { createDockerClient } from '../../services/docker/client';

export interface PushImageResult {
  success: boolean;
  sessionId: string;
  registry: string;
  digest: string;
  pushedTags: string[];
}

/**
 * Compatibility wrapper for existing code expecting old push-image interface
 */
export async function pushImage(
  params: PushImageParams,
  context: ToolContext,
): Promise<Result<PushImageResult>> {
  const logger = context.logger || createLogger({ name: 'push-image' });
  const dockerClient = createDockerClient(logger);
  const tool = makePushImage(dockerClient);

  const result = await tool.handler(params);

  if ('error' in result) {
    return Failure(result.error);
  }

  if (result.value) {
    const pushedTag = result.value.pushedTag;
    return Success({
      success: true,
      sessionId: params.sessionId || 'default',
      registry: result.value.registry,
      digest: result.value.digest,
      pushedTags: [pushedTag],
    });
  }

  return Failure('Unexpected result from push-image tool');
}
