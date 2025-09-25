/**
 * Schema definition for analyze-repo tool
 */

import { z } from 'zod';
import { sessionId, path, analysisOptions } from '../shared/schemas';

export const analyzeRepoSchema = z.object({
  sessionId: sessionId.optional(),
  path,
  ...analysisOptions,
  serviceRootsAbsolute: z
    .array(z.string())
    .describe(
      'List of absolute paths to service root directories within the repository (use forward slashes: /path/to/service)',
    ),
  dockerfilePaths: z
    .array(z.string())
    .optional()
    .describe(
      'List of Dockerfile paths for generating separate Dockerfiles (use forward slashes: /path/to/Dockerfile)',
    ),
  language: z.enum(['java', 'dotnet', 'other']).optional().describe('Primary programming language'),
});

export type AnalyzeRepoParams = z.infer<typeof analyzeRepoSchema>;
