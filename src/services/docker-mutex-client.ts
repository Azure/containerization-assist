/**
 * Mutex-enabled Docker client wrapper for thread-safe operations
 */

import type { Logger } from 'pino';
import { createHash } from 'crypto';
import { createKeyedMutex } from '../lib/mutex';
import { config } from '../config';
import {
  createDockerClient as createBaseDockerClient,
  type DockerClient,
  type DockerBuildOptions,
  type DockerBuildResult,
  type DockerImageInfo,
  type DockerPushResult,
} from './docker-client';
import { Failure, type Result } from '../types';

/**
 * Creates a hash key for Docker build operations to prevent concurrent builds
 * of the same context/dockerfile combination
 */
function hashBuildContext(options: DockerBuildOptions): string {
  const key = {
    context: options.context || '.',
    dockerfile: options.dockerfile || 'Dockerfile',
    platform: options.platform,
  };

  return createHash('sha256').update(JSON.stringify(key)).digest('hex').substring(0, 16);
}

/**
 * Creates a mutex-protected Docker client that prevents concurrent operations
 * on the same resources
 */
export function createMutexDockerClient(logger: Logger): DockerClient {
  const baseClient = createBaseDockerClient(logger);
  const mutex = createKeyedMutex({
    defaultTimeout: config.mutex.defaultTimeout,
    monitoringEnabled: config.mutex.monitoringEnabled,
  });

  return {
    async buildImage(options: DockerBuildOptions): Promise<Result<DockerBuildResult>> {
      const lockKey = `docker:build:${hashBuildContext(options)}`;
      const timeout = config.mutex.dockerBuildTimeout;

      logger.debug({ lockKey, timeout }, 'Acquiring build mutex');

      try {
        return await mutex.withLock(
          lockKey,
          async () => {
            logger.debug({ lockKey }, 'Build mutex acquired');
            const result = await baseClient.buildImage(options);
            logger.debug({ lockKey, success: result.ok }, 'Build completed');
            return result;
          },
          timeout,
        );
      } catch (error) {
        if (error instanceof Error && error.message.includes('Mutex timeout')) {
          logger.error({ lockKey, timeout }, 'Build mutex timeout');
          return Failure(
            `Build operation timed out after ${timeout}ms - another build may be in progress`,
          );
        }
        throw error;
      }
    },

    async getImage(id: string): Promise<Result<DockerImageInfo>> {
      // Image inspection is read-only, no mutex needed
      return baseClient.getImage(id);
    },

    async tagImage(imageId: string, repository: string, tag: string): Promise<Result<void>> {
      const lockKey = `docker:tag:${imageId}`;

      return mutex.withLock(lockKey, async () => {
        logger.debug({ imageId, repository, tag }, 'Tagging image with mutex');
        return baseClient.tagImage(imageId, repository, tag);
      });
    },

    async pushImage(repository: string, tag: string): Promise<Result<DockerPushResult>> {
      const lockKey = `docker:push:${repository}:${tag}`;
      const timeout = config.mutex.dockerBuildTimeout; // Use same timeout as builds

      logger.debug({ lockKey, repository, tag }, 'Acquiring push mutex');

      try {
        return await mutex.withLock(
          lockKey,
          async () => {
            logger.debug({ lockKey }, 'Push mutex acquired');
            const result = await baseClient.pushImage(repository, tag);
            logger.debug({ lockKey, success: result.ok }, 'Push completed');
            return result;
          },
          timeout,
        );
      } catch (error) {
        if (error instanceof Error && error.message.includes('Mutex timeout')) {
          logger.error({ lockKey, timeout }, 'Push mutex timeout');
          return Failure(
            `Push operation timed out after ${timeout}ms - another push may be in progress`,
          );
        }
        throw error;
      }
    },
  };
}

/**
 * Get mutex status for monitoring
 */
export function getDockerMutexStatus(): Map<string, any> {
  const mutex = createKeyedMutex();
  return mutex.getStatus();
}
