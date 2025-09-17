/**
 * Application configuration with environment overrides
 */

import type { ApplicationConfig } from './types';
import { DEFAULT_NETWORK, DEFAULT_TIMEOUTS, getDefaultPort } from './defaults';
import os from 'os';
import path from 'path';
import { autoDetectDockerSocket } from '../services/docker-client';

/**
 * Create default configuration with sensible defaults
 * @returns ApplicationConfig with default values for all sections
 */
function createDefaultConfig(): ApplicationConfig {
  return {
    logLevel: 'info',
    workspaceDir: process.cwd(),
    server: {
      nodeEnv: 'development',
      logLevel: 'info',
      port: getDefaultPort('javascript'),
      host: DEFAULT_NETWORK.host,
    },
    session: {
      store: 'memory',
      ttl: 86400, // 24h
      maxSessions: 1000,
      persistencePath: './data/sessions.db',
      persistenceInterval: 60000, // 1min
      cleanupInterval: DEFAULT_TIMEOUTS.cacheCleanup,
    },
    mcp: {
      name: 'containerization-assist',
      version: '1.0.0',
      storePath: './data/sessions.db',
      sessionTTL: '24h',
      maxSessions: 100,
      enableMetrics: true,
      enableEvents: true,
    },
    docker: {
      socketPath: autoDetectDockerSocket(),
      host: 'localhost',
      port: 2375,
      registry: 'docker.io',
      timeout: 60000,
      buildArgs: {},
    },
    kubernetes: {
      namespace: 'default',
      kubeconfig: path.join(os.homedir(), '.kube', 'config'),
      timeout: 30000,
    },
    workspace: {
      workspaceDir: process.cwd(),
      tempDir: os.tmpdir(),
      cleanupOnExit: true,
    },
    logging: {
      level: 'info',
      format: 'json',
    },
    workflow: {
      mode: 'interactive',
    },
  };
}

/**
 * Parse integer with fallback and optional validation
 */
function parseIntWithFallback(
  value: string | undefined,
  fallback: number,
  varName?: string,
): number {
  if (!value) return fallback;
  const parsed = parseInt(value);
  if (isNaN(parsed)) {
    if (varName) {
      console.warn(`Invalid ${varName}: ${value}. Using default: ${fallback}`);
    }
    return fallback;
  }
  return parsed;
}

/**
 * Handle empty string environment variables
 */
function getEnvValue(key: string, fallback: string): string {
  const value = process.env[key];
  if (value === '') return value; // Preserve empty strings
  return value || fallback;
}

/**
 * Create configuration with environment overrides
 * @returns ApplicationConfig with environment variable overrides applied
 */
function createConfiguration(): ApplicationConfig {
  const defaultConfig = createDefaultConfig();

  // Apply environment variable overrides
  return {
    ...defaultConfig,
    server: {
      ...defaultConfig.server,
      nodeEnv: (process.env.NODE_ENV as any) || defaultConfig.server.nodeEnv,
      logLevel: (process.env.LOG_LEVEL as any) || defaultConfig.server.logLevel,
      port: parseIntWithFallback(process.env.PORT, defaultConfig.server.port),
      host: process.env.HOST || defaultConfig.server.host,
    },
    mcp: {
      ...defaultConfig.mcp,
      storePath: process.env.MCP_STORE_PATH || defaultConfig.mcp.storePath,
      maxSessions: parseIntWithFallback(
        process.env.MAX_SESSIONS,
        defaultConfig.mcp.maxSessions,
        'MAX_SESSIONS',
      ),
    },
    docker: {
      ...defaultConfig.docker,
      socketPath:
        process.env.DOCKER_HOST || process.env.DOCKER_SOCKET || defaultConfig.docker.socketPath,
      registry: process.env.DOCKER_REGISTRY || defaultConfig.docker.registry,
      timeout: parseIntWithFallback(process.env.DOCKER_TIMEOUT, defaultConfig.docker.timeout),
      port: parseIntWithFallback(process.env.DOCKER_PORT, defaultConfig.docker.port),
    },
    kubernetes: {
      ...defaultConfig.kubernetes,
      namespace:
        process.env.KUBE_NAMESPACE ||
        process.env.K8S_NAMESPACE ||
        defaultConfig.kubernetes.namespace,
      kubeconfig: getEnvValue('KUBECONFIG', defaultConfig.kubernetes.kubeconfig),
      timeout: parseIntWithFallback(process.env.K8S_TIMEOUT, defaultConfig.kubernetes.timeout),
    },
    logging: {
      ...defaultConfig.logging,
      level: (process.env.LOG_LEVEL as any) || defaultConfig.logging.level,
      format: process.env.LOG_FORMAT || defaultConfig.logging.format,
    },
  };
}

// Export functions used by tests
export { createDefaultConfig, createConfiguration };
