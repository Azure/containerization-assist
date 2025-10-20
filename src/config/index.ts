/**
 * Consolidated Configuration - Main Config
 *
 * Single source of configuration replacing multiple separate config files.
 * Simple, focused configuration without complex validation overhead.
 */
import { autoDetectDockerSocket } from '@/infra/docker/socket-validation';
import { parseIntEnv, parseStringEnv } from './env-utils';

// Export consolidated constants (includes environment schema and defaults)
export * from './constants';

export const config = {
  server: {
    logLevel: parseStringEnv('LOG_LEVEL', 'info'),
    port: parseIntEnv('PORT', 3000),
  },

  workspace: {
    workspaceDir: parseStringEnv('WORKSPACE_DIR', process.cwd()),
    maxFileSize: parseIntEnv('MAX_FILE_SIZE', 10485760),
  },

  docker: {
    socketPath: parseStringEnv('DOCKER_SOCKET', autoDetectDockerSocket()),
    timeout: parseIntEnv('DOCKER_TIMEOUT', 60000),
  },

  toolLogging: {
    dirPath: parseStringEnv('CONTAINERIZATION_ASSIST_TOOL_LOGS_DIR_PATH', ''),
    get enabled() {
      return this.dirPath.trim().length > 0;
    },
  },
} as const;

// Export the type for use throughout the application
export type AppConfig = typeof config;

/**
 * Configuration utilities
 */

export function logConfigSummaryIfDev(logger?: {
  info: (message: string, data?: any) => void;
}): void {
  if (process.env.NODE_ENV === 'development') {
    const configData = {
      server: {
        logLevel: config.server.logLevel,
        port: config.server.port,
      },
      workspace: config.workspace.workspaceDir,
      docker: config.docker.socketPath,
      toolLogging: {
        enabled: config.toolLogging.enabled,
        dirPath: config.toolLogging.dirPath || 'not configured',
      },
    };

    if (logger) {
      logger.info('Configuration loaded', configData);
    }
  }
}
