/**
 * Common helper utilities for MCP tools to reduce code duplication
 */

import { createLogger, createTimer, type Logger, type Timer } from './logger.js';
import { logToolStart, logToolComplete, logToolFailure } from './runtime-logging.js';
import type { ToolContext } from '@/mcp/context.js';

// Re-export Timer type for use by consumers
export type { Timer };

/**
 * Gets or creates a logger for a tool.
 * Consolidates the pattern: context.logger || createLogger({ name: 'tool-name' })
 * Invariant: Always returns a valid logger instance
 *
 * @param context - The tool context that may contain a logger
 * @param toolName - Name of the tool for logger creation
 * @returns Logger instance from context or newly created
 */
export function getToolLogger(context: ToolContext, toolName: string): Logger {
  return context.logger || createLogger({ name: toolName });
}

/**
 * Creates a timer for measuring tool execution time with safeguards.
 * Consolidates timer creation pattern for consistent usage across tools.
 * Wraps the timer to prevent double end/error calls which can skew metrics.
 *
 * @param logger - Logger instance for timer output
 * @param toolName - Name of the tool for timer identification
 * @returns Timer instance with double-call protection
 */
export function createToolTimer(logger: Logger, toolName: string): Timer {
  const timer = createTimer(logger, toolName);
  let completed = false;

  return {
    end(additionalContext?: Record<string, unknown>): void {
      if (completed) return;
      completed = true;
      timer.end(additionalContext);
    },

    error(error: unknown, additionalContext?: Record<string, unknown>): void {
      if (completed) return;
      completed = true;
      timer.error(error, additionalContext);
    },

    checkpoint(label: string, additionalContext?: Record<string, unknown>): number {
      // Checkpoints are allowed even after completion for debugging
      return timer.checkpoint(label, additionalContext);
    },
  };
}

/**
 * Create a standardized tool execution tracker with automatic start/complete logging.
 * Uses consistent "Starting X" / "Completed X" format for Copilot transcripts.
 *
 * @param toolName - Name of the tool (e.g., 'build-image', 'generate-k8s-manifests')
 * @param params - Key parameters to log at start
 * @param logger - Logger instance
 * @returns Object with complete() and fail() methods for structured logging
 *
 * @example
 * ```typescript
 * const tracker = createStandardizedToolTracker('build-image', { path: './app', tags }, logger);
 * try {
 *   const result = await performBuild();
 *   tracker.complete({ imageId: result.imageId });
 *   return Success(result);
 * } catch (error) {
 *   tracker.fail(error);
 *   return Failure(error.message);
 * }
 * ```
 */
export function createStandardizedToolTracker(
  toolName: string,
  params: Record<string, unknown>,
  logger: Logger,
): {
  complete: (result: Record<string, unknown>) => void;
  fail: (error: string | Error, context?: Record<string, unknown>) => void;
} {
  const startTime = Date.now();
  logToolStart(toolName, params, logger);

  return {
    complete: (result: Record<string, unknown>) => {
      const duration = Date.now() - startTime;
      logToolComplete(toolName, result, logger, duration);
    },
    fail: (error: string | Error, context?: Record<string, unknown>) => {
      logToolFailure(toolName, error, logger, context);
    },
  };
}
