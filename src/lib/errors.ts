/**
 * Error handling utilities and message templates
 *
 * Consolidates error utilities and centralized error messages
 * for consistent error handling across the application.
 */

import type { ErrorGuidance } from '@/types';

// ============================================================================
// Error Message Templates
// ============================================================================

/**
 * Centralized Error Messages
 *
 * Provides consistent error message templates across the application.
 * Uses template functions for parameterized messages.
 */
export const ERROR_MESSAGES = {
  // Tool-related errors
  TOOL_NOT_FOUND: (name: string) => `Tool not found: ${name}`,
  VALIDATION_FAILED: (issues: string) => `Validation failed: ${issues}`,

  // Policy-related errors
  POLICY_BLOCKED: (rules: string[]) =>
    `Blocked by policy rules: ${rules.join(', ')}\n` +
    `Tip: Review policy configuration or adjust enforcement level. See https://github.com/Azure/containerization-assist/blob/main/docs/policy-guide.md`,
  POLICY_VALIDATION_FAILED: (issues: string) =>
    `Policy validation failed: ${issues}\n` +
    `Tip: Check policy file syntax against schema. Available policies in policies/ directory.`,
  POLICY_LOAD_FAILED: (error: string) =>
    `Failed to load policy: ${error}\n` +
    `Tip: Verify policy file exists and is valid YAML. See https://github.com/Azure/containerization-assist/blob/main/docs/policy-guide.md for format.`,

  // Infrastructure-related errors
  DOCKER_OPERATION_FAILED: (operation: string, error: string) =>
    `Docker ${operation} failed: ${error}`,
  K8S_OPERATION_FAILED: (operation: string, error: string) =>
    `Kubernetes ${operation} failed: ${error}`,
  K8S_APPLY_FAILED: (kind: string, name: string, error: string) =>
    `Failed to apply ${kind}/${name}: ${error}`,

  // Execution errors

  // Generic templates
  OPERATION_FAILED: (operation: string, error: string) => `${operation} failed: ${error}`,
  RESOURCE_NOT_FOUND: (type: string, id: string) => `${type} not found: ${id}`,
} as const;

// ============================================================================
// Error Utilities
// ============================================================================

/**
 * Safely extracts error message from unknown error types.
 * Invariant: Always returns a string message
 */
export function extractErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
}

/**
 * Create error guidance with context
 */
export function createErrorGuidance(
  message: string,
  hint?: string,
  resolution?: string,
  details?: Record<string, unknown>,
): ErrorGuidance {
  const guidance: ErrorGuidance = { message };
  if (hint !== undefined) guidance.hint = hint;
  if (resolution !== undefined) guidance.resolution = resolution;
  if (details !== undefined) guidance.details = details;
  return guidance;
}
