/**
 * Schema definition for generate-manifest-plan tool
 */

import { z } from 'zod';
import {
  sessionId as sharedSessionId,
  environment,
  repositoryPathAbsoluteUnix as sharedPath,
} from '../shared/schemas';
import { ModuleInfo, moduleInfo } from '../analyze-repo/schema';

export const generateManifestPlanSchema = z.object({
  ...moduleInfo.shape,
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
  repositoryInfo: ModuleInfo;
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
