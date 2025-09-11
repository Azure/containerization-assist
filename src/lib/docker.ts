/**
 * Docker Client - Library Export
 *
 * Re-exports Docker client functionality from infrastructure for lib/ imports
 */

// Re-export from infrastructure
export { createDockerClient, type DockerBuildOptions } from '../services/docker/client';

export { getImageMetadata } from '../services/docker/registry';
