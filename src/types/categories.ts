/**
 * Enhanced type definitions for tool parameters and results
 * Provides strong typing for categories, environments, and quality metrics
 */

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
 * Security/quality grade ratings
 */
export type SecurityGrade = 'A' | 'B' | 'C' | 'D' | 'F';

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
