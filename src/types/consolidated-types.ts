/**
 * Consolidated types for validation and testing results
 * Centralizes duplicate type definitions found across the codebase
 */

// Re-export validation types from the canonical source
export type {
  ValidationResult,
  ValidationReport,
  ValidationSeverity,
  ValidationGrade,
  ValidationCategory,
} from '@/validation/core-types.js';

// Import for local use
import type { ValidationResult } from '@/validation/core-types.js';

// Test-specific result types
export interface TestResult {
  success: boolean;
  duration: number;
  message?: string;
  details?: Record<string, unknown>;
  performance?: PerformanceMetrics;
}

export interface PerformanceMetrics {
  responseTime: number;
  memoryUsage?: number;
  cpuUsage?: number;
}

// Build result types
export interface BuildResult {
  success: boolean;
  imageId?: string;
  imageTag: string;
  buildLog: string;
  error?: string;
}

// Deployment result types
export interface DeployResult {
  success: boolean;
  resources: Array<{
    kind: string;
    name: string;
    namespace: string;
    status: string;
  }>;
  error?: string;
}

// ValidationResult now imported from canonical source

// Kubernetes manifest type
export interface K8sManifest {
  apiVersion: string;
  kind: string;
  metadata: {
    name: string;
    namespace?: string;
    [key: string]: unknown;
  };
  spec?: Record<string, unknown>;
}

// Extended validation result for Kubernetes utilities
export interface KubernetesValidationResult extends ValidationResult {
  manifest: K8sManifest;
}

// Re-export extended test infrastructure types for convenience
export type {
  TestInfrastructureResult,
  TestInfrastructurePerformanceMetrics,
} from '@/../../test/integration/mcp-inspector/infrastructure/test-runner.js';

// Re-export extended Docker types
export type { DockerBuildResult } from '@/../../test/integration/mcp-inspector/lib/docker-utils.js';

// Re-export extended Kubernetes types
export type { KubernetesDeployResult } from '@/../../test/integration/mcp-inspector/lib/kubernetes-utils.js';
