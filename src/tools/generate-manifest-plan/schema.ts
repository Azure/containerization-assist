/**
 * Schema definition for generate-manifest-plan tool
 */

import { z } from 'zod';
import { sessionId as sharedSessionId, environment, path as sharedPath } from '../shared/schemas';

export const generateManifestPlanSchema = z.object({
  sessionId: sharedSessionId
    .optional()
    .describe(
      'Session identifier to retrieve analysis results. If provided, uses analyze-repo data from session.',
    ),
  path: sharedPath.describe(
    'Repository path (use forward slashes: /path/to/repo). Required if sessionId not provided.',
  ),
  manifestType: z
    .enum(['kubernetes', 'helm', 'aca', 'kustomize'])
    .describe('Type of manifest to generate'),
  language: z.string().optional().describe('Primary programming language (e.g., "java", "python")'),
  framework: z.string().optional().describe('Framework used (e.g., "spring", "django")'),
  environment: environment.describe('Target environment (production, development, etc.)'),
});

export type GenerateManifestPlanParams = z.infer<typeof generateManifestPlanSchema>;

export interface ManifestRequirement {
  id: string;
  category: string;
  recommendation: string;
  example?: string;
  severity?: 'high' | 'medium' | 'low' | 'required';
  tags?: string[];
  matchScore: number;
}

export interface ManifestPlan {
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
  manifestType: 'kubernetes' | 'helm' | 'aca' | 'kustomize';
  recommendations: {
    securityConsiderations: ManifestRequirement[];
    resourceManagement: ManifestRequirement[];
    bestPractices: ManifestRequirement[];
  };
  knowledgeMatches: ManifestRequirement[];
  confidence: number;
  summary: string;
}
