/**
 * Consolidated Configuration - Main Config
 *
 * Single source of configuration replacing multiple separate config files.
 * Simple, focused configuration without complex validation overhead.
 */
import { autoDetectDockerSocket } from '@/infra/docker/socket-validation';

// Export unified environment module
export * from './environment';

// Export consolidated constants
export * from './constants';

export const config = {
  server: {
    logLevel: process.env.LOG_LEVEL || 'info',
    port: parseInt(process.env.PORT || '3000'),
  },

  workspace: {
    workspaceDir: process.env.WORKSPACE_DIR || process.cwd(),
    maxFileSize: parseInt(process.env.MAX_FILE_SIZE || '10485760'),
  },

  docker: {
    socketPath: process.env.DOCKER_SOCKET || autoDetectDockerSocket(),
    timeout: parseInt(process.env.DOCKER_TIMEOUT || '60000'),
  },

  mutex: {
    defaultTimeout: parseInt(process.env.MUTEX_DEFAULT_TIMEOUT || '30000'),
    dockerBuildTimeout: parseInt(process.env.MUTEX_DOCKER_TIMEOUT || '300000'),
    monitoringEnabled: process.env.MUTEX_MONITORING !== 'false',
  },

  toolLogging: (() => {
    const logDir = process.env.CONTAINERIZATION_ASSIST_TOOL_LOGS_DIR_PATH ?? '';
    return {
      enabled: logDir.trim().length > 0,
      dirPath: logDir,
    };
  })(),

  validation: {
    imageAllowlist:
      process.env.CONTAINERIZATION_ASSIST_IMAGE_ALLOWLIST?.split(',')
        .map((s) => s.trim())
        .filter(Boolean) || [],
    imageDenylist:
      process.env.CONTAINERIZATION_ASSIST_IMAGE_DENYLIST?.split(',')
        .map((s) => s.trim())
        .filter(Boolean) || [],
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
