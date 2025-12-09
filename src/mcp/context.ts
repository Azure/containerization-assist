/**
 * MCP Context - Tool execution environment with MCP protocol support
 *
 * This module re-exports the core ToolContext interface and adds
 * MCP-specific functionality for progress notifications via the
 * MCP protocol.
 *
 * Design: MCP layer builds ON TOP of core layer, not the other way around.
 *
 * Invariant: All tools receive consistent context interface
 * Trade-off: Abstraction overhead for tool isolation and testability
 */

import type { Logger } from 'pino';
import type { RegoEvaluator } from '@/config/policy-rego';
import {
  createToolContext as createCoreContext,
  type ToolContext,
  type ProgressReporter,
  type ContextOptions as CoreContextOptions,
} from '@/core/context';
import { extractProgressReporter } from './context-helpers.js';

// ===== CORE RE-EXPORTS =====
// Canonical types - import from here for backward compatibility
// New code should import directly from '@/core/context'

export type { ToolContext, ProgressReporter };

/**
 * Re-exported as CoreContextOptions to distinguish from MCP-specific ContextOptions.
 * The core version only accepts ProgressReporter functions, while the MCP version
 * also accepts MCP request objects with progress tokens.
 */
export type { CoreContextOptions };

export { createCoreContext as createCoreToolContext };

// ===== MCP-SPECIFIC TYPES =====

/**
 * MCP request object that may contain a progress token.
 * This matches the shape of MCP protocol request structure.
 *
 * The progress token is located at `params._meta.progressToken` per MCP spec.
 * @see https://modelcontextprotocol.io/specification#progress
 */
export interface MCPProgressRequest {
  params?: {
    _meta?: {
      progressToken?: string | number;
    };
  };
}

/**
 * Progress input can be:
 * - A direct ProgressReporter function
 * - An MCP request object with optional progress token
 * - null or undefined for no progress reporting
 */
export type ProgressInput = ProgressReporter | MCPProgressRequest | null | undefined;

/**
 * MCP context options with notification support.
 *
 * This interface supports MCP-specific functionality for
 * progress notifications via the MCP protocol.
 *
 * Note: This does not extend CoreContextOptions because the progress
 * property has a different type (accepts MCP request objects).
 */
export interface ContextOptions {
  /** Optional abort signal for cancellation */
  signal?: AbortSignal;

  /**
   * Progress can be:
   * - A ProgressReporter function directly
   * - An MCP request object with progress token (will be extracted)
   * - undefined for no progress reporting
   */
  progress?: ProgressInput;

  /**
   * MCP notification callback for progress updates.
   * When provided, progress updates are sent via MCP protocol.
   */
  sendNotification?: (notification: unknown) => Promise<void>;

  /** Optional Rego policy evaluator to pass to tools */
  policy?: RegoEvaluator;
}

// ===== MCP CONTEXT FACTORY =====

/**
 * Create a ToolContext with optional MCP progress notification support.
 *
 * This factory extends the core createToolContext with MCP-specific
 * functionality. If sendNotification is provided, progress updates
 * are forwarded through the MCP protocol.
 *
 * @param logger - Pino logger instance
 * @param options - Context options including MCP-specific sendNotification
 * @returns ToolContext configured for MCP or core usage
 *
 * @example
 * ```typescript
 * // MCP usage with notifications
 * const ctx = createToolContext(logger, {
 *   signal: request.signal,
 *   progress: request.params,
 *   sendNotification: server.sendNotification,
 * });
 *
 * // Simple usage (no MCP)
 * const ctx = createToolContext(logger);
 * ```
 */
export function createToolContext(logger: Logger, options: ContextOptions = {}): ToolContext {
  const { sendNotification, progress, signal, policy } = options;

  // Extract progress reporter using MCP-aware helper
  // This handles both direct ProgressReporter functions and MCP request objects with progress tokens
  const progressReporter = extractProgressReporter(progress, logger, sendNotification);

  // Build options object explicitly (clearer than conditional spread)
  const coreOptions: CoreContextOptions = {};
  if (signal !== undefined) coreOptions.signal = signal;
  if (policy !== undefined) coreOptions.policy = policy;
  if (progressReporter !== undefined) coreOptions.progress = progressReporter;

  // Use core factory with extracted progress reporter
  return createCoreContext(logger, coreOptions);
}

// ===== MCP-SPECIFIC EXPORTS =====

// Progress handling utilities specific to MCP protocol
export type { EnhancedProgressReporter } from './context-helpers.js';
export { extractProgressToken, createProgressReporter } from './context-helpers.js';
