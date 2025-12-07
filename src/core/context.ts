/**
 * Core Tool Context
 *
 * Provides the foundational ToolContext interface and factory function
 * with zero MCP dependencies. This module is the single source of truth
 * for ToolContext - all other modules should import from here.
 *
 * Design Principle: Core layer has no dependencies on MCP or protocol-specific code.
 */

import type { Logger } from 'pino';
import type { RegoEvaluator } from '@/config/policy-rego';

// ===== TYPES =====

/**
 * Progress reporting function for tool execution feedback.
 *
 * Tools call this to report progress during long-running operations.
 * The implementation may forward these updates to various destinations
 * (console, MCP notifications, UI callbacks, etc.).
 *
 * @param message - Human-readable progress message
 * @param progress - Current progress value (optional)
 * @param total - Total progress value (optional)
 */
export type ProgressReporter = (
  message: string,
  progress?: number,
  total?: number,
) => Promise<void>;

/**
 * Core tool execution context.
 *
 * This interface defines what every tool receives during execution.
 * It provides access to logging, cancellation, progress reporting,
 * and optional policy evaluation.
 *
 * IMPORTANT: This interface has no MCP-specific types or dependencies.
 * MCP integration layers can extend or wrap this context as needed.
 */
export interface ToolContext {
  /**
   * Optional abort signal for cancellation support.
   * Tools should check this signal periodically for long-running operations.
   */
  signal?: AbortSignal;

  /**
   * Optional progress reporting function for user feedback.
   * Should be called at regular intervals during long operations.
   */
  progress?: ProgressReporter;

  /**
   * Logger for debugging and error tracking.
   * Required for all tools - use this for structured logging instead of console.log.
   */
  logger: Logger;

  /**
   * Optional Rego policy evaluator for tool self-validation.
   * Tools can use this to validate generated content against organizational policies.
   */
  policy?: RegoEvaluator;

  /**
   * Query policy for configuration data.
   *
   * Convenience method that wraps policy.queryConfig() with null-safety.
   * Returns null if no policy is configured.
   *
   * @param packageName - OPA package name to query (e.g., 'containerization.generation_config')
   * @param input - Input data for the query
   * @returns Configuration object from policy or null if no policy configured
   */
  queryConfig<T = unknown>(packageName: string, input: Record<string, unknown>): Promise<T | null>;
}

// ===== CONTEXT OPTIONS =====

/**
 * Options for creating a tool context.
 *
 * These are the core options that don't depend on MCP.
 * MCP-specific options (like sendNotification) are handled
 * in the MCP layer which extends these options.
 */
export interface ContextOptions {
  /** Optional abort signal for cancellation */
  signal?: AbortSignal;

  /** Optional progress reporter function */
  progress?: ProgressReporter;

  /** Optional Rego policy evaluator */
  policy?: RegoEvaluator;
}

// ===== CONTEXT FACTORY =====

/**
 * Create a ToolContext for tool execution.
 *
 * This is the core factory function that creates a minimal ToolContext
 * with no MCP dependencies. For MCP-aware context creation with
 * notification support, use the factory from '@/mcp/context' instead.
 *
 * @param logger - Pino logger instance for debugging and error tracking
 * @param options - Optional configuration for signal, progress, and policy
 * @returns Configured ToolContext ready for tool execution
 *
 * @example
 * ```typescript
 * import { createToolContext } from '@/core/context';
 * import { createLogger } from '@/lib/logger';
 *
 * const logger = createLogger({ name: 'my-tool' });
 * const ctx = createToolContext(logger, {
 *   signal: abortController.signal,
 *   progress: async (msg) => console.log(msg),
 * });
 *
 * const result = await myTool.handler(input, ctx);
 * ```
 */
export function createToolContext(logger: Logger, options: ContextOptions = {}): ToolContext {
  const { signal, progress, policy } = options;

  // Build context object, only including optional properties if defined
  const ctx: ToolContext = {
    logger,
    queryConfig: async <T = unknown>(
      packageName: string,
      input: Record<string, unknown>,
    ): Promise<T | null> => {
      if (!policy) {
        logger.debug({ packageName }, 'No policy configured, returning null for config query');
        return null;
      }
      return policy.queryConfig<T>(packageName, input);
    },
  };

  if (signal !== undefined) ctx.signal = signal;
  if (progress !== undefined) ctx.progress = progress;
  if (policy !== undefined) ctx.policy = policy;

  return ctx;
}
