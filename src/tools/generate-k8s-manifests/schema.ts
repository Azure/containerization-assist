import { z } from 'zod';

export const generateK8sManifestsSchema = z.object({
  sessionId: z.string().optional().describe('Session identifier for tracking operations'),
  appName: z.string().optional().describe('Application name'),
  imageId: z.string().optional().describe('Docker image to deploy'),
  replicas: z.number().optional().describe('Number of replicas'),
  port: z.number().optional().describe('Application port'),
  environment: z
    .enum(['development', 'staging', 'production'])
    .optional()
    .describe('Target environment'),
});

export type GenerateK8sManifestsParams = z.infer<typeof generateK8sManifestsSchema>;
