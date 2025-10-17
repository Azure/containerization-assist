/**
 * Kubernetes Resource Operations - Consolidated K8s resource management
 *
 * Provides unified operations for creating, updating, deleting, and reading K8s resources.
 * Uses server-side apply for idempotent operations.
 */

import * as k8s from '@kubernetes/client-node';
import type { Logger } from 'pino';
import { Success, Failure, type Result } from '@/types';
import { extractK8sErrorGuidance } from './errors';

export interface K8sResource {
  apiVersion: string;
  kind: string;
  metadata: {
    name: string;
    namespace?: string;
    labels?: Record<string, string>;
    annotations?: Record<string, string>;
  };
  spec?: Record<string, unknown>;
  data?: Record<string, unknown>;
}

// Resource mapping for type-safe resource creation
type ResourceCreateConfig = {
  api: unknown;
  createMethod: string;
  patchMethod: string;
  namespaced: boolean;
};

/**
 * Get resource configuration for a given resource kind
 * Maps Kubernetes resource types to their corresponding API clients and methods
 */
function getResourceConfig(
  kc: k8s.KubeConfig,
  kind: string,
): ResourceCreateConfig | undefined {
  const coreApi = kc.makeApiClient(k8s.CoreV1Api);
  const appsApi = kc.makeApiClient(k8s.AppsV1Api);
  const networkingApi = kc.makeApiClient(k8s.NetworkingV1Api);
  const batchApi = kc.makeApiClient(k8s.BatchV1Api);
  const rbacApi = kc.makeApiClient(k8s.RbacAuthorizationV1Api);
  const autoscalingApi = kc.makeApiClient(k8s.AutoscalingV2Api);

  const resourceMap: Record<string, ResourceCreateConfig> = {
    Namespace: {
      api: coreApi,
      createMethod: 'createNamespace',
      patchMethod: 'patchNamespace',
      namespaced: false,
    },
    Deployment: {
      api: appsApi,
      createMethod: 'createNamespacedDeployment',
      patchMethod: 'patchNamespacedDeployment',
      namespaced: true,
    },
    Service: {
      api: coreApi,
      createMethod: 'createNamespacedService',
      patchMethod: 'patchNamespacedService',
      namespaced: true,
    },
    ConfigMap: {
      api: coreApi,
      createMethod: 'createNamespacedConfigMap',
      patchMethod: 'patchNamespacedConfigMap',
      namespaced: true,
    },
    Secret: {
      api: coreApi,
      createMethod: 'createNamespacedSecret',
      patchMethod: 'patchNamespacedSecret',
      namespaced: true,
    },
    ServiceAccount: {
      api: coreApi,
      createMethod: 'createNamespacedServiceAccount',
      patchMethod: 'patchNamespacedServiceAccount',
      namespaced: true,
    },
    Ingress: {
      api: networkingApi,
      createMethod: 'createNamespacedIngress',
      patchMethod: 'patchNamespacedIngress',
      namespaced: true,
    },
    StatefulSet: {
      api: appsApi,
      createMethod: 'createNamespacedStatefulSet',
      patchMethod: 'patchNamespacedStatefulSet',
      namespaced: true,
    },
    DaemonSet: {
      api: appsApi,
      createMethod: 'createNamespacedDaemonSet',
      patchMethod: 'patchNamespacedDaemonSet',
      namespaced: true,
    },
    Job: {
      api: batchApi,
      createMethod: 'createNamespacedJob',
      patchMethod: 'patchNamespacedJob',
      namespaced: true,
    },
    CronJob: {
      api: batchApi,
      createMethod: 'createNamespacedCronJob',
      patchMethod: 'patchNamespacedCronJob',
      namespaced: true,
    },
    Role: {
      api: rbacApi,
      createMethod: 'createNamespacedRole',
      patchMethod: 'patchNamespacedRole',
      namespaced: true,
    },
    RoleBinding: {
      api: rbacApi,
      createMethod: 'createNamespacedRoleBinding',
      patchMethod: 'patchNamespacedRoleBinding',
      namespaced: true,
    },
    ClusterRole: {
      api: rbacApi,
      createMethod: 'createClusterRole',
      patchMethod: 'patchClusterRole',
      namespaced: false,
    },
    ClusterRoleBinding: {
      api: rbacApi,
      createMethod: 'createClusterRoleBinding',
      patchMethod: 'patchClusterRoleBinding',
      namespaced: false,
    },
    PersistentVolumeClaim: {
      api: coreApi,
      createMethod: 'createNamespacedPersistentVolumeClaim',
      patchMethod: 'patchNamespacedPersistentVolumeClaim',
      namespaced: true,
    },
    PersistentVolume: {
      api: coreApi,
      createMethod: 'createPersistentVolume',
      patchMethod: 'patchPersistentVolume',
      namespaced: false,
    },
    HorizontalPodAutoscaler: {
      api: autoscalingApi,
      createMethod: 'createNamespacedHorizontalPodAutoscaler',
      patchMethod: 'patchNamespacedHorizontalPodAutoscaler',
      namespaced: true,
    },
  };

  return resourceMap[kind];
}

