import { z } from 'zod';
import { environmentSchema } from '@/config/environment';

export const prepareClusterSchema = z.object({
  sessionId: z.string().optional().describe('Session identifier for tracking operations'),
  environment: environmentSchema.optional(),
  namespace: z.string().optional().describe('Kubernetes namespace'),
});

export type PrepareClusterParams = z.infer<typeof prepareClusterSchema>;
