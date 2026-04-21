import { z } from 'zod';
import { workspacePath } from '../shared/schemas';

export const verifyDeploySchema = z.object({
  deploymentName: z.string().describe('Deployment name to verify (required)'),
  workspacePath: workspacePath.optional(),
  namespace: z.string().optional().describe('Kubernetes namespace'),
  checks: z
    .array(z.enum(['pods', 'services', 'ingress', 'health']))
    .optional()
    .describe('Checks to perform'),
});

export type VerifyDeployParams = z.infer<typeof verifyDeploySchema>;
