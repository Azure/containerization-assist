/**
 * Schema definition for generate-dockerfile-plan tool
 */

import { z } from 'zod';
import {
  sessionId as sharedSessionId,
  environment,
  respositoryPathAbsoluteUnix,
} from '../shared/schemas';
import { ModuleInfo } from '../analyze-repo/schema';

export const generateDockerfilePlanSchema = z.object({
  sessionId: sharedSessionId.optional().describe('Session identifier for tracking operations'),
  respositoryPathAbsoluteUnix: respositoryPathAbsoluteUnix.describe(
    'Repository path (use forward slashes: /path/to/repo).',
  ),
  modulePathAbsoluteUnix: z
    .string()
    .optional()
    .describe(
      'Module path for monorepo/multi-module projects to locate where the Dockerfile should be generated (use forward slashes).',
    ),
  language: z.string().optional().describe('Primary programming language (e.g., "java", "python")'),
  framework: z.string().optional().describe('Framework used (e.g., "spring", "django")'),
  environment: environment.describe('Target environment (production, development, etc.)'),
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
  repositoryInfo: ModuleInfo;
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
