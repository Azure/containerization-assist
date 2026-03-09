/**
 * Schema definition for generate-k8s-manifests tool
 */

import { z } from 'zod';
import { environment, platform, repositoryPath, type ToolNextAction } from '../shared/schemas';
import type { PolicyValidationResult } from '@/lib/policy-helpers';

export const generateK8sManifestsSchema = z
  .object({
    repositoryPath: repositoryPath
      .optional()
      .describe(
        'Absolute path to the repository root. Required when generating from repository analysis.',
      ),
    name: z
      .string()
      .optional()
      .describe('Module name. Defaults to directory basename of repositoryPath if not provided.'),
    modulePath: z
      .string()
      .optional()
      .describe('Absolute path to module root. Required when generating from repository analysis.'),
    // dockerfilePath removed — not used by k8s manifest generation
    targetPlatform: platform.describe(
      'Target platform for the Docker image (e.g., "linux/amd64", "linux/arm64"). Defaults to linux/amd64 for maximum compatibility.',
    ),
    language: z
      .string()
      .optional()
      .describe('Primary programming language (e.g., "java", "python")'),
    languageVersion: z.string().optional(),
    framework: z.string().optional().describe('Framework used (e.g., "spring", "django")'),
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
    // dependencies removed — use detectedDependencies instead
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
      .optional()
      .default('kubernetes')
      .describe('Type of manifest to generate (defaults to kubernetes)'),
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
      .describe(
        'Expected traffic level for dynamic defaults calculation (affects replica counts and scaling).',
      ),
    criticalityTier: z
      .enum(['tier-1', 'tier-2', 'tier-3'])
      .optional()
      .describe(
        'Criticality tier for dynamic defaults calculation (tier-1=mission-critical, tier-3=low-priority).',
      ),
  })
  // NOTE: zod-to-json-schema does not encode superRefine constraints into JSON Schema.
  // SDK/VS Code JSON Schema validation may accept inputs that Zod rejects at runtime
  // (e.g. providing both acaManifest and repositoryPath). Runtime validation catches these.
  .superRefine((data, ctx) => {
    const hasAcaManifest = !!data.acaManifest;
    const hasRepoPath = !!data.repositoryPath;

    // Require exactly one mode: ACA conversion OR repository analysis
    if (!hasAcaManifest && !hasRepoPath) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message:
          'Either provide acaManifest (for ACA conversion) or repositoryPath+modulePath (for repository analysis)',
        path: ['acaManifest'],
      });
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message:
          'Either provide acaManifest (for ACA conversion) or repositoryPath+modulePath (for repository analysis)',
        path: ['repositoryPath'],
      });
    } else if (hasAcaManifest && hasRepoPath) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message:
          'Provide either acaManifest (for ACA conversion) or repositoryPath+modulePath (for repository analysis), but not both',
        path: ['acaManifest'],
      });
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message:
          'Provide either acaManifest (for ACA conversion) or repositoryPath+modulePath (for repository analysis), but not both',
        path: ['repositoryPath'],
      });
    }

    // Repository mode requires repositoryPath and modulePath
    if (!hasAcaManifest && hasRepoPath) {
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
  severity?: 'high' | 'medium' | 'low';
  tags?: string[];
  matchScore: number;
  /** Indicates if this recommendation was injected by policy template */
  policyDriven?: boolean;
}

export interface RepositoryInfo {
  repositoryPath?: string | undefined;
  name: string | undefined;
  modulePath: string | undefined;
  language?: string | undefined;
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
  ports?: number[] | undefined;
  entryPoint?: string | undefined;
  targetPlatform?: string | undefined;
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
