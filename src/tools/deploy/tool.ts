/**
 * Deploy Application Tool
 *
 * Deploys applications to Kubernetes clusters
 */

import * as yaml from 'js-yaml';
import { getToolLogger, createToolTimer } from '@/lib/tool-helpers';
import type { Logger } from '@/lib/logger';
import { extractErrorMessage } from '@/lib/error-utils';
import { validateNamespace } from '@/lib/validation';
import type { ToolContext } from '@/mcp/context';
import { createKubernetesClient, type K8sManifest } from '@/infra/kubernetes/client';

import { Success, Failure, type Result } from '@/types';
import { deployApplicationSchema, type DeployApplicationParams } from './schema';

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
      // In js-yaml v4, loadAll is safe by default (no code execution)
      const documents = yaml.loadAll(content);
      const filtered = documents.filter((doc: unknown) => doc !== null && doc !== undefined);
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
 * Check if a kind is in the manifest order
 */
function getManifestOrderIndex(kind: string | undefined): number {
  if (!kind) return 999;
  const index = MANIFEST_ORDER.indexOf(kind as (typeof MANIFEST_ORDER)[number]);
  return index >= 0 ? index : 999;
}

/**
 * Order manifests for deployment based on resource dependencies
 */
function orderManifests(manifests: KubernetesManifest[]): KubernetesManifest[] {
  return manifests.sort((a, b) => {
    const aIndex = getManifestOrderIndex(a.kind);
    const bIndex = getManifestOrderIndex(b.kind);
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
): Promise<
  Result<{
    kind: string;
    name: string;
    namespace: string;
    guidance?: import('@/types').ErrorGuidance;
  }>
> {
  const { kind = 'unknown', metadata } = manifest;
  const name = metadata?.name ?? 'unknown';

  try {
    const applyResult = await k8sClient.applyManifest(manifest as K8sManifest, namespace);

    if (!applyResult.ok) {
      logger.error(
        {
          kind,
          name,
          error: applyResult.error,
          hint: applyResult.guidance?.hint,
          resolution: applyResult.guidance?.resolution,
        },
        'Failed to apply manifest',
      );
      // Propagate K8s error guidance
      return Failure(applyResult.error, applyResult.guidance);
    }

    logger.info({ kind, name }, 'Successfully deployed resource');

    return Success({
      kind,
      name,
      namespace: metadata?.namespace ?? namespace,
    });
  } catch (error) {
    logger.error({ kind, name, error: extractErrorMessage(error) }, 'Exception deploying resource');
    return Failure(extractErrorMessage(error));
  }
}

/**
 * Core deployment implementation
 */
async function handleDeploy(
  params: DeployApplicationParams,
  context: ToolContext,
): Promise<Result<DeployApplicationResult>> {
  const logger = getToolLogger(context, 'deploy');
  const timer = createToolTimer(logger, 'deploy');

  const {
    namespace = DEPLOYMENT_CONFIG.DEFAULT_NAMESPACE,
    replicas = DEPLOYMENT_CONFIG.DEFAULT_REPLICAS,
    environment = DEPLOYMENT_CONFIG.DEFAULT_ENVIRONMENT,
  } = params;

  // Validate namespace
  const namespaceValidation = validateNamespace(namespace);
  if (!namespaceValidation.ok) {
    return namespaceValidation;
  }

  const dryRun = DEPLOYMENT_CONFIG.DRY_RUN;
  const wait = DEPLOYMENT_CONFIG.WAIT_FOR_READY;
  const timeout = DEPLOYMENT_CONFIG.WAIT_TIMEOUT_SECONDS;

  try {
    logger.info({ namespace, environment }, 'Starting Kubernetes deployment');
    const k8sClient = createKubernetesClient(logger);

    // Parse and validate manifests
    let manifests: KubernetesManifest[];
    try {
      // The manifests are already a string containing all YAML documents
      manifests = parseManifest(params.manifestsPath, logger);
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
    const failedResources: Array<{
      kind: string;
      name: string;
      error: string;
      guidance?: import('@/types').ErrorGuidance;
    }> = [];

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

        if (result.ok) {
          deployedResources.push(result.value);
        } else {
          failedResources.push({
            kind: manifest.kind ?? 'unknown',
            name: manifest.metadata?.name ?? 'unknown',
            error: result.error,
            ...(result.guidance && { guidance: result.guidance }),
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

        // If ALL manifests failed, return failure with guidance from the first failure
        if (deployedResources.length === 0) {
          const firstFailure = failedResources[0];
          if (firstFailure?.guidance) {
            return Failure(
              `All manifest deployments failed. First error: ${firstFailure.error}`,
              firstFailure.guidance,
            );
          }
          return Failure(
            `All manifest deployments failed. Errors: ${failedResources.map((f) => `${f.kind}/${f.name}: ${f.error}`).join(', ')}`,
          );
        }
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
      // Use shared waitForDeploymentReady from client
      logger.info(
        { deploymentName, timeoutSeconds: timeout },
        'Waiting for deployment to be ready',
      );

      const waitResult = await k8sClient.waitForDeploymentReady(namespace, deploymentName, timeout);

      if (waitResult.ok) {
        ready = true;
        readyReplicas = waitResult.value.readyReplicas || 0;
        logger.info(
          {
            deploymentName,
            readyReplicas,
          },
          'Deployment is ready',
        );
      } else {
        ready = false;
        logger.error(
          { deploymentName, timeoutSeconds: timeout, error: waitResult.error },
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
      success: ready, // Success depends on deployment readiness
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

    timer.end({ deploymentName, ready });

    return Success(result);
  } catch (error) {
    timer.error(error);
    return Failure(extractErrorMessage(error));
  }
}

/**
 * Export the deploy tool directly
 */
export const deployApplication = handleDeploy;

// New Tool interface export
import { tool } from '@/types/tool';

export default tool({
  name: 'deploy',
  description: 'Deploy applications to Kubernetes clusters',
  category: 'kubernetes',
  version: '2.0.0',
  schema: deployApplicationSchema,
  metadata: {
    knowledgeEnhanced: false,
    enhancementCapabilities: [],
  },
  handler: handleDeploy,
});
