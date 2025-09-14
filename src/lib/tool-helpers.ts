/**
 * Common helper utilities for MCP tools to reduce code duplication
 */

import { createLogger, createTimer, type Logger, type Timer } from './logger.js';
import type { ToolContext } from '../mcp/context.js';

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
 * Creates a timer for measuring tool execution time.
 * Consolidates timer creation and adds automatic cleanup.
 * Invariant: Timer is automatically ended on process exit if not ended manually
 *
 * @param logger - Logger instance for timer output
 * @param toolName - Name of the tool for timer identification
 * @returns Timer instance with auto-cleanup
 */
export function createToolTimer(logger: Logger, toolName: string): Timer {
  const timer = createTimer(logger, toolName);

  // Track if timer has been ended to avoid double-ending
  let ended = false;
  const originalEnd = timer.end.bind(timer);

  // Override end method to track state
  timer.end = () => {
    if (!ended) {
      ended = true;
      originalEnd();
    }
  };

  // Auto-cleanup on process exit
  const cleanup = (): void => {
    if (!ended) {
      ended = true;
      originalEnd();
    }
  };

  process.once('beforeExit', cleanup);
  process.once('exit', cleanup);

  return timer;
}

/**
 * Initialize both logger and timer for a tool in one call.
 * Common pattern used by most tools.
 *
 * @param context - The tool context
 * @param toolName - Name of the tool
 * @returns Object containing both logger and timer
 */
export function initializeToolInstrumentation(
  context: ToolContext,
  toolName: string,
): { logger: Logger; timer: Timer } {
  const logger = getToolLogger(context, toolName);
  const timer = createToolTimer(logger, toolName);
  return { logger, timer };
}
