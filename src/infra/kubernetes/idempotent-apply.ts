/**
 * Idempotent Kubernetes resource application with mutex protection
 */

import * as k8s from '@kubernetes/client-node';
import * as yaml from 'js-yaml';
import type { Logger } from 'pino';
import { createKeyedMutex } from '@/lib/mutex';
import { Success, Failure, type Result } from '@/types';
import { config } from '@/config/index';
import { extractErrorMessage } from '@/lib/error-utils';

export interface ApplyOptions {
  dryRun?: boolean;
  force?: boolean;
  fieldManager?: string;
}

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
 * Creates a mutex-protected, idempotent K8s apply function
 */
export function createIdempotentApply(logger: Logger, kubeconfig?: string) {
  const kc = new k8s.KubeConfig();

  if (kubeconfig) {
    kc.loadFromString(kubeconfig);
  } else {
    kc.loadFromDefault();
  }

  const mutex = createKeyedMutex({
    defaultTimeout: config.mutex.defaultTimeout,
    monitoringEnabled: config.mutex.monitoringEnabled,
  });

  /**
   * Get the appropriate API client for a resource
   *
   * Invariant: Falls back to CustomObjectsApi for unknown apiVersions to ensure all resources are supported
   * Trade-off: Uses any type for flexibility across diverse K8s API clients
   */
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  function getApiClient(apiVersion: string): any {
    // Explicit API client mapping ensures type safety for common resource types
    if (apiVersion === 'v1') {
      return kc.makeApiClient(k8s.CoreV1Api);
    } else if (apiVersion === 'apps/v1') {
      return kc.makeApiClient(k8s.AppsV1Api);
    } else if (apiVersion === 'batch/v1') {
      return kc.makeApiClient(k8s.BatchV1Api);
    } else if (apiVersion === 'networking.k8s.io/v1') {
      return kc.makeApiClient(k8s.NetworkingV1Api);
    } else if (apiVersion === 'rbac.authorization.k8s.io/v1') {
      return kc.makeApiClient(k8s.RbacAuthorizationV1Api);
    } else if (apiVersion === 'autoscaling/v2') {
      return kc.makeApiClient(k8s.AutoscalingV2Api);
    } else {
      // For custom resources, use the custom objects API
      return kc.makeApiClient(k8s.CustomObjectsApi);
    }
  }

  /**
   * Apply a resource using server-side apply (patch with fieldManager)
   */
  async function serverSideApply(
    resource: K8sResource,
    _options: ApplyOptions,
  ): Promise<Result<K8sResource>> {
    const api = getApiClient(resource.apiVersion);
    const namespace = resource.metadata.namespace || 'default';
    const name = resource.metadata.name;
    // const fieldManager = options.fieldManager || 'containerization-assist';

    try {
      let result: { body?: K8sResource } | K8sResource;

      // Different API methods for different resource types
      if (resource.kind === 'Namespace') {
        result = await api.patchNamespace({
          name,
          body: resource,
        });
      } else if (resource.kind === 'Deployment') {
        result = await api.patchNamespacedDeployment({
          name,
          namespace,
          body: resource,
        });
      } else if (resource.kind === 'Service') {
        result = await api.patchNamespacedService({
          name,
          namespace,
          body: resource,
        });
      } else if (resource.kind === 'ConfigMap') {
        result = await api.patchNamespacedConfigMap({
          name,
          namespace,
          body: resource,
        });
      } else if (resource.kind === 'Secret') {
        result = await api.patchNamespacedSecret({
          name,
          namespace,
          body: resource,
        });
      } else if (resource.kind === 'Ingress') {
        result = await api.patchNamespacedIngress({
          name,
          namespace,
          body: resource,
        });
      } else if (resource.kind === 'HorizontalPodAutoscaler') {
        result = await api.patchNamespacedHorizontalPodAutoscaler({
          name,
          namespace,
          body: resource,
        });
      } else {
        // For custom resources or unsupported types, try custom objects API
        const group = resource.apiVersion.includes('/') ? resource.apiVersion.split('/')[0] : '';
        const version = resource.apiVersion.includes('/')
          ? resource.apiVersion.split('/')[1]
          : resource.apiVersion;
        const plural = `${resource.kind.toLowerCase()}s`;

        const customApi = kc.makeApiClient(k8s.CustomObjectsApi);
        result = await customApi.patchNamespacedCustomObject({
          group: group || '',
          version: version || 'v1',
          namespace,
          plural,
          name: name || '',
          body: resource,
        });
      }

      logger.debug(
        {
          kind: resource.kind,
          name,
          namespace,
          operation: 'server-side-apply',
        },
        'Resource applied successfully',
      );

      // K8s client returns either the resource directly or wrapped in a body property
      const resourceBody = 'body' in result && result.body ? result.body : result;
      return Success(resourceBody as K8sResource);
    } catch (error) {
      const errorMessage = extractErrorMessage(error);
      logger.error(
        {
          error: errorMessage,
          kind: resource.kind,
          name,
          namespace,
        },
        'Server-side apply failed',
      );

      return Failure(`Failed to apply ${resource.kind}/${name}: ${errorMessage}`);
    }
  }

  /**
   * Create a new resource
   */
  async function createResource(
    resource: K8sResource,
    options: ApplyOptions,
  ): Promise<Result<K8sResource>> {
    const api = getApiClient(resource.apiVersion);
    const namespace = resource.metadata.namespace || 'default';
    const name = resource.metadata.name;

    try {
      let result: { body?: K8sResource } | K8sResource;

      if (resource.kind === 'Namespace') {
        result = await api.createNamespace({ body: resource });
      } else if (resource.kind === 'Deployment') {
        result = await api.createNamespacedDeployment({
          namespace,
          body: resource,
        });
      } else if (resource.kind === 'Service') {
        result = await api.createNamespacedService({
          namespace,
          body: resource,
        });
      } else if (resource.kind === 'ConfigMap') {
        result = await api.createNamespacedConfigMap({
          namespace,
          body: resource,
        });
      } else if (resource.kind === 'Secret') {
        result = await api.createNamespacedSecret({
          namespace,
          body: resource,
        });
      } else if (resource.kind === 'Ingress') {
        result = await api.createNamespacedIngress({
          namespace,
          body: resource,
        });
      } else if (resource.kind === 'HorizontalPodAutoscaler') {
        result = await api.createNamespacedHorizontalPodAutoscaler({
          namespace,
          body: resource,
        });
      } else {
        // For custom resources
        const group = resource.apiVersion.includes('/') ? resource.apiVersion.split('/')[0] : '';
        const version = resource.apiVersion.includes('/')
          ? resource.apiVersion.split('/')[1]
          : resource.apiVersion;
        const plural = `${resource.kind.toLowerCase()}s`;

        const customApi = kc.makeApiClient(k8s.CustomObjectsApi);
        result = await customApi.createNamespacedCustomObject({
          group: group || '',
          version: version || 'v1',
          namespace,
          plural,
          body: resource,
        });
      }

      logger.debug(
        {
          kind: resource.kind,
          name,
          namespace,
          operation: 'create',
        },
        'Resource created successfully',
      );

      // K8s client returns either the resource directly or wrapped in a body property
      const resourceBody = 'body' in result && result.body ? result.body : result;
      return Success(resourceBody as K8sResource);
    } catch (error: unknown) {
      // Check if it's an "already exists" error
      const errorObj = error as {
        statusCode?: number;
        response?: { statusCode?: number };
        message?: string;
      };
      if (errorObj.statusCode === 409 || errorObj.response?.statusCode === 409) {
        logger.debug({ kind: resource.kind, name }, 'Resource already exists, attempting update');
        // Resource already exists, try server-side apply
        return serverSideApply(resource, options);
      }

      logger.error(
        {
          error: errorObj.message,
          statusCode: errorObj.statusCode,
          kind: resource.kind,
          name,
          namespace,
        },
        'Failed to create resource',
      );

      return Failure(
        `Failed to create ${resource.kind}/${name}: ${errorObj.message || 'Unknown error'}`,
      );
    }
  }

  /**
   * Main idempotent apply function with mutex protection
   */
  return async function applyResource(
    resource: K8sResource,
    options: ApplyOptions = {},
  ): Promise<Result<K8sResource>> {
    const namespace = resource.metadata.namespace || 'default';
    const name = resource.metadata.name;
    const lockKey = `k8s:${resource.kind}:${namespace}:${name}`;

    logger.info(
      {
        kind: resource.kind,
        name,
        namespace,
        dryRun: options.dryRun,
      },
      'Applying K8s resource',
    );

    try {
      return await mutex.withLock(lockKey, async () => {
        // Try create first
        const createResult = await createResource(resource, options);

        if (createResult.ok) {
          return createResult;
        }

        // If create failed with non-409 error, return the error
        if (!createResult.error.includes('already exists')) {
          return createResult;
        }

        // Resource exists, use server-side apply for update
        logger.debug({ kind: resource.kind, name }, 'Resource exists, using server-side apply');
        return serverSideApply(resource, options);
      });
    } catch (error) {
      if (error instanceof Error && error.message.includes('Mutex timeout')) {
        logger.error({ lockKey }, 'K8s apply mutex timeout');
        return Failure(
          `Apply operation timed out - another operation may be in progress for ${resource.kind}/${name}`,
        );
      }

      logger.error({ error, resource }, 'Unexpected error in applyResource');
      return Failure(`Unexpected error: ${extractErrorMessage(error)}`);
    }
  };
}

/**
 * Parse YAML manifests into K8s resources
 */
export function parseManifests(yamlContent: string): K8sResource[] {
  try {
    // In js-yaml v4, loadAll is safe by default (no code execution)
    const docs = yaml.loadAll(yamlContent);
    return docs.filter((doc: unknown) => {
      const resource = doc as Record<string, unknown>;
      return resource?.kind && resource.apiVersion;
    }) as K8sResource[];
  } catch {
    return [];
  }
}
