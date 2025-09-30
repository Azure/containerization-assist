/**
 * Kubernetes Client - Library Export
 *
 * Re-exports Kubernetes client functionality from infrastructure for lib/ imports
 */

// Re-export from infrastructure
/** @public */
export {
  createKubernetesClient,
  type KubernetesClient,
  type KubernetesClientConfig,
  type DeploymentResult,
  type ClusterInfo,
} from '@/infra/kubernetes/client';

/** @public */
export {
  discoverKubeconfigPath,
  validateKubeconfig,
  discoverAndValidateKubeconfig,
  isInCluster,
  type KubeconfigInfo,
} from '@/infra/kubernetes/kubeconfig-discovery';
