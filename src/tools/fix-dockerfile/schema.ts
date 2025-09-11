import { z } from 'zod';

export const fixDockerfileSchema = z.object({
  sessionId: z.string().optional().describe('Session identifier for tracking operations'),
  dockerfile: z.string().optional().describe('Dockerfile content to fix'),
  error: z.string().optional().describe('Build error message to address'),
  issues: z.array(z.string()).optional().describe('Specific issues to fix'),

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
