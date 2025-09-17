/**
 * Consolidated Configuration - Main Config
 *
 * Single source of configuration replacing 9 separate config files
 * Simple, focused configuration without complex validation overhead
 */
import { autoDetectDockerSocket } from '../services/docker-client';

export const config = {
  mcp: {
    name: process.env.MCP_SERVER_NAME || 'containerization-assist',
    version: process.env.MCP_SERVER_VERSION || '1.0.0',
  },

  server: {
    logLevel: process.env.LOG_LEVEL || 'info',
    port: parseInt(process.env.PORT || '3000'),
  },

  workspace: {
    workspaceDir: process.env.WORKSPACE_DIR || process.cwd(),
    maxFileSize: parseInt(process.env.MAX_FILE_SIZE || '10485760'),
  },

  sampling: {
    maxCandidates: parseInt(process.env.MAX_CANDIDATES || '5'),
    timeout: parseInt(process.env.SAMPLING_TIMEOUT || '30000'),
    weights: {
      dockerfile: {
        build: 30,
        size: 30,
        security: 25,
        speed: 15,
      },
      k8s: {
        validation: 20,
        security: 20,
        resources: 20,
        best_practices: 20,
      },
    },
  },

  cache: {
    ttl: parseInt(process.env.CACHE_TTL || '3600'),
    maxSize: parseInt(process.env.CACHE_MAX_SIZE || '100'),
  },

  docker: {
    socketPath: process.env.DOCKER_SOCKET || autoDetectDockerSocket(),
    timeout: parseInt(process.env.DOCKER_TIMEOUT || '60000'),
  },

  kubernetes: {
    namespace: process.env.K8S_NAMESPACE || 'default',
    timeout: parseInt(process.env.K8S_TIMEOUT || '60000'),
  },

  security: {
    scanTimeout: parseInt(process.env.SCAN_TIMEOUT || '300000'),
    failOnCritical: process.env.FAIL_ON_CRITICAL === 'true',
  },

  logging: {
    level: process.env.LOG_LEVEL || 'info',
    format: process.env.LOG_FORMAT || 'json',
  },

  ai: {
    maxRetries: parseInt(process.env.AI_MAX_RETRIES || '3'),
    retryDelay: parseInt(process.env.AI_RETRY_DELAY || '1000'),
    defaultMaxTokens: parseInt(process.env.AI_MAX_TOKENS || '4096'),
    defaultTemperature: parseFloat(process.env.AI_TEMPERATURE || '0.7'),
    timeout: parseInt(process.env.AI_TIMEOUT || '30000'),
    caching: {
      enabled: process.env.AI_CACHE_ENABLED !== 'false',
      ttl: parseInt(process.env.AI_CACHE_TTL || '300000'), // 5 minutes
    },
  },

  mutex: {
    defaultTimeout: parseInt(process.env.MUTEX_DEFAULT_TIMEOUT || '30000'),
    dockerBuildTimeout: parseInt(process.env.MUTEX_DOCKER_TIMEOUT || '300000'),
    monitoringEnabled: process.env.MUTEX_MONITORING !== 'false',
  },

  errors: {
    suggestionsEnabled: process.env.ERROR_SUGGESTIONS !== 'false',
    documentationBaseUrl:
      process.env.ERROR_DOCS_URL || 'https://docs.containerization-assist.io/errors',
  },

  correlation: {
    enabled: process.env.CORRELATION_ENABLED !== 'false',
    headerName: process.env.CORRELATION_HEADER || 'X-Correlation-ID',
  },

  orchestrator: {
    defaultCandidates: parseInt(process.env.DEFAULT_CANDIDATES || '3'),
    maxCandidates: parseInt(process.env.MAX_CANDIDATES || '5'),
    earlyStopThreshold: parseInt(process.env.EARLY_STOP_THRESHOLD || '90'),
    tiebreakMargin: parseInt(process.env.TIEBREAK_MARGIN || '5'),

    scanThresholds: {
      critical: parseInt(process.env.SCAN_CRITICAL_THRESHOLD || '0'),
      high: parseInt(process.env.SCAN_HIGH_THRESHOLD || '2'),
      medium: parseInt(process.env.SCAN_MEDIUM_THRESHOLD || '10'),
    },

    buildSizeLimits: {
      sanityFactor: parseFloat(process.env.BUILD_SANITY_FACTOR || '1.25'),
      rejectFactor: parseFloat(process.env.BUILD_REJECT_FACTOR || '2.5'),
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
    };

    if (logger) {
      logger.info('Configuration loaded', configData);
    }
  }
}
