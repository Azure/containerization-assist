/**
 * Schema definition for generate-dockerfile tool
 */

import { z } from 'zod';
import { environment, repositoryPath } from '../shared/schemas';
import { ModuleInfo } from '../analyze-repo/schema';

export const generateDockerfileSchema = z.object({
  repositoryPath: repositoryPath.describe(
    'Repository path (automatically normalized to forward slashes on all platforms).',
  ),
  modulePath: z
    .string()
    .optional()
    .describe(
      'Module path for monorepo/multi-module projects to locate where the Dockerfile should be generated (automatically normalized to forward slashes).',
    ),
  language: z.string().optional().describe('Primary programming language (e.g., "java", "python")'),
  framework: z.string().optional().describe('Framework used (e.g., "spring", "django")'),
  environment: environment.describe('Target environment (production, development, etc.)'),
  detectedDependencies: z
    .array(z.string())
    .optional()
    .describe(
      'Detected libraries/frameworks/features from repository analysis (e.g., ["redis", "ef-core", "signalr", "mongodb", "health-checks"]). This helps match relevant knowledge entries.',
    ),
});

export type GenerateDockerfileParams = z.infer<typeof generateDockerfileSchema>;

// Legacy export for compatibility
export const generateDockerfilePlanSchema = generateDockerfileSchema;
export type GenerateDockerfilePlanParams = GenerateDockerfileParams;

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
