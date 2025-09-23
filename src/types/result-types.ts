/**
 * Result types for validation, testing, and operation outcomes
 * Keep this file runtime-safe and decoupled from test paths.
 */

import type { ValidationResult } from '@/validation/core-types';

// If you also need to re-export the other validation types, do so from the same local module:
export type {
  ValidationResult as CoreValidationResult,
  ValidationReport,
  ValidationSeverity,
  ValidationGrade,
  ValidationCategory,
} from '@/validation/core-types';

// Test-specific result types — keep these generic and runtime-safe.
// Do not import from test files here; tests can import from src, not the other way around.
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
