/**
 * Schema definition for generate-dockerfile tool
 */

import { z } from 'zod';

const sessionIdSchema = z.string().describe('Session identifier for tracking operations');

export const environmentSchema = z
  .enum(['development', 'staging', 'production'])
  .optional()
  .describe('Target deployment environment');

export const optimizationSchema = z
  .enum(['size', 'security', 'performance', 'balanced'])
  .optional()
  .describe('Optimization strategy for containerization');

export const securityLevelSchema = z
  .enum(['basic', 'standard', 'strict'])
  .optional()
  .describe('Security level for container configuration');

export const generateDockerfileSchema = z.object({
  sessionId: sessionIdSchema.optional(),
  baseImage: z.string().optional().describe('Base Docker image to use'),
  environment: environmentSchema,
  optimization: z.union([optimizationSchema, z.boolean()]).optional(),
  securityLevel: securityLevelSchema,
  customCommands: z.array(z.string()).optional().describe('Custom Dockerfile commands'),
});

export type GenerateDockerfileParams = z.infer<typeof generateDockerfileSchema>;
