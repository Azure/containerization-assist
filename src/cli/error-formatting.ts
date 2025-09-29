/**
 * Centralized error formatting for CLI commands
 * Ensures consistent error messages and exit behavior
 */

import type { Result } from '@/types/core';

/**
 * Narrow logger interface for tools - only exposes safe logging methods
 */
export type ToolLogger = {
  debug: (message: string, context?: Record<string, unknown>) => void;
  info: (message: string, context?: Record<string, unknown>) => void;
  warn: (message: string, context?: Record<string, unknown>) => void;
  error: (message: string, context?: Record<string, unknown>) => void;
};

/**
 * Standard error formatting for CLI commands
 */
export function formatError(message: string, error?: unknown): string {
  const prefix = '❌';
  const baseMessage = `${prefix} ${message}`;

  if (!error) {
    return baseMessage;
  }

  if (typeof error === 'string') {
    return `${baseMessage}: ${error}`;
  }

  if (error instanceof Error) {
    return `${baseMessage}: ${error.message}`;
  }

  return `${baseMessage}: ${String(error)}`;
}

/**
 * Format GitHub Actions annotation for CI environments
 */
export function formatGitHubAnnotation(
  type: 'error' | 'warning',
  message: string,
  file?: string,
): string {
  const filePrefix = file ? `file=${file}::` : '';
  return `::${type}::${filePrefix}${message}`;
}

/**
 * Handle Result errors consistently across CLI commands
 */
export function handleResultError<T>(result: Result<T>, message: string): never {
  if (result.ok) {
    throw new Error('Called handleResultError on successful result');
  }

  console.error(formatError(message, result.error));
  process.exit(1);
}

/**
 * Handle generic errors consistently across CLI commands
 */
export function handleGenericError(message: string, error?: unknown): never {
  console.error(formatError(message, error));
  process.exit(1);
}

/**
 * Handle GitHub Actions CI errors with annotations
 */
export function handleCIError(message: string, error?: unknown, file?: string): never {
  const formattedError = formatError(message, error);
  const annotation = formatGitHubAnnotation('error', message, file);

  console.error(annotation);
  console.error(formattedError);
  process.exit(1);
}
