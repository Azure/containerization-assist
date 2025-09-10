/**
 * Deploy Application Tool - Standardized Implementation
 *
 * Deploys applications to Kubernetes clusters using standardized helpers
 * for consistency and improved error handling
 *
 * @example
 * ```typescript
 * const result = await deployApplication({
 *   sessionId: 'session-123',
 *   namespace: 'my-app',
 *   environment: 'production'
 * }, context, logger);
 *
 * if (result.success) {
 *   logger.info('Application deployed', {
 *     deployment: result.deploymentName,
 *     endpoints: result.endpoints
 *   });
 * }
 * ```
 */

import * as yaml from 'js-yaml';
import { getSession, updateSession } from '@mcp/tools/session-helpers';
import type { ToolContext } from '../../mcp/context/types';
import { createKubernetesClient } from '../../lib/kubernetes';
import { createTimer, createLogger } from '../../lib/logger';
import { Success, Failure, type Result } from '../../domain/types';
import { DEFAULT_TIMEOUTS } from '../../config/defaults';
import type { DeployApplicationParams } from './schema';

export interface DeployApplicationResult {
  success: boolean;
  sessionId: string;
  namespace: string;
  deploymentName: string;
  serviceName: string;
  endpoints: Array<{
    type: 'internal' | 'external';
    url: string;
    port: number;
  }>;
  ready: boolean;
  replicas: number;
  status?: {
    readyReplicas: number;
    totalReplicas: number;
    conditions: Array<{
      type: string;
      status: string;
      message: string;
    }>;
  };
}

/**
 * Parse YAML/JSON manifest content
 */
function parseManifest(content: string): unknown[] {
  try {
    // Try parsing as JSON first
    const parsed = JSON.parse(content);
    return Array.isArray(parsed) ? parsed : [parsed];
  } catch {
    // Parse YAML documents (supports multi-document YAML)
    const documents = yaml.loadAll(content);
    return documents.filter((doc) => doc !== null && doc !== undefined);
  }
}

/**
 * Order manifests for deployment (ConfigMaps/Secrets first, then Services, then Deployments)
 */
function orderManifests(manifests: unknown[]): unknown[] {
  const order = [
    'Namespace',
    'ConfigMap',
    'Secret',
    'Service',
    'Deployment',
    'Ingress',
    'HorizontalPodAutoscaler',
  ];

  return manifests.sort((a, b) => {
    const aObj = a as { kind?: string };
    const bObj = b as { kind?: string };
    const aIndex = aObj.kind && order.indexOf(aObj.kind) !== -1 ? order.indexOf(aObj.kind) : 999;
    const bIndex = bObj.kind && order.indexOf(bObj.kind) !== -1 ? order.indexOf(bObj.kind) : 999;
    return aIndex - bIndex;
  });
}

/**
 * Core deployment implementation
 */
