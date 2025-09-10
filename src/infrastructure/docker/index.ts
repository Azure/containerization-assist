/**
 * Docker infrastructure - External Docker client interface
 */

export {
  type DockerClient,
  createDockerClient,
  type DockerBuildOptions,
  type DockerBuildResult,
  type DockerPushResult,
  type DockerImageInfo,
} from './client';
export { createDockerRegistryClient } from './registry';
