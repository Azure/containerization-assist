/**
 * Health Check Module
 * Consolidated health check logic for Docker and Kubernetes dependencies
 */

import type { Logger } from 'pino';
import Docker from 'dockerode';
import { autoDetectDockerSocket } from '@/infra/docker/socket-validation';
import { createKubernetesClient } from '@/infra/kubernetes/client';
import { extractErrorMessage } from './error-utils';

/**
 * Status of an individual dependency
 */
export interface DependencyStatus {
  available: boolean;
  version?: string;
  error?: string;
}

/**
 * Default timeout for health checks in milliseconds
 */
const DEFAULT_TIMEOUT_MS = 3000;

/**
 * Check Docker daemon health and connectivity
 *
 * @param logger - Logger instance for diagnostic output
 * @param options - Optional configuration
 * @returns Docker availability status with version or error details
 */
export async function checkDockerHealth(
  logger: Logger,
  options: { timeout?: number } = {},
): Promise<DependencyStatus> {
  const timeout = options.timeout ?? DEFAULT_TIMEOUT_MS;

  try {
    const socketPath = autoDetectDockerSocket();
    const docker = new Docker({ socketPath });

    const versionInfo = await Promise.race([
      docker.version(),
      new Promise<never>((_, reject) =>
        setTimeout(() => reject(new Error('Docker connection timeout')), timeout),
      ),
    ]);

    return {
      available: true,
      version: versionInfo.Version,
    };
  } catch (error) {
    logger.debug({ error }, 'Docker health check failed');
    return {
      available: false,
      error: extractErrorMessage(error),
    };
  }
}

/**
 * Check Kubernetes cluster health and connectivity
 *
 * @param logger - Logger instance for diagnostic output
 * @param options - Optional configuration
 * @returns Kubernetes availability status with version or error details
 */
export async function checkKubernetesHealth(
  logger: Logger,
  options: { timeout?: number } = {},
): Promise<DependencyStatus> {
  const timeout = options.timeout ?? DEFAULT_TIMEOUT_MS;

  try {
    const k8sClient = createKubernetesClient(logger);

    const connected = await Promise.race([
      k8sClient.ping(),
      new Promise<never>((_, reject) =>
        setTimeout(() => reject(new Error('Kubernetes connection timeout')), timeout),
      ),
    ]);

    if (connected) {
      return {
        available: true,
        version: 'connected',
      };
    }

    return {
      available: false,
      error: 'Unable to connect to Kubernetes cluster',
    };
  } catch (error) {
    logger.debug({ error }, 'Kubernetes health check failed');
    return {
      available: false,
      error: extractErrorMessage(error),
    };
  }
}
