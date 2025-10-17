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
  | 'utility'; // General utilities
