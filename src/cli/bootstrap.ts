/**
 * Bootstrap Helper - Shared MCP Server Lifecycle Management
 *
 * Provides unified bootstrap logic for CLI and server entry points:
 * 1. Set MCP_MODE environment variable
 * 2. Create application runtime
 * 3. Start MCP server with stdio transport
 * 4. Install shutdown handlers (SIGTERM, SIGINT, uncaught errors)
 * 5. Log startup progress
 *
 * This module is INTERNAL to the CLI and not exported as part of the public API.
 */

import type { Logger } from 'pino';
import type { AppRuntime, AppRuntimeConfig } from '@/types/runtime';
import type { AllToolTypes } from '@/tools';
import { createApp, type TransportConfig } from '@/app';
import {
  logStartup,
  logStartupSuccess,
  logStartupFailure,
  installShutdownHandlers,
  type StartupInfo,
} from '@/lib/runtime-logging';

/**
 * Configuration for bootstrap process
 */
export interface BootstrapConfig {
  /** Application name for logging */
  appName: string;

  /** Application version for logging */
  version: string;

  /** Logger instance (required) */
  logger: Logger;

  /** Policy configuration file path */
  policyPath?: string;

  /** Policy environment (development, production) */
  policyEnvironment?: string;

  /** Transport configuration (defaults to stdio) */
  transport?: TransportConfig;

  /** Quiet mode - suppress console output */
  quiet?: boolean;

  /** Workspace directory for logging */
  workspace?: string;

  /** Development mode flag for logging */
  devMode?: boolean;

  /** Optional custom tools (defaults to all internal tools) */
  tools?: readonly AllToolTypes[];

  /** Optional tool aliases */
  toolAliases?: Record<string, string>;

  /** Optional shutdown hook for cleanup */
  onShutdown?: () => Promise<void>;

  /** Tool count for startup logging */
  toolCount?: number;
}

/**
 * Result from bootstrap process
 */
export interface BootstrapResult {
  /** Application runtime instance */
  app: AppRuntime;

  /** Shutdown function for graceful termination */
  shutdown: (signal: string) => Promise<void>;
}

/**
 * Set MCP_MODE environment variable if not already set
 * @internal
 */
export function ensureMcpMode(): void {
  if (!process.env.MCP_MODE) {
    process.env.MCP_MODE = 'true';
  }
}

/**
 * Build AppRuntimeConfig from BootstrapConfig
 * @internal
 */
function buildAppConfig(config: BootstrapConfig): AppRuntimeConfig {
  const appConfig: AppRuntimeConfig = {
    logger: config.logger,
  };

  if (config.policyPath !== undefined) {
    appConfig.policyPath = config.policyPath;
  }

  if (config.policyEnvironment !== undefined) {
    appConfig.policyEnvironment = config.policyEnvironment;
  }

  if (config.tools !== undefined) {
    appConfig.tools = config.tools as Array<AllToolTypes>;
  }

  if (config.toolAliases !== undefined) {
    appConfig.toolAliases = config.toolAliases;
  }

  return appConfig;
}

/**
 * Build TransportConfig from BootstrapConfig
 * @internal
 */
function buildTransportConfig(config: BootstrapConfig): TransportConfig {
  return config.transport || { transport: 'stdio' as const };
}

/**
 * Build StartupInfo for logging
 * @internal
 */
function buildStartupInfo(
  config: BootstrapConfig,
  transport: TransportConfig,
  toolCount: number,
): StartupInfo {
  const startupInfo: StartupInfo = {
    appName: config.appName,
    version: config.version,
    workspace: config.workspace || process.cwd(),
    logLevel: config.logger.level,
    transport,
    toolCount,
  };

  if (config.devMode !== undefined) {
    startupInfo.devMode = config.devMode;
  }

  return startupInfo;
}

/**
 * Bootstrap the MCP server with unified lifecycle management
 *
 * Responsibilities:
 * 1. Set MCP_MODE environment variable
 * 2. Create application runtime
 * 3. Start MCP server with stdio transport
 * 4. Install shutdown handlers (SIGTERM, SIGINT, uncaught errors)
 * 5. Log startup progress
 *
 * @param config - Bootstrap configuration
 * @returns Bootstrap result with app instance and shutdown function
 * @throws Error if app creation or server startup fails
 *
 * @example
 * ```typescript
 * const { app, shutdown } = await bootstrap({
 *   appName: 'my-mcp-server',
 *   version: '1.0.0',
 *   logger: createLogger({ name: 'app' }),
 *   policyPath: 'config/policy.yaml',
 * });
 * ```
 */
export async function bootstrap(config: BootstrapConfig): Promise<BootstrapResult> {
  const { logger, quiet = false, onShutdown } = config;

  try {
    // Step 1: Ensure MCP_MODE is set
    ensureMcpMode();

    // Step 2: Create application runtime
    const appConfig = buildAppConfig(config);
    const app = createApp(appConfig);

    // Step 3: Get tool count for logging
    const toolCount = config.toolCount ?? app.listTools().length;

    // Step 4: Build transport configuration
    const transportConfig = buildTransportConfig(config);

    // Step 5: Log startup
    const startupInfo = buildStartupInfo(config, transportConfig, toolCount);
    logStartup(startupInfo, logger, quiet);

    // Step 6: Start MCP server
    await app.startServer(transportConfig);

    // Step 7: Log success
    logStartupSuccess(transportConfig, logger, quiet);

    // Step 8: Install shutdown handlers
    installShutdownHandlers(app, logger, quiet);

    // Step 9: Create shutdown function with optional cleanup hook
    const shutdown = async (signal: string): Promise<void> => {
      try {
        if (onShutdown) {
          await onShutdown();
        }
        await app.stop();
      } catch (error) {
        logger.error({ error, signal }, 'Error during shutdown');
        throw error;
      }
    };

    return { app, shutdown };
  } catch (error) {
    // Log failure and re-throw for caller to handle
    logStartupFailure(error as Error, logger, quiet);
    throw error;
  }
}
