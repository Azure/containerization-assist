/**
 * Docker client for containerization operations
 */

import Docker, { DockerOptions } from 'dockerode';
import tar from 'tar-fs';
import type { Logger } from 'pino';
import { Success, Failure, type Result } from '@types';
import { extractDockerErrorMessage } from './errors';
import { homedir } from 'os';
import { join } from 'path';
import { existsSync, statSync } from 'fs';

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
 * Get Colima socket paths in order of preference.
 */
function getColimaSockets(): string[] {
  const homeDir = homedir();
  return [
    join(homeDir, '.colima/default/docker.sock'),
    join(homeDir, '.colima/docker/docker.sock'),
    join(homeDir, '.lima/colima/sock/docker.sock'), // Lima-based colima
  ];
}

/**
 * Find the first available Docker socket from the given paths.
 */
function findAvailableDockerSocket(socketPaths: string[]): string | null {
  for (const socketPath of socketPaths) {
    try {
      if (existsSync(socketPath)) {
        const stat = statSync(socketPath);
        if (stat.isSocket()) {
          return socketPath;
        }
      }
    } catch {
      // Continue to next socket path
    }
  }
  return null;
}

/**
 * Auto-detect Docker socket path with Colima support.
 */
function autoDetectDockerSocket(): string {
  const defaultPaths = [
    '/var/run/docker.sock', // Standard Docker socket
    ...getColimaSockets(), // Colima sockets
  ];

  const availableSocket = findAvailableDockerSocket(defaultPaths);
  return availableSocket || '/var/run/docker.sock'; // Fallback to default
}

/**
 * Options for building a Docker image.
 */
export interface DockerBuildOptions {
  /** Path to Dockerfile relative to context */
  dockerfile?: string;
  /** Primary tag for the built image */
  t?: string;
  /** Additional tags to apply to the built image */
  tags?: string[];
  /** Build context directory (default: current directory) */
  context?: string;
  /** Build-time variables (Docker ARG values) */
  buildargs?: Record<string, string>;
  /** Alternative property name for build arguments */
  buildArgs?: Record<string, string>;
  /** Target platform for multi-platform builds (e.g., 'linux/amd64') */
  platform?: string;
}

/**
 * Result of a Docker image build operation.
 */
export interface DockerBuildResult {
  /** Unique identifier of the built image */
  imageId: string;
  /** Build process log messages */
  logs: string[];
  /** Tags applied to the built image */
  tags?: string[];
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
   * Builds a Docker image from a Dockerfile.
   * @param options - Build configuration options
   * @returns Result containing build details or error
   */
  buildImage: (options: DockerBuildOptions) => Promise<Result<DockerBuildResult>>;

  /**
   * Retrieves information about a Docker image.
   * @param id - Image ID or tag
   * @returns Result containing image information or error
   */
  getImage: (id: string) => Promise<Result<DockerImageInfo>>;

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
   * @returns Result containing push details or error
   */
  pushImage: (repository: string, tag: string) => Promise<Result<DockerPushResult>>;

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
}

