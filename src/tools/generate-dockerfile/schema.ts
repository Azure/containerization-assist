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

const sessionIdSchema = sharedSessionId.describe('Session identifier for workflow tracking.');

const optimizationSchema = z
  .enum(['size', 'security', 'performance', 'balanced'])
  .optional()
  .describe('Optimization strategy for containerization');

const securityLevelSchema = z
  .enum(['basic', 'standard', 'strict'])
  .optional()
  .describe('Security level for container configuration');

const baseImagePreferenceSchema = z
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
  repositoryPath: z.string().describe('Repository path (use forward slashes: /path/to/repo).'),
  modules: z
    .array(
      z.object({
        name: z.string(),
        modulePath: z.string().describe('Module source code path'),
        dockerfilePath: z
          .string()
          .optional()
          .describe(
            'Path where the Dockerfile should be generated (use forward slashes). If not provided, defaults to the module path.',
          ),
        language: z.string().optional(),
        framework: z.string().optional(),
        languageVersion: z.string().optional(),
        frameworkVersion: z.string().optional(),
        dependencies: z.array(z.string()).optional(),
        ports: z.array(z.number()).optional(),
        entryPoint: z.string().optional(),
        buildSystem: z
          .object({
            type: z.string().optional(),
            configFile: z.string().optional(),
          })
          .passthrough()
          .optional(),
      }),
    )
    .optional()
    .describe(
      'Array of module information. To generate Dockerfiles for specific modules, pass only those modules in this array. Each module can specify a dockerfilePath for where to place the Dockerfile.',
    ),
  ...samplingOptions,
});

export type GenerateDockerfileParams = z.infer<typeof generateDockerfileSchema>;
