/**
 * Enhanced type definitions for tool parameters and results
 * Provides strong typing for categories, environments, and quality metrics
 */

/**
 * Content categories for scoring and validation
 */
export type ContentCategory = 'dockerfile' | 'kubernetes' | 'security' | 'generic';

/**
 * Deployment environment types
 */
export type Environment = 'development' | 'staging' | 'production' | 'testing';

/**
 * Security/quality grade ratings
 */
export type SecurityGrade = 'A' | 'B' | 'C' | 'D' | 'F';

/**
 * Quality metrics for scored content
 */
export interface QualityMetrics {
  score: number;
  grade: SecurityGrade;
  breakdown: Record<string, number>;
  issues: string[];
  recommendations: string[];
}

/**
 * Supported programming languages
 */
export type SupportedLanguage =
  | 'javascript'
  | 'typescript'
  | 'python'
  | 'java'
  | 'go'
  | 'rust'
  | 'ruby'
  | 'php'
  | 'dotnet'
  | 'unknown';

/**
 * Supported frameworks
 */
export type SupportedFramework =
  | 'express'
  | 'nestjs'
  | 'nextjs'
  | 'react'
  | 'vue'
  | 'angular'
  | 'django'
  | 'flask'
  | 'fastapi'
  | 'spring'
  | 'rails'
  | 'laravel'
  | 'aspnet-core'
  | 'blazor'
  | 'minimal-api';

/**
 * Base parameters common to all tools
 */
export interface BaseToolParams {
  sessionId?: string;
  environment?: Environment;
  includeDebugInfo?: boolean;
}

/**
 * Enhanced parameters for AI-powered tools
 */
export interface AIEnhancedParams extends BaseToolParams {
  disableSampling?: boolean;
  maxCandidates?: number;
  includeScoreBreakdown?: boolean;
  returnAllCandidates?: boolean;
}

/**
 * Quality assessment result
 */
export interface QualityAssessment {
  score: number;
  grade: SecurityGrade;
  breakdown?: Record<string, number>;
  validationErrors?: string[];
}

/**
 * Scoring comparison for before/after states
 */
export interface ScoringComparison {
  before?: QualityAssessment;
  after: QualityAssessment;
  improvement?: number;
  improvementPercentage?: number;
}

/**
 * Helper function to get quality grade from score
 */
export function getQualityGrade(score: number): SecurityGrade {
  if (score >= 90) return 'A';
  if (score >= 80) return 'B';
  if (score >= 70) return 'C';
  if (score >= 60) return 'D';
  return 'F';
}

/**
 * Helper function to get security grade for base images
 */
export function getSecurityGrade(image: string): SecurityGrade {
  const lowerImage = image.toLowerCase();
  if (lowerImage.includes('distroless')) return 'A';
  if (lowerImage.includes('alpine')) return 'B';
  if (lowerImage.includes('slim')) return 'C';
  if (lowerImage.includes(':latest')) return 'F';
  return 'D';
}

/**
 * Environment configuration profiles
 */
export const ENVIRONMENT_PROFILES = {
  development: {
    securityWeight: 0.2,
    performanceWeight: 0.3,
    debuggingWeight: 0.5,
  },
  staging: {
    securityWeight: 0.4,
    performanceWeight: 0.4,
    debuggingWeight: 0.2,
  },
  production: {
    securityWeight: 0.5,
    performanceWeight: 0.4,
    debuggingWeight: 0.1,
  },
  testing: {
    securityWeight: 0.3,
    performanceWeight: 0.2,
    debuggingWeight: 0.5,
  },
} as const;