/**
 * Helper function to check if an error is a 409 Conflict (AlreadyExists) error
 */
function isConflictError(error: unknown): boolean {
  return (error as { response?: { statusCode?: number } }).response?.statusCode === 409;
}

/**
 * Handle K8s errors with actionable guidance
 */
function handleK8sError(error: unknown, operation: string, logger: Logger): Result<never> {
  const guidance = extractK8sErrorGuidance(error, operation);
  logger.error({ error: guidance.message, operation }, 'K8s operation failed');
  return Failure(guidance.message, guidance);
}

/**
 * Apply a Kubernetes resource using server-side apply.
 * Idempotent - safe to call multiple times.
 *
 * **IMPORTANT: Sequential Execution Required**
 *
 * This function should be called **sequentially per resource** to avoid race conditions
 * during the create-or-update flow. While Kubernetes server-side apply handles concurrent
 * updates safely at the server level, this client-side implementation uses a create-then-patch
 * strategy that can lead to unexpected behavior if called concurrently for the same resource.
 *
 * Recommended usage:
 * - Call sequentially when applying multiple resources for the same application
 * - Use `for...of` or `Promise.all` with different resources only
 * - Avoid parallel calls for the same resource
 *
 * @param kc - Kubernetes config
 * @param resource - Resource to apply (must include kind, apiVersion, and metadata.name)
 * @param logger - Logger instance for operation tracking
 * @returns Success with applied resource, or Failure with error guidance
 *
 * @example
 * ```typescript
 * // Good: Sequential application
 * for (const resource of resources) {
 *   const result = await applyResource(kc, resource, logger);
 *   if (!result.ok) return result;
 * }
 *
 * // Avoid: Concurrent application of same resource
 * await Promise.all(resources.map(r => applyResource(kc, r, logger)));
 * ```
 */
export async function applyResource(
  kc: k8s.KubeConfig,
  resource: K8sResource,
  logger: Logger,
): Promise<Result<K8sResource>> {
  try {
    const namespace = resource.metadata.namespace || 'default';
    const name = resource.metadata.name;
    const kind = resource.kind;

    logger.debug({ kind, name, namespace }, 'Applying K8s resource');

    // Validate manifest has required metadata
    if (!name || name.trim() === '') {
      const errorMessage = 'Resource is missing required metadata.name';
      logger.error({ kind, metadata: resource.metadata }, errorMessage);
      return Failure(errorMessage);
    }

    const config = getResourceConfig(kc, kind);

    // Try to create the resource first (optimistic approach)
    if (config) {
      const api = config.api as Record<string, (args: unknown) => Promise<{ body?: K8sResource }>>;
      const createMethod = api[config.createMethod];

      if (!createMethod) {
        return Failure(`Method ${config.createMethod} not found on API client`);
      }

      try {
        const result = config.namespaced
          ? await createMethod({ namespace, body: resource })
          : await createMethod({ body: resource });

        const resourceBody = result.body || (result as unknown as K8sResource);
        logger.info({ kind, name, namespace }, 'Resource created successfully');
        return Success(resourceBody);
      } catch (createError) {
        // If resource already exists, try to update it with patch
        if (isConflictError(createError)) {
          logger.debug({ kind, name, namespace }, 'Resource exists, updating with patch');

          const patchMethod = api[config.patchMethod];
          if (!patchMethod) {
            return Failure(`Method ${config.patchMethod} not found on API client`);
          }

          const patchResult = config.namespaced
            ? await patchMethod({ name, namespace, body: resource })
            : await patchMethod({ name, body: resource });

          const patchBody = patchResult.body || (patchResult as unknown as K8sResource);
          logger.info({ kind, name, namespace }, 'Resource updated successfully');
          return Success(patchBody);
        }

        // Other errors - propagate
        throw createError;
      }
    } else {
      // For unsupported resource types, use the generic KubernetesObjectApi
      const objectApi = k8s.KubernetesObjectApi.makeApiClient(kc);

      try {
        const result = await objectApi.create(resource as k8s.KubernetesObject);
        logger.info({ kind, name, namespace }, 'Resource created successfully (generic API)');
        // KubernetesObjectApi returns the resource directly
        return Success(result as K8sResource);
      } catch (createError) {
        // If resource already exists, try to patch it
        if (isConflictError(createError)) {
          logger.debug(
            { kind, name, namespace },
            'Resource exists, updating with patch (generic API)',
          );
          const patchResult = await objectApi.patch(resource as k8s.KubernetesObject);
          logger.info({ kind, name, namespace }, 'Resource updated successfully (generic API)');
          // KubernetesObjectApi returns the resource directly
          return Success(patchResult as K8sResource);
        }

        // Other errors - propagate
        throw createError;
      }
    }
  } catch (error) {
    return handleK8sError(error, 'apply resource', logger);
  }
}

