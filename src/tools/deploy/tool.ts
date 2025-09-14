/**
 * Deploy Application Tool - Standardized Implementation
 *
 * Deploys applications to Kubernetes clusters using standardized helpers
 * for consistency and improved error handling
 * @example
 * ```typescript
 * const result = await deployApplication({
 *   sessionId: 'session-123',
 *   namespace: 'my-app',
 *   environment: 'production'
 * }, context, logger);
 * if (result.success) {
 *   logger.info('Application deployed', {
 *     deployment: result.deploymentName,
 *     endpoints: result.endpoints
 *   });
 * }
 * ```
 */

import * as yaml from 'js-yaml';
import { getToolLogger, createToolTimer } from '@lib/tool-helpers';
import type { Logger } from '../../lib/logger';
import { extractErrorMessage } from '../../lib/error-utils';
import { ensureSession, defineToolIO, useSessionSlice } from '@mcp/tool-session-helpers';
import type { ToolContext } from '../../mcp/context';
import { createKubernetesClient } from '../../lib/kubernetes';

import { Success, Failure, type Result } from '../../types';
import { DEFAULT_TIMEOUTS } from '../../config/defaults';
import { deployApplicationSchema, type DeployApplicationParams } from './schema';
import { z } from 'zod';
import type { SessionData } from '../session-types';

// Type definitions for Kubernetes manifests
interface KubernetesManifest {
  kind?: string;
  metadata?: {
    name?: string;
    namespace?: string;
  };
}

interface DeploymentManifest extends KubernetesManifest {
  kind: 'Deployment';
  spec?: {
    replicas?: number;
  };
}

interface ServiceManifest extends KubernetesManifest {
  kind: 'Service';
  spec?: {
    ports?: Array<{ port?: number; targetPort?: number; nodePort?: number }>;
    type?: 'ClusterIP' | 'NodePort' | 'LoadBalancer';
  };
}

interface IngressManifest extends KubernetesManifest {
  kind: 'Ingress';
  spec?: {
    rules?: Array<{ host?: string; http?: unknown }>;
  };
}

// Configuration constants
const DEPLOYMENT_CONFIG = {
  DEFAULT_NAMESPACE: 'default',
  DEFAULT_REPLICAS: 1,
  DEFAULT_ENVIRONMENT: 'development',
  DEFAULT_CLUSTER: 'default',
  DEFAULT_PORT: 80,
  WAIT_TIMEOUT_SECONDS: 300,
  DRY_RUN: false,
  WAIT_FOR_READY: true,
  PENDING_LB_URL: 'http://pending-loadbalancer',
  DEFAULT_INGRESS_HOST: 'app.example.com',
} as const;

// Manifest deployment order for proper resource creation
const MANIFEST_ORDER = [
  'Namespace',
  'ConfigMap',
  'Secret',
  'PersistentVolume',
  'PersistentVolumeClaim',
  'ServiceAccount',
  'Role',
  'RoleBinding',
  'ClusterRole',
  'ClusterRoleBinding',
  'Service',
  'Deployment',
  'StatefulSet',
  'DaemonSet',
  'Job',
  'CronJob',
  'Ingress',
  'HorizontalPodAutoscaler',
  'VerticalPodAutoscaler',
  'NetworkPolicy',
] as const;

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
  chainHint?: string; // Hint for next tool in workflow chain
}

// Define the result schema for type safety
const DeployApplicationResultSchema = z.object({
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
    }),
  ),
  ready: z.boolean(),
  replicas: z.number(),
  status: z
    .object({
      readyReplicas: z.number(),
      totalReplicas: z.number(),
      conditions: z.array(
        z.object({
          type: z.string(),
          status: z.string(),
          message: z.string(),
        }),
      ),
    })
    .optional(),
  chainHint: z.string().optional(),
});

// Define tool IO for type-safe session operations
const io = defineToolIO(deployApplicationSchema, DeployApplicationResultSchema);

// Tool-specific state schema
const StateSchema = z.object({
  lastDeployedAt: z.date().optional(),
  lastDeployedNamespace: z.string().optional(),
  lastDeploymentName: z.string().optional(),
  lastServiceName: z.string().optional(),
  totalDeployments: z.number().optional(),
  lastDeploymentReady: z.boolean().optional(),
  lastEndpointCount: z.number().optional(),
});

