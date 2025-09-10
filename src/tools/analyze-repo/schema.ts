/**
 * Schema definition for analyze-repo tool
 */

import { z } from 'zod';

const sessionIdSchema = z.string().describe('Session identifier for tracking operations');
export const repoPathSchema = z.string().describe('Path to the repository to analyze');

export const analyzeRepoSchema = z.object({
  sessionId: sessionIdSchema.optional(),
  repoPath: repoPathSchema.optional(),
  depth: z.number().optional().describe('Analysis depth (1-5)'),
  includeTests: z.boolean().optional().describe('Include test files in analysis'),
  securityFocus: z.boolean().optional().describe('Focus on security aspects'),
  performanceFocus: z.boolean().optional().describe('Focus on performance aspects'),
  moduleRoots: z.array(z.string()).optional().describe('Root directories of modules'),
  language: z.enum(['java', 'dotnet', 'other']).optional().describe('Primary programming language'),
});

export type AnalyzeRepoParams = z.infer<typeof analyzeRepoSchema>;
