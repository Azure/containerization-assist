/**
 * Progress Token Handler
 *
 * Handles progress token detection and forwarding through MCP notifications.
 * Integrates with existing progress reporting infrastructure.
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
 *
 * @param request - MCP request object with potential progress metadata
 * @returns Progress token string if found, undefined otherwise
 */
export function extractProgressToken(request: unknown): string | undefined {
  if (!request || typeof request !== 'object' || request === null) {
    return undefined;
  }

  const req = request as Record<string, unknown>;

  if (typeof req.progressToken === 'string') {
    return req.progressToken;
  }

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

  const topMeta = req._meta;
  if (topMeta && typeof topMeta === 'object' && topMeta !== null) {
    const m = topMeta as Record<string, unknown>;
    if (typeof m.progressToken === 'string') {
      return m.progressToken;
    }
  }

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
 * Creates a progress reporter that forwards notifications
 * through the MCP protocol
 *
 * @param server - MCP server instance for sending notifications
 * @param progressToken - Progress token for this reporting session
 * @param logger - Logger instance for debugging and error handling
 * @returns Progress reporter function or undefined if no token
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
 * Future implementations should use the transport layer for actual notification sending.
 *
 * @param server - MCP server instance (currently unused)
 * @param notification - Progress notification to send
 * @param logger - Logger for debugging and structured output
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

/**
 * Validates progress token format.
 * Tokens must be non-empty strings with reasonable length limits.
 *
 * @param token - Token to validate
 * @returns True if token appears to be valid
 */
export function isValidProgressToken(token: string): boolean {
  return typeof token === 'string' && token.length > 0 && token.length <= 256;
}

/**
 * Creates a scoped progress reporter for a specific operation
 * Useful for tools that perform multiple sub-operations
 *
 * @param baseReporter - Base progress reporter
 * @param operationName - Name of the operation for context
 * @param baseProgress - Starting progress value
 * @param progressRange - Range of progress values this operation will use
 * @returns Scoped progress reporter
 */
export function createScopedProgressReporter(
  baseReporter: EnhancedProgressReporter,
  operationName: string,
  baseProgress: number = 0,
  progressRange: number = 100,
): EnhancedProgressReporter {
  return async (
    message: string,
    progress?: number,
    total?: number,
    metadata?: Record<string, unknown>,
  ) => {
    const scopedProgress =
      progress !== undefined && total !== undefined
        ? baseProgress + (progress / total) * progressRange
        : undefined;

    const scopedMessage = `[${operationName}] ${message}`;

    await baseReporter(scopedMessage, scopedProgress, baseProgress + progressRange, {
      operation: operationName,
      originalProgress: progress,
      originalTotal: total,
      ...metadata,
    });
  };
}
