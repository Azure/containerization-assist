/**
 * Shared Runtime Logging - Harmonized Startup/Shutdown/Tool Logging
 *
 * Provides consistent logging behavior across CLI, server entry points,
 * and tool executions.
 */

import type { Logger } from 'pino';
import type { TransportConfig } from '@/app';
import type { MCPServer } from '@/mcp/mcp-server';

/**
 * Runtime startup information
 */
export interface StartupInfo {
  /** Application name */
  appName: string;
  /** Application version */
  version: string;
  /** Workspace directory */
  workspace: string;
  /** Log level */
  logLevel: string;
  /** Transport configuration */
  transport: TransportConfig;
  /** Development mode flag */
  devMode?: boolean;
  /** Number of tools loaded */
  toolCount: number;
}

/**
 * Runtime shutdown information
 */
export interface ShutdownInfo {
  /** Shutdown signal received */
  signal: string;
  /** Duration of operation in milliseconds */
  duration: number;
  /** Exit code */
  exitCode: number;
  /** Whether shutdown was graceful */
  graceful: boolean;
}

/**
 * Log startup messages in a consistent format
 */
export function logStartup(info: StartupInfo, logger: Logger, quiet = false): void {
  logger.info(
    {
      version: info.version,
      config: {
        logLevel: info.logLevel,
        workspace: info.workspace,
        devMode: info.devMode,
        transport: info.transport,
      },
      toolCount: info.toolCount,
    },
    'Starting Containerization Assist MCP Server',
  );

  if (!quiet) {
    console.error('🚀 Starting Containerization Assist MCP Server...');
    console.error(`📦 Version: ${info.version}`);
    console.error(`🏠 Workspace: ${info.workspace}`);
    console.error(`📊 Log Level: ${info.logLevel}`);
    console.error('🔌 Transport: stdio');
    console.error(`🛠️ Tools: ${info.toolCount} loaded`);

    if (info.devMode) {
      console.error('🔧 Development mode enabled');
    }
  }
}

/**
 * Log successful startup
 */
export function logStartupSuccess(transport: TransportConfig, logger: Logger, quiet = false): void {
  logger.info({ transport }, 'MCP server started successfully');

  if (!quiet) {
    console.error('✅ Server started successfully');
    console.error('📡 Ready to accept MCP requests via stdio');
    console.error('💡 Send JSON-RPC messages to stdin for interaction');
  }
}

/**
 * Log startup failure
 */
export function logStartupFailure(error: Error, logger: Logger, quiet = false): void {
  logger.error({ error }, 'Server startup failed');

  if (!quiet) {
    console.error('❌ Server startup failed');
    console.error(`🔍 Error: ${error.message}`);
  }
}

/**
 * Log shutdown initiation
 * @public
 */
export function logShutdownStart(signal: string, logger: Logger, quiet = false): void {
  logger.info({ signal }, 'Shutdown initiated');

  if (!quiet) {
    console.error(`\n🛑 Received ${signal}, shutting down gracefully...`);
  }
}

/**
 * Log successful shutdown
 * @public
 */
export function logShutdownSuccess(info: ShutdownInfo, logger: Logger, quiet = false): void {
  logger.info(
    {
      signal: info.signal,
      duration: info.duration,
      graceful: info.graceful,
    },
    'Shutdown completed successfully',
  );

  if (!quiet) {
    console.error('✅ Shutdown complete');
  }
}

/**
 * Log shutdown failure
 * @public
 */
export function logShutdownFailure(
  error: Error,
  info: Partial<ShutdownInfo>,
  logger: Logger,
  quiet = false,
): void {
  logger.error(
    {
      error,
      signal: info.signal,
      duration: info.duration,
      graceful: false,
    },
    'Shutdown error',
  );

  if (!quiet) {
    console.error('❌ Shutdown error:', error.message);
  }
}

/**
 * Log forced shutdown due to timeout
 * @public
 */
export function logForcedShutdown(logger: Logger, quiet = false): void {
  logger.error('Forced shutdown due to timeout');

  if (!quiet) {
    console.error('⚠️ Forced shutdown - some resources may not have cleaned up properly');
  }
}

/**
 * Create a shutdown handler with proper logging
 * @public
 */
