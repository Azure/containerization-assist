/**
 * Schema definition for generate-k8s-manifests tool
 */

import { z } from 'zod';
import { environment, repositoryPath as sharedPath } from '../shared/schemas';
import { ModuleInfo, moduleInfo } from '../analyze-repo/schema';

export const generateK8sManifestsSchema = z
  .object({
    ...moduleInfo.shape,
    path: sharedPath
      .optional()
      .describe(
        'Repository path (automatically normalized to forward slashes on all platforms). Required when generating from repository analysis.',
      ),
    acaManifest: z
      .string()
      .optional()
      .describe(
        'Azure Container Apps manifest content to convert (YAML or JSON). Required when converting from ACA manifest.',
      ),
    manifestType: z
      .enum(['kubernetes', 'helm', 'aca', 'kustomize'])
      .describe('Type of manifest to generate'),
    environment: environment.describe('Target environment (production, development, etc.)'),
    detectedDependencies: z
      .array(z.string())
      .optional()
      .describe(
        'Detected libraries/frameworks/features from repository analysis (e.g., ["redis", "ef-core", "signalr", "mongodb", "health-checks"]). This helps match relevant knowledge entries.',
      ),
    includeComments: z
      .boolean()
      .optional()
      .default(true)
      .describe('Add helpful comments in the output (primarily for ACA conversions)'),
    namespace: z.string().optional().describe('Target Kubernetes namespace'),
  })
  .refine((data) => Boolean(data.path) !== Boolean(data.acaManifest), {
    message:
      'Provide either path (for repository analysis) or acaManifest (for ACA conversion), not both',
  });

export type GenerateK8sManifestsParams = z.infer<typeof generateK8sManifestsSchema>;

// Legacy export for compatibility

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
  repositoryInfo?: ModuleInfo;
  acaAnalysis?: {
    containerApps: Array<{
      name: string;
      containers: number;
      hasIngress: boolean;
      hasScaling: boolean;
      hasSecrets: boolean;
    }>;
    warnings: string[];
  };
  manifestType: 'kubernetes' | 'helm' | 'aca' | 'kustomize';
  recommendations: {
    fieldMappings?: ManifestRequirement[];
    securityConsiderations: ManifestRequirement[];
    resourceManagement?: ManifestRequirement[];
    bestPractices: ManifestRequirement[];
  };
  knowledgeMatches: ManifestRequirement[];
  confidence: number;
  summary: string;
}
