import { z } from 'zod';

export const convertAcaToK8sSchema = z.object({
  // Just the essentials
  acaManifest: z
    .string()
    .min(1)
    .describe('Azure Container Apps manifest content to convert (required)'),
  sessionId: z.string().optional().describe('Session identifier for tracking operations'),
  namespace: z.string().optional().default('default').describe('Target Kubernetes namespace'),

  // Keep it simple - always output YAML, always include basic conversions
  includeComments: z
    .boolean()
    .optional()
    .default(true)
    .describe('Add helpful comments in the output'),
});

export type ConvertAcaToK8sParams = z.infer<typeof convertAcaToK8sSchema>;
