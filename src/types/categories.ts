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
}

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

// Removed unused helper functions and constants:
// - getQualityGrade - not used
// - getSecurityGrade - not used
// - ENVIRONMENT_PROFILES - not used
// These were flagged by knip as unused exports

/**
 * Tool category mappings for all available tools (internal use only)
 */
const TOOL_CATEGORIES: Record<string, ToolCategory> = {
  // Docker tools
  'build-image': 'docker',
  'fix-dockerfile': 'docker',
  'generate-dockerfile': 'docker',
  'push-image': 'docker',
  'tag-image': 'docker',
  'validate-dockerfile': 'docker',

  // Kubernetes tools
  'generate-k8s-manifests': 'kubernetes',
  deploy: 'kubernetes',
  'prepare-cluster': 'kubernetes',
  'verify-deploy': 'kubernetes',

  // Analysis tools
  'analyze-repo': 'analysis',

  // Security tools
  'scan-image': 'security',

  // Utility tools
  ops: 'utility',
} as const;

/**
 * Get category for a tool by name
 */
export function getToolCategory(toolName: string): ToolCategory {
  return TOOL_CATEGORIES[toolName] ?? 'utility';
}
