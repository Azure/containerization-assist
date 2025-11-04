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
});

export type PrepareClusterParams = z.infer<typeof prepareClusterSchema>;
