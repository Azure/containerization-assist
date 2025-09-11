/**
 * Standardized Logger Utility
 *
 * Wrapper around Pino logger with helper functions
 * Avoids creating yet another logger interface - just use Pino!
 */

import pino from 'pino';
import { extractErrorMessage } from './error-utils';

export type { Logger } from 'pino';

/**
 * Create a Pino logger with sensible defaults for containerization assist
 */
export function createLogger(options: pino.LoggerOptions = {}): pino.Logger {
  // When running as MCP server, output to stderr to avoid interfering with JSON-RPC protocol
  const isMCPMode = process.env.MCP_MODE === 'true' || process.argv.includes('--mcp');
  const transport = isMCPMode
    ? pino.transport({
        target: 'pino/file',
        options: { destination: 2 }, // 2 is stderr
      })
    : undefined;

  return pino(
    {
      name: 'containerization-assist',
      level: process.env.LOG_LEVEL ?? (process.env.NODE_ENV === 'development' ? 'debug' : 'info'),
      ...options,
    },
    transport,
  );
}

/**
 * Performance timer interface
 */
export interface Timer {
  end: (additionalContext?: Record<string, unknown>) => void;
  error: (error: unknown, additionalContext?: Record<string, unknown>) => void;
  checkpoint: (label: string, additionalContext?: Record<string, unknown>) => number;
}

/**
 * Create a performance timer for an operation - functional approach
 */
export function createTimer(
  logger: pino.Logger,
  operation: string,
  context: Record<string, unknown> = {},
): Timer {
  const startTime = Date.now();

  // Log start of operation
  logger.debug({ operation, ...context }, `Starting ${operation}`);

  return {
    end(additionalContext: Record<string, unknown> = {}): void {
      const duration = Date.now() - startTime;

      logger.info(
        {
          operation,
          duration_ms: duration,
          ...context,
          ...additionalContext,
        },
        `Completed ${operation} in ${duration}ms`,
      );
    },

    error(error: unknown, additionalContext: Record<string, unknown> = {}): void {
      const duration = Date.now() - startTime;

      logger.error(
        {
          operation,
          duration_ms: duration,
          error: extractErrorMessage(error),
          stack: error instanceof Error ? error.stack : undefined,
          ...context,
          ...additionalContext,
        },
        `Failed ${operation} after ${duration}ms`,
      );
    },

    checkpoint(label: string, additionalContext: Record<string, unknown> = {}): number {
      const elapsed = Date.now() - startTime;

      logger.debug(
        {
          operation,
          checkpoint: label,
          elapsed_ms: elapsed,
          ...context,
          ...additionalContext,
        },
        `${operation} checkpoint: ${label} at ${elapsed}ms`,
      );

      return elapsed;
    },
  };
}
