/**
 * Idempotent Kubernetes resource application
 */

import * as k8s from '@kubernetes/client-node';
import * as yaml from 'js-yaml';
import type { Logger } from 'pino';
import { Success, Failure, type Result, type K8sResource } from '@types';
import { extractErrorMessage } from '@lib/error-utils';

export interface ApplyOptions {
  dryRun?: boolean;
  force?: boolean;
  fieldManager?: string;
}

/**
 * Creates a simplified idempotent K8s apply function
 */
export function createIdempotentApply(
  logger: Logger,
  kubeconfig?: string,
): {
  applyResource: (resource: K8sResource, options?: ApplyOptions) => Promise<Result<K8sResource>>;
  applyManifests: (
    manifests: K8sResource[],
    options?: ApplyOptions,
  ) => Promise<Result<K8sResource[]>>;
} {
  const kc = new k8s.KubeConfig();

  if (kubeconfig) {
    kc.loadFromString(kubeconfig);
  } else {
    kc.loadFromDefault();
  }

  // Create API clients
  const coreApi = kc.makeApiClient(k8s.CoreV1Api);
  const appsApi = kc.makeApiClient(k8s.AppsV1Api);
  const networkingApi = kc.makeApiClient(k8s.NetworkingV1Api);

  /**
   * Simple apply function - creates or updates resources
   */
  async function applyResource(
    resource: K8sResource,
    _options: ApplyOptions = {},
  ): Promise<Result<K8sResource>> {
    const namespace = resource.metadata.namespace || 'default';
    const name = resource.metadata.name;

    if (!name) {
      return Failure('Resource metadata.name is required');
    }

    try {
      // Try to create the resource first
      let result;

      if (resource.kind === 'Deployment') {
        try {
          result = await appsApi.createNamespacedDeployment({ namespace, body: resource as any });
        } catch (error: any) {
          // If it already exists, try to patch it
          if (error.response?.statusCode === 409) {
            result = await appsApi.patchNamespacedDeployment({
              name,
              namespace,
              body: resource as any,
            });
          } else {
            throw error;
          }
        }
      } else if (resource.kind === 'Service') {
        try {
          result = await coreApi.createNamespacedService({ namespace, body: resource as any });
        } catch (error: any) {
          if (error.response?.statusCode === 409) {
            result = await coreApi.patchNamespacedService({
              name,
              namespace,
              body: resource as any,
            });
          } else {
            throw error;
          }
        }
      } else if (resource.kind === 'ConfigMap') {
        try {
          result = await coreApi.createNamespacedConfigMap({ namespace, body: resource as any });
        } catch (error: any) {
          if (error.response?.statusCode === 409) {
            result = await coreApi.patchNamespacedConfigMap({
              name,
              namespace,
              body: resource as any,
            });
          } else {
            throw error;
          }
        }
      } else if (resource.kind === 'Secret') {
        try {
          result = await coreApi.createNamespacedSecret({ namespace, body: resource as any });
        } catch (error: any) {
          if (error.response?.statusCode === 409) {
            result = await coreApi.patchNamespacedSecret({
              name,
              namespace,
              body: resource as any,
            });
          } else {
            throw error;
          }
        }
      } else if (resource.kind === 'Ingress') {
        try {
          result = await networkingApi.createNamespacedIngress({
            namespace,
            body: resource as any,
          });
        } catch (error: any) {
          if (error.response?.statusCode === 409) {
            result = await networkingApi.patchNamespacedIngress({
              name,
              namespace,
              body: resource as any,
            });
          } else {
            throw error;
          }
        }
      } else {
        return Failure(`Unsupported resource kind: ${resource.kind}`);
      }

      logger.info(
        {
          kind: resource.kind,
          name,
          namespace,
        },
        'Resource applied successfully',
      );

      return Success((result as any).body || result);
    } catch (error: unknown) {
      logger.error(
        {
          error: extractErrorMessage(error),
          kind: resource.kind,
          name,
          namespace,
        },
        'Failed to apply resource',
      );

      return Failure(`Failed to apply ${resource.kind}/${name}: ${extractErrorMessage(error)}`);
    }
  }

  return {
    applyResource,
    applyManifests: async (manifests: K8sResource[], options?: ApplyOptions) => {
      const results: K8sResource[] = [];
      for (const manifest of manifests) {
        const result = await applyResource(manifest, options);
        if (result.ok) {
          results.push(result.value);
        } else {
          return Failure(result.error);
        }
      }
      return Success(results);
    },
  };
}

/**
 * Parse YAML manifests into K8s resources
 */
export function parseManifests(yamlContent: string): K8sResource[] {
  try {
    const docs = yaml.loadAll(yamlContent);
    return docs.filter((doc: unknown): doc is K8sResource => {
      return typeof doc === 'object' && doc !== null && 'kind' in doc && 'apiVersion' in doc;
    });
  } catch {
    // Return empty array if js-yaml parsing fails
    return [];
  }
}
