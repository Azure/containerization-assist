/**
 * CLI Configuration Loader
 *
 * Centralized configuration loader that consumes parsed CLI options
 * and environment variables to produce typed configuration for bootstrap
 * and app runtime.
 *
 * Responsibilities:
 * - Map CLI options to AppRuntimeConfig
 * - Resolve environment variables
 * - Apply defaults consistently
 * - Validate configuration values
 */

import type { Logger } from 'pino';
import type { AppRuntimeConfig } from '@/types/runtime';
import type { TransportConfig } from '@/app';
import { autoDetectDockerSocket } from '@/infra/docker/client';

/**
 * CLI options parsed by Commander
 */
export interface CLIOptions {
  /** Configuration file path */
  config?: string;

  /** Log level (debug, info, warn, error) */
  logLevel?: string;

  /** Workspace directory */
  workspace?: string;

  /** Development mode */
  dev?: boolean;

  /** Docker socket path */
  dockerSocket?: string;

  /** Kubernetes namespace */
  k8sNamespace?: string;

  /** Validate configuration only */
  validate?: boolean;

  /** List available tools */
  listTools?: boolean;

  /** Perform health check */
  healthCheck?: boolean;
}

/**
 * Environment variable configuration
 */
export interface EnvironmentConfig {
  /** Log level from env */
  logLevel: string;

  /** Workspace directory from env */
  workspaceDir: string;

  /** Docker socket path from env */
  dockerSocket: string;

  /** Kubernetes namespace from env */
  k8sNamespace: string;

  /** Node environment (development, production) */
  nodeEnv: string;

  /** MCP mode flag */
  mcpMode: boolean;

  /** MCP quiet mode */
  mcpQuiet: boolean;

  /** Policy file path */
  policyPath?: string;
}

/**
 * Bootstrap configuration (superset of AppRuntimeConfig)
 */
export interface BootstrapConfig {
  /** Application name */
  appName: string;

  /** Application version */
  version: string;

  /** Logger instance */
  logger: Logger;

  /** Policy file path */
  policyPath?: string;

  /** Policy environment */
  policyEnvironment?: string;

  /** Transport configuration */
  transport?: TransportConfig;

  /** Quiet mode */
  quiet?: boolean;

  /** Workspace directory */
  workspace?: string;

  /** Development mode */
  devMode?: boolean;

  /** Tool count for logging */
  toolCount?: number;
}

/**
 * Load environment configuration from process.env
 */
export function loadEnvironmentConfig(): EnvironmentConfig {
  const config: EnvironmentConfig = {
    logLevel: process.env.LOG_LEVEL || 'info',
    workspaceDir: process.env.WORKSPACE_DIR || process.cwd(),
    dockerSocket: process.env.DOCKER_SOCKET || autoDetectDockerSocket(),
    k8sNamespace: process.env.K8S_NAMESPACE || 'default',
    nodeEnv: process.env.NODE_ENV || 'production',
    mcpMode: process.env.MCP_MODE === 'true',
    mcpQuiet: process.env.MCP_QUIET === 'true',
  };

  if (process.env.POLICY_PATH !== undefined) {
    config.policyPath = process.env.POLICY_PATH;
  }

  return config;
}

/**
 * Apply CLI options to environment (mutates process.env)
 *
 * Priority: CLI options > Environment variables > Defaults
 */
export function applyOptionsToEnvironment(options: CLIOptions): void {
  if (options.logLevel) {
    process.env.LOG_LEVEL = options.logLevel;
  }

  if (options.workspace) {
    process.env.WORKSPACE_DIR = options.workspace;
  }

  if (options.dockerSocket) {
    process.env.DOCKER_SOCKET = options.dockerSocket;
  }

  if (options.k8sNamespace) {
    process.env.K8S_NAMESPACE = options.k8sNamespace;
  }

  if (options.dev) {
    process.env.NODE_ENV = 'development';
  }
}

/**
 * Create AppRuntimeConfig from CLI options and environment
 */
export function createRuntimeConfig(logger: Logger, options: CLIOptions = {}): AppRuntimeConfig {
  const env = loadEnvironmentConfig();

  return {
    logger,
    policyPath: options.config || env.policyPath || 'config/policy.yaml',
    policyEnvironment: options.dev ? 'development' : env.nodeEnv,
  };
}

/**
 * Create BootstrapConfig from CLI options and environment
 */
export function createBootstrapConfig(
  appName: string,
  version: string,
  logger: Logger,
  options: CLIOptions = {},
  toolCount = 0,
): BootstrapConfig {
  const env = loadEnvironmentConfig();

  const config: BootstrapConfig = {
    appName,
    version,
    logger,
    policyPath: options.config || env.policyPath || 'config/policy.yaml',
    policyEnvironment: options.dev ? 'development' : env.nodeEnv,
    transport: { transport: 'stdio' as const },
    quiet: env.mcpQuiet,
    workspace: options.workspace || env.workspaceDir,
    toolCount,
  };

  if (options.dev !== undefined) {
    config.devMode = options.dev;
  }

  return config;
}

/**
 * Get current configuration summary for display/validation
 */
export function getConfigSummary(): {
  logLevel: string;
  workspace: string;
  dockerSocket: string;
  k8sNamespace: string;
  nodeEnv: string;
} {
  const env = loadEnvironmentConfig();

  return {
    logLevel: env.logLevel,
    workspace: env.workspaceDir,
    dockerSocket: env.dockerSocket,
    k8sNamespace: env.k8sNamespace,
    nodeEnv: env.nodeEnv,
  };
}
