/**
 * Error handling utilities for consistent error message extraction
 */

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
