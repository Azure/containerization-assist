import { z } from 'zod';

export const prepareClusterSchema = z.object({
  sessionId: z.string().optional().describe('Session identifier for tracking operations'),
  environment: z
    .enum(['development', 'staging', 'production', 'testing'])
    .optional()
    .describe('Target environment'),
  namespace: z.string().optional().describe('Kubernetes namespace'),
});

export type PrepareClusterParams = z.infer<typeof prepareClusterSchema>;
