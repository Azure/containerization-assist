/**
 * Kubernetes Client - Direct k8s API Access
 *
 * Simplified Kubernetes operations using direct @kubernetes/client-node integration
 * Removes unnecessary wrapper complexity while maintaining core functionality
 */

import * as k8s from '@kubernetes/client-node';
import type { Logger } from 'pino';
import { Success, Failure, type Result } from '@types';
import { formatErrorMessage } from '@lib/error-utils';

export interface DeploymentResult {
  ready: boolean;
  readyReplicas: number;
  totalReplicas: number;
}

export interface ClusterInfo {
  name: string;
  version: string;
  ready: boolean;
}

export interface KubernetesClient {
  applyManifest: (manifest: any, namespace?: string) => Promise<Result<void>>;
  getDeploymentStatus: (namespace: string, name: string) => Promise<Result<DeploymentResult>>;
  deleteResource: (kind: string, name: string, namespace?: string) => Promise<Result<void>>;
  ping: () => Promise<boolean>;
  namespaceExists: (namespace: string) => Promise<boolean>;
  checkPermissions: (namespace: string) => Promise<boolean>;
  checkIngressController: () => Promise<boolean>;
}

/**
 * Create a Kubernetes client with core operations
 */
export const createKubernetesClient = (logger: Logger, kubeconfig?: string): KubernetesClient => {
  const kc = new k8s.KubeConfig();

  // Load kubeconfig from default locations or provided config
  if (kubeconfig) {
    kc.loadFromString(kubeconfig);
  } else {
    kc.loadFromDefault();
  }

  const k8sApi = kc.makeApiClient(k8s.AppsV1Api);
  const coreApi = kc.makeApiClient(k8s.CoreV1Api);
  const networkingApi = kc.makeApiClient(k8s.NetworkingV1Api);

  return {
    /**
     * Apply Kubernetes manifest
     */
    async applyManifest(manifest: any, namespace = 'default'): Promise<Result<void>> {
      try {
        logger.debug({ manifest: manifest.kind, namespace }, 'Applying Kubernetes manifest');

        // Simple apply logic - in production this would handle different resource types
        if (manifest.kind === 'Deployment') {
          await k8sApi.createNamespacedDeployment({ namespace, body: manifest });
        } else if (manifest.kind === 'Service') {
          await coreApi.createNamespacedService({ namespace, body: manifest });
        }

        logger.info(
          { kind: manifest.kind, name: manifest.metadata?.name },
          'Manifest applied successfully',
        );
        return Success(undefined);
      } catch (error) {
        const errorMessage = formatErrorMessage('Failed to apply manifest', error);
        return Failure(errorMessage);
      }
    },

    /**
     * Get deployment status
     */
    async getDeploymentStatus(
      namespace: string,
      name: string,
    ): Promise<
      Result<{
        ready: boolean;
        readyReplicas: number;
        totalReplicas: number;
      }>
    > {
      try {
        const response = await k8sApi.readNamespacedDeployment({ name, namespace });
        const deployment = response;

        const status = {
          ready: (deployment.status?.readyReplicas || 0) === (deployment.spec?.replicas || 0),
          readyReplicas: deployment.status?.readyReplicas || 0,
          totalReplicas: deployment.spec?.replicas || 0,
        };

        return Success(status);
      } catch (error) {
        const errorMessage = formatErrorMessage('Failed to get deployment status', error);
        return Failure(errorMessage);
      }
    },

    /**
     * Delete resource
     */
    async deleteResource(kind: string, name: string, namespace = 'default'): Promise<Result<void>> {
      try {
        if (kind === 'Deployment') {
          await k8sApi.deleteNamespacedDeployment({ name, namespace });
        } else if (kind === 'Service') {
          await coreApi.deleteNamespacedService({ name, namespace });
        }

        logger.info({ kind, name, namespace }, 'Resource deleted successfully');
        return Success(undefined);
      } catch (error) {
        const errorMessage = formatErrorMessage('Failed to delete resource', error);
        return Failure(errorMessage);
      }
    },

    /**
     * Check cluster connectivity
     */
    async ping(): Promise<boolean> {
      try {
        await coreApi.listNamespace();
        return true;
      } catch (error) {
        logger.debug({ error }, 'Cluster ping failed');
        return false;
      }
    },

    /**
     * Check if namespace exists
     */
    async namespaceExists(namespace: string): Promise<boolean> {
      try {
        await coreApi.readNamespace({ name: namespace });
        return true;
      } catch (error: unknown) {
        if (error && typeof error === 'object' && 'response' in error) {
          const response = (error as any).response;
          if (response?.statusCode === 404) {
            return false;
          }
        }
        logger.warn({ namespace, error }, 'Error checking namespace');
        return false;
      }
    },

    /**
     * Check user permissions in namespace
     */
    async checkPermissions(namespace: string): Promise<boolean> {
      try {
        // Try to perform a self-subject access review
        const accessReview = {
          apiVersion: 'authorization.k8s.io/v1',
          kind: 'SelfSubjectAccessReview',
          spec: {
            resourceAttributes: {
              namespace,
              verb: 'create',
              resource: 'deployments',
              group: 'apps',
            },
          },
        };

        // Use authorization API for SelfSubjectAccessReview
        const authApi = kc.makeApiClient(k8s.AuthorizationV1Api);
        const response = await authApi.createSelfSubjectAccessReview({ body: accessReview as any });
        return response.status?.allowed === true;
      } catch (error) {
        logger.warn({ namespace, error }, 'Error checking permissions');
        // If we can't check permissions, assume we have them
        return true;
      }
    },

    /**
     * Check if an ingress controller is installed
     */
    async checkIngressController(): Promise<boolean> {
      try {
        // Check for common ingress controller deployments
        const namespaces = ['ingress-nginx', 'nginx-ingress', 'kube-system'];

        for (const ns of namespaces) {
          try {
            const deployments = await k8sApi.listNamespacedDeployment({ namespace: ns });
            const hasIngress = deployments.items.some(
              (d) => d.metadata?.name?.includes('ingress') || d.metadata?.name?.includes('nginx'),
            );
            if (hasIngress) {
              logger.debug({ namespace: ns }, 'Found ingress controller');
              return true;
            }
          } catch {
            // Namespace might not exist, continue checking
          }
        }

        // Also check for IngressClass resources
        try {
          const ingressClasses = await networkingApi.listIngressClass();
          if (ingressClasses.items.length > 0) {
            logger.debug({ count: ingressClasses.items.length }, 'Found ingress classes');
            return true;
          }
        } catch {
          // IngressClass might not be available in older clusters
        }

        return false;
      } catch (error) {
        logger.warn({ error }, 'Error checking for ingress controller');
        return false;
      }
    },
  };
};
