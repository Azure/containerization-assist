/**
 * Schema definition for generate-dockerfile tool
 */

import { z } from 'zod';
import {
  sessionId as sharedSessionId,
  environment,
  platform,
  samplingOptions,
} from '../shared/schemas';

const sessionIdSchema = sharedSessionId.describe(
  'Session identifier for sharing data between tools. Use the sessionId from analyze-repo to leverage detailed analysis results.',
);

export const optimizationSchema = z
  .enum(['size', 'security', 'performance', 'balanced'])
  .optional()
  .describe('Optimization strategy for containerization');

export const securityLevelSchema = z
  .enum(['basic', 'standard', 'strict'])
  .optional()
  .describe('Security level for container configuration');

export const baseImagePreferenceSchema = z
  .string()
  .optional()
  .describe(
    'Base image preference hint (e.g., "microsoft", "distroless", "alpine", "security-focused", "size-optimized")',
  );

export const generateDockerfileSchema = z.object({
  sessionId: sessionIdSchema.optional(),
  baseImage: z
    .string()
    .optional()
    .describe('Base Docker image to use (overrides automatic selection)'),
  runtimeImage: z.string().optional().describe('Runtime image for multi-stage builds'),
  baseImagePreference: baseImagePreferenceSchema,
  environment,
  optimization: z.union([optimizationSchema, z.boolean()]).optional(),
  preferAI: z
    .boolean()
    .optional()
    .describe('Force AI analysis even with high-confidence hardcoded detection'),
  multistage: z.boolean().optional().describe('Use multi-stage build pattern'),
  securityHardening: z.boolean().optional().describe('Apply security hardening practices'),
  includeHealthcheck: z.boolean().optional().describe('Include health check configuration'),
  customInstructions: z.string().optional().describe('Custom Dockerfile instructions to include'),
  optimizeSize: z.boolean().optional().describe('Optimize for smaller image size'),
  securityLevel: securityLevelSchema,
  customCommands: z.array(z.string()).optional().describe('Custom Dockerfile commands'),
  platform: platform.describe('Target platform (e.g., linux/amd64, linux/arm64, windows/amd64)'),
  path: z.string().describe('Repository path (use forward slashes: /path/to/repo)'),
  dockerfileDirectoryPaths: z
    .array(z.string())
    .nonempty()
    .describe(
      'List of paths in the repository to generate separate Dockerfiles (use forward slashes: /path/to/directory/where/dockerfile/will/be/placed/)',
    ),
  ...samplingOptions,
});

export type GenerateDockerfileParams = z.infer<typeof generateDockerfileSchema>;
