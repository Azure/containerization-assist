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

import { ensureSession, defineToolIO, useSessionSlice } from '@mcp/tool-session-helpers';
import { getToolLogger, createToolTimer } from '@lib/tool-helpers';
import { extractErrorMessage } from '@lib/error-utils';
import type { ToolContext } from '@mcp/context';
import { createKubernetesClient, KubernetesClient } from '@lib/kubernetes';

import { DEFAULT_TIMEOUTS } from '@config/defaults';
import { Success, Failure, type Result } from '@types';
import { verifyDeploymentSchema, type VerifyDeploymentParams } from './schema';
import { z } from 'zod';
import type { SessionData } from '@tools/session-types';

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

// Define the result schema for type safety
const VerifyDeploymentResultSchema = z.object({
  success: z.boolean(),
  sessionId: z.string(),
  namespace: z.string(),
  deploymentName: z.string(),
  serviceName: z.string(),
  endpoints: z.array(
    z.object({
      type: z.enum(['internal', 'external']),
      url: z.string(),
      port: z.number(),
      healthy: z.boolean().optional(),
    }),
  ),
  ready: z.boolean(),
  replicas: z.number(),
  status: z.object({
    readyReplicas: z.number(),
    totalReplicas: z.number(),
    conditions: z.array(
      z.object({
        type: z.string(),
        status: z.string(),
        message: z.string(),
      }),
    ),
  }),
  healthCheck: z
    .object({
      status: z.enum(['healthy', 'unhealthy', 'unknown']),
      message: z.string(),
      checks: z
        .array(
          z.object({
            name: z.string(),
            status: z.enum(['pass', 'fail']),
            message: z.string().optional(),
          }),
        )
        .optional(),
    })
    .optional(),
});

// Define tool IO for type-safe session operations
const io = defineToolIO(verifyDeploymentSchema, VerifyDeploymentResultSchema);

// Tool-specific state schema
const StateSchema = z.object({
  lastVerifiedAt: z.date().optional(),
  lastVerifiedDeployment: z.string().optional(),
  lastNamespace: z.string().optional(),
  verificationsPassed: z.number().optional(),
  lastHealthStatus: z.enum(['healthy', 'unhealthy', 'unknown']).optional(),
});

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
  const logger = getToolLogger(context, 'verify-deployment');
  const timer = createToolTimer(logger, 'verify-deployment');

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

    // Ensure session exists and get typed slice operations
    const sessionResult = await ensureSession(context, params.sessionId);
    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }

    const { id: sessionId, state: session } = sessionResult.value;
    const slice = useSessionSlice('verify-deployment', io, context, StateSchema);

    if (!slice) {
      return Failure('Session manager not available');
    }

    logger.info({ sessionId, checks }, 'Starting Kubernetes deployment verification with session');

    // Record input in session slice
    await slice.patch(sessionId, { input: params });

    const k8sClient = createKubernetesClient(logger);

    // Get deployment info from session or config
    const sessionData = session as SessionData;
    const deploymentResult = sessionData?.deployment_result as
      | {
          namespace?: string;
          deploymentName?: string;
          serviceName?: string;
          endpoints?: Array<{ type: string; url: string; port: number; healthy?: boolean }>;
        }
      | undefined;
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

    // Prepare the result
    const result: VerifyDeploymentResult = {
      success: overallStatus === 'healthy',
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

    // Update typed session slice with output and state
    await slice.patch(sessionId, {
      output: result,
      state: {
        lastVerifiedAt: new Date(),
        lastVerifiedDeployment: deploymentName,
        lastNamespace: namespace,
        verificationsPassed:
          (session.completed_steps || []).filter((s) => s === 'verify-deployment').length + 1,
        lastHealthStatus: overallStatus,
      },
    });

    timer.end({ deploymentName, ready: health.ready, sessionId });

    if (overallStatus === 'healthy') {
      logger.info(
        {
          sessionId,
          deploymentName,
          namespace,
          ready: health.ready,
          healthStatus: overallStatus,
        },
        'Kubernetes deployment verification successful - deployment is healthy',
      );
    } else {
      logger.warn(
        {
          sessionId,
          deploymentName,
          namespace,
          ready: health.ready,
          healthStatus: overallStatus,
          healthChecks: healthChecks.length > 0 ? healthChecks : undefined,
        },
        `Kubernetes deployment verification found issues - status: ${overallStatus}`,
      );
    }

    const enrichedResult = {
      ...result,
    };

    return Success(enrichedResult);
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'Deployment verification failed');

    return Failure(extractErrorMessage(error));
  }
}

/**
 * Verify deployment tool
 */
export const verifyDeployment = verifyDeploymentImpl;