/**
 * Parse YAML/JSON manifest content with validation
 */
function parseManifest(content: string, logger: Logger): KubernetesManifest[] {
  try {
    // Try parsing as JSON first
    const parsed = JSON.parse(content);
    const manifests = Array.isArray(parsed) ? parsed : [parsed];
    return validateManifests(manifests, logger);
  } catch {
    // Parse YAML documents (supports multi-document YAML)
    try {
      const documents = yaml.loadAll(content);
      const filtered = documents.filter((doc) => doc !== null && doc !== undefined);
      return validateManifests(filtered, logger);
    } catch (yamlError) {
      logger.error({ error: yamlError }, 'Failed to parse manifests as YAML');
      throw new Error(`Invalid manifest format: ${extractErrorMessage(yamlError)}`);
    }
  }
}

/**
 * Validate manifests have required structure
 */
function validateManifests(manifests: unknown[], logger: Logger): KubernetesManifest[] {
  const validated: KubernetesManifest[] = [];

  for (const manifest of manifests) {
    if (!manifest || typeof manifest !== 'object') {
      logger.warn({ manifest }, 'Skipping invalid manifest: not an object');
      continue;
    }

    const m = manifest as KubernetesManifest;
    if (!m.kind) {
      logger.warn({ manifest }, 'Skipping manifest without kind');
      continue;
    }

    if (!m.metadata?.name) {
      logger.warn({ kind: m.kind }, 'Manifest missing metadata.name');
    }

    validated.push(m);
  }

  return validated;
}

/**
 * Order manifests for deployment based on resource dependencies
 */
function orderManifests(manifests: KubernetesManifest[]): KubernetesManifest[] {
  return manifests.sort((a, b) => {
    const aIndex =
      a.kind && MANIFEST_ORDER.includes(a.kind as any)
        ? MANIFEST_ORDER.indexOf(a.kind as any)
        : 999;
    const bIndex =
      b.kind && MANIFEST_ORDER.includes(b.kind as any)
        ? MANIFEST_ORDER.indexOf(b.kind as any)
        : 999;
    return aIndex - bIndex;
  });
}

/**
 * Find manifest by kind with type safety
 */
function findManifestByKind<T extends KubernetesManifest>(
  manifests: KubernetesManifest[],
  kind: string,
): T | undefined {
  return manifests.find((m) => m.kind === kind) as T | undefined;
}

/**
 * Deploy a single manifest with error recovery
 */
async function deployManifest(
  manifest: KubernetesManifest,
  namespace: string,
  k8sClient: ReturnType<typeof createKubernetesClient>,
  logger: Logger,
): Promise<{ success: boolean; resource?: { kind: string; name: string; namespace: string } }> {
  const { kind = 'unknown', metadata } = manifest;
  const name = metadata?.name ?? 'unknown';

  try {
    const applyResult = await k8sClient.applyManifest(manifest, namespace);

    if (!applyResult.ok) {
      logger.error({ kind, name, error: applyResult.error }, 'Failed to apply manifest');
      return { success: false };
    }

    logger.info({ kind, name }, 'Successfully deployed resource');

    return {
      success: true,
      resource: {
        kind,
        name,
        namespace: metadata?.namespace ?? namespace,
      },
    };
  } catch (error) {
    logger.error({ kind, name, error: extractErrorMessage(error) }, 'Exception deploying resource');
    return { success: false };
  }
}

/**
 * Core deployment implementation
 */
