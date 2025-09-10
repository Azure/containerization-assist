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

/**
 * Extracts stack trace from error if available
 */
export function extractStackTrace(error: unknown): string | undefined {
  if (error instanceof Error && error.stack) {
    return error.stack;
  }
  return undefined;
}

/**
 * Type guard to check if value is an Error instance
 */
export function isError(value: unknown): value is Error {
  return value instanceof Error;
}

/**
 * Wraps unknown error in Error instance if not already an Error
 */
export function ensureError(error: unknown): Error {
  if (error instanceof Error) {
    return error;
  }
  return new Error(String(error));
}
