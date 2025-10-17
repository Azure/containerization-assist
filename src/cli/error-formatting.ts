/**
 * Centralized error formatting for CLI commands
 * Ensures consistent error messages and exit behavior
 */

import type { Result } from '@/types/core';

/**
 * Standard error formatting for CLI commands
 */
function formatError(message: string, error?: unknown): string {
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
