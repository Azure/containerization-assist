/**
 * Verify Deployment Tool - Standardized Implementation
 *
 * Verifies Kubernetes deployment health and retrieves endpoints using
 * standardized helpers for consistency and improved error handling
 *
 * @example
 * ```typescript
 * const result = await verifyDeployment({
 *   sessionId: 'session-123',
 *   deploymentName: 'my-app',
 *   namespace: 'production',
 *   checks: ['pods', 'services', 'health']
 * }, context, logger);
 *
 * if (result.success) {
 *   logger.info('Deployment verified', {
 *     ready: result.ready,
 *     endpoints: result.endpoints
 *   });
 * }
 * ```
 */

import { getSession, updateSession } from '@mcp/tools/session-helpers';
import type { ToolContext } from '../../mcp/context/types';
import { createKubernetesClient, type KubernetesClient } from '../../lib/kubernetes';
import { createTimer, createLogger } from '../../lib/logger';
import { Success, Failure, type Result } from '../../domain/types';
import { DEFAULT_TIMEOUTS } from '../../config/defaults';
import type { VerifyDeploymentParams } from './schema';

export interface VerifyDeploymentResult {
  success: boolean;
  sessionId: string;
  namespace: string;
  deploymentName: string;
  serviceName: string;
  endpoints: Array<{
    type: 'internal' | 'external';
    url: string;
    port: number;
    healthy?: boolean;
  }>;
  ready: boolean;
  replicas: number;
  status: {
    readyReplicas: number;
    totalReplicas: number;
    conditions: Array<{
      type: string;
      status: string;
      message: string;
    }>;
  };
  healthCheck?: {
    status: 'healthy' | 'unhealthy' | 'unknown';
    message: string;
    checks?: Array<{
      name: string;
      status: 'pass' | 'fail';
      message?: string;
    }>;
  };
}

/**
 * Check deployment health
 */
async function checkDeploymentHealth(
  k8sClient: KubernetesClient,
  namespace: string,
  deploymentName: string,
  timeout: number,
): Promise<{
  ready: boolean;
  readyReplicas: number;
  totalReplicas: number;
  status: 'healthy' | 'unhealthy' | 'unknown';
  message: string;
}> {
  const startTime = Date.now();
  const pollInterval = DEFAULT_TIMEOUTS.healthCheck || 5000;

  while (Date.now() - startTime < timeout * 1000) {
    const statusResult = await k8sClient.getDeploymentStatus(namespace, deploymentName);

    if (statusResult.ok && statusResult.value?.ready) {
      return {
        ready: true,
        readyReplicas: statusResult.value.readyReplicas ?? 0,
        totalReplicas: statusResult.value.totalReplicas ?? 0,
        status: 'healthy',
        message: 'Deployment is healthy and ready',
      };
    }

    // Wait before checking again using configured interval
    await new Promise((resolve) => setTimeout(resolve, pollInterval));
  }

  return {
    ready: false,
    readyReplicas: 0,
    totalReplicas: 1,
    status: 'unhealthy',
    message: 'Deployment health check timed out',
  };
}

/**
 * Check endpoint health
 */
async function checkEndpointHealth(url: string): Promise<boolean> {
  try {
    // Make HTTP health check request
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), DEFAULT_TIMEOUTS.healthCheck || 5000);

    try {
      const response = await fetch(url, {
        method: 'GET',
        signal: controller.signal,
        headers: {
          'User-Agent': 'containerization-assist-health-check',
        },
      });

      clearTimeout(timeoutId);

      // Consider 2xx and 3xx responses as healthy
      return response.ok || (response.status >= 300 && response.status < 400);
    } catch (fetchError: unknown) {
      clearTimeout(timeoutId);

      // If it's an abort error, the request timed out
      if (fetchError instanceof Error && fetchError.name === 'AbortError') {
        return false;
      }

      // For other errors (network issues, etc.), consider unhealthy
      return false;
    }
  } catch {
    return false;
  }
}

/**
 * Deployment verification implementation - direct execution without wrapper
 */
