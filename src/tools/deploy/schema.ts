/**
 * Schema definition for deploy tool
 */

import { z } from 'zod';

const sessionIdSchema = z.string().describe('Session identifier for tracking operations');

export const environmentSchema = z
  .enum(['development', 'staging', 'production', 'testing'])
  .optional()
  .describe('Target deployment environment');

export const deployApplicationSchema = z.object({
  sessionId: sessionIdSchema.optional(),
  imageId: z.string().optional().describe('Docker image to deploy'),
  namespace: z.string().optional().describe('Kubernetes namespace'),
  replicas: z.number().optional().describe('Number of replicas'),
  port: z.number().optional().describe('Application port'),
  environment: environmentSchema,
});

export type DeployApplicationParams = z.infer<typeof deployApplicationSchema>;
