/**
 * Enhanced type definitions for tool parameters and results
 * Provides strong typing for categories, environments, and quality metrics
 */

import type { Environment } from '@/config/environment';

/**
 * Content categories for scoring and validation
 */
export type ContentCategory = 'dockerfile' | 'kubernetes' | 'security' | 'generic';

/**
 * Tool categories for grouping and organization
 */
export type ToolCategory =
  | 'docker' // Docker image operations (build, tag, push, fix)
  | 'kubernetes' // Kubernetes deployment & management
  | 'azure' // Azure-specific tools (ACA)
  | 'analysis' // Repository and code analysis
  | 'security' // Security scanning and validation
  | 'utility'; // General utilities and session management

/**
 * Tool category metadata for enhanced organization
 */
export interface ToolCategoryMetadata {
  category: ToolCategory;
  tags?: string[];
  aiDriven?: boolean;
}

/**
 * Re-export Environment type from unified module for backwards compatibility
 */
export type { Environment };

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

/**
 * Tool category mappings for all available tools
 */
export const TOOL_CATEGORIES: Record<string, ToolCategory> = {
  // Docker tools
  'build-image': 'docker',
  'fix-dockerfile': 'docker',
  'generate-dockerfile': 'docker',
  'push-image': 'docker',
  'tag-image': 'docker',
  'resolve-base-images': 'docker',

  // Kubernetes tools
  'generate-k8s-manifests': 'kubernetes',
  deploy: 'kubernetes',
  'prepare-cluster': 'kubernetes',
  'verify-deploy': 'kubernetes',
  'generate-helm-charts': 'kubernetes',

  // Azure-specific tools
  'generate-aca-manifests': 'azure',
  'convert-aca-to-k8s': 'azure',

  // Analysis tools
  'analyze-repo': 'analysis',

  // Security tools
  scan: 'security',

  // Utility tools
  'inspect-session': 'utility',
  ops: 'utility',
} as const;

/**
 * Get category for a tool by name
 */
export function getToolCategory(toolName: string): ToolCategory {
  return TOOL_CATEGORIES[toolName] ?? 'utility';
}
