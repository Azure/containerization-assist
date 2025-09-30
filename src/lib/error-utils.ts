/**
 * Error handling utilities for consistent error message extraction
 */

import type { ErrorGuidance } from '@/types';

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
 * Creates a formatted error message with optional context
 */
export function formatErrorMessage(context: string, error: unknown): string {
  const message = extractErrorMessage(error);
  return `${context}: ${message}`;
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
