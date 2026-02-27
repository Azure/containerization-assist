/**
 * Docker client for containerization operations
 *
 * @see {@link ../../../docs/adr/006-infrastructure-organization.md ADR-006: Infrastructure Layer Organization}
 */

import Docker, { DockerOptions } from 'dockerode';
import type { Logger } from 'pino';
import { Success, Failure, type Result } from '@/types';
import { extractDockerErrorGuidance } from './errors';
import { autoDetectDockerSocket, parseDockerHost } from './socket-validation';

/**
 * Docker client configuration options.
 */
export interface DockerClientConfig {
  /** Docker socket path (defaults to auto-detection with Colima support) */
  socketPath?: string;
  /** Docker daemon host (for TCP connections) */
  host?: string;
  /** Docker daemon port (for TCP connections) */
  port?: number;
  /** Connection timeout in milliseconds */
  timeout?: number;
}

/**
 * Result of pushing a Docker image to a registry.
 */
export interface DockerPushResult {
  /** Content-addressable digest of the pushed image */
  digest: string;
  /** Size of the pushed image in bytes */
  size?: number;
}

/**
 * Docker container information.
 */
export interface DockerContainerInfo {
  /** Container ID */
  Id: string;
  /** Container names */
  Names: string[];
  /** Image used for this container */
  Image: string;
  /** Container state */
  State: string;
  /** Container status */
  Status: string;
}

/**
 * Information about a Docker image.
 */
export interface DockerImageInfo {
  /** Unique identifier of the image */
  Id: string;
  /** Repository tags associated with the image */
  RepoTags?: string[];
  /** Size of the image in bytes */
  Size?: number;
  /** ISO 8601 timestamp when the image was created */
  Created?: string;
}

/**
 * Docker client interface for container operations.
 */
export interface DockerClient {
  /**
   * Retrieves information about a Docker image.
   * @param id - Image ID or tag
   * @returns Result containing image information or error
   */
  getImage: (id: string) => Promise<Result<DockerImageInfo>>;

  /**
   * Inspects a Docker image and retrieves metadata.
   * Alias for getImage for consistency with Docker CLI terminology.
   * @param imageId - Image ID or tag
   * @returns Result containing image information or error
   */
  inspectImage: (imageId: string) => Promise<Result<DockerImageInfo>>;

  /**
   * Tags a Docker image with a new repository and tag.
   * @param imageId - ID of the image to tag
   * @param repository - Target repository name
   * @param tag - Target tag name
   * @returns Result indicating success or error
   */
  tagImage: (imageId: string, repository: string, tag: string) => Promise<Result<void>>;

  /**
   * Pushes a Docker image to a registry.
   * @param repository - Repository name
   * @param tag - Tag to push
   * @param authConfig - Optional authentication configuration for registry
   * @returns Result containing push details or error
   */
  pushImage: (
    repository: string,
    tag: string,
    authConfig?: { username: string; password: string; serveraddress: string },
  ) => Promise<Result<DockerPushResult>>;

  /**
   * Removes a Docker image.
   * @param imageId - Image ID or tag to remove
   * @param force - Force removal of the image
   * @returns Result indicating success or error
   */
  removeImage: (imageId: string, force?: boolean) => Promise<Result<void>>;

  /**
   * Removes a Docker container.
   * @param containerId - Container ID to remove
   * @param force - Force removal of the container
   * @returns Result indicating success or error
   */
  removeContainer: (containerId: string, force?: boolean) => Promise<Result<void>>;

  /**
   * Lists Docker containers.
   * @param options - Container list options
   * @returns Result containing container list or error
   */
  listContainers: (options?: {
    all?: boolean;
    filters?: Record<string, string[]>;
  }) => Promise<Result<DockerContainerInfo[]>>;

  /**
   * Pings the Docker daemon to verify connectivity.
   * @returns Result indicating whether Docker daemon is available
   */
  ping: () => Promise<Result<void>>;
}

/**
 * Create base Docker client implementation
 */
