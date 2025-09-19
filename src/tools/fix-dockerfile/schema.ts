import { z } from 'zod';

export const fixDockerfileSchema = z.object({
  sessionId: z.string().optional().describe('Session identifier for tracking operations'),
  dockerfile: z.string().optional().describe('Dockerfile content to fix'),
  error: z.string().optional().describe('Build error message to address'),
  issues: z.array(z.string()).optional().describe('Specific issues to fix'),
  targetEnvironment: z
    .enum(['development', 'staging', 'production', 'testing'])
    .optional()
    .describe('Target environment'),

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

export type FixDockerfileParams = z.infer<typeof fixDockerfileSchema>;

/**
 * Output schema for fix-dockerfile tool (AI-first)
 */
export const DockerfileFixOutputSchema = z.object({
  fixedDockerfile: z.string().describe('Fixed Dockerfile content'),
  changes: z
    .array(
      z.object({
        type: z
          .enum(['security', 'performance', 'best-practice', 'syntax', 'deprecation'])
          .describe('Type of change made'),
        description: z.string().describe('Description of the change'),
        before: z.string().describe('Content before the change'),
        after: z.string().describe('Content after the change'),
        impact: z.enum(['high', 'medium', 'low']).describe('Impact level of the change'),
      }),
    )
    .describe('List of changes made'),
  securityImprovements: z.array(z.string()).describe('List of security improvements applied'),
  performanceOptimizations: z
    .array(z.string())
    .describe('List of performance optimizations applied'),
  sizeReduction: z.string().nullable().describe('Size reduction techniques used or null if none'),
  warnings: z
    .array(z.string())
    .nullable()
    .describe('Warnings about potential issues or null if none'),
  isValid: z.boolean().describe('Whether the fixed Dockerfile is valid'),
  estimatedBuildTime: z.string().nullable().describe('Estimated build time or null if unknown'),
  estimatedImageSize: z.string().nullable().describe('Estimated image size or null if unknown'),
});

export type DockerfileFixOutput = z.infer<typeof DockerfileFixOutputSchema>;
