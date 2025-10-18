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
import { applyResource as applyK8sResource } from './resource-operations';

export interface DeploymentResult {
  ready: boolean;
  readyReplicas: number;
  totalReplicas: number;
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
    pollIntervalMs?: number,
  ) => Promise<Result<DeploymentResult>>;
  ensureNamespace: (namespace: string) => Promise<Result<void>>;
  ping: () => Promise<boolean>;
  namespaceExists: (namespace: string) => Promise<boolean>;
  checkPermissions: (namespace: string) => Promise<boolean>;
  checkIngressController: () => Promise<boolean>;
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
  const authApi = kc.makeApiClient(k8s.AuthorizationV1Api);

  // Define helper functions that can be reused internally
  const checkNamespaceExists = async (namespace: string): Promise<boolean> => {
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
  };

  const fetchDeploymentStatus = async (
    namespace: string,
    name: string,
  ): Promise<Result<DeploymentResult>> => {
    try {
      const deployment = await k8sApi.readNamespacedDeployment({ name, namespace });

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
  };

  return {
    /**
     * Apply Kubernetes manifest (supports all resource types)
     * Creates a new resource. If the resource already exists (409 error), the operation succeeds idempotently.
     *
     * Uses the consolidated resource-operations module for idempotent apply.
     *
     * @param manifest - Kubernetes resource manifest to apply
     * @param namespace - Default namespace for namespaced resources (default: 'default')
     * @returns Success if resource was created or already exists, Failure otherwise
     */
    async applyManifest(manifest: K8sManifest, namespace = 'default'): Promise<Result<void>> {
      // Set namespace in metadata if not already set and not a cluster-scoped resource
      const isClusterScoped = ['Namespace', 'ClusterRole', 'ClusterRoleBinding'].includes(
        manifest.kind || '',
      );

      // Create a working copy to avoid mutating the input manifest
      const workingManifest = !isClusterScoped && !manifest.metadata.namespace
        ? {
            ...manifest,
            metadata: {
              ...manifest.metadata,
              namespace,
            },
          }
        : manifest;

      // Use the consolidated resource operations module
      const result = await applyK8sResource(kc, workingManifest, logger);

      if (!result.ok) {
        return Failure(result.error, result.guidance);
      }

      return Success(undefined);
    },

    /**
     * Get deployment status
     * Retrieves current status information for a deployment
     *
     * @param namespace - Kubernetes namespace containing the deployment
     * @param name - Deployment name
     * @returns Result with deployment readiness status and replica counts
     */
    async getDeploymentStatus(namespace: string, name: string): Promise<Result<DeploymentResult>> {
      return fetchDeploymentStatus(namespace, name);
    },

    /**
     * Check cluster connectivity with timeout
     * Tests connection to the Kubernetes API server
     *
     * @returns true if cluster is reachable, false otherwise
     */
    async ping(): Promise<boolean> {
      let timeoutId: NodeJS.Timeout | undefined;
      let timeoutCleared = false;

      // Helper function to cleanup timeout
      const cleanupTimeout = (): void => {
        timeoutCleared = true;
        if (timeoutId) {
          clearTimeout(timeoutId);
        }
      };

      try {
        // Use a shorter timeout for ping operations
        const pingTimeout = timeout || 5000;

        // Create timeout promise with proper cleanup handling
        const timeoutPromise = new Promise<never>((_, reject) => {
          timeoutId = setTimeout(() => {
            if (!timeoutCleared) {
              reject(new Error('Connection timeout'));
            }
          }, pingTimeout);
        }).catch((error) => {
          // Only propagate the error if the timeout wasn't cleared
          // This prevents unhandled rejection warnings when the API call succeeds
          if (!timeoutCleared) {
            throw error;
          }
        });

        try {
          await Promise.race([coreApi.listNamespace(), timeoutPromise]);
          return true;
        } finally {
          // Always clear the timeout to prevent hanging timers
          cleanupTimeout();
        }
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
      } finally {
        // Always clear the timeout to prevent hanging timers
        cleanupTimeout();
      }
    },

    /**
     * Check if namespace exists
     * Verifies if a namespace is present in the cluster
     *
     * @param namespace - Namespace name to check
     * @returns true if namespace exists, false otherwise
     */
    async namespaceExists(namespace: string): Promise<boolean> {
      return checkNamespaceExists(namespace);
    },

    /**
     * Check user permissions in namespace
     * Verifies if the current user has permission to create deployments in the specified namespace
     *
     * @param namespace - Kubernetes namespace to check permissions for
     * @returns true if user has permissions or if check fails (fail-open for single-user scenarios)
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
     * Verifies the cluster has an ingress controller available for routing external traffic
     *
     * @returns true if an ingress controller is detected, false otherwise
     */
    async checkIngressController(): Promise<boolean> {
      try {
        // Check for IngressClass resources as primary indicator
        const ingressClasses = await networkingApi.listIngressClass();
        if (ingressClasses.items.length > 0) {
          const hasDefault = ingressClasses.items.some(
            (ic) =>
              ic.metadata?.annotations?.['ingressclass.kubernetes.io/is-default-class'] === 'true',
          );
          logger.debug({ count: ingressClasses.items.length, hasDefault }, 'Found ingress classes');
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
     * Creates the namespace if it doesn't exist, otherwise does nothing.
     * Handles concurrent creation gracefully by treating 409 errors as success.
     *
     * @param namespace - Namespace name to create
     * @returns Success if namespace exists or was created, Failure on errors
     */
    async ensureNamespace(namespace: string): Promise<Result<void>> {
      try {
        // Check if namespace already exists
        const exists = await checkNamespaceExists(namespace);
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
     * Polls deployment status until it becomes ready or timeout is reached
     *
     * @param namespace - Kubernetes namespace containing the deployment
     * @param name - Deployment name
     * @param timeoutSeconds - Maximum wait time in seconds
     * @param pollIntervalMs - Optional polling interval in milliseconds (default: 5000ms)
     * @returns Result with deployment status on success, error on timeout or failure
     */
    async waitForDeploymentReady(
      namespace: string,
      name: string,
      timeoutSeconds: number,
      pollIntervalMs?: number,
    ): Promise<Result<DeploymentResult>> {
      try {
        const pollInterval = pollIntervalMs ?? DEPLOYMENT_POLL_INTERVAL_MS;
        const startTime = Date.now();
        const maxWaitTime = timeoutSeconds * 1000;

        logger.debug(
          { namespace, name, timeoutSeconds, pollInterval },
          'Waiting for deployment to be ready',
        );

        let lastStatusResult: Result<DeploymentResult> | undefined;

        while (Date.now() - startTime < maxWaitTime) {
          lastStatusResult = await fetchDeploymentStatus(namespace, name);

          if (lastStatusResult.ok && lastStatusResult.value?.ready) {
            logger.info(
              {
                namespace,
                name,
                readyReplicas: lastStatusResult.value.readyReplicas,
                elapsedSeconds: Math.round((Date.now() - startTime) / 1000),
              },
              'Deployment is ready',
            );
            return lastStatusResult;
          }

          // Wait before checking again
          await new Promise((resolve) => setTimeout(resolve, pollInterval));
        }

        // Timeout reached - use last status result to avoid redundant API call
        const errorMessage = `Deployment did not become ready within ${timeoutSeconds} seconds. Check pod status and logs to diagnose deployment issues.`;

        logger.error(
          {
            namespace,
            name,
            timeoutSeconds,
            currentStatus: lastStatusResult?.ok ? lastStatusResult.value : undefined,
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
