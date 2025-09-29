/**
 * Docker Client - Library Export
 *
 * Re-exports Docker client functionality from infrastructure for lib/ imports
 */

// Re-export from infrastructure
export { createDockerClient, type DockerBuildOptions } from '@/infra/docker/client';
