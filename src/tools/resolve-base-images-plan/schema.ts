/**
 * Schema definition for resolve-base-images-plan tool
 */

import { z } from 'zod';
import { sessionId as sharedSessionId, environment } from '../shared/schemas';

export const resolveBaseImagesPlanSchema = z.object({
  sessionId: sharedSessionId.optional().describe('Session identifier for tracking operations'),
  technology: z
    .string()
    .describe('Technology stack to resolve (e.g., "node", "python", "java", "go", "rust")'),
  languageVersion: z
    .string()
    .optional()
    .describe('Specific language version (e.g., "20", "3.11", "21")'),
  framework: z.string().optional().describe('Framework used (e.g., "express", "django", "spring")'),
  buildSystem: z
    .string()
    .optional()
    .describe('Build system (e.g., "maven", "gradle", "npm", "pip")'),
  environment: environment.describe('Target environment (production, development, etc.)'),
});

export type ResolveBaseImagesPlanParams = z.infer<typeof resolveBaseImagesPlanSchema>;

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
 * Base image requirement from knowledge base
 */
export interface BaseImageRequirement {
  id: string;
  category: string;
  recommendation: string;
  tags?: string[] | undefined;
  matchScore: number;
}

/**
 * Complete base image plan with categorized recommendations
 */
export interface BaseImagePlan {
  repositoryInfo: {
    language: string;
    languageVersion?: string | undefined;
    framework?: string | undefined;
    buildSystem?: string | undefined;
  };
  recommendations: {
    /** Official images from Docker Hub (e.g., node:20-alpine) */
    officialImages: BaseImageRecommendation[];
    /** Distroless images for enhanced security */
    distrolessOptions: BaseImageRecommendation[];
    /** Security-hardened images (Chainguard, Wolfi, etc.) */
    securityHardened: BaseImageRecommendation[];
    /** Size-optimized images (alpine, slim variants) */
    sizeOptimized: BaseImageRecommendation[];
  };
  knowledgeMatches: BaseImageRequirement[];
  confidence: number;
  summary: string;
}