async function verifyDeploymentImpl(
  params: VerifyDeploymentParams,
  context: ToolContext,
): Promise<Result<VerifyDeploymentResult>> {
  // Basic parameter validation (essential validation only)
  if (!params || typeof params !== 'object') {
    return Failure('Invalid parameters provided');
  }
  const logger = context.logger || createLogger({ name: 'verify-deployment' });
  const timer = createTimer(logger, 'verify-deployment');

  try {
    const {
      deploymentName: configDeploymentName,
      namespace: configNamespace,
      checks = ['pods', 'services', 'health'],
    } = params;

    const timeout = 60;

    logger.info(
      { deploymentName: configDeploymentName, namespace: configNamespace },
      'Starting deployment verification',
    );

    // Resolve session (now always optional)
    const sessionResult = await getSession(params.sessionId, context);

    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }

    const { id: sessionId, state: session } = sessionResult.value;
    logger.info({ sessionId, checks }, 'Starting Kubernetes deployment verification');

    const k8sClient = createKubernetesClient(logger);

    // Get deployment info from session or config
    const sessionState = session as
      | {
          deployment_result?: {
            namespace?: string;
            deploymentName?: string;
            serviceName?: string;
            endpoints?: Array<{
              type: 'internal' | 'external';
              url: string;
              port: number;
              healthy?: boolean;
            }>;
          };
        }
      | null
      | undefined;
    const deploymentResult = sessionState?.deployment_result;
    if (!deploymentResult && !configDeploymentName) {
      return Failure(
        'No deployment found. Provide deploymentName parameter or run deploy tool first.',
      );
    }

    const namespace = configNamespace ?? deploymentResult?.namespace ?? 'default';
    const deploymentName = configDeploymentName ?? deploymentResult?.deploymentName ?? 'app';
    const serviceName = deploymentResult?.serviceName ?? deploymentName;
    const endpoints = deploymentResult?.endpoints ?? [];

    logger.info({ namespace, deploymentName }, 'Checking deployment health');

    // Check deployment health
    const health = await checkDeploymentHealth(k8sClient, namespace, deploymentName, timeout);

    // Initialize health checks
    const healthChecks: Array<{ name: string; status: 'pass' | 'fail'; message?: string }> = [];

    // Check each endpoint if 'health' is in checks
    if (checks.includes('health')) {
      for (const endpoint of endpoints) {
        if (endpoint.type === 'external') {
          const isHealthy = await checkEndpointHealth(endpoint.url);
          endpoint.healthy = isHealthy;
          healthChecks.push({
            name: `${endpoint.type}-endpoint`,
            status: isHealthy ? 'pass' : 'fail',
            message: `${endpoint.url}:${endpoint.port}`,
          });
        }
      }
    }

    // Determine overall health status
    const allHealthy = healthChecks.every((check) => check.status === 'pass');
    const overallStatus =
      health.ready && (healthChecks.length === 0 || allHealthy)
        ? 'healthy'
        : health.ready
          ? 'unhealthy'
          : 'unknown';

    // Update session with verification results using standardized helper
    const updateResult = await updateSession(
      sessionId,
      {
        verification_result: {
          success: true,
          namespace,
          deploymentName,
          serviceName,
          endpoints,
          ready: health.ready,
          replicas: health.totalReplicas,
          status: {
            readyReplicas: health.readyReplicas,
            totalReplicas: health.totalReplicas,
            conditions: [
              {
                type: 'Available',
                status: health.ready ? 'True' : 'False',
                message: health.message,
              },
            ],
          },
          healthCheck: {
            status: overallStatus,
            message: health.message,
            checks: healthChecks,
          },
        },
        completed_steps: [...(session.completed_steps || []), 'verify-deployment'],
      },
      context,
    );

    if (!updateResult.ok) {
      logger.warn(
        { error: updateResult.error },
        'Failed to update session, but verification succeeded',
      );
    }

    timer.end({ deploymentName, ready: health.ready, sessionId });
    logger.info(
      {
        sessionId,
        deploymentName,
        namespace,
        ready: health.ready,
        healthStatus: overallStatus,
      },
      'Kubernetes deployment verification completed',
    );

    const result: VerifyDeploymentResult = {
      success: true,
      sessionId,
      namespace,
      deploymentName,
      serviceName,
      endpoints: endpoints as Array<{
        type: 'internal' | 'external';
        url: string;
        port: number;
        healthy?: boolean;
      }>,
      ready: health.ready,
      replicas: health.totalReplicas,
      status: {
        readyReplicas: health.readyReplicas,
        totalReplicas: health.totalReplicas,
        conditions: [
          {
            type: 'Available',
            status: health.ready ? 'True' : 'False',
            message: health.message,
          },
        ],
      },
      healthCheck: {
        status: overallStatus,
        message: health.message,
        ...(healthChecks.length > 0 && { checks: healthChecks }),
      },
    };

    // Add chain hint based on verification status
    const enrichedResult = {
      ...result,
      _chainHint:
        health.ready && overallStatus === 'healthy'
          ? 'Deployment verified successfully! Your application is running.'
          : overallStatus === 'healthy'
            ? 'Deployment is starting up. Wait and verify again, or check logs for issues.'
            : 'Deployment has issues. Check healthCheck details and pod logs for troubleshooting.',
    };

    return Success(enrichedResult);
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'Deployment verification failed');

    return Failure(error instanceof Error ? error.message : String(error));
  }
}

/**
 * Verify deployment tool
 */
export const verifyDeployment = verifyDeploymentImpl;
