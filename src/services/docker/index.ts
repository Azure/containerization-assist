/**
 * Docker infrastructure module exports
 */

export {
  createDockerClient,
  type DockerClient,
  type DockerBuildOptions,
  type DockerBuildResult,
  type DockerPushResult,
  type DockerImageInfo,
} from './client';

export { getImageMetadata, type ImageMetadata } from './registry';
