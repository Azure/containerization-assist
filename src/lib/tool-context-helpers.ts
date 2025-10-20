/**
 * Tool context helper utilities for standardized logger and timer setup
 * Reduces code duplication across tool implementations
 */

import type { Logger } from 'pino';
import type { ToolContext } from '@/mcp/context.js';
import { getToolLogger, createToolTimer, type Timer } from './tool-helpers.js';

/**
 * Combined logger and timer for tool execution
 */
export interface ToolExecutionContext {
  /** Logger for debugging and error tracking */
  logger: Logger;
  /** Timer for measuring execution time */
  timer: Timer;
  /** Original context for access to signal and progress */
  context: ToolContext;
}

/**
 * Set up standardized logger and timer for tool execution
 *
 * This helper consolidates the common pattern used across all tools:
 * - Get or create a logger from the context
 * - Create a timer for performance tracking
 *
 * @param context - The tool context passed to the tool handler
 * @param toolName - Name of the tool (e.g., 'analyze-repo', 'build-image')
 * @returns Object with logger and timer ready for use
 *
 * @example
 * ```typescript
 * async function handleMyTool(input: Input, context: ToolContext): Promise<Result<Output>> {
 *   const { logger, timer } = setupToolContext(context, 'my-tool');
 *
 *   timer.start();
 *   logger.info({ input }, 'Starting tool execution');
 *
 *   // ... tool logic
 *
 *   timer.end();
 *   return Success(result);
 * }
 * ```
 */
export function setupToolContext(
  context: ToolContext,
  toolName: string,
): { logger: Logger; timer: Timer } {
  const logger = getToolLogger(context, toolName);
  const timer = createToolTimer(logger, toolName);

  return { logger, timer };
}

/**
 * Alternative: Returns full context with logger and timer
 *
 * Use this when you need access to the original context along with
 * the standardized logger and timer.
 *
 * @param context - The tool context passed to the tool handler
 * @param toolName - Name of the tool (e.g., 'analyze-repo', 'build-image')
 * @returns Object with logger, timer, and original context
 *
 * @example
 * ```typescript
 * async function handleMyTool(input: Input, ctx: ToolContext): Promise<Result<Output>> {
 *   const { logger, timer, context } = createToolExecutionContext(ctx, 'my-tool');
 *
 *   timer.start();
 *
 *   // Access to context.signal, context.progress
 *   if (context.progress) {
 *     await context.progress('Starting work...');
 *   }
 *
 *   timer.end();
 *   return Success(result);
 * }
 * ```
 */
export function createToolExecutionContext(
  context: ToolContext,
  toolName: string,
): ToolExecutionContext {
  const { logger, timer } = setupToolContext(context, toolName);

  return {
    logger,
    timer,
    context,
  };
}
