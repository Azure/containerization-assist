import { z } from 'zod';
import { environmentSchema } from '@/config/constants';
import { platform } from '../shared/schemas';

export const prepareClusterSchema = z.object({
  environment: environmentSchema.optional(),
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
  useRemoteCluster: z
    .boolean()
    .optional()
    .default(false)
    .describe('Use a remote Kubernetes cluster instead of creating a local kind cluster'),
  clusterName: z
    .string()
    .optional()
    .describe('Name of the remote cluster (for display purposes)'),
  registry: z
    .object({
      type: z.enum(['acr', 'ecr', 'gcr', 'dockerhub', 'generic']).describe('Registry type'),
      url: z.string().describe('Registry URL (e.g., myregistry.azurecr.io)'),
    })
    .optional()
    .describe('Remote container registry configuration. User must be already logged in via docker login.'),
});

export type PrepareClusterParams = z.infer<typeof prepareClusterSchema>;
