import { z } from 'zod';
import { environmentSchema } from '@/config/constants';

export const prepareClusterSchema = z.object({
  environment: environmentSchema.optional(),
  namespace: z.string().optional().describe('Kubernetes namespace'),
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
