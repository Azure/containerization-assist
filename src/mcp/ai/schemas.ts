/**
 * AI Response Schemas
 *
 * Comprehensive Zod schemas for AI service responses
 */

import { z } from 'zod';

// Base validation enums and types (internal - not exported)
const ValidationSeveritySchema = z.enum(['error', 'warning', 'info']);
const ValidationCategorySchema = z.enum([
  'security',
  'performance',
  'best-practice',
  'compliance',
  'optimization',
]);
const ValidationGradeSchema = z.enum(['A', 'B', 'C', 'D', 'F']);

// Core validation schemas (internal - not exported)
const ValidationResultSchema = z.object({
  isValid: z.boolean(),
  errors: z.array(z.string()),
  warnings: z.array(z.string()).optional(),
  ruleId: z.string().optional(),
  passed: z.boolean().optional(),
  message: z.string().optional(),
  suggestions: z.array(z.string()).optional(),
  confidence: z.number().min(0).max(1).optional(),
  metadata: z
    .object({
      validationTime: z.number().optional(),
      rulesApplied: z.array(z.string()).optional(),
      severity: ValidationSeveritySchema.optional(),
      location: z.string().optional(),
      aiEnhanced: z.boolean().optional(),
      category: ValidationCategorySchema.optional(),
      fixSuggestion: z.string().optional(),
    })
    .optional(),
});

const _ValidationReportSchema = z.object({
  results: z.array(ValidationResultSchema),
  score: z.number().int().min(0).max(100),
  grade: ValidationGradeSchema,
  passed: z.number().int().min(0),
  failed: z.number().int().min(0),
  errors: z.number().int().min(0),
  warnings: z.number().int().min(0),
  info: z.number().int().min(0),
  timestamp: z.string(),
});

// Knowledge Enhancement Response Schema (internal schemas - not exported)
const EnhancementAreaSchema = z.object({
  area: z.string(),
  description: z.string(),
  impact: z.enum(['low', 'medium', 'high']),
});

const TechnicalDebtSchema = z.object({
  category: ValidationCategorySchema,
  description: z.string(),
  effort: z.enum(['low', 'medium', 'high']),
});

const KnowledgeEnhancementAnalysisSchema = z.object({
  improvementsSummary: z.string(),
  enhancementAreas: z.array(EnhancementAreaSchema).max(5),
  knowledgeSources: z.array(z.string()).max(8),
  bestPracticesApplied: z.array(z.string()).max(10),
  technicalDebt: z.array(TechnicalDebtSchema).optional(),
});

export const KnowledgeEnhancementResponseSchema = z.object({
  enhancedContent: z.string(),
  knowledgeApplied: z.array(z.string()).max(10),
  confidence: z.number().min(0).max(1),
  suggestions: z.array(z.string()).max(8),
  analysis: KnowledgeEnhancementAnalysisSchema,
});

// AI Enhancement Response Schema (internal schemas - not exported)
const EnhancementPrioritySchema = z.object({
  area: z.string(),
  severity: ValidationSeveritySchema,
  description: z.string(),
  impact: z.string(),
});

// TechnicalDebtSchema already declared above

const AIEnhancementAnalysisSchema = z.object({
  assessment: z.string(),
  riskLevel: z.enum(['low', 'medium', 'high', 'critical']),
  priorities: z.array(EnhancementPrioritySchema).max(5),
  technicalDebt: z.array(TechnicalDebtSchema).max(5).optional(),
});

export const AIEnhancementResponseSchema = z.object({
  suggestions: z.array(z.string()).max(10),
  fixes: z.string().optional(),
  analysis: AIEnhancementAnalysisSchema,
});

// AI Validation Response Schema
export const ValidationReportResponseSchema = z.object({
  passed: z.boolean(),
  results: z.array(ValidationResultSchema),
  summary: z.object({
    totalIssues: z.number().int().min(0),
    errorCount: z.number().int().min(0),
    warningCount: z.number().int().min(0),
    categories: z.record(z.string(), z.number().int().min(0)),
  }),
});

// Repository Analysis Schema (for analyze-repo tool) - internal schemas not exported
const RepositoryFrameworkSchema = z.object({
  name: z.string(),
  version: z.string().optional(),
  confidence: z.number().min(0).max(1),
  files: z.array(z.string()),
  features: z.array(z.string()).optional(),
});