async function deployApplicationImpl(
  params: DeployApplicationParams,
  context: ToolContext,
): Promise<Result<DeployApplicationResult>> {
  const logger = getToolLogger(context, 'deploy');
  const timer = createToolTimer(logger, 'deploy');
  try {
    const {
      namespace = DEPLOYMENT_CONFIG.DEFAULT_NAMESPACE,
      replicas = DEPLOYMENT_CONFIG.DEFAULT_REPLICAS,
      environment = DEPLOYMENT_CONFIG.DEFAULT_ENVIRONMENT,
    } = params;

    const cluster = DEPLOYMENT_CONFIG.DEFAULT_CLUSTER;
    const dryRun = DEPLOYMENT_CONFIG.DRY_RUN;
    const wait = DEPLOYMENT_CONFIG.WAIT_FOR_READY;
    const timeout = DEPLOYMENT_CONFIG.WAIT_TIMEOUT_SECONDS;
    logger.info({ namespace, cluster, dryRun, environment }, 'Starting application deployment');

    // Ensure session exists and get typed slice operations
    const sessionResult = await ensureSession(context, params.sessionId);
    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }

    const { id: sessionId, state: session } = sessionResult.value;
    const slice = useSessionSlice('deploy', io, context, StateSchema);

    if (!slice) {
      return Failure('Session manager not available');
    }

    logger.info(
      { sessionId, namespace, environment },
      'Starting Kubernetes deployment with session',
    );

    // Record input in session slice
    await slice.patch(sessionId, { input: params });
    const k8sClient = createKubernetesClient(logger);
    // Get K8s manifests from session with type safety
    const sessionData = session as SessionData;
    const k8sResult = sessionData?.k8s_result;

    if (!k8sResult?.manifests) {
      return Failure(
        'No Kubernetes manifests found in session. Please run generate-k8s-manifests tool first.',
      );
    }

    // Parse and validate manifests
    let manifests: KubernetesManifest[];
    try {
      // Extract content from manifests and join them
      const manifestContents = k8sResult.manifests
        .map((m) => m.content)
        .filter((content): content is string => Boolean(content))
        .join('\n---\n');

      if (!manifestContents) {
        return Failure('No valid manifest content found in session');
      }

      manifests = parseManifest(manifestContents, logger);
    } catch (error) {
      return Failure(`Failed to parse manifests: ${extractErrorMessage(error)}`);
    }

    if (manifests.length === 0) {
      return Failure('No valid manifests found in session');
    }
    // Order manifests for deployment
    const orderedManifests = orderManifests(manifests);
    logger.info(
      { manifestCount: orderedManifests.length, dryRun, namespace },
      'Deploying manifests to Kubernetes',
    );
    // Deploy manifests with proper error handling
    const deployedResources: Array<{ kind: string; name: string; namespace: string }> = [];
    const failedResources: Array<{ kind: string; name: string; error: string }> = [];

    if (!dryRun) {
      // Report progress
      logger.info({ totalManifests: orderedManifests.length }, 'Starting manifest deployment');

      for (let i = 0; i < orderedManifests.length; i++) {
        const manifest = orderedManifests[i];
        if (!manifest) continue; // Skip undefined entries

        const progress = `[${i + 1}/${orderedManifests.length}]`;

        logger.debug(
          {
            progress,
            kind: manifest.kind,
            name: manifest.metadata?.name,
          },
          'Deploying manifest',
        );

        const result = await deployManifest(manifest, namespace, k8sClient, logger);

        if (result.success && result.resource) {
          deployedResources.push(result.resource);
        } else {
          failedResources.push({
            kind: manifest.kind ?? 'unknown',
            name: manifest.metadata?.name ?? 'unknown',
            error: 'Deployment failed',
          });
        }
      }

      // Log deployment summary
      logger.info(
        {
          deployed: deployedResources.length,
          failed: failedResources.length,
          total: orderedManifests.length,
        },
        'Manifest deployment completed',
      );

      if (failedResources.length > 0) {
        logger.warn({ failedResources }, 'Some resources failed to deploy');
      }
    } else {
      // For dry run, simulate deployment
      logger.info('Dry run mode - simulating deployment');
      for (const manifest of orderedManifests) {
        deployedResources.push({
          kind: manifest.kind ?? 'unknown',
          name: manifest.metadata?.name ?? 'unknown',
          namespace: manifest.metadata?.namespace ?? namespace,
        });
      }
    }
    // Find deployment and service with type safety
    const deployment = findManifestByKind<DeploymentManifest>(orderedManifests, 'Deployment');
    const service = findManifestByKind<ServiceManifest>(orderedManifests, 'Service');
    const ingress = findManifestByKind<IngressManifest>(orderedManifests, 'Ingress');

    const deploymentName = deployment?.metadata?.name ?? 'app';
    const serviceName = service?.metadata?.name ?? deploymentName;
    // Wait for deployment to be ready
    let ready = false;
    let readyReplicas = 0;
    const totalReplicas = deployment?.spec?.replicas ?? replicas;
    if (wait && !dryRun) {
      // Wait for deployment with configurable retry delay
      logger.info(
        { deploymentName, timeoutSeconds: timeout },
        'Waiting for deployment to be ready',
      );

      const startTime = Date.now();
      const retryDelay = DEFAULT_TIMEOUTS.deploymentPoll || 5000;
      const maxWaitTime = timeout * 1000;
      let attempts = 0;

      while (Date.now() - startTime < maxWaitTime) {
        attempts++;
        const statusResult = await k8sClient.getDeploymentStatus(namespace, deploymentName);

        if (statusResult.ok && statusResult.value?.ready) {
          ready = true;
          readyReplicas = statusResult.value.readyReplicas || 0;
          logger.info(
            {
              deploymentName,
              readyReplicas,
              attempts,
              elapsedSeconds: Math.round((Date.now() - startTime) / 1000),
            },
            'Deployment is ready',
          );
          break;
        }

        // Log progress periodically
        if (attempts % 6 === 0) {
          // Every ~30 seconds at 5s intervals
          logger.debug(
            {
              deploymentName,
              attempt: attempts,
              elapsedSeconds: Math.round((Date.now() - startTime) / 1000),
              currentStatus: statusResult.ok ? statusResult.value : undefined,
            },
            'Still waiting for deployment',
          );
        }

        // Wait before checking again using configured delay
        await new Promise((resolve) => setTimeout(resolve, retryDelay));
      }

      if (!ready) {
        logger.warn(
          { deploymentName, timeoutSeconds: timeout },
          'Deployment did not become ready within timeout',
        );
      }
    } else if (dryRun) {
      // For dry runs, mark as ready
      ready = true;
      readyReplicas = totalReplicas;
    }
    // Build endpoints with proper configuration
    const endpoints: Array<{ type: 'internal' | 'external'; url: string; port: number }> = [];

    if (service) {
      const port = service.spec?.ports?.[0]?.port ?? DEPLOYMENT_CONFIG.DEFAULT_PORT;

      // Internal endpoint
      endpoints.push({
        type: 'internal',
        url: `http://${serviceName}.${namespace}.svc.cluster.local`,
        port,
      });

      // External endpoint if LoadBalancer
      if (service.spec?.type === 'LoadBalancer') {
        endpoints.push({
          type: 'external',
          url: DEPLOYMENT_CONFIG.PENDING_LB_URL,
          port,
        });
      }

      // NodePort endpoint
      if (service.spec?.type === 'NodePort') {
        const nodePort = service.spec?.ports?.[0]?.nodePort;
        if (nodePort) {
          endpoints.push({
            type: 'external',
            url: `http://<node-ip>`,
            port: nodePort,
          });
        }
      }
    }

    // Check for ingress
    if (ingress) {
      const host = ingress.spec?.rules?.[0]?.host ?? DEPLOYMENT_CONFIG.DEFAULT_INGRESS_HOST;
      endpoints.push({
        type: 'external',
        url: `http://${host}`,
        port: DEPLOYMENT_CONFIG.DEFAULT_PORT,
      });
    }

    // Prepare the result
    const result: DeployApplicationResult = {
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
    };

    // Update typed session slice with output and state
    await slice.patch(sessionId, {
      output: result,
      state: {
        lastDeployedAt: new Date(),
        lastDeployedNamespace: namespace,
        lastDeploymentName: deploymentName,
        lastServiceName: serviceName,
        totalDeployments:
          (sessionData?.completed_steps || []).filter((s) => s === 'deploy').length + 1,
        lastDeploymentReady: ready,
        lastEndpointCount: endpoints.length,
      },
    });
    timer.end({ deploymentName, ready, sessionId });
    logger.info(
      { sessionId, deploymentName, serviceName, ready, namespace },
      'Kubernetes deployment completed',
    );

    return Success(result);
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'Application deployment failed');
    return Failure(extractErrorMessage(error));
  }
}

/**
 * Export the deploy tool directly
 */
export const deployApplication = deployApplicationImpl;