export function createShutdownHandler(
  server: MCPServer | { stop: () => Promise<void> },
  logger: Logger,
  quiet = false,
  timeoutMs = 10000,
): (signal: string) => Promise<void> {
  return async (signal: string): Promise<void> => {
    const startTime = Date.now();
    logShutdownStart(signal, logger, quiet);

    // Set a timeout for shutdown
    const shutdownTimeout = setTimeout(() => {
      logForcedShutdown(logger, quiet);
      process.exit(1);
    }, timeoutMs);

    try {
      await server.stop();
      clearTimeout(shutdownTimeout);

      const duration = Date.now() - startTime;
      logShutdownSuccess(
        {
          signal,
          duration,
          exitCode: 0,
          graceful: true,
        },
        logger,
        quiet,
      );

      process.exit(0);
    } catch (error) {
      clearTimeout(shutdownTimeout);
      const duration = Date.now() - startTime;

      logShutdownFailure(
        error as Error,
        {
          signal,
          duration,
          exitCode: 1,
          graceful: false,
        },
        logger,
        quiet,
      );

      process.exit(1);
    }
  };
}

/**
 * Install signal handlers for graceful shutdown
 */
export function installShutdownHandlers(
  server: MCPServer | { stop: () => Promise<void> },
  logger: Logger,
  quiet = false,
): void {
  const shutdownHandler = createShutdownHandler(server, logger, quiet);

  process.on('SIGTERM', () => {
    shutdownHandler('SIGTERM').catch((error) => {
      logger.error({ error }, 'Error during SIGTERM shutdown');
      process.exit(1);
    });
  });

  process.on('SIGINT', () => {
    shutdownHandler('SIGINT').catch((error) => {
      logger.error({ error }, 'Error during SIGINT shutdown');
      process.exit(1);
    });
  });

  // Handle uncaught exceptions and rejections
  process.on('uncaughtException', (error) => {
    logger.fatal({ error }, 'Uncaught exception');
    console.error('❌ Uncaught exception:', error.message);
    process.exit(1);
  });

  process.on('unhandledRejection', (reason, promise) => {
    logger.fatal({ reason, promise }, 'Unhandled rejection');
    console.error('❌ Unhandled rejection:', reason);
    process.exit(1);
  });
}

/**
 * Standard log message format constants for tool execution
 * Ensures all tools use consistent "starting"/"completed" phrasing
 */
export const LOG_FORMAT = {
  /** Tool execution started */
  STARTING: 'starting',
  /** Tool execution completed successfully */
  COMPLETED: 'completed',
  /** Tool execution failed */
  FAILED: 'failed',
} as const;

/**
 * Log tool execution start with consistent format
 * @param toolName - Name of the tool being executed
 * @param params - Key parameters for the operation
 * @param logger - Pino logger instance
 *
 * @example
 * ```typescript
 * logToolStart('build-image', { path: './app', tags: ['myapp:latest'] }, logger);
 * // Logs: "Starting build-image" with structured params
 * ```
 */
export function logToolStart(
  toolName: string,
  params: Record<string, unknown>,
  logger: Logger,
): void {
  logger.info(params, `Starting ${toolName}`);
}

/**
 * Log tool execution completion with consistent format
 * @param toolName - Name of the tool that completed
 * @param result - Key results from the operation
 * @param durationMs - Optional execution duration in milliseconds
 * @param logger - Pino logger instance
 *
 * @example
 * ```typescript
 * logToolComplete('build-image', { imageId: 'sha256:abc...' }, 1234, logger);
 * // Logs: "Completed build-image" with structured result and duration
 * ```
 */
export function logToolComplete(
  toolName: string,
  result: Record<string, unknown>,
  logger: Logger,
  durationMs?: number,
): void {
  const logData = durationMs !== undefined ? { ...result, durationMs } : result;
  logger.info(logData, `Completed ${toolName}`);
}

/**
 * Log tool execution failure with consistent format
 * @param toolName - Name of the tool that failed
 * @param error - Error message or object
 * @param context - Optional context about the failure
 * @param logger - Pino logger instance
 *
 * @example
 * ```typescript
 * logToolFailure('build-image', 'Docker daemon not running', { path: './app' }, logger);
 * // Logs: "Failed build-image" with error and context
 * ```
 */
export function logToolFailure(
  toolName: string,
  error: string | Error,
  logger: Logger,
  context?: Record<string, unknown>,
): void {
  const errorMessage = typeof error === 'string' ? error : error.message;
  const logData = context ? { ...context, error: errorMessage } : { error: errorMessage };
  logger.error(logData, `Failed ${toolName}`);
}
