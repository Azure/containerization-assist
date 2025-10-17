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

import { getToolLogger, createToolTimer } from '@/lib/tool-helpers';
import { extractErrorMessage } from '@/lib/error-utils';
import type { ToolContext } from '@/mcp/context';
import { createKubernetesClient, type KubernetesClient } from '@/infra/kubernetes/client';

import { DEFAULT_TIMEOUTS } from '@/config/defaults';
import { Success, Failure, type Result } from '@/types';
import { verifyDeploymentSchema, type VerifyDeploymentParams } from './schema';

export interface VerifyDeploymentResult extends Record<string, unknown> {
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
 * Discover service endpoints
 */
async function discoverServiceEndpoints(
  k8sClient: KubernetesClient,
  namespace: string,
  logger: ReturnType<typeof getToolLogger>,
  options: { includeClusterIP?: boolean } = {},
): Promise<
  Result<Array<{ type: 'internal' | 'external'; url: string; port: number; serviceName: string }>>
> {
  const endpoints: Array<{
    type: 'internal' | 'external';
    url: string;
    port: number;
    serviceName: string;
  }> = [];

  try {
    const servicesResult = await k8sClient.listServices(namespace);

    if (!servicesResult.ok) {
      return servicesResult;
    }

    for (const service of servicesResult.value) {
      const serviceName = service.metadata?.name;
      const serviceType = service.spec?.type || 'ClusterIP';

      if (!serviceName) continue;

      // Handle different service types
      switch (serviceType) {
        case 'LoadBalancer': {
          const lbIngress = service.status?.loadBalancer?.ingress || [];
          for (const ing of lbIngress) {
            if (ing.ip) {
              const ports = service.spec?.ports || [];
              for (const port of ports) {
                if (port.port) {
                  endpoints.push({
                    type: 'external',
                    url: `http://${ing.ip}`,
                    port: port.port,
                    serviceName,
                  });
                }
              }
            } else if (ing.hostname) {
              const ports = service.spec?.ports || [];
              for (const port of ports) {
                if (port.port) {
                  endpoints.push({
                    type: 'external',
                    url: `http://${ing.hostname}`,
                    port: port.port,
                    serviceName,
                  });
                }
              }
            }
          }
          break;
        }

        case 'NodePort': {
          const ports = service.spec?.ports || [];
          for (const port of ports) {
            if (port.nodePort) {
              endpoints.push({
                type: 'external',
                url: '<node-ip>',
                port: port.nodePort,
                serviceName,
              });
            }
          }
          break;
        }

        case 'ClusterIP': {
          if (options.includeClusterIP) {
            const clusterIP = service.spec?.clusterIP;
            const ports = service.spec?.ports || [];
            for (const port of ports) {
              if (clusterIP && clusterIP !== 'None' && port.port) {
                endpoints.push({
                  type: 'internal',
                  url: `http://${clusterIP}`,
                  port: port.port,
                  serviceName,
                });
              }
            }
          }
          break;
        }
      }
    }

    logger.debug({ count: endpoints.length }, 'Service endpoints discovered');
    return Success(endpoints);
  } catch (error) {
    return Failure(`Failed to discover service endpoints: ${extractErrorMessage(error)}`);
  }
}

/**
 * Discover ingress endpoints
 */
async function discoverIngressEndpoints(
  k8sClient: KubernetesClient,
  namespace: string,
  logger: ReturnType<typeof getToolLogger>,
): Promise<
  Result<Array<{ type: 'internal' | 'external'; url: string; port: number; ingressName: string }>>
> {
  const endpoints: Array<{
    type: 'internal' | 'external';
    url: string;
    port: number;
    ingressName: string;
  }> = [];

  try {
    const ingressesResult = await k8sClient.listIngresses(namespace);

    if (!ingressesResult.ok) {
      return ingressesResult;
    }

    for (const ingress of ingressesResult.value) {
      const ingressName = ingress.metadata?.name || 'unknown';
      const rules = ingress.spec?.rules || [];

      for (const rule of rules) {
        const host = rule.host || '<host>';
        const paths = rule.http?.paths || [];

        for (const path of paths) {
          const pathStr = path.path || '/';

          // Check if TLS is configured
          const tls = ingress.spec?.tls || [];
          const hasTLS = tls.some((t) => t.hosts?.includes(host));
          const protocol = hasTLS ? 'https' : 'http';
          const port = hasTLS ? 443 : 80;

          endpoints.push({
            type: 'external',
            url: `${protocol}://${host}${pathStr}`,
            port,
            ingressName,
          });
        }
      }

      // Also check load balancer status
      const lbIngress = ingress.status?.loadBalancer?.ingress || [];
      for (const ing of lbIngress) {
        if (ing.ip) {
          endpoints.push({
            type: 'external',
            url: `http://${ing.ip}`,
            port: 80,
            ingressName,
          });
        } else if (ing.hostname) {
          endpoints.push({
            type: 'external',
            url: `http://${ing.hostname}`,
            port: 80,
            ingressName,
          });
        }
      }
    }

    logger.debug({ count: endpoints.length }, 'Ingress endpoints discovered');
    return Success(endpoints);
  } catch (error) {
    return Failure(`Failed to discover ingress endpoints: ${extractErrorMessage(error)}`);
  }
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
  params: VerifyDeploymentParams,
  context: ToolContext,
): Promise<Result<VerifyDeploymentResult>> {
  if (!params || typeof params !== 'object') {
    return Failure('Invalid parameters provided');
  }
  const logger = getToolLogger(context, 'verify-deploy');
  const timer = createToolTimer(logger, 'verify-deploy');

  const {
    deploymentName: configDeploymentName,
    namespace: configNamespace,
    checks = ['pods', 'services', 'health'],
  } = params;

  const timeout = 60;

  try {
    logger.info({ checks }, 'Starting Kubernetes deployment verification');

    const k8sClient = createKubernetesClient(logger);

    if (!configDeploymentName) {
      return Failure('Deployment name is required. Provide deploymentName parameter.');
    }

    const namespace = configNamespace ?? 'default';
    const deploymentName = configDeploymentName;
    const serviceName = deploymentName;
    const endpoints: Array<{ type: string; url: string; port: number; healthy?: boolean }> = [];

    logger.info({ namespace, deploymentName }, 'Checking deployment health');

    // Check deployment health
    const health = await checkDeploymentHealth(k8sClient, namespace, deploymentName, timeout);

    // Discover service endpoints if requested
    if (checks.includes('services')) {
      logger.info('Discovering service endpoints...');
      const serviceEndpointsResult = await discoverServiceEndpoints(
        k8sClient,
        namespace,
        logger,
        { includeClusterIP: true },
      );

      if (serviceEndpointsResult.ok) {
        for (const endpoint of serviceEndpointsResult.value) {
          endpoints.push({
            type: endpoint.type,
            url: endpoint.url,
            port: endpoint.port,
          });
        }
        logger.info({ count: serviceEndpointsResult.value.length }, 'Service endpoints discovered');
      } else {
        logger.warn({ error: serviceEndpointsResult.error }, 'Failed to discover service endpoints');
      }
    }

    // Discover ingress endpoints if requested
    if (checks.includes('ingress')) {
      logger.info('Discovering ingress endpoints...');
      const ingressEndpointsResult = await discoverIngressEndpoints(k8sClient, namespace, logger);

      if (ingressEndpointsResult.ok) {
        for (const endpoint of ingressEndpointsResult.value) {
          endpoints.push({
            type: endpoint.type,
            url: endpoint.url,
            port: endpoint.port,
          });
        }
        logger.info({ count: ingressEndpointsResult.value.length }, 'Ingress endpoints discovered');
      } else {
        logger.warn({ error: ingressEndpointsResult.error }, 'Failed to discover ingress endpoints');
      }
    }

    // Initialize health checks
    const healthChecks: Array<{ name: string; status: 'pass' | 'fail'; message?: string }> = [];

    // Check each endpoint if 'health' is in checks
    if (checks.includes('health')) {
      for (const endpoint of endpoints) {
        if (endpoint.type === 'external') {
          // Construct full URL with port if not already included
          let fullUrl = endpoint.url;
          if (
            !fullUrl.includes('<node-ip>') &&
            !fullUrl.includes('<host>') &&
            !fullUrl.match(/:\d+$/)
          ) {
            // Add port if not default HTTP/HTTPS port
            if (endpoint.port !== 80 && endpoint.port !== 443) {
              fullUrl = `${endpoint.url}:${endpoint.port}`;
            }
          }

          const isHealthy = await checkEndpointHealth(fullUrl);
          endpoint.healthy = isHealthy;
          healthChecks.push({
            name: `${endpoint.type}-endpoint`,
            status: isHealthy ? 'pass' : 'fail',
            message: fullUrl,
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

    // Prepare the result
    const result: VerifyDeploymentResult = {
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
          ? `Deployment verification successful. Use "ops" for operational tasks, or review the deployment status.`
          : `Deployment verification found issues. Check deployment status and pod logs to diagnose issues.`,
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

    return Failure(extractErrorMessage(error));
  }
}

import { tool } from '@/types/tool';

export default tool({
  name: 'verify-deploy',
  description: 'Verify Kubernetes deployment status',
  category: 'kubernetes',
  version: '2.0.0',
  schema: verifyDeploymentSchema,
  metadata: {
    knowledgeEnhanced: false,
  },
  handler: handleVerifyDeployment,
});
