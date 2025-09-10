/**
 * Kubernetes infrastructure - External K8s client interface
 */

export {
  type KubernetesClient,
  createKubernetesClient,
  type DeploymentResult,
  type ClusterInfo,
} from './client';
