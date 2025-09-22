/**
 * Simple Tool Executor - Direct execution without orchestration
 *
 * Purpose:
 * Provides a fast path for executing simple tools that don't require:
 * - Dependency resolution
 * - Complex policy enforcement
 * - Multi-step workflows
 * - Session management
 *
 * This is a pure function approach - no classes, no state, just execution.
 */

import { z } from 'zod';
import { type Result, Failure } from '@/types';
import type { Logger } from '@/lib/logger';
import type { RegisteredTool, ToolContext, ProgressReporter } from '@/app/types';

/**
 * Minimal progress reporter for simple tools
 */
function createMinimalProgress(logger: Logger): ProgressReporter {
  return {
    start: (message: string) => logger.info(`[START] ${message}`),
    update: (message: string, percentage?: number) => {
      if (percentage !== undefined) {
        logger.info(`[${percentage}%] ${message}`);
      } else {
        logger.info(`[UPDATE] ${message}`);
      }
    },
    complete: (message: string) => logger.info(`[COMPLETE] ${message}`),
    fail: (message: string) => logger.error(`[FAILED] ${message}`),
  };
}

/**
 * Execute a simple tool directly without orchestration
 *
 * @param tool - The tool to execute
 * @param params - Parameters for the tool
 * @param logger - Logger for tracking execution
 * @returns Result of the tool execution
 */
export async function executeSimpleTool(
  tool: RegisteredTool,
  params: unknown,
  logger: Logger,
): Promise<Result<unknown>> {
  const toolLogger = logger.child({ tool: tool.name });

  try {
    // 1. Validate parameters
    const validatedParams = await tool.schema.parseAsync(params);

    // 2. Create minimal context (no session, just basics)
    const context: ToolContext = {
      session: new Map<string, unknown>() as any, // Minimal session for tool compatibility
      logger: toolLogger,
      progress: createMinimalProgress(toolLogger),
    };

    // 3. Execute the tool
    toolLogger.info('Executing simple tool');
    const result = await tool.handler(validatedParams, context);

    // 4. Log result status
    if (result.ok) {
      toolLogger.info('Tool completed successfully');
    } else {
      toolLogger.warn(`Tool failed: ${result.error}`);
    }

    return result;
  } catch (error) {
    // Handle validation errors specially
    if (error instanceof z.ZodError) {
      const issues = error.issues.map((i) => `${i.path.join('.')}: ${i.message}`).join(', ');
      return Failure(`Validation failed: ${issues}`);
    }

    // Handle other errors
    const errorMessage = error instanceof Error ? error.message : String(error);
    toolLogger.error(`Tool execution failed: ${errorMessage}`);
    return Failure(errorMessage);
  }
}

/**
 * Check if a tool execution request qualifies for simple execution
 *
 * @param tool - The tool to check
 * @param params - Parameters for the tool
 * @returns true if the tool can be executed simply
 */
export function canExecuteSimply(tool: RegisteredTool, params: any): boolean {
  // Check if tool has dependencies
  if (tool.requires && tool.requires.length > 0) {
    return false;
  }

  // Check if this is a workflow or multi-step operation
  if (Array.isArray(params?.steps) || params?.workflow) {
    return false;
  }

  // Check if tool explicitly requires orchestration
  if (tool.requiresOrchestration) {
    return false;
  }

  // All checks passed - this is a simple tool
  return true;
}
