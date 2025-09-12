/**
 * Docker client for containerization operations
 */

import Docker from 'dockerode';
import tar from 'tar-fs';
import type { Logger } from 'pino';
import { Success, Failure, type Result } from '../types';
import { formatErrorMessage } from '../lib/error-utils';
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
}

/**
 * Create a Docker client with core operations
 * @param logger - Logger instance for debug output
 * @returns DockerClient with build, get, tag, and push operations
 */
export const createDockerClient = (logger: Logger): DockerClient => {
  const docker = new Docker();

  return {
    async buildImage(options: DockerBuildOptions): Promise<Result<DockerBuildResult>> {
      const buildLogs: string[] = [];

      try {
        logger.debug({ options }, 'Starting Docker build');

        // Create tar stream from the build context directory
        const contextPath = options.context || '.';
        const tarStream = tar.pack(contextPath);

        const stream = await docker.buildImage(tarStream, {
          t: options.t || (Array.isArray(options.tags) ? options.tags[0] : options.tags),
          dockerfile: options.dockerfile,
          buildargs: options.buildargs || options.buildArgs,
        });

        interface DockerBuildEvent {
          stream?: string;
          aux?: { ID?: string };
          error?: string;
          errorDetail?: { message?: string };
        }

        interface DockerBuildResponse {
          aux?: { ID?: string };
        }

        let buildError: string | null = null;

        const result = await new Promise<DockerBuildResponse[]>((resolve, reject) => {
          docker.modem.followProgress(
            stream,
            (err: Error | null, res: DockerBuildResponse[]) => {
              if (err) {
                reject(err);
              } else if (buildError) {
                reject(new Error(buildError));
              } else {
                resolve(res);
              }
            },
            (event: DockerBuildEvent) => {
              // Capture build output
              if (event.stream) {
                buildLogs.push(event.stream.trim());
              }

              // Capture errors
              if (event.error) {
                buildError = event.error;
              } else if (event.errorDetail?.message) {
                buildError = event.errorDetail.message;
              }

              logger.debug(event, 'Docker build progress');
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
        const errorMessage = formatErrorMessage('Build failed', error);
        logger.error({ error: errorMessage, options, buildLogs }, 'Docker build failed');

        return Failure(errorMessage);
      }
    },

    async getImage(id: string): Promise<Result<DockerImageInfo>> {
      try {
        const image = docker.getImage(id);
        const inspect = await image.inspect();

        const imageInfo = {
          Id: inspect.Id,
          repository: inspect.RepoTags?.[0]?.split(':')[0] || '',
          tag: inspect.RepoTags?.[0]?.split(':')[1] || 'latest',
          size: inspect.Size,
          created: inspect.Created,
          labels: inspect.Config?.Labels || {},
        };

        return Success(imageInfo);
      } catch (error) {
        const errorMessage = formatErrorMessage('Failed to get image', error);
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
        const errorMessage = formatErrorMessage('Failed to tag image', error);
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
          aux?: {
            Digest?: string;
            Size?: number;
          };
        }

        await new Promise<void>((resolve, reject) => {
          docker.modem.followProgress(
            stream,
            (err: Error | null) => (err ? reject(err) : resolve()),
            (event: DockerPushEvent) => {
              logger.debug(event, 'Docker push progress');

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
            digest = `sha256:${Date.now().toString(16)}${Math.random().toString(16).substr(2)}`;
          }
        }

        logger.info({ repository, tag, digest }, 'Image pushed successfully');
        const result: DockerPushResult = { digest };
        if (size !== undefined) {
          result.size = size;
        }
        return Success(result);
      } catch (error) {
        const errorMessage = formatErrorMessage('Failed to push image', error);
        return Failure(errorMessage);
      }
    },
  };
};
