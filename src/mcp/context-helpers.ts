/**
 * MCP Context Helper Functions
 *
 * Extracted helper functions for progress handling and context utilities.
 * This module contains the internal implementation details that were
 * previously embedded in the main context.ts file.
 */

import type { Server } from '@modelcontextprotocol/sdk/server/index.js';
import type { Logger } from 'pino';

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

/**
 * Extracts progress token from MCP request metadata
 * Checks various locations where the progress token might be stored
 */
export function extractProgressToken(request: unknown): string | undefined {
  if (!request || typeof request !== 'object' || request === null) {
    return undefined;
  }

  const req = request as Record<string, unknown>;

  // Direct token
  if (typeof req.progressToken === 'string') {
    return req.progressToken;
  }

  // In params._meta
  const params = req.params;
  if (params && typeof params === 'object' && params !== null) {
    const p = params as Record<string, unknown>;
    const meta = p._meta;
    if (meta && typeof meta === 'object' && meta !== null) {
      const m = meta as Record<string, unknown>;
      if (typeof m.progressToken === 'string') {
        return m.progressToken;
      }
    }
  }

  // In top-level _meta
  const topMeta = req._meta;
  if (topMeta && typeof topMeta === 'object' && topMeta !== null) {
    const m = topMeta as Record<string, unknown>;
    if (typeof m.progressToken === 'string') {
      return m.progressToken;
    }
  }

  // In headers
  const headers = req.headers;
  if (headers && typeof headers === 'object' && headers !== null) {
    const h = headers as Record<string, unknown>;
    if (typeof h.progressToken === 'string') {
      return h.progressToken;
    }
    if (typeof h['x-progress-token'] === 'string') {
      return h['x-progress-token'];
    }
  }

  return undefined;
}

/**
 * Creates a progress reporter that forwards notifications through the MCP protocol
 */
export function createProgressReporter(
  server: Server,
  progressToken?: string,
  logger?: Logger,
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
        progressToken,
        message,
        ...(progress !== undefined && { progress }),
        ...(total !== undefined && { total }),
        ...(metadata && { metadata }),
      };

      sendProgressNotification(server, notification, logger);
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
 * Sends a progress notification through the MCP server.
 * Currently logs notifications as the MCP SDK doesn't expose direct notification methods.
 */
function sendProgressNotification(
  _server: Server,
  notification: ProgressNotification,
  logger?: Logger,
): void {
  logger?.debug(
    {
      progressToken: notification.progressToken,
      message: notification.message,
      progress: notification.progress,
      total: notification.total,
      metadata: notification.metadata,
      type: 'progress_notification',
    },
    'Progress notification logged - MCP transport implementation pending',
  );
}

// Previously had unused helper functions - removed to fix typecheck errors

/**
 * Extract progress reporter from various input types
 */
export function extractProgressReporter(
  progress: unknown,
  server: Server,
  logger: Logger,
): EnhancedProgressReporter | undefined {
  if (!progress) return undefined;

  // Already a function
  if (typeof progress === 'function') {
    return progress as EnhancedProgressReporter;
  }

  // Extract token and create reporter
  const progressToken = extractProgressToken(progress);
  if (progressToken) {
    return createProgressReporter(server, progressToken, logger);
  }

  return undefined;
}
