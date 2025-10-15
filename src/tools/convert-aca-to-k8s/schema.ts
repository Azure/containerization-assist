import { z } from 'zod';

export const convertAcaToK8sSchema = z.object({
  acaManifest: z
    .string()
    .min(1)
    .describe('Azure Container Apps manifest content to convert (required)'),
  namespace: z.string().optional().default('default').describe('Target Kubernetes namespace'),

  includeComments: z
    .boolean()
    .optional()
    .default(true)
    .describe('Add helpful comments in the output'),
});
