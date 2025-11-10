/**
 * Schema definition for generate-k8s-manifests tool
 */

import { z } from 'zod';
import { environment, type ToolNextAction } from '../shared/schemas';
import type { PolicyValidationResult } from '@/lib/policy-helpers';

export const generateK8sManifestsSchema = z
  .object({
    // Module info fields - required for repository mode, optional for ACA mode
    name: z
      .string()
      .optional()
      .describe('Module name. Required when generating from repository analysis.'),
    modulePath: z
      .string()
      .optional()
      .describe(
        'Absolute path to module root. Required when generating from repository analysis. Paths are automatically normalized to forward slashes on all platforms.',
      ),
    dockerfilePath: z.string().optional().describe('Path where the Dockerfile should be generated'),
    language: z
      .enum(['java', 'dotnet', 'javascript', 'typescript', 'python', 'rust', 'go', 'other'])
      .optional()
      .describe('Primary programming language used in the module'),
    languageVersion: z.string().optional(),
    frameworks: z
      .array(
        z.object({
          name: z.string().describe('Framework name (e.g., Spring Boot, Express, Flask)'),
          version: z.string().optional(),
        }),
      )
      .optional(),
    buildSystem: z
      .object({
        type: z.string().optional(),
        configFile: z.string().optional(),
      })
      .optional()
      .describe('Build system information'),
    dependencies: z
      .array(z.string())
      .optional()
      .describe('List of module dependencies including database drivers and system libraries'),
    ports: z.array(z.number()).optional(),
    entryPoint: z.string().optional(),

    // ACA conversion field
    acaManifest: z
      .string()
      .optional()
      .describe(
        'Azure Container Apps manifest content to convert (YAML or JSON). Required when converting from ACA manifest; omit when generating from repository analysis.',
      ),

    // Common fields
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
    trafficLevel: z
      .enum(['high', 'medium', 'low'])
      .optional()
      .describe('Expected traffic level for dynamic defaults calculation (affects replica counts and scaling).'),
    criticalityTier: z
      .enum(['tier-1', 'tier-2', 'tier-3'])
      .optional()
      .describe('Criticality tier for dynamic defaults calculation (tier-1=mission-critical, tier-3=low-priority).'),
  })
  .superRefine((data, ctx) => {
    const hasAcaManifest = !!data.acaManifest;
    const hasModuleInfo = !!data.name && !!data.modulePath;

    // Require exactly one mode: ACA conversion OR repository analysis
    if (!hasAcaManifest && !hasModuleInfo) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message:
          'Either provide acaManifest (for ACA conversion) or name+modulePath (for repository analysis)',
        path: ['acaManifest'],
      });
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message:
          'Either provide acaManifest (for ACA conversion) or name+modulePath (for repository analysis)',
        path: ['name'],
      });
    }

    // Repository mode requires both name and modulePath
    if (!hasAcaManifest) {
      if (!data.name) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: 'name is required when not using acaManifest',
          path: ['name'],
        });
      }
      if (!data.modulePath) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: 'modulePath is required when not using acaManifest',
          path: ['modulePath'],
        });
      }
    }
  });

export type GenerateK8sManifestsParams = z.infer<typeof generateK8sManifestsSchema>;

export interface ManifestRequirement {
  id: string;
  category: string;
  recommendation: string;
  example?: string;
  severity?: 'high' | 'medium' | 'low' | 'required';
  tags?: string[];
  matchScore: number;
  /** Indicates if this recommendation was injected by policy template */
  policyDriven?: boolean;
}

export interface RepositoryInfo {
  name: string | undefined;
  modulePath: string | undefined;
  dockerfilePath?: string | undefined;
  language?:
    | 'java'
    | 'dotnet'
    | 'javascript'
    | 'typescript'
    | 'python'
    | 'rust'
    | 'go'
    | 'other'
    | undefined;
  languageVersion?: string | undefined;
  frameworks?:
    | Array<{
        name: string;
        version?: string;
      }>
    | undefined;
  buildSystem?:
    | {
        type?: string;
        configFile?: string;
      }
    | undefined;
  dependencies?: string[] | undefined;
  ports?: number[] | undefined;
  entryPoint?: string | undefined;
}

export interface ManifestPlan {
  /** Next action directive - provides explicit guidance on what files to create/update */
  nextAction: ToolNextAction;
  repositoryInfo?: RepositoryInfo;
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
  policyValidation?: PolicyValidationResult;
}