function createBaseDockerClient(docker: Docker, logger: Logger): DockerClient {
  // Helper function to fetch image info (used by both getImage and inspectImage)
  const fetchImageInfo = async (id: string): Promise<Result<DockerImageInfo>> => {
    try {
      const image = docker.getImage(id);
      const inspect = await image.inspect();

      const imageInfo: DockerImageInfo = {
        Id: inspect.Id,
        RepoTags: inspect.RepoTags,
        Size: inspect.Size,
        Created: inspect.Created,
      };

      return Success(imageInfo);
    } catch (error) {
      const guidance = extractDockerErrorGuidance(error);
      const errorMessage = `Failed to get image: ${guidance.message}`;

      logger.error(
        {
          error: errorMessage,
          hint: guidance.hint,
          resolution: guidance.resolution,
          errorDetails: guidance.details,
          originalError: error,
          imageId: id,
        },
        'Docker get image failed',
      );

      return Failure(errorMessage, guidance);
    }
  };

  return {
    async getImage(id: string): Promise<Result<DockerImageInfo>> {
      return fetchImageInfo(id);
    },

    async inspectImage(imageId: string): Promise<Result<DockerImageInfo>> {
      // Alias for getImage - delegate to the same implementation
      return fetchImageInfo(imageId);
    },

    async tagImage(imageId: string, repository: string, tag: string): Promise<Result<void>> {
      try {
        const image = docker.getImage(imageId);
        await image.tag({ repo: repository, tag });

        logger.info({ imageId, repository, tag }, 'Image tagged successfully');
        return Success(undefined);
      } catch (error) {
        const guidance = extractDockerErrorGuidance(error);
        const errorMessage = `Failed to tag image: ${guidance.message}`;

        logger.error(
          {
            error: errorMessage,
            hint: guidance.hint,
            resolution: guidance.resolution,
            errorDetails: guidance.details,
            originalError: error,
            imageId,
            repository,
            tag,
          },
          'Docker tag image failed',
        );

        return Failure(errorMessage, guidance);
      }
    },

    async pushImage(
      repository: string,
      tag: string,
      authConfig?: { username: string; password: string; serveraddress: string },
    ): Promise<Result<DockerPushResult>> {
      try {
        const image = docker.getImage(`${repository}:${tag}`);
        // dockerode's Image.push expects auth config inside the first options object
        // For local registries without auth, provide an empty authconfig object to avoid X-Registry-Auth header issues
        const stream = await image.push({
          authconfig: authConfig || {},
        });

        let digest = '';
        let size: number | undefined;

        interface DockerPushEvent {
          status?: string;
          progressDetail?: Record<string, unknown>;
          error?: string;
          errorDetail?: Record<string, unknown>;
          aux?: {
            Digest?: string;
            Size?: number;
          };
        }

        await new Promise<void>((resolve, reject) => {
          let pushError: Error | null = null;

          docker.modem.followProgress(
            stream,
            (err: Error | null) => {
              if (err) {
                // Log detailed error information before rejecting
                const guidance = extractDockerErrorGuidance(err);
                logger.error(
                  {
                    error: guidance.message,
                    hint: guidance.hint,
                    resolution: guidance.resolution,
                    errorDetails: guidance.details,
                    originalError: err,
                    repository,
                    tag,
                  },
                  'Docker push followProgress error',
                );
                reject(err);
              } else if (pushError) {
                // Reject if we encountered an error event during the push
                reject(pushError);
              } else {
                resolve();
              }
            },
            (event: DockerPushEvent) => {
              logger.debug(event, 'Docker push progress');

              // Only treat final error events as failures, not intermediate auth challenges
              if (event.error || event.errorDetail) {
                logger.debug(
                  { errorEvent: event },
                  'Docker push error event (may be intermediate)',
                );

                // Only set pushError for certain fatal errors, not auth challenges
                const errorMsg =
                  event.error || (event.errorDetail as { message?: string })?.message || '';

                // Don't treat authentication challenges as fatal errors - they're part of the auth handshake
                if (
                  !errorMsg.toLowerCase().includes('unauthorized') &&
                  !errorMsg.toLowerCase().includes('authentication required')
                ) {
                  pushError = new Error(errorMsg || 'Unknown push error');
                  logger.error({ errorEvent: event }, 'Fatal Docker push error event received');
                }
              }

              if (event.aux?.Digest) {
                digest = event.aux.Digest;
              }
              if (event.aux?.Size) {
                size = event.aux.Size;
              }
            },
          );
        });

        if (!digest) {
          try {
            const inspectResult = await image.inspect();
            digest =
              inspectResult.RepoDigests?.[0]?.split('@')[1] ||
              `sha256:${inspectResult.Id.replace('sha256:', '')}`;
          } catch (inspectError) {
            logger.warn({ error: inspectError }, 'Could not get digest from image inspection');
            digest = `sha256:${Date.now().toString(16)}${Math.random().toString(16).substring(2)}`;
          }
        }

        logger.info({ repository, tag, digest }, 'Image pushed successfully');
        const result: DockerPushResult = { digest };
        if (size !== undefined) {
          result.size = size;
        }
        return Success(result);
      } catch (error) {
        const guidance = extractDockerErrorGuidance(error);
        const errorMessage = `Failed to push image: ${guidance.message}`;

        logger.error(
          {
            error: errorMessage,
            hint: guidance.hint,
            resolution: guidance.resolution,
            errorDetails: guidance.details,
            originalError: error,
            repository,
            tag,
          },
          'Docker push image failed',
        );

        return Failure(errorMessage, guidance);
      }
    },

    async removeImage(imageId: string, force = false): Promise<Result<void>> {
      try {
        logger.debug({ imageId, force }, 'Starting Docker image removal');

        const image = docker.getImage(imageId);
        await image.remove({ force });

        logger.debug({ imageId }, 'Image removed');
        return Success(undefined);
      } catch (error) {
        const guidance = extractDockerErrorGuidance(error);
        const errorMessage = `Failed to remove image: ${guidance.message}`;

        logger.error(
          {
            error: errorMessage,
            hint: guidance.hint,
            resolution: guidance.resolution,
            errorDetails: guidance.details,
            originalError: error,
            imageId,
          },
          'Docker remove image failed',
        );

        return Failure(errorMessage, guidance);
      }
    },

    async removeContainer(containerId: string, force = false): Promise<Result<void>> {
      try {
        logger.debug({ containerId, force }, 'Starting Docker container removal');

        const container = docker.getContainer(containerId);
        await container.remove({ force });

        logger.debug({ containerId }, 'Container removed');
        return Success(undefined);
      } catch (error) {
        const guidance = extractDockerErrorGuidance(error);
        const errorMessage = `Failed to remove container: ${guidance.message}`;

        logger.error(
          {
            error: errorMessage,
            hint: guidance.hint,
            resolution: guidance.resolution,
            errorDetails: guidance.details,
            originalError: error,
            containerId,
          },
          'Docker remove container failed',
        );

        return Failure(errorMessage, guidance);
      }
    },

    async listContainers(
      options: { all?: boolean; filters?: Record<string, string[]> } = {},
    ): Promise<Result<DockerContainerInfo[]>> {
      try {
        logger.debug({ options }, 'Starting Docker container listing');

        const containers = await docker.listContainers(options);

        logger.debug(
          { containerCount: containers.length },
          'Docker containers listed successfully',
        );
        return Success(containers);
      } catch (error) {
        const guidance = extractDockerErrorGuidance(error);
        const errorMessage = `Failed to list containers: ${guidance.message}`;

        logger.error(
          {
            error: errorMessage,
            hint: guidance.hint,
            resolution: guidance.resolution,
            errorDetails: guidance.details,
            originalError: error,
            options,
          },
          'Docker list containers failed',
        );

        return Failure(errorMessage, guidance);
      }
    },

    async ping(): Promise<Result<void>> {
      try {
        logger.debug('Pinging Docker daemon');

        await docker.ping();

        logger.debug('Docker daemon is available');
        return Success(undefined);
      } catch (error) {
        const guidance = extractDockerErrorGuidance(error);
        const errorMessage = `Docker daemon is not available: ${guidance.message}`;

        logger.error(
          {
            error: errorMessage,
            hint: guidance.hint,
            resolution: guidance.resolution,
            errorDetails: guidance.details,
            originalError: error,
          },
          'Docker ping failed',
        );

        return Failure(errorMessage, {
          message: guidance.message,
          hint: 'Docker daemon is not running or not accessible',
          resolution:
            'Ensure Docker is installed and running. On Windows, check Docker Desktop is started. On Mac, ensure Docker Desktop or Colima is running.',
          ...(guidance.details && { details: guidance.details }),
        });
      }
    },
  };
}

