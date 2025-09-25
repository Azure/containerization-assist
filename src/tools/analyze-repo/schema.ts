/**
 * Schema definition for analyze-repo tool
 */

import { z } from 'zod';
import { sessionId, path, analysisOptions } from '../shared/schemas';

export const analyzeRepoSchema = z.object({
  sessionId: sessionId.optional(),
  path,
  ...analysisOptions,
  dockerfilePaths: z
    .array(z.string())
    .optional()
    .describe(
      'List of Dockerfile paths for generating separate Dockerfiles (use forward slashes: /path/to/Dockerfile)',
    ),
  language: z.enum(['java', 'dotnet', 'other']).optional().describe('Primary programming language'),
});

export type AnalyzeRepoParams = z.infer<typeof analyzeRepoSchema>;

export interface RepositoryAnalysis {
  language?: string;
  framework?: string;
  languageVersion?: string;
  frameworkVersion?: string;
  buildSystem?: {
    type?: string;
    configFile?: string;
    [key: string]: unknown;
  };
  dependencies?: string[];
  suggestedPorts?: number[];
  entryPoint?: string;
  [key: string]: unknown;
}
