/**
 * Schema definition for fix-dockerfile-plan tool
 */

import { z } from 'zod';
import { environment } from '../shared/schemas';
import type { ValidationResult } from '@/validation/core-types';

export const fixDockerfilePlanSchema = z
  .object({
    dockerfile: z.string().optional().describe('Dockerfile content to analyze for fixes'),
    path: z.string().optional().describe('Path to Dockerfile file to analyze for fixes'),
    environment: environment.describe('Target environment (production, development, etc.)'),
  })
  .refine((data) => data.dockerfile || data.path, {
    message: "Either 'dockerfile' content or 'path' must be provided",
  });

export type FixDockerfilePlanParams = z.infer<typeof fixDockerfilePlanSchema>;

/**
 * Validation issue with associated fix recommendations
 */
export interface ValidationIssue extends ValidationResult {
  category?: 'security' | 'performance' | 'bestPractices';
  priority?: 'high' | 'medium' | 'low';
}

/**
 * Fix recommendation from knowledge base
 */
export interface FixRecommendation {
  id: string;
  issueId?: string; // Links to ValidationIssue ruleId
  category: 'security' | 'performance' | 'bestPractices';
  title: string;
  description: string;
  example?: string;
  priority: 'high' | 'medium' | 'low';
  effort?: 'low' | 'medium' | 'high';
  impact?: string;
  tags?: string[];
  matchScore: number;
}

/**
 * Structured plan for fixing Dockerfile issues
 */
export interface DockerfileFixPlan {
  /** Current issues found in the Dockerfile */
  currentIssues: {
    security: ValidationIssue[];
    performance: ValidationIssue[];
    bestPractices: ValidationIssue[];
  };

  /** Fix recommendations from knowledge base */
  fixes: {
    security: FixRecommendation[];
    performance: FixRecommendation[];
    bestPractices: FixRecommendation[];
  };

  /** Overall validation score (0-100) */
  validationScore: number;

  /** Validation grade (A-F) */
  validationGrade: 'A' | 'B' | 'C' | 'D' | 'F';

  /** Overall priority based on issue severity */
  priority: 'high' | 'medium' | 'low';

  /** Estimated impact of applying fixes */
  estimatedImpact: string;

  /** All knowledge matches from knowledge base */
  knowledgeMatches: FixRecommendation[];

  /** Confidence in the fix recommendations (0-1) */
  confidence: number;

  /** Human-readable summary */
  summary: string;
}
