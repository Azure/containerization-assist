/**
 * Docker client for containerization operations
 *
 * @see {@link ../../../docs/adr/006-infrastructure-organization.md ADR-006: Infrastructure Layer Organization}
 */

import Docker, { DockerOptions } from 'dockerode';
import tar from 'tar-fs';
import path from 'path';
import type { Logger } from 'pino';
import { Success, Failure, type Result } from '@/types';
import { extractDockerErrorGuidance } from './errors';
import { autoDetectDockerSocket } from './socket-validation';
import { getDockerBuildFiles } from '@/lib/dockerignore-parser';
import { createProgressTracker, type ProgressCallback } from './progress';

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
 * Callback for Docker build progress events
 */
export type DockerBuildProgressCallback = ProgressCallback;

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
  /** Optional callback for build progress events */
  onProgress?: DockerBuildProgressCallback;
}

/**
 * Result of a Docker image build operation.
 */
export interface DockerBuildResult {
  /** Unique identifier of the built image */
  imageId: string;
  /** Content-addressable digest of the built image */
  digest: string;
  /** Size of the built image in bytes */
  size: number;
  /** Number of layers in the image */
  layers?: number;
  /** Total build time in milliseconds */
  buildTime: number;
  /** Build process log messages */
  logs: string[];
  /** Tags applied to the built image */
  tags?: string[];
  /** Build-time warnings */
  warnings: string[];
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
}

/**
 * Generate a digest from an image ID
 * @param imageId - The Docker image ID
 * @param logger - Logger instance
 * @returns A SHA-256 digest string or empty string if invalid
 */
function generateDigestFromImageId(imageId: string, logger: Logger): string {
  // If already prefixed, validate the hash portion
  if (imageId.startsWith('sha256:')) {
    const hash = imageId.substring(7);
    if (/^[a-f0-9]{64}$/.test(hash)) {
      return imageId;
    }
  } else {
    // If not prefixed, validate and add prefix
    if (/^[a-f0-9]{64}$/.test(imageId)) {
      return `sha256:${imageId}`;
    }
  }

  logger.warn({ imageId }, 'Image ID is not a valid SHA-256 hash, cannot generate digest');
  return '';
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
    async buildImage(options: DockerBuildOptions): Promise<Result<DockerBuildResult>> {
      const buildLogs: string[] = [];
      const buildWarnings: string[] = [];
      const startTime = Date.now();

      try {
        logger.debug({ options }, 'Starting Docker build');

        const contextPath = options.context || '.';
        const dockerfilePath = options.dockerfile
          ? path.resolve(contextPath, options.dockerfile)
          : undefined;
        const files = await getDockerBuildFiles(contextPath, dockerfilePath);

        const tarStream = tar.pack(contextPath, {
          entries: files,
        });

        const stream = await docker.buildImage(tarStream, {
          t: options.t || options.tags?.[0],
          dockerfile: options.dockerfile,
          buildargs: options.buildargs || options.buildArgs,
          ...(options.platform && { platform: options.platform }),
          version: '2', // Use BuildKit backend for cross-platform builds
        });

        interface DockerBuildEvent {
          stream?: string;
          aux?: { ID?: string };
          id?: string;
          error?: string;
          errorDetail?: Record<string, unknown>;
        }

        interface DockerBuildResponse {
          aux?: { ID?: string };
        }

        let buildError: string | null = null;

        // Create progress tracker for BuildKit trace decoding and progress notifications
        const trackerOptions: { onProgress?: ProgressCallback; logger: Logger } = { logger };
        if (options.onProgress) {
          trackerOptions.onProgress = options.onProgress;
        }
        const progressTracker = createProgressTracker(trackerOptions);

        const result = await new Promise<DockerBuildResponse[]>((resolve, reject) => {
          docker.modem.followProgress(
            stream,
            (err: Error | null, res: DockerBuildResponse[]) => {
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
                    options,
                  },
                  'Docker build followProgress error',
                );
                reject(err);
              } else if (buildError) {
                // If we detected an error during build progress, treat it as a failure
                const errorObj = new Error(buildError);
                logger.error({ buildError, options }, 'Docker build failed with error event');
                reject(errorObj);
              } else {
                resolve(res);
              }
            },
            (event: DockerBuildEvent) => {
              // For BuildKit progress events, decode and send build status updates
              if (event.id === 'moby.buildkit.trace') {
                const buildKitMessage = progressTracker.processBuildKitTrace(event.aux);
                // Also capture BuildKit messages in build logs
                if (buildKitMessage) {
                  buildLogs.push(buildKitMessage);
                }
              }

              if (event.error || event.errorDetail) {
                logger.error({ errorEvent: event }, 'Docker build error event received');
                const errorMsg = event.error || 'Build step failed';
                const errorLogLine = `ERROR: ${errorMsg}`;
                buildLogs.push(errorLogLine);
                logger.error(errorLogLine);

                // Capture the first error encountered during the build
                if (!buildError) {
                  buildError =
                    event.error ||
                    (event.errorDetail &&
                    typeof event.errorDetail === 'object' &&
                    'message' in event.errorDetail
                      ? String(event.errorDetail.message)
                      : 'Build step failed');
                }
              }
            },
          );
        });

        const imageId = result[result.length - 1]?.aux?.ID || '';
        const buildTime = Date.now() - startTime;

        // Inspect the image to get size, digest, and layers
        let size = 0;
        let digest = '';
        let layers: number | undefined;

        if (imageId) {
          try {
            const image = docker.getImage(imageId);
            const inspect = await image.inspect();

            size = inspect.Size || 0;
            // Use RepoDigests if available, otherwise fallback to image ID if it looks like a valid SHA-256 hash
            if (inspect.RepoDigests?.[0]) {
              digest = inspect.RepoDigests[0];
            } else {
              digest = generateDigestFromImageId(inspect.Id, logger);
            }
            layers = inspect.RootFS?.Layers?.length;
          } catch (inspectError) {
            logger.warn({ error: inspectError, imageId }, 'Could not inspect image after build');
            buildWarnings.push('Could not retrieve complete image metadata');
            // Use fallback digest from image ID
            digest = generateDigestFromImageId(imageId, logger);
          }
        }

        const buildResult: DockerBuildResult = {
          imageId,
          digest,
          size,
          ...(layers !== undefined && { layers }),
          buildTime,
          logs: buildLogs,
          tags: options.tags || [],
          warnings: buildWarnings,
        };

        logger.debug({ buildResult }, 'Docker build completed successfully');
        return Success(buildResult);
      } catch (error) {
        const guidance = extractDockerErrorGuidance(error);
        const errorMessage = `Build failed: ${guidance.message}`;

        logger.error(
          {
            error: errorMessage,
            hint: guidance.hint,
            resolution: guidance.resolution,
            errorDetails: guidance.details,
            originalError: error,
            options,
            buildLogs,
          },
          'Docker build failed',
        );

        const enhancedGuidance = {
          ...guidance,
          details: {
            ...guidance.details,
            buildLogs: buildLogs.length > 0 ? buildLogs : ['No build logs captured'],
            buildTime: Date.now() - startTime,
          },
        };

        return Failure(errorMessage, enhancedGuidance);
      }
    },

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
  };
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

  // Create and return client
  return createBaseDockerClient(docker, logger);
};
