/**
 * Kubernetes Client - Library Export
 *
 * Re-exports Kubernetes client functionality from infrastructure for lib/ imports
 */

// Re-export from infrastructure
export {
  createKubernetesClient,
  type KubernetesClient,
  type DeploymentResult,
} from '../infrastructure/kubernetes/client';
