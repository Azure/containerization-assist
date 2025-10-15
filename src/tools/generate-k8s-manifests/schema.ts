/**
 * Schema definition for generate-k8s-manifests tool
 */

import { z } from 'zod';
import { environment, repositoryPath as sharedPath } from '../shared/schemas';
import { ModuleInfo, moduleInfo } from '../analyze-repo/schema';

export const generateK8sManifestsSchema = z.object({
  ...moduleInfo.shape,
  path: sharedPath.describe(
    'Repository path (automatically normalized to forward slashes on all platforms).',
  ),
  manifestType: z
    .enum(['kubernetes', 'helm', 'aca', 'kustomize'])
    .describe('Type of manifest to generate'),
  environment: environment.describe('Target environment (production, development, etc.)'),
});

export type GenerateK8sManifestsParams = z.infer<typeof generateK8sManifestsSchema>;

// Legacy export for compatibility
export const generateManifestPlanSchema = generateK8sManifestsSchema;
export type GenerateManifestPlanParams = GenerateK8sManifestsParams;

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
