/**
 * Kubernetes Client - Direct k8s API Access
 *
 * Kubernetes operations using direct @kubernetes/client-node integration
 */

import * as k8s from '@kubernetes/client-node';
import type { Logger } from 'pino';
import { Success, Failure, type Result } from '@/types';
import { extractK8sErrorGuidance } from './errors';
import { discoverAndValidateKubeconfig } from './kubeconfig-discovery';

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

export interface K8sManifest {
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

export interface KubernetesClient {
  applyManifest: (manifest: K8sManifest, namespace?: string) => Promise<Result<void>>;
  getDeploymentStatus: (namespace: string, name: string) => Promise<Result<DeploymentResult>>;
  waitForDeploymentReady: (
    namespace: string,
    name: string,
    timeoutSeconds: number,
  ) => Promise<Result<DeploymentResult>>;
  ensureNamespace: (namespace: string) => Promise<Result<void>>;
  ping: () => Promise<boolean>;
  namespaceExists: (namespace: string) => Promise<boolean>;
  checkPermissions: (namespace: string) => Promise<boolean>;
  checkIngressController: () => Promise<boolean>;
}

export interface KubernetesClientConfig {
  logger: Logger;
  kubeconfig?: string;
  timeout?: number;
}

// Constants for deployment polling
const DEPLOYMENT_POLL_INTERVAL_MS = 5000; // 5 seconds

/**
 * Create a Kubernetes client with core operations
 *
 * @throws Error if kubeconfig is invalid or not found (fast-fail for single-user scenarios)
 */
export const createKubernetesClient = (
  logger: Logger,
  kubeconfig?: string,
  timeout?: number,
): KubernetesClient => {
  const kc = new k8s.KubeConfig();

  // Load kubeconfig from default locations or provided config
  if (kubeconfig) {
    try {
      kc.loadFromString(kubeconfig);
      logger.debug('Loaded kubeconfig from provided string');
    } catch (error) {
      const errorMsg = `Failed to load kubeconfig: ${error instanceof Error ? error.message : String(error)}`;
      logger.error({ error: errorMsg }, 'Kubeconfig load failed');
      throw new Error(errorMsg);
    }
  } else {
    // Validate kubeconfig before attempting to load
    const validation = discoverAndValidateKubeconfig();
    if (!validation.ok) {
      const errorMsg = `${validation.error}. ${validation.guidance?.hint || ''}`;
      logger.error(
        {
          error: validation.error,
          hint: validation.guidance?.hint,
          resolution: validation.guidance?.resolution,
          details: validation.guidance?.details,
        },
        'Kubeconfig validation failed',
      );
      throw new Error(errorMsg);
    }

    try {
      kc.loadFromDefault();
      logger.debug(
        {
          path: validation.value.path,
          context: validation.value.contextName,
          cluster: validation.value.clusterName,
        },
        'Loaded kubeconfig from default location',
      );
    } catch (error) {
      const errorMsg = `Failed to load kubeconfig: ${error instanceof Error ? error.message : String(error)}`;
      logger.error({ error: errorMsg }, 'Kubeconfig load failed');
      throw new Error(errorMsg);
    }
  }

  const k8sApi = kc.makeApiClient(k8s.AppsV1Api);
  const coreApi = kc.makeApiClient(k8s.CoreV1Api);
  const networkingApi = kc.makeApiClient(k8s.NetworkingV1Api);
  const batchApi = kc.makeApiClient(k8s.BatchV1Api);
  const rbacApi = kc.makeApiClient(k8s.RbacAuthorizationV1Api);
  const objectApi = k8s.KubernetesObjectApi.makeApiClient(kc);

  return {
    /**
     * Apply Kubernetes manifest (supports all resource types)
     * Uses generic KubernetesObjectApi for maximum compatibility
     */
    async applyManifest(manifest: K8sManifest, namespace = 'default'): Promise<Result<void>> {
      try {
        logger.debug({ manifest: manifest.kind, namespace }, 'Applying Kubernetes manifest');

        // Set namespace in metadata if not already set and not a cluster-scoped resource
        const isClusterScoped = ['Namespace', 'ClusterRole', 'ClusterRoleBinding'].includes(
          manifest.kind || '',
        );
        if (!isClusterScoped && !manifest.metadata.namespace) {
          manifest.metadata.namespace = namespace;
        }

        // Use specific API for common resources (more reliable)
        switch (manifest.kind) {
          case 'Namespace':
            await coreApi.createNamespace({
              body: manifest as unknown as k8s.V1Namespace,
            });
            break;
          case 'Deployment':
            await k8sApi.createNamespacedDeployment({
              namespace: manifest.metadata.namespace || namespace,
              body: manifest as unknown as k8s.V1Deployment,
            });
            break;
          case 'Service':
            await coreApi.createNamespacedService({
              namespace: manifest.metadata.namespace || namespace,
              body: manifest as unknown as k8s.V1Service,
            });
            break;
          case 'ConfigMap':
            await coreApi.createNamespacedConfigMap({
              namespace: manifest.metadata.namespace || namespace,
              body: manifest as unknown as k8s.V1ConfigMap,
            });
            break;
          case 'Secret':
            await coreApi.createNamespacedSecret({
              namespace: manifest.metadata.namespace || namespace,
              body: manifest as unknown as k8s.V1Secret,
            });
            break;
          case 'ServiceAccount':
            await coreApi.createNamespacedServiceAccount({
              namespace: manifest.metadata.namespace || namespace,
              body: manifest as unknown as k8s.V1ServiceAccount,
            });
            break;
          case 'Ingress':
            await networkingApi.createNamespacedIngress({
              namespace: manifest.metadata.namespace || namespace,
              body: manifest as unknown as k8s.V1Ingress,
            });
            break;
          case 'StatefulSet':
            await k8sApi.createNamespacedStatefulSet({
              namespace: manifest.metadata.namespace || namespace,
              body: manifest as unknown as k8s.V1StatefulSet,
            });
            break;
          case 'DaemonSet':
            await k8sApi.createNamespacedDaemonSet({
              namespace: manifest.metadata.namespace || namespace,
              body: manifest as unknown as k8s.V1DaemonSet,
            });
            break;
          case 'Job':
            await batchApi.createNamespacedJob({
              namespace: manifest.metadata.namespace || namespace,
              body: manifest as unknown as k8s.V1Job,
            });
            break;
          case 'CronJob':
            await batchApi.createNamespacedCronJob({
              namespace: manifest.metadata.namespace || namespace,
              body: manifest as unknown as k8s.V1CronJob,
            });
            break;
          case 'Role':
            await rbacApi.createNamespacedRole({
              namespace: manifest.metadata.namespace || namespace,
              body: manifest as unknown as k8s.V1Role,
            });
            break;
          case 'RoleBinding':
            await rbacApi.createNamespacedRoleBinding({
              namespace: manifest.metadata.namespace || namespace,
              body: manifest as unknown as k8s.V1RoleBinding,
            });
            break;
          case 'ClusterRole':
            await rbacApi.createClusterRole({
              body: manifest as unknown as k8s.V1ClusterRole,
            });
            break;
          case 'ClusterRoleBinding':
            await rbacApi.createClusterRoleBinding({
              body: manifest as unknown as k8s.V1ClusterRoleBinding,
            });
            break;
          case 'PersistentVolumeClaim':
            await coreApi.createNamespacedPersistentVolumeClaim({
              namespace: manifest.metadata.namespace || namespace,
              body: manifest as unknown as k8s.V1PersistentVolumeClaim,
            });
            break;
          case 'PersistentVolume':
            await coreApi.createPersistentVolume({
              body: manifest as unknown as k8s.V1PersistentVolume,
            });
            break;
          default:
            // For other resource types, use the generic KubernetesObjectApi
            await objectApi.create(manifest as k8s.KubernetesObject);
            break;
        }

        logger.info(
          { kind: manifest.kind, name: manifest.metadata?.name },
          'Manifest applied successfully',
        );
        return Success(undefined);
      } catch (error) {
        const guidance = extractK8sErrorGuidance(error, 'apply manifest');
        const errorMessage = `Failed to apply manifest: ${guidance.message}`;

        logger.error(
          {
            error: errorMessage,
            hint: guidance.hint,
            resolution: guidance.resolution,
            details: guidance.details,
          },
          'Manifest apply failed',
        );

        return Failure(errorMessage, guidance);
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
        const guidance = extractK8sErrorGuidance(error, 'get deployment status');
        const errorMessage = `Failed to get deployment status: ${guidance.message}`;

        logger.error(
          {
            error: errorMessage,
            hint: guidance.hint,
            resolution: guidance.resolution,
            details: guidance.details,
            namespace,
            name,
          },
          'Get deployment status failed',
        );

        return Failure(errorMessage, guidance);
      }
    },

    /**
     * Check cluster connectivity with timeout
     */
    async ping(): Promise<boolean> {
      try {
        // Use a shorter timeout for ping operations
        const pingTimeout = timeout || 5000;
        const timeoutPromise = new Promise<never>((_, reject) =>
          setTimeout(() => reject(new Error('Connection timeout')), pingTimeout),
        );

        await Promise.race([coreApi.listNamespace(), timeoutPromise]);
        return true;
      } catch (error) {
        const guidance = extractK8sErrorGuidance(error, 'ping cluster');
        logger.debug(
          {
            error: guidance.message,
            hint: guidance.hint,
            resolution: guidance.resolution,
          },
          'Cluster ping failed',
        );
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
          const response = (error as { response?: { statusCode?: number } }).response;
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
        const response = await authApi.createSelfSubjectAccessReview({
          body: accessReview as k8s.V1SelfSubjectAccessReview,
        });
        return response.status?.allowed === true;
      } catch (error) {
        logger.warn({ namespace, error }, 'Error checking permissions');
        // If we can't check permissions, assume we have them
        return true;
      }
    },

    /**
     * Check if an ingress controller is installed
     * Simplified for single-app scenarios - checks for IngressClass resources
     */
    async checkIngressController(): Promise<boolean> {
      try {
        // Check for IngressClass resources as primary indicator
        const ingressClasses = await networkingApi.listIngressClass();
        if (ingressClasses.items.length > 0) {
          logger.debug({ count: ingressClasses.items.length }, 'Found ingress classes');
          return true;
        }

        // Fallback: check for common ingress controller in kube-system
        const deployments = await k8sApi.listNamespacedDeployment({ namespace: 'kube-system' });
        const hasIngress = deployments.items.some(
          (d) =>
            d.metadata?.name?.includes('ingress') ||
            d.metadata?.name?.includes('nginx') ||
            d.metadata?.name?.includes('traefik'),
        );
        if (hasIngress) {
          logger.debug({ namespace: 'kube-system' }, 'Found ingress controller');
          return true;
        }

        return false;
      } catch (error) {
        logger.debug({ error }, 'Error checking for ingress controller');
        return false;
      }
    },

    /**
     * Ensure namespace exists (idempotent)
     * Creates the namespace if it doesn't exist, otherwise does nothing
     */
    async ensureNamespace(namespace: string): Promise<Result<void>> {
      try {
        // Check if namespace already exists
        const exists = await this.namespaceExists(namespace);
        if (exists) {
          logger.debug({ namespace }, 'Namespace already exists');
          return Success(undefined);
        }

        // Create namespace
        logger.debug({ namespace }, 'Creating namespace');
        const namespaceManifest: K8sManifest = {
          apiVersion: 'v1',
          kind: 'Namespace',
          metadata: {
            name: namespace,
          },
        };

        await coreApi.createNamespace({
          body: namespaceManifest as unknown as k8s.V1Namespace,
        });

        logger.info({ namespace }, 'Namespace created successfully');
        return Success(undefined);
      } catch (error) {
        // Handle 409 Conflict (AlreadyExists) as success to maintain idempotency
        if (error && typeof error === 'object' && 'response' in error) {
          const response = (error as { response?: { statusCode?: number } }).response;
          if (response?.statusCode === 409) {
            logger.debug({ namespace }, 'Namespace already exists (created by another process)');
            return Success(undefined);
          }
        }

        const guidance = extractK8sErrorGuidance(error, 'ensure namespace');
        const errorMessage = `Failed to ensure namespace exists: ${guidance.message}`;

        logger.error(
          {
            error: errorMessage,
            hint: guidance.hint,
            resolution: guidance.resolution,
            details: guidance.details,
            namespace,
          },
          'Ensure namespace failed',
        );

        return Failure(errorMessage, guidance);
      }
    },

    /**
     * Wait for deployment to be ready with polling
     */
    async waitForDeploymentReady(
      namespace: string,
      name: string,
      timeoutSeconds: number,
    ): Promise<Result<DeploymentResult>> {
      try {
        const startTime = Date.now();
        const maxWaitTime = timeoutSeconds * 1000;

        logger.debug({ namespace, name, timeoutSeconds }, 'Waiting for deployment to be ready');

        while (Date.now() - startTime < maxWaitTime) {
          const statusResult = await this.getDeploymentStatus(namespace, name);

          if (statusResult.ok && statusResult.value?.ready) {
            logger.info(
              {
                namespace,
                name,
                readyReplicas: statusResult.value.readyReplicas,
                elapsedSeconds: Math.round((Date.now() - startTime) / 1000),
              },
              'Deployment is ready',
            );
            return statusResult;
          }

          // Wait before checking again
          await new Promise((resolve) => setTimeout(resolve, DEPLOYMENT_POLL_INTERVAL_MS));
        }

        // Timeout reached
        const finalStatus = await this.getDeploymentStatus(namespace, name);
        const errorMessage = `Deployment did not become ready within ${timeoutSeconds} seconds. Check pod status and logs to diagnose deployment issues.`;

        logger.error(
          {
            namespace,
            name,
            timeoutSeconds,
            currentStatus: finalStatus.ok ? finalStatus.value : undefined,
          },
          errorMessage,
        );

        return Failure(errorMessage);
      } catch (error) {
        const guidance = extractK8sErrorGuidance(error, 'wait for deployment ready');
        const errorMessage = `Failed to wait for deployment: ${guidance.message}`;

        logger.error(
          {
            error: errorMessage,
            hint: guidance.hint,
            resolution: guidance.resolution,
            details: guidance.details,
            namespace,
            name,
          },
          'Wait for deployment failed',
        );

        return Failure(errorMessage, guidance);
      }
    },
  };
};
