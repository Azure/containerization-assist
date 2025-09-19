/**
 * Schema definition for analyze-repo tool
 */

import { z } from 'zod';

const sessionIdSchema = z.string().describe('Session identifier for tracking operations');
export const pathSchema = z
  .string()
  .describe('Path to the repository to analyze (use forward slashes: /path/to/repo)');

export const analyzeRepoSchema = z.object({
  sessionId: sessionIdSchema.optional(),
  path: pathSchema,
  depth: z.number().optional().describe('Analysis depth (1-5)'),
  includeTests: z.boolean().optional().describe('Include test files in analysis'),
  securityFocus: z.boolean().optional().describe('Focus on security aspects'),
  performanceFocus: z.boolean().optional().describe('Focus on performance aspects'),
  dockerfilePaths: z
    .array(z.string())
    .nonempty()
    .optional()
    .describe(
      'List of Dockerfile paths for generating separate Dockerfiles (use forward slashes: /path/to/Dockerfile)',
    ),
  language: z.enum(['java', 'dotnet', 'other']).optional().describe('Primary programming language'),
});

export type AnalyzeRepoParams = z.infer<typeof analyzeRepoSchema>;
