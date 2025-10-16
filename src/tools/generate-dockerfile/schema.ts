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

/**
 * Base image recommendation with details
 */
export interface BaseImageRecommendation {
  /** Full image name (e.g., "node:20-alpine") */
  image: string;
  /** Category of the image */
  category: 'official' | 'distroless' | 'security' | 'size';
  /** Reason for recommendation */
  reason: string;
  /** Estimated size in MB (if known) */
  size?: string | undefined;
  /** Security rating (if applicable) */
  securityRating?: 'high' | 'medium' | 'low' | undefined;
  /** Tags applied to this recommendation */
  tags?: string[] | undefined;
  /** Match score from knowledge base */
  matchScore: number;
}

/**
 * Analysis of an existing Dockerfile
 */
export interface DockerfileAnalysis {
  /** Base images found in the Dockerfile */
  baseImages: string[];
  /** Whether the Dockerfile uses multi-stage builds */
  isMultistage: boolean;
  /** Whether a HEALTHCHECK instruction exists */
  hasHealthCheck: boolean;
  /** Whether a non-root USER is set */
  hasNonRootUser: boolean;
  /** Total number of instructions */
  instructionCount: number;
  /** Complexity estimate based on structure */
  complexity: 'simple' | 'moderate' | 'complex';
  /** Security posture summary */
  securityPosture: 'good' | 'needs-improvement' | 'poor';
}

/**
 * Guidance for enhancing an existing Dockerfile
 */
export interface EnhancementGuidance {
  /** Elements to preserve from the existing Dockerfile */
  preserve: string[];
  /** Areas that should be improved */
  improve: string[];
  /** Missing features that should be added */
  addMissing: string[];
  /** Recommended enhancement strategy */
  strategy: 'minor-tweaks' | 'moderate-refactor' | 'major-overhaul';
}

export interface DockerfilePlan {
  repositoryInfo: ModuleInfo;
  recommendations: {
    buildStrategy: {
      multistage: boolean;
      reason: string;
    };
    baseImages: BaseImageRecommendation[];
    securityConsiderations: DockerfileRequirement[];
    optimizations: DockerfileRequirement[];
    bestPractices: DockerfileRequirement[];
  };
  knowledgeMatches: DockerfileRequirement[];
  confidence: number;
  summary: string;
  existingDockerfile?: {
    path: string;
    content: string;
    analysis: DockerfileAnalysis;
    guidance: EnhancementGuidance;
  };
}
