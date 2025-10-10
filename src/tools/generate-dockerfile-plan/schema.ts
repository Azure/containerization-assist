/**
 * Schema definition for generate-dockerfile-plan tool
 */

import { z } from 'zod';
import { sessionId as sharedSessionId, environment, path as sharedPath } from '../shared/schemas';

export const generateDockerfilePlanSchema = z.object({
  sessionId: sharedSessionId
    .optional()
    .describe(
      'Session identifier to retrieve analysis results. If provided, uses analyze-repo data from session.',
    ),
  path: sharedPath.describe(
    'Repository path (use forward slashes: /path/to/repo). Required if sessionId not provided.',
  ),
  language: z.string().optional().describe('Primary programming language (e.g., "java", "python")'),
  framework: z.string().optional().describe('Framework used (e.g., "spring", "django")'),
  environment: environment.describe('Target environment (production, development, etc.)'),
  includeExamples: z
    .boolean()
    .optional()
    .default(true)
    .describe('Include code examples in recommendations'),
});

export type GenerateDockerfilePlanParams = z.infer<typeof generateDockerfilePlanSchema>;

export interface DockerfileRequirement {
  id: string;
  category: string;
  recommendation: string;
  example?: string;
  severity?: 'high' | 'medium' | 'low';
  tags?: string[];
  matchScore: number;
}

export interface DockerfilePlan {
  repositoryInfo: {
    path?: string;
    language?: string;
    framework?: string;
    languageVersion?: string;
    frameworkVersion?: string;
    buildSystem?: {
      type?: string;
      configFile?: string;
    };
    dependencies?: string[];
    ports?: number[];
    entryPoint?: string;
  };
  recommendations: {
    buildStrategy: {
      multistage: boolean;
      reason: string;
    };
    securityConsiderations: DockerfileRequirement[];
    optimizations: DockerfileRequirement[];
    bestPractices: DockerfileRequirement[];
  };
  knowledgeMatches: DockerfileRequirement[];
  confidence: number;
  summary: string;
}
