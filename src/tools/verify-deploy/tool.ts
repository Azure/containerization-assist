/**
 * Verify Deployment Tool - Standardized Implementation
 *
 * Verifies Kubernetes deployment health and retrieves endpoints using
 * standardized helpers for consistency and improved error handling
 *
 * This is a deterministic operational tool with no AI calls.
 *
 * @example
 * ```typescript
 * const result = await verifyDeployment({
 *   deploymentName: 'my-app',
 *   namespace: 'production',
 *   checks: ['pods', 'services', 'health']
 * }, context);
 *
 * if (result.success) {
 *   logger.info('Deployment verified', {
 *     ready: result.ready,
 *     endpoints: result.endpoints
 *   });
 * }
 * ```
 */

import { setupToolContext } from '@/lib/tool-context-helpers';
import { extractErrorMessage } from '@/lib/errors';
import type { ToolContext } from '@/core/context';
import { createKubernetesClient, type KubernetesClient } from '@/infra/kubernetes/client';

import { DEFAULT_TIMEOUTS } from '@/config/constants';
import { Success, Failure, type Result } from '@/types';
import { verifyDeploySchema, type VerifyDeployParams } from './schema';
import { buildStatusSummary } from '@/lib/summary-helpers';

export interface VerifyDeploymentResult extends Record<string, unknown> {
  /**
   * Natural language summary for user display.
   * 1-3 sentences describing the deployment verification outcome.
   * @example "✅ Deployment myapp is healthy. 3/3 pods ready. All health checks passing."
   */
  summary?: string;
  success: boolean;
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
  pods?: Array<{
    name: string;
    status: string;
    ready: boolean;
    healthy: boolean;
    restarts: number;
    age: string;
    port?: number;
  }>;
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
  workflowHints?: {
    nextStep: string;
    message: string;
  };
}

/**
 * Check deployment health using shared client method
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
  // Use shared waitForDeploymentReady from client
  const waitResult = await k8sClient.waitForDeploymentReady(namespace, deploymentName, timeout);

  if (waitResult.ok && waitResult.value?.ready) {
    return {
      ready: true,
      readyReplicas: waitResult.value.readyReplicas ?? 0,
      totalReplicas: waitResult.value.totalReplicas ?? 0,
      status: 'healthy',
      message: 'Deployment is healthy and ready',
    };
  }

  // If not ready, get current status
  const statusResult = await k8sClient.getDeploymentStatus(namespace, deploymentName);

  return {
    ready: false,
    readyReplicas: statusResult.ok ? (statusResult.value.readyReplicas ?? 0) : 0,
    totalReplicas: statusResult.ok ? (statusResult.value.totalReplicas ?? 1) : 1,
    status: 'unhealthy',
    message: !waitResult.ok ? waitResult.error : 'Deployment not ready',
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
async function handleVerifyDeployment(
  params: VerifyDeployParams,
  context: ToolContext,
): Promise<Result<VerifyDeploymentResult>> {
  if (!params || typeof params !== 'object') {
    return Failure('Invalid parameters provided', {
      message: 'Parameters must be a valid object',
      hint: 'Tool received invalid or missing parameters',
      resolution: 'Ensure parameters are provided as a JSON object',
    });
  }
  const { logger, timer } = setupToolContext(context, 'verify-deploy');

  const {
    deploymentName: configDeploymentName,
    namespace: configNamespace,
    checks = ['pods', 'services', 'health'],
  } = params;

  const timeout = Math.floor(DEFAULT_TIMEOUTS.verification / 1000); // Convert ms to seconds

  try {
    logger.info({ checks }, 'Starting Kubernetes deployment verification');

    const k8sClient = createKubernetesClient(logger);

    if (!configDeploymentName) {
      return Failure('Deployment name is required. Provide deploymentName parameter.', {
        message: 'Missing required parameter: deploymentName',
        hint: 'Deployment name must be specified to verify the deployment',
        resolution: 'Add deploymentName parameter with the name of the deployment to verify',
      });
    }

    const namespace = configNamespace ?? 'default';
    const deploymentName = configDeploymentName;
    const serviceName = deploymentName;
    const endpoints: Array<{ type: string; url: string; port: number; healthy?: boolean }> = [];

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

    // Determine success status
    const isSuccessful = overallStatus === 'healthy';

    // Generate summary
    const summary = buildStatusSummary(
      isSuccessful,
      `Deployment ${deploymentName} is healthy. ${health.readyReplicas}/${health.totalReplicas} pods ready. All health checks passing.`,
      `Deployment ${deploymentName} has issues. ${health.readyReplicas}/${health.totalReplicas} pods ready. Check pod logs for details.`,
    );

    // Prepare the result
    const result: VerifyDeploymentResult = {
      summary,
      success: isSuccessful,
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
      workflowHints: {
        nextStep: isSuccessful ? 'ops' : 'fix-deployment-issues',
        message: isSuccessful
          ? 'Deployment verified successfully. Use ops tool for operational tasks.'
          : 'Deployment verification found issues. Review the status and fix deployment problems.',
      },
    };

    logger.info(
      { deploymentName, ready: health.ready, status: overallStatus },
      'Verification complete',
    );

    timer.end({ deploymentName, ready: health.ready });

    return Success(result);
  } catch (error) {
    timer.error(error);

    return Failure(extractErrorMessage(error), {
      message: extractErrorMessage(error),
      hint: 'An unexpected error occurred during deployment verification',
      resolution: 'Check the error message for details. Verify the deployment exists, cluster is accessible, and you have proper permissions',
    });
  }
}

import { tool } from '@/types/tool';

export default tool({
  name: 'verify-deploy',
  description: 'Verify Kubernetes deployment status',
  category: 'kubernetes',
  version: '2.0.0',
  schema: verifyDeploySchema,
  metadata: {
    knowledgeEnhanced: false,
  },
  handler: handleVerifyDeployment,
});
