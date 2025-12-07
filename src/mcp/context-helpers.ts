/**
 * MCP Context Helper Functions
 *
 * Helper functions for progress handling and context utilities.
 */

import type { Logger } from 'pino';
import type { ProgressReporter } from '@/core/context';
import type { MCPProgressRequest, ProgressInput } from './context.js';

/**
 * Progress notification data structure
 */
export interface ProgressNotification {
  /** Unique token identifying this progress stream */
  progressToken: string;
  /** Human-readable progress message */
  message: string;
  /** Current progress value (optional) */
  progress?: number;
  /** Total progress value (optional) */
  total?: number;
  /** Additional metadata */
  metadata?: Record<string, unknown>;
}

/**
 * Enhanced progress reporter that forwards through MCP notifications
 */
export type EnhancedProgressReporter = (
  message: string,
  progress?: number,
  total?: number,
  metadata?: Record<string, unknown>,
) => Promise<void>;

// ===== TYPE GUARDS =====

/**
 * Type guard to check if value is a ProgressReporter function.
 */
function isProgressReporter(value: unknown): value is ProgressReporter {
  return typeof value === 'function';
}

/**
 * Type guard to check if value is an MCP progress request object.
 * Checks for the `params` property which contains `_meta.progressToken`.
 */
function isMCPProgressRequest(value: unknown): value is MCPProgressRequest {
  return typeof value === 'object' && value !== null && 'params' in value;
}

/**
 * Type guard to check if value is a record object
 */
function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/**
 * Extract progress token from MCP request
 *
 * Per MCP spec, progress token is in params._meta.progressToken
 * @see https://modelcontextprotocol.io/specification#progress
 */
export function extractProgressToken(request: unknown): string | undefined {
  if (!isRecord(request)) {
    return undefined;
  }

  const params = request.params;
  if (!isRecord(params)) {
    return undefined;
  }

  const meta = params._meta;
  if (!isRecord(meta)) {
    return undefined;
  }

  const token = meta.progressToken;
  return typeof token === 'string' ? token : undefined;
}

/**
 * Creates a progress reporter that forwards notifications through the MCP protocol
 */
export function createProgressReporter(
  progressToken?: string | number,
  logger?: Logger,
  sendNotification?: (notification: unknown) => Promise<void>,
): EnhancedProgressReporter | undefined {
  if (!progressToken) {
    return undefined;
  }

  return async (
    message: string,
    progress?: number,
    total?: number,
    metadata?: Record<string, unknown>,
  ) => {
    try {
      const notification: ProgressNotification = {
        progressToken: String(progressToken),
        message,
        ...(progress !== undefined && { progress }),
        ...(total !== undefined && { total }),
        ...(metadata && { metadata }),
      };

      await sendProgressNotification(notification, logger, sendNotification);
    } catch (error) {
      logger?.warn(
        {
          progressToken,
          message,
          error: error instanceof Error ? error.message : String(error),
        },
        'Failed to send progress notification',
      );
    }
  };
}

/**
 * Sends a progress notification through the MCP server using the proper MCP protocol.
 * Uses sendNotification callback if available (from request handler), otherwise falls back to logging.
 */
async function sendProgressNotification(
  notification: ProgressNotification,
  logger?: Logger,
  sendNotification?: (notification: unknown) => Promise<void>,
): Promise<void> {
  // If we have a real sendNotification callback from the MCP request handler, use it
  if (sendNotification) {
    try {
      await sendNotification({
        method: 'notifications/progress',
        params: {
          progressToken: notification.progressToken,
          progress: notification.progress || 0,
          ...(notification.total !== undefined && { total: notification.total }),
          ...(notification.message && { message: notification.message }),
          ...(notification.metadata && { ...notification.metadata }),
        },
      });

      logger?.debug(
        {
          progressToken: notification.progressToken,
          message: notification.message,
          progress: notification.progress,
          total: notification.total,
          type: 'progress_notification',
        },
        'Progress notification sent via MCP protocol',
      );
    } catch (error) {
      logger?.warn(
        {
          error: error instanceof Error ? error.message : String(error),
          progressToken: notification.progressToken,
        },
        'Failed to send MCP progress notification',
      );
    }
  } else {
    // Fallback to logging if no notification callback available
    logger?.debug(
      {
        progressToken: notification.progressToken,
        message: notification.message,
        progress: notification.progress,
        total: notification.total,
        metadata: notification.metadata,
        type: 'progress_notification',
      },
      'Progress notification logged - no sendNotification callback available',
    );
  }
}

/**
 * Extract progress reporter from various input types.
 *
 * Uses type guards for type-safe handling of different progress input types:
 * - Direct ProgressReporter functions are returned as-is
 * - MCP request objects have their progress token extracted
 * - undefined returns undefined
 *
 * @param progress - Progress input (function, MCP request, or undefined)
 * @param logger - Logger for creating progress reporter
 * @param sendNotification - MCP notification callback for progress updates
 * @returns ProgressReporter function or undefined
 */
export function extractProgressReporter(
  progress: ProgressInput,
  logger: Logger,
  sendNotification?: (notification: unknown) => Promise<void>,
): EnhancedProgressReporter | undefined {
  if (!progress) return undefined;

  // Direct ProgressReporter function
  if (isProgressReporter(progress)) {
    return progress as EnhancedProgressReporter;
  }

  // MCP request object with progress token in params._meta
  if (isMCPProgressRequest(progress)) {
    const token = progress.params?._meta?.progressToken;
    if (token !== undefined) {
      return createProgressReporter(token, logger, sendNotification);
    }
  }

  return undefined;
}
