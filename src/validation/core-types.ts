/**
 * Core validation types and interfaces
 */

export interface ValidationResult {
  isValid: boolean; // Primary validation state
  errors: string[]; // Critical issues
  warnings?: string[]; // Non-critical issues
  ruleId?: string; // Rule identifier (for detailed validation)
  passed?: boolean; // Alias for isValid (needed for current implementation)
  message?: string; // Primary message (for simple validation)
  suggestions?: string[]; // Improvement suggestions
  confidence?: number; // AI validation confidence (0-1)
  metadata?: {
    // Optional metadata
    validationTime?: number;
    rulesApplied?: string[];
    severity?: ValidationSeverity;
    location?: string;
    aiEnhanced?: boolean;
  };
}

export interface ValidationReport {
  results: ValidationResult[];
  score: number; // 0-100
  grade: ValidationGrade;
  passed: number;
  failed: number;
  errors: number;
  warnings: number;
  info: number;
  timestamp: string;
}

export enum ValidationSeverity {
  ERROR = 'error', // Must fix - blocks deployment
  WARNING = 'warning', // Should fix - potential issues
  INFO = 'info', // Consider fixing - improvements
}

export type ValidationGrade = 'A' | 'B' | 'C' | 'D' | 'F';

export interface DockerfileValidationRule {
  id: string;
  name: string;
  description: string;
  check: (commands: any[]) => boolean; // Uses docker-file-parser Command[]
  message: string;
  severity: ValidationSeverity;
  fix?: string;
  category: ValidationCategory;
}

export interface KubernetesValidationRule {
  id: string;
  name: string;
  description: string;
  check: (manifest: any) => boolean; // Uses parsed YAML object
  message: string;
  severity: ValidationSeverity;
  fix?: string;
  category: ValidationCategory;
}

export enum ValidationCategory {
  SECURITY = 'security',
  PERFORMANCE = 'performance',
  BEST_PRACTICE = 'best-practice',
  COMPLIANCE = 'compliance',
  OPTIMIZATION = 'optimization',
}

export interface ValidationConfig {
  enabled: boolean;
  minScore: number;
  severityThreshold: ValidationSeverity;
  categories: ValidationCategory[];
}

// Validator interface removed - use direct functions instead
