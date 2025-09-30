/**
 * Common helper utilities for MCP tools to reduce code duplication
 */

import { createLogger, createTimer, type Logger, type Timer } from './logger.js';
import { logToolStart, logToolComplete, logToolFailure } from './runtime-logging.js';
import type { ToolContext } from '@/mcp/context.js';
import type { Result } from '@/types';

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

  // Auto-cleanup on process exit
  const cleanup = (): void => {
    if (!ended) {
      ended = true;
      originalEnd();
      // Remove listeners to prevent memory leaks
      process.removeListener('beforeExit', cleanup);
      process.removeListener('exit', cleanup);
    }
  };

  // Override end method to track state and clean up listeners
  timer.end = () => {
    if (!ended) {
      ended = true;
      originalEnd();
      // Remove listeners when timer ends normally
      process.removeListener('beforeExit', cleanup);
      process.removeListener('exit', cleanup);
    }
  };

  process.once('beforeExit', cleanup);
  process.once('exit', cleanup);

  return timer;
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

/**
 * Store tool results in sessionManager for cross-tool persistence.
 * Consolidates the repetitive pattern of creating/updating sessions.
 *
 * @param ctx - The tool context with sessionManager
 * @param sessionId - The session ID to store results under
 * @param toolName - Name of the tool (e.g., 'analyze-repo', 'build-image')
 * @param results - The results to store
 * @param metadata - Optional additional metadata to store
 * @returns Result indicating success or failure of storage operation
 *
 * @example
 * ```typescript
 * await storeToolResults(ctx, sessionId, 'analyze-repo', {
 *   language: 'Java',
 *   framework: 'Spring Boot'
 * }, { analyzedPath: '/path/to/repo' });
 * ```
 */
export async function storeToolResults(
  ctx: ToolContext,
  sessionId: string | undefined,
  toolName: string,
  results: Record<string, unknown>,
  metadata?: Record<string, unknown>,
): Promise<Result<void>> {
  if (!sessionId || !ctx.sessionManager) {
    return { ok: true, value: undefined }; // Not an error, just skip storage
  }

  const logger = ctx.logger || createLogger({ name: 'tool-helpers' });

  try {
    // Get existing session - it MUST exist before tool execution
    const sessionResult = await ctx.sessionManager.get(sessionId);
    if (!sessionResult.ok) {
      const error = `Session lookup failed: ${sessionResult.error}`;
      logger.error({ sessionId, toolName }, error);
      return { ok: false, error };
    }

    if (!sessionResult.value) {
      const error = `Session ${sessionId} does not exist. Sessions must be created before tool execution.`;
      logger.error({ sessionId, toolName }, error);
      return { ok: false, error };
    }

    // Get existing results to merge (don't overwrite existing results)
    const existingResults = sessionResult.value.results ? sessionResult.value.results : {};

    // Prepare update payload - merge new results with existing ones
    const updatePayload: {
      results: Record<string, unknown>;
      metadata?: Record<string, unknown>;
    } = {
      results: {
        ...existingResults,
        [toolName]: results,
      },
    };

    if (metadata) {
      updatePayload.metadata = metadata;
    }

    // Update session with merged results
    await ctx.sessionManager.update(sessionId, updatePayload);

    logger.info(
      { sessionId, toolName },
      'Stored tool results in sessionManager for cross-tool access',
    );

    return { ok: true, value: undefined };
  } catch (sessionError) {
    const errorMsg = sessionError instanceof Error ? sessionError.message : String(sessionError);
    logger.warn(
      { sessionId, toolName, error: errorMsg },
      'Failed to store tool results in sessionManager',
    );
    return { ok: false, error: `Failed to store results: ${errorMsg}` };
  }
}
