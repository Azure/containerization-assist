/**
 * Kubernetes Type Definitions
 *
 * Provides type-safe interfaces for Kubernetes API clients and operations
 */

import * as k8s from '@kubernetes/client-node';

/**
 * Union type of all standard K8s API clients
 */
export type StandardK8sApiClient =
  | k8s.CoreV1Api
  | k8s.AppsV1Api
  | k8s.BatchV1Api
  | k8s.NetworkingV1Api
  | k8s.RbacAuthorizationV1Api
  | k8s.AutoscalingV2Api
  | k8s.AuthorizationV1Api;

/**
 * Type-safe resource configuration for K8s API operations
 * Uses string for method names to avoid union type constraints
 */
export interface ResourceConfig {
  api: StandardK8sApiClient;
  createMethod: string;
  patchMethod: string;
  deleteMethod: string;
  readMethod: string;
  namespaced: boolean;
}

/**
 * Safely access a method on a K8s API client
 * This is needed because TypeScript doesn't allow string indexing on union types
 */
export function getApiMethod(api: StandardK8sApiClient, methodName: string): unknown {
  return (api as unknown as Record<string, unknown>)[methodName];
}

/**
 * Get typed API client for a specific K8s resource kind
 *
 * @param kc - Kubernetes config
 * @param kind - Resource kind (e.g., 'Deployment', 'Service')
 * @returns Resource configuration with typed API client
 */
export function getResourceConfig(kc: k8s.KubeConfig, kind: string): ResourceConfig | undefined {
  const coreApi = kc.makeApiClient(k8s.CoreV1Api);
  const appsApi = kc.makeApiClient(k8s.AppsV1Api);
  const networkingApi = kc.makeApiClient(k8s.NetworkingV1Api);
  const batchApi = kc.makeApiClient(k8s.BatchV1Api);
  const rbacApi = kc.makeApiClient(k8s.RbacAuthorizationV1Api);
  const autoscalingApi = kc.makeApiClient(k8s.AutoscalingV2Api);

  const resourceMap: Record<string, ResourceConfig> = {
    Namespace: {
      api: coreApi,
      createMethod: 'createNamespace',
      patchMethod: 'patchNamespace',
      deleteMethod: 'deleteNamespace',
      readMethod: 'readNamespace',
      namespaced: false,
    },
    Deployment: {
      api: appsApi,
      createMethod: 'createNamespacedDeployment',
      patchMethod: 'patchNamespacedDeployment',
      deleteMethod: 'deleteNamespacedDeployment',
      readMethod: 'readNamespacedDeployment',
      namespaced: true,
    },
    Service: {
      api: coreApi,
      createMethod: 'createNamespacedService',
      patchMethod: 'patchNamespacedService',
      deleteMethod: 'deleteNamespacedService',
      readMethod: 'readNamespacedService',
      namespaced: true,
    },
    ConfigMap: {
      api: coreApi,
      createMethod: 'createNamespacedConfigMap',
      patchMethod: 'patchNamespacedConfigMap',
      deleteMethod: 'deleteNamespacedConfigMap',
      readMethod: 'readNamespacedConfigMap',
      namespaced: true,
    },
    Secret: {
      api: coreApi,
      createMethod: 'createNamespacedSecret',
      patchMethod: 'patchNamespacedSecret',
      deleteMethod: 'deleteNamespacedSecret',
      readMethod: 'readNamespacedSecret',
      namespaced: true,
    },
    ServiceAccount: {
      api: coreApi,
      createMethod: 'createNamespacedServiceAccount',
      patchMethod: 'patchNamespacedServiceAccount',
      deleteMethod: 'deleteNamespacedServiceAccount',
      readMethod: 'readNamespacedServiceAccount',
      namespaced: true,
    },
    Ingress: {
      api: networkingApi,
      createMethod: 'createNamespacedIngress',
      patchMethod: 'patchNamespacedIngress',
      deleteMethod: 'deleteNamespacedIngress',
      readMethod: 'readNamespacedIngress',
      namespaced: true,
    },
    StatefulSet: {
      api: appsApi,
      createMethod: 'createNamespacedStatefulSet',
      patchMethod: 'patchNamespacedStatefulSet',
      deleteMethod: 'deleteNamespacedStatefulSet',
      readMethod: 'readNamespacedStatefulSet',
      namespaced: true,
    },
    DaemonSet: {
      api: appsApi,
      createMethod: 'createNamespacedDaemonSet',
      patchMethod: 'patchNamespacedDaemonSet',
      deleteMethod: 'deleteNamespacedDaemonSet',
      readMethod: 'readNamespacedDaemonSet',
      namespaced: true,
    },
    Job: {
      api: batchApi,
      createMethod: 'createNamespacedJob',
      patchMethod: 'patchNamespacedJob',
      deleteMethod: 'deleteNamespacedJob',
      readMethod: 'readNamespacedJob',
      namespaced: true,
    },
    CronJob: {
      api: batchApi,
      createMethod: 'createNamespacedCronJob',
      patchMethod: 'patchNamespacedCronJob',
      deleteMethod: 'deleteNamespacedCronJob',
      readMethod: 'readNamespacedCronJob',
      namespaced: true,
    },
    Role: {
      api: rbacApi,
      createMethod: 'createNamespacedRole',
      patchMethod: 'patchNamespacedRole',
      deleteMethod: 'deleteNamespacedRole',
      readMethod: 'readNamespacedRole',
      namespaced: true,
    },
    RoleBinding: {
      api: rbacApi,
      createMethod: 'createNamespacedRoleBinding',
      patchMethod: 'patchNamespacedRoleBinding',
      deleteMethod: 'deleteNamespacedRoleBinding',
      readMethod: 'readNamespacedRoleBinding',
      namespaced: true,
    },
    ClusterRole: {
      api: rbacApi,
      createMethod: 'createClusterRole',
      patchMethod: 'patchClusterRole',
      deleteMethod: 'deleteClusterRole',
      readMethod: 'readClusterRole',
      namespaced: false,
    },
    ClusterRoleBinding: {
      api: rbacApi,
      createMethod: 'createClusterRoleBinding',
      patchMethod: 'patchClusterRoleBinding',
      deleteMethod: 'deleteClusterRoleBinding',
      readMethod: 'readClusterRoleBinding',
      namespaced: false,
    },
    PersistentVolumeClaim: {
      api: coreApi,
      createMethod: 'createNamespacedPersistentVolumeClaim',
      patchMethod: 'patchNamespacedPersistentVolumeClaim',
      deleteMethod: 'deleteNamespacedPersistentVolumeClaim',
      readMethod: 'readNamespacedPersistentVolumeClaim',
      namespaced: true,
    },
    PersistentVolume: {
      api: coreApi,
      createMethod: 'createPersistentVolume',
      patchMethod: 'patchPersistentVolume',
      deleteMethod: 'deletePersistentVolume',
      readMethod: 'readPersistentVolume',
      namespaced: false,
    },
    HorizontalPodAutoscaler: {
      api: autoscalingApi,
      createMethod: 'createNamespacedHorizontalPodAutoscaler',
      patchMethod: 'patchNamespacedHorizontalPodAutoscaler',
      deleteMethod: 'deleteNamespacedHorizontalPodAutoscaler',
      readMethod: 'readNamespacedHorizontalPodAutoscaler',
      namespaced: true,
    },
  };

  return resourceMap[kind];
}
