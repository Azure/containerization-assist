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
  runtimeImage: z.string().optional().describe('Runtime image for multi-stage builds'),
  environment: environmentSchema,
  optimization: z.union([optimizationSchema, z.boolean()]).optional(),
  multistage: z.boolean().optional().describe('Use multi-stage build pattern'),
  securityHardening: z.boolean().optional().describe('Apply security hardening practices'),
  includeHealthcheck: z.boolean().optional().describe('Include health check configuration'),
  customInstructions: z.string().optional().describe('Custom Dockerfile instructions to include'),
  optimizeSize: z.boolean().optional().describe('Optimize for smaller image size'),
  securityLevel: securityLevelSchema,
  customCommands: z.array(z.string()).optional().describe('Custom Dockerfile commands'),
  repoPath: z.string().optional().describe('Repository path'),
  moduleRoots: z
    .array(z.string())
    .min(1)
    .optional()
    .describe(
      'List of module root paths for generating separate Dockerfiles (defaults to root directory)',
    ),

  // Sampling options
  disableSampling: z
    .boolean()
    .optional()
    .describe('Disable multi-candidate sampling (sampling is enabled by default)'),
  maxCandidates: z
    .number()
    .min(1)
    .max(10)
    .optional()
    .describe('Maximum number of candidates to generate (1-10)'),
  earlyStopThreshold: z
    .number()
    .min(0)
    .max(100)
    .optional()
    .describe('Score threshold for early stopping (0-100)'),
  includeScoreBreakdown: z
    .boolean()
    .optional()
    .describe('Include detailed score breakdown in response'),
  returnAllCandidates: z
    .boolean()
    .optional()
    .describe('Return all candidates instead of just the winner'),
  useCache: z.boolean().optional().describe('Use caching for repeated requests'),
});

export type GenerateDockerfileParams = z.infer<typeof generateDockerfileSchema>;
