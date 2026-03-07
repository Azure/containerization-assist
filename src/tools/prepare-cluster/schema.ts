import { z } from 'zod';
import { environmentSchema } from '@/config/constants';
import { platform } from '../shared/schemas';

/**
 * Cluster type determines infrastructure setup behavior.
 * - `kind`: Create/manage a local Kind cluster with local Docker registry
 * - `generic`: Assume an existing cluster (AKS, EKS, GKE, minikube, etc.)
 */
export const clusterTypeSchema = z
  .enum(['kind', 'generic'])
  .optional()
  .describe(
    'Cluster type to prepare. "kind" creates a local Kind cluster with a local Docker registry. "generic" assumes an existing cluster (AKS, EKS, GKE, minikube, etc.). Defaults to inferring from environment if omitted.',
  );

export type ClusterType = z.infer<typeof clusterTypeSchema>;

export const prepareClusterSchema = z.object({
  clusterType: clusterTypeSchema,
  environment: environmentSchema
    .optional()
    .describe(
      'Target environment for knowledge filtering and policy context. Does not control cluster setup — use clusterType for that.',
    ),
  namespace: z.string().optional().describe('Kubernetes namespace'),
  targetPlatform: platform.describe(
    'Target platform for cluster validation. Ensures the cluster can run images built for this platform. Defaults to linux/amd64.',
  ),
  strictPlatformValidation: z
    .boolean()
    .optional()
    .default(true)
    .describe(
      'Fail if cluster architecture does not match target platform. When true, prevents deployment to incompatible clusters. Set to false to allow emulation (may have performance impact).',
    ),
});

export type PrepareClusterParams = z.infer<typeof prepareClusterSchema>;
