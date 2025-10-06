import { z } from 'zod';

export const verifyDeploymentSchema = z.object({
  sessionId: z.string().optional().describe('Session identifier for tracking operations'),
  deploymentName: z.string().describe('Deployment name to verify (required)'),
  namespace: z.string().optional().describe('Kubernetes namespace'),
  checks: z
    .array(z.enum(['pods', 'services', 'ingress', 'health']))
    .optional()
    .describe('Checks to perform'),
});

export type VerifyDeploymentParams = z.infer<typeof verifyDeploymentSchema>;