const _RepositoryAnalysisSchema = z.object({
  projectType: z.string(),
  frameworks: z.array(RepositoryFrameworkSchema),
  languages: z.array(
    z.object({
      name: z.string(),
      percentage: z.number().min(0).max(100),
      files: z.array(z.string()),
    }),
  ),
  buildTools: z.array(z.string()),
  packageManagers: z.array(z.string()),
  containerization: z.object({
    hasDockerfile: z.boolean(),
    hasDockerCompose: z.boolean(),
    hasKubernetes: z.boolean(),
    suggestions: z.array(z.string()),
  }),
  recommendations: z.object({
    baseImages: z.array(z.string()),
    ports: z.array(z.number().int().positive()),
    environmentVariables: z.array(z.string()),
    buildSteps: z.array(z.string()),
  }),
});

// Docker Image Resolution Schema (internal schemas - not exported)
const BaseImageRecommendationSchema = z.object({
  image: z.string(),
  version: z.string(),
  reason: z.string(),
  confidence: z.number().min(0).max(1),
  security: z.object({
    vulnerabilities: z.number().int().min(0),
    lastUpdated: z.string(),
    severity: z.enum(['low', 'medium', 'high', 'critical']),
  }),
  size: z.object({
    compressed: z.string(),
    uncompressed: z.string(),
  }),
  compatibility: z.array(z.string()),
});

const _BaseImageResolutionSchema = z.object({
  recommendations: z.array(BaseImageRecommendationSchema).max(5),
  analysis: z.object({
    summary: z.string(),
    factors: z.array(z.string()),
    tradeoffs: z.array(
      z.object({
        factor: z.string(),
        description: z.string(),
      }),
    ),
  }),
});

// Dockerfile Generation Schema (internal schemas - not exported)
const DockerfileInstructionSchema = z.object({
  instruction: z.string(),
  value: z.string(),
  comment: z.string().optional(),
});

const _DockerfileGenerationSchema = z.object({
  dockerfile: z.string(),
  instructions: z.array(DockerfileInstructionSchema),
  explanation: z.object({
    summary: z.string(),
    keyDecisions: z.array(
      z.object({
        decision: z.string(),
        rationale: z.string(),
      }),
    ),
    optimizations: z.array(z.string()),
    securityConsiderations: z.array(z.string()),
  }),
  metadata: z.object({
    baseImage: z.string(),
    targetSize: z.string().optional(),
    buildTime: z.string().optional(),
    complexity: z.enum(['simple', 'moderate', 'complex']),
  }),
});

// Kubernetes Manifest Generation Schema
export const KubernetesResourceSchema = z.object({
  apiVersion: z.string(),
  kind: z.string(),
  metadata: z.object({
    name: z.string(),
    namespace: z.string().optional(),
    labels: z.record(z.string(), z.string()).optional(),
    annotations: z.record(z.string(), z.string()).optional(),
  }),
  spec: z.record(z.string(), z.any()).optional(),
});

export const KubernetesManifestGenerationSchema = z.object({
  manifests: z.array(KubernetesResourceSchema),
  explanation: z.object({
    summary: z.string(),
    resourceTypes: z.array(
      z.object({
        type: z.string(),
        purpose: z.string(),
        configuration: z.string(),
      }),
    ),
    networkingStrategy: z.string(),
    scalingStrategy: z.string(),
    securityFeatures: z.array(z.string()),
  }),
  deployment: z.object({
    steps: z.array(z.string()),
    verification: z.array(z.string()),
    rollbackPlan: z.array(z.string()),
  }),
});

// Type inference helpers
export type ValidationResult = z.infer<typeof ValidationResultSchema>;
export type ValidationReport = z.infer<typeof _ValidationReportSchema>;
export type KnowledgeEnhancementResponse = z.infer<typeof KnowledgeEnhancementResponseSchema>;
export type AIEnhancementResponse = z.infer<typeof AIEnhancementResponseSchema>;
export type ValidationReportResponse = z.infer<typeof ValidationReportResponseSchema>;
export type RepositoryAnalysis = z.infer<typeof _RepositoryAnalysisSchema>;
export type BaseImageResolution = z.infer<typeof _BaseImageResolutionSchema>;
export type DockerfileGeneration = z.infer<typeof _DockerfileGenerationSchema>;
export type KubernetesManifestGeneration = z.infer<typeof KubernetesManifestGenerationSchema>;

// Removed unused schema registry and getSchema function (flagged by knip)