async function deployApplicationImpl(
  params: DeployApplicationParams,
  context: ToolContext,
): Promise<Result<DeployApplicationResult>> {
  const logger = context.logger || createLogger({ name: 'deploy-application' });
  const timer = createTimer(logger, 'deploy-application');

  try {
    const { namespace = 'default', replicas = 1, environment = 'development' } = params;

    const cluster = 'default';
    const dryRun = false;
    const wait = true;
    const timeout = 300;

    logger.info({ namespace, cluster, dryRun, environment }, 'Starting application deployment');

    // Get session using standardized helper
    const sessionResult = await getSession(params.sessionId, context);

    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }

    const { id: sessionId, state: session } = sessionResult.value;
    logger.info({ sessionId, namespace, environment }, 'Starting Kubernetes deployment');

    const k8sClient = createKubernetesClient(logger);

    // Get K8s manifests from session
    const sessionState = session as { k8s_manifests?: { manifests?: string } } | null | undefined;
    const k8sManifests = sessionState?.k8s_manifests;
    if (!k8sManifests?.manifests) {
      return Failure(
        'No Kubernetes manifests found in session. Please run generate-k8s-manifests tool first.',
      );
    }

    // Parse manifests
    const manifests = parseManifest(k8sManifests.manifests);
    if (manifests.length === 0) {
      return Failure('No valid manifests found in session');
    }

    // Order manifests for deployment
    const orderedManifests = orderManifests(manifests);

    logger.info(
      { manifestCount: orderedManifests.length, dryRun, namespace },
      'Deploying manifests to Kubernetes',
    );

    // Deploy manifests
    const deployedResources: Array<{ kind: string; name: string; namespace: string }> = [];

    if (!dryRun) {
      for (let i = 0; i < orderedManifests.length; i++) {
        const manifest = orderedManifests[i];
        try {
          const manifestObj = manifest as {
            kind?: string;
            metadata?: { name?: string; namespace?: string };
          };
          // Apply manifest using K8s client
          const applyResult = await k8sClient.applyManifest(manifest, namespace);
          if (!applyResult.ok) {
            logger.warn(
              {
                kind: manifestObj.kind,
                name: manifestObj.metadata?.name,
                error: applyResult.error,
              },
              'Failed to apply manifest',
            );
            continue;
          }

          deployedResources.push({
            kind: manifestObj.kind ?? 'unknown',
            name: manifestObj.metadata?.name ?? 'unknown',
            namespace: manifestObj.metadata?.namespace ?? namespace,
          });

          logger.debug(
            {
              kind: manifestObj.kind,
              name: manifestObj.metadata?.name,
            },
            'Deployed resource',
          );
        } catch (error) {
          const manifestObj = manifest as { kind?: string; metadata?: { name?: string } };
          logger.warn(
            {
              kind: manifestObj.kind,
              name: manifestObj.metadata?.name,
              error,
            },
            'Failed to deploy resource',
          );
        }
      }
    }

    // Find deployment and service info
    const deployment = orderedManifests.find(
      (m) => (m as { kind?: string }).kind === 'Deployment',
    ) as { metadata?: { name?: string }; spec?: { replicas?: number } } | undefined;
    const service = orderedManifests.find((m) => (m as { kind?: string }).kind === 'Service') as
      | { metadata?: { name?: string }; spec?: { ports?: Array<{ port?: number }>; type?: string } }
      | undefined;

    const deploymentName = deployment?.metadata?.name ?? 'app';
    const serviceName = service?.metadata?.name ?? deploymentName;

    // Wait for deployment to be ready
    let ready = false;
    let readyReplicas = 0;
    const totalReplicas = deployment?.spec?.replicas ?? replicas;

    if (wait && !dryRun) {
      // Wait for deployment with configurable retry delay
      logger.info({ deploymentName, timeout }, 'Waiting for deployment to be ready');
      const startTime = Date.now();
      const retryDelay = DEFAULT_TIMEOUTS.deploymentPoll || 5000;
      while (Date.now() - startTime < timeout * 1000) {
        const statusResult = await k8sClient.getDeploymentStatus(namespace, deploymentName);
        if (statusResult.ok && statusResult.value?.ready) {
          ready = true;
          readyReplicas = statusResult.value?.readyReplicas || 0;
          logger.info({ deploymentName, readyReplicas }, 'Deployment is ready');
          break;
        }
        // Wait before checking again using configured delay
        await new Promise((resolve) => setTimeout(resolve, retryDelay));
      }
    } else if (dryRun) {
      // For dry runs, mark as ready
      ready = true;
      readyReplicas = totalReplicas;
    }

    // Build endpoints
    const endpoints: Array<{ type: 'internal' | 'external'; url: string; port: number }> = [];

    if (service) {
      const port = service.spec?.ports?.[0]?.port ?? 80;

      // Internal endpoint
      endpoints.push({
        type: 'internal',
        url: `http://${serviceName}.${namespace}.svc.cluster.local`,
        port,
      });

      // External endpoint if LoadBalancer or Ingress
      if (service.spec?.type === 'LoadBalancer') {
        endpoints.push({
          type: 'external',
          url: `http://pending-loadbalancer`,
          port,
        });
      }
    }

    // Check for ingress
    const ingress = orderedManifests.find((m) => (m as { kind?: string }).kind === 'Ingress') as
      | { spec?: { rules?: Array<{ host?: string }> } }
      | undefined;
    if (ingress) {
      const host = ingress.spec?.rules?.[0]?.host ?? 'app.example.com';
      endpoints.push({
        type: 'external',
        url: `http://${host}`,
        port: 80,
      });
    }

    // Update session with deployment result using standardized helper
    const updateResult = await updateSession(
      sessionId,
      {
        deployment_result: {
          success: true,
          namespace,
          deploymentName,
          serviceName,
          endpoints,
          ready,
          replicas: totalReplicas,
          status: {
            readyReplicas,
            totalReplicas,
            conditions: [
              {
                type: 'Available',
                status: ready ? 'True' : 'False',
                message: ready ? 'Deployment is available' : 'Deployment is pending',
              },
            ],
          },
        },
        completed_steps: [...(session.completed_steps || []), 'deploy'],
      },
      context,
    );

    if (!updateResult.ok) {
      logger.warn(
        { error: updateResult.error },
        'Failed to update session, but deployment succeeded',
      );
    }

    timer.end({ deploymentName, ready, sessionId });
    logger.info(
      { sessionId, deploymentName, serviceName, ready, namespace },
      'Kubernetes deployment completed',
    );

    return Success({
      success: true,
      sessionId,
      namespace,
      deploymentName,
      serviceName,
      endpoints,
      ready,
      replicas: totalReplicas,
      status: {
        readyReplicas,
        totalReplicas,
        conditions: [
          {
            type: 'Available',
            status: ready ? 'True' : 'False',
            message: ready ? 'Deployment is available' : 'Deployment is pending',
          },
        ],
      },
      _chainHint: ready
        ? 'Next: verify_deployment to confirm app is working correctly'
        : 'Deployment in progress. Wait and run verify_deployment to check status',
    });
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'Application deployment failed');

    return Failure(error instanceof Error ? error.message : String(error));
  }
}

/**
 * Export the deploy tool directly
 */
export const deployApplication = deployApplicationImpl;