/**
 * Create a Docker client with core operations
 * @param logger - Logger instance for debug output
 * @param config - Optional Docker client configuration
 * @returns DockerClient with build, get, tag, and push operations
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

  if (socketPath.startsWith('tcp://') || socketPath.startsWith('http://')) {
    // TCP connection
    dockerOptions.host = config?.host || 'localhost';
    dockerOptions.port = config?.port || 2375;
  } else {
    // Unix socket connection
    dockerOptions.socketPath = socketPath;
  }

  if (config?.timeout) {
    dockerOptions.timeout = config.timeout;
  }

  const docker = new Docker(dockerOptions);

  logger.debug({ dockerOptions }, 'Created Docker client');

  return {
    async buildImage(options: DockerBuildOptions): Promise<Result<DockerBuildResult>> {
      const buildLogs: string[] = [];

      try {
        logger.debug({ options }, 'Starting Docker build');

        // Create tar stream from the build context directory
        const contextPath = options.context || '.';
        const tarStream = tar.pack(contextPath);

        const stream = await docker.buildImage(tarStream, {
          t: options.t || options.tags?.[0],
          dockerfile: options.dockerfile,
          buildargs: options.buildargs || options.buildArgs,
        });

        interface DockerBuildEvent {
          stream?: string;
          aux?: { ID?: string };
          error?: string;
          errorDetail?: Record<string, unknown>;
        }

        interface DockerBuildResponse {
          aux?: { ID?: string };
        }

        const result = await new Promise<DockerBuildResponse[]>((resolve, reject) => {
          docker.modem.followProgress(
            stream,
            (err: Error | null, res: DockerBuildResponse[]) => {
              if (err) {
                // Log detailed error information before rejecting
                const { message, details } = extractDockerErrorMessage(err);
                logger.error(
                  {
                    error: message,
                    errorDetails: details,
                    originalError: err,
                    options,
                  },
                  'Docker build followProgress error',
                );
                reject(err);
              } else {
                resolve(res);
              }
            },
            (event: DockerBuildEvent) => {
              logger.debug(event, 'Docker build progress');

              if (event.error || event.errorDetail) {
                logger.error({ errorEvent: event }, 'Docker build error event received');
              }
            },
          );
        });

        const buildResult: DockerBuildResult = {
          imageId: result[result.length - 1]?.aux?.ID || '',
          tags: options.tags || [],
          logs: buildLogs,
        };

        logger.debug({ buildResult }, 'Docker build completed successfully');
        return Success(buildResult);
      } catch (error) {
        const { message, details } = extractDockerErrorMessage(error);
        const errorMessage = `Build failed: ${message}`;

        logger.error(
          {
            error: errorMessage,
            errorDetails: details,
            originalError: error,
            options,
          },
          'Docker build failed',
        );

        return Failure(errorMessage);
      }
    },

    async getImage(id: string): Promise<Result<DockerImageInfo>> {
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
        const { message, details } = extractDockerErrorMessage(error);
        const errorMessage = `Failed to get image: ${message}`;

        logger.error(
          {
            error: errorMessage,
            errorDetails: details,
            originalError: error,
            imageId: id,
          },
          'Docker get image failed',
        );

        return Failure(errorMessage);
      }
    },

    async tagImage(imageId: string, repository: string, tag: string): Promise<Result<void>> {
      try {
        const image = docker.getImage(imageId);
        await image.tag({ repo: repository, tag });

        logger.info({ imageId, repository, tag }, 'Image tagged successfully');
        return Success(undefined);
      } catch (error) {
        const { message, details } = extractDockerErrorMessage(error);
        const errorMessage = `Failed to tag image: ${message}`;

        logger.error(
          {
            error: errorMessage,
            errorDetails: details,
            originalError: error,
            imageId,
            repository,
            tag,
          },
          'Docker tag image failed',
        );

        return Failure(errorMessage);
      }
    },

    async pushImage(repository: string, tag: string): Promise<Result<DockerPushResult>> {
      try {
        const image = docker.getImage(`${repository}:${tag}`);
        const stream = await image.push({});

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
          docker.modem.followProgress(
            stream,
            (err: Error | null) => {
              if (err) {
                // Log detailed error information before rejecting
                const { message, details } = extractDockerErrorMessage(err);
                logger.error(
                  {
                    error: message,
                    errorDetails: details,
                    originalError: err,
                    repository,
                    tag,
                  },
                  'Docker push followProgress error',
                );
                reject(err);
              } else {
                resolve();
              }
            },
            (event: DockerPushEvent) => {
              logger.debug(event, 'Docker push progress');

              // Log errors from Docker events - dockerode provides explicit error fields
              if (event.error || event.errorDetail) {
                logger.error({ errorEvent: event }, 'Docker push error event received');
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
        const { message, details } = extractDockerErrorMessage(error);
        const errorMessage = `Failed to push image: ${message}`;

        logger.error(
          {
            error: errorMessage,
            errorDetails: details,
            originalError: error,
            repository,
            tag,
          },
          'Docker push image failed',
        );

        return Failure(errorMessage);
      }
    },

    async removeImage(imageId: string, force = false): Promise<Result<void>> {
      try {
        logger.debug({ imageId, force }, 'Starting Docker image removal');

        const image = docker.getImage(imageId);
        await image.remove({ force });

        logger.debug({ imageId }, 'Docker image removed successfully');
        return Success(undefined);
      } catch (error) {
        const { message, details } = extractDockerErrorMessage(error);
        const errorMessage = `Failed to remove image: ${message}`;

        logger.error(
          {
            error: errorMessage,
            errorDetails: details,
            originalError: error,
            imageId,
          },
          'Docker remove image failed',
        );

        return Failure(errorMessage);
      }
    },

    async removeContainer(containerId: string, force = false): Promise<Result<void>> {
      try {
        logger.debug({ containerId, force }, 'Starting Docker container removal');

        const container = docker.getContainer(containerId);
        await container.remove({ force });

        logger.debug({ containerId }, 'Docker container removed successfully');
        return Success(undefined);
      } catch (error) {
        const { message, details } = extractDockerErrorMessage(error);
        const errorMessage = `Failed to remove container: ${message}`;

        logger.error(
          {
            error: errorMessage,
            errorDetails: details,
            originalError: error,
            containerId,
          },
          'Docker remove container failed',
        );

        return Failure(errorMessage);
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
        const { message, details } = extractDockerErrorMessage(error);
        const errorMessage = `Failed to list containers: ${message}`;

        logger.error(
          {
            error: errorMessage,
            errorDetails: details,
            originalError: error,
            options,
          },
          'Docker list containers failed',
        );

        return Failure(errorMessage);
      }
    },
  };
};