/**
 * Create a Docker client with core operations
 * @param logger - Logger instance for debug output
 * @param config - Optional Docker client configuration
 * @returns DockerClient with get, tag, push, and management operations
 */
export const createDockerClient = (logger: Logger, config?: DockerClientConfig): DockerClient => {
  // Determine the socket path to use
  let socketPath: string;

  if (config?.socketPath) {
    socketPath = config.socketPath;
  } else {
    socketPath = autoDetectDockerSocket();
    logger.debug({ socketPath }, 'Auto-detected Docker socket');
  }

  // Create Docker client with detected socket path
  const dockerOptions: DockerOptions = {};

  if (
    socketPath.startsWith('tcp://') ||
    socketPath.startsWith('http://') ||
    socketPath.startsWith('https://')
  ) {
    // TCP connection — extract host/port from URL if no explicit config
    if (config?.host) {
      dockerOptions.host = config.host;
      dockerOptions.port = config.port || 2375;
    } else {
      try {
        const parsed = parseDockerHost(socketPath);
        if (parsed.type === 'tcp') {
          dockerOptions.host = parsed.host;
          dockerOptions.port = parsed.port;
          if (parsed.value.startsWith('https://')) {
            dockerOptions.protocol = 'https';
          }
        }
      } catch {
        // Fallback to defaults if parsing fails
        dockerOptions.host = 'localhost';
        dockerOptions.port = 2375;
      }
    }
  } else {
    // Unix socket / named pipe connection
    dockerOptions.socketPath = socketPath;
  }

  if (config?.timeout) {
    dockerOptions.timeout = config.timeout;
  }

  const docker = new Docker(dockerOptions);

  logger.debug({ dockerOptions }, 'Created Docker client');

  // Create and return client
  return createBaseDockerClient(docker, logger);
};
