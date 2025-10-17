/**
 * MCP Context - Tool execution environment abstraction
 *
 * Invariant: All tools receive consistent context interface
 * Trade-off: Abstraction overhead for tool isolation and testability
 * Design: Factory pattern enables context mocking in tests
 */

import type { Logger } from 'pino';
import { extractProgressReporter } from './context-helpers.js';

// ===== TYPES =====

/**
 * Progress reporting function
 * Forwards progress updates through MCP notifications
 */
export type ProgressReporter = (
  /** Progress message or step name */
  message: string,
  /** Current progress value */
  progress?: number,
  /** Total progress value */
  total?: number,
) => Promise<void>;

/**
 * Main context object passed to tools - Unified interface for all tool implementations
 */
export interface ToolContext {
  /**
   * Optional abort signal for cancellation support
   * Tools should check this signal periodically for long-running operations
   */
  signal: AbortSignal | undefined;

  /**
   * Optional progress reporting function for user feedback
   * Should be called at regular intervals during long operations
   */
  progress: ProgressReporter | undefined;

  /**
   * Logger for debugging and error tracking - Required for all tools
   * Use this for structured logging instead of console.log
   */
  logger: Logger;
}

// ===== PROGRESS HANDLING =====

// Re-export types and utilities from helpers
export type { EnhancedProgressReporter } from './context-helpers.js';
export { extractProgressToken, createProgressReporter } from './context-helpers.js';

// ===== CONTEXT CREATION =====

/**
 * Options for creating a tool context
 */
export interface ContextOptions {
  /** Optional abort signal for cancellation */
  signal?: AbortSignal;
  /** Optional progress reporter or request with progress token */
  progress?: ProgressReporter | unknown;
  /** MCP notification callback for progress updates */
  sendNotification?: (notification: unknown) => Promise<void>;
}

/**
 * Create a ToolContext for tool execution
 *
 * @param logger - Logger for debugging and error tracking
 * @param options - Optional configuration
 * @returns Configured ToolContext
 *
 * @example
 * ```typescript
 * const context = createToolContext(logger, {
 *   signal: abortController.signal,
 *   progress: async (msg) => console.log(msg),
 * });
 * ```
 */
export function createToolContext(
  logger: Logger,
  options: ContextOptions = {},
): ToolContext {
  const progressReporter = extractProgressReporter(
    options.progress,
    logger,
    options.sendNotification,
  );

  return {
    logger,
    signal: options.signal,
    progress: progressReporter,
  };
}

