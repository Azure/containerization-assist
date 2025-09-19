/**
 * Schema definition for generate-dockerfile tool
 */

import { z } from 'zod';

const sessionIdSchema = z
  .string()
  .describe(
    'Session identifier for sharing data between tools. Use the sessionId from analyze-repo to leverage detailed analysis results.',
  );

export const environmentSchema = z
  .enum(['development', 'staging', 'production', 'testing'])
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
  path: z.string().describe('Repository path (use forward slashes: /path/to/repo)'),
  dockerfileDirectoryPaths: z
    .array(z.string())
    .nonempty()
    .optional()
    .describe(
      'List of paths in the repository to generate separate Dockerfiles (use forward slashes: /path/to/directory/where/dockerfile/will/be/placed/)',
    ),

  // Sampling options
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

/**
 * Single Dockerfile result schema
 */
export const SingleDockerfileOutputSchema = z.object({
  content: z.string().min(20).describe('Generated Dockerfile content'),
  path: z.string().describe('Path where the Dockerfile should be created'),
  moduleRoot: z.string().describe('Root directory of the module'),
  metadata: z
    .object({
      baseImage: z.string().describe('Base image used'),
      runtimeImage: z.string().optional().describe('Runtime image for multi-stage builds'),
      exposedPorts: z.array(z.number()).optional().describe('List of exposed ports'),
      hasHealthCheck: z.boolean().describe('Whether health check is included'),
      isMultiStage: z.boolean().describe('Whether multi-stage build is used'),
      optimizationStrategy: z
        .enum(['size', 'security', 'performance', 'balanced'])
        .optional()
        .describe('Applied optimization strategy'),
      securityLevel: z
        .enum(['basic', 'standard', 'strict'])
        .optional()
        .describe('Applied security level'),
      estimatedSize: z.string().optional().describe('Estimated final image size'),
    })
    .optional()
    .describe('Metadata about the generated Dockerfile'),
  recommendations: z.array(z.string()).optional().describe('Additional recommendations'),
});

/**
 * Output schema for generate-dockerfile tool (supports multiple modules)
 */
export const DockerfileOutputSchema = z.object({
  dockerfiles: z
    .array(SingleDockerfileOutputSchema)
    .describe('Generated Dockerfiles for each module'),
  count: z.number().describe('Number of Dockerfiles generated'),
  warnings: z.array(z.string()).optional().describe('Warnings encountered during generation'),
});

export type DockerfileOutput = z.infer<typeof DockerfileOutputSchema>;
