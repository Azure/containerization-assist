/**
 * Kubernetes Resource Operations - Consolidated K8s resource management
 *
 * Provides operations for creating, updating, deleting, and reading K8s resources.
 * Uses create/patch strategy for idempotent operations.
 */

import * as k8s from '@kubernetes/client-node';
import type { Logger } from 'pino';
import { Success, Failure, type Result } from '@/types';
import { extractK8sErrorGuidance } from './errors';
import { getResourceConfig, getApiMethod } from './types';

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

/**
 * Explicit interfaces for create method signatures
 */
interface NamespacedCreateMethod {
  (args: { namespace: string; body: unknown }): Promise<{ body?: K8sResource }>;
}

interface ClusterCreateMethod {
  (args: { body: unknown }): Promise<{ body?: K8sResource }>;
}

/**
 * Explicit interfaces for patch method signatures
 */
interface NamespacedPatchMethod {
  (args: { name: string; namespace: string; body: unknown }): Promise<{ body?: K8sResource }>;
}

interface ClusterPatchMethod {
  (args: { name: string; body: unknown }): Promise<{ body?: K8sResource }>;
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
 * Helper function to call create method with appropriate arguments based on whether resource is namespaced
 */
async function callCreateMethod(
  createMethod: unknown,
  isNamespaced: boolean,
  namespace: string,
  resource: K8sResource,
): Promise<{ body?: K8sResource }> {
  if (typeof createMethod !== 'function') {
    throw new TypeError('createMethod must be a function');
  }

  return isNamespaced
    ? await (createMethod as NamespacedCreateMethod)({ namespace, body: resource })
    : await (createMethod as ClusterCreateMethod)({ body: resource });
}

/**
 * Helper function to call patch method with appropriate arguments based on whether resource is namespaced
 */
async function callPatchMethod(
  patchMethod: unknown,
  isNamespaced: boolean,
  name: string,
  namespace: string,
  resource: K8sResource,
): Promise<{ body?: K8sResource }> {
  if (typeof patchMethod !== 'function') {
    throw new TypeError('patchMethod must be a function');
  }

  return isNamespaced
    ? await (patchMethod as NamespacedPatchMethod)({ name, namespace, body: resource })
    : await (patchMethod as ClusterPatchMethod)({ name, body: resource });
}

/**
 * Apply a Kubernetes resource using create/patch strategy.
 * Idempotent - safe to call multiple times.
 *
 * Note: This function should be called sequentially per resource.
 * Creates the resource if it doesn't exist, or patches it if it already exists.
 *
 * @param kc - Kubernetes config
 * @param resource - Resource to apply
 * @param logger - Logger instance
 * @returns Success with applied resource, or Failure with error guidance
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
    if (config?.api) {
      const createMethod = getApiMethod(config.api, config.createMethod);

      if (typeof createMethod !== 'function') {
        return Failure(`Method ${config.createMethod} not found on API client`);
      }

      try {
        const result = await callCreateMethod(createMethod, config.namespaced, namespace, resource);

        const resourceBody = result.body || (result as unknown as K8sResource);
        logger.info({ kind, name, namespace }, 'Resource created successfully');
        return Success(resourceBody);
      } catch (createError) {
        // If resource already exists, try to update it with patch
        if (isConflictError(createError)) {
          logger.debug({ kind, name, namespace }, 'Resource exists, updating with patch');

          const patchMethod = getApiMethod(config.api, config.patchMethod);
          if (typeof patchMethod !== 'function') {
            return Failure(`Method ${config.patchMethod} not found on API client`);
          }

          const patchResult = await callPatchMethod(
            patchMethod,
            config.namespaced,
            name,
            namespace,
            resource,
          );

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
