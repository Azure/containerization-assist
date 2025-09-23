/**
 * Unified Application Configuration
 *
 * Single source of truth for all configuration with Zod validation.
 * Consolidates environment variables, constants, and defaults.
 */

import { z } from 'zod';
import { readFileSync } from 'fs';
import { join } from 'path';
import os from 'os';
import { autoDetectDockerSocket } from '@/services/docker-client';

/**
 * Flattened configuration defaults
 * Uses cross-platform utilities for paths
 */
const DEFAULT_CONFIG = {
  MCP_NAME: 'containerization-assist',
  SESSION_TTL: 86400, // 24h in seconds
  DOCKER_TIMEOUT: 60000, // 60s
  K8S_TIMEOUT: 30000, // 30s
  SCAN_TIMEOUT: 300000, // 5min
  MAX_FILE_SIZE: 10 * 1024 * 1024, // 10MB
  CACHE_TTL: 3600, // 1 hour
  CACHE_MAX_SIZE: 100,
  MAX_SESSIONS: 100,
  HOST: '0.0.0.0',
  PORT: 3000,
  DOCKER_SOCKET: autoDetectDockerSocket(),
  DOCKER_REGISTRY: 'docker.io',
  K8S_NAMESPACE: 'default',
  KUBECONFIG: join(os.homedir(), '.kube', 'config'),
  TEMP_DIR: os.tmpdir(),
} as const;

const NodeEnvSchema = z.enum(['development', 'production', 'test']).default('development');
const LogLevelSchema = z.enum(['error', 'warn', 'info', 'debug', 'trace']).default('info');
const WorkflowModeSchema = z.enum(['interactive', 'auto', 'batch']).default('interactive');
const StoreTypeSchema = z.enum(['memory', 'file', 'redis']).default('memory');
const AppConfigSchema = z.object({
  server: z.object({
    nodeEnv: NodeEnvSchema,
    logLevel: LogLevelSchema,
    port: z.coerce.number().int().min(1024).max(65535).default(DEFAULT_CONFIG.PORT),
    host: z.string().min(1).default(DEFAULT_CONFIG.HOST),
  }),
  mcp: z.object({
    name: z.string().default(DEFAULT_CONFIG.MCP_NAME),
    version: z.string(),
    storePath: z.string().default('./data/sessions.db'),
    maxSessions: z.coerce.number().int().positive().default(DEFAULT_CONFIG.MAX_SESSIONS),
    enableMetrics: z.boolean().default(true),
    enableEvents: z.boolean().default(true),
  }),
  session: z.object({
    store: StoreTypeSchema,
    ttl: z.coerce.number().int().positive().default(DEFAULT_CONFIG.SESSION_TTL),
    maxSessions: z.coerce.number().int().positive().default(DEFAULT_CONFIG.MAX_SESSIONS),
    persistencePath: z.string().default('./data/sessions.db'),
    persistenceInterval: z.coerce.number().int().positive().default(60000),
    cleanupInterval: z.coerce
      .number()
      .int()
      .positive()
      .default(DEFAULT_CONFIG.CACHE_TTL * 1000),
  }),
  services: z.object({
    docker: z.object({
      socketPath: z.string().default(DEFAULT_CONFIG.DOCKER_SOCKET),
      host: z.string().default('localhost'),
      port: z.coerce.number().int().min(1).max(65535).default(2375),
      registry: z.string().default(DEFAULT_CONFIG.DOCKER_REGISTRY),
      timeout: z.coerce.number().int().positive().default(DEFAULT_CONFIG.DOCKER_TIMEOUT),
      buildArgs: z.record(z.string()).default({}),
    }),
    kubernetes: z.object({
      namespace: z.string().default(DEFAULT_CONFIG.K8S_NAMESPACE),
      kubeconfig: z.string().default(DEFAULT_CONFIG.KUBECONFIG),
      timeout: z.coerce.number().int().positive().default(DEFAULT_CONFIG.K8S_TIMEOUT),
    }),
  }),
  workspace: z.object({
    workspaceDir: z.string().default(() => process.cwd()),
    tempDir: z.string().default(() => os.tmpdir()),
    maxFileSize: z.coerce.number().int().positive().default(DEFAULT_CONFIG.MAX_FILE_SIZE),
  }),
  logging: z.object({
    level: LogLevelSchema,
  }),
  workflow: z.object({
    mode: WorkflowModeSchema,
  }),
  cache: z.object({
    ttl: z.coerce.number().int().positive().default(DEFAULT_CONFIG.CACHE_TTL),
    maxSize: z.coerce.number().int().positive().default(DEFAULT_CONFIG.CACHE_MAX_SIZE),
  }),
  security: z.object({
    scanTimeout: z.coerce.number().int().positive().default(DEFAULT_CONFIG.SCAN_TIMEOUT),
    failOnCritical: z.boolean().default(false),
  }),
});

export type AppConfig = z.infer<typeof AppConfigSchema>;

/**
 * Get package version from package.json
 */
function getPackageVersion(): string {
  try {
    const packageJsonPath = join(process.cwd(), 'package.json');
    const packageJson = JSON.parse(readFileSync(packageJsonPath, 'utf-8'));
    return packageJson.version || '1.0.0';
  } catch {
    return '1.0.0';
  }
}

/**
 * Handle empty string environment variables (preserve them as empty)
 *
 * Invariant: Empty strings are valid config values and must be preserved
 * Rationale: Some configs require explicit empty string vs. undefined
 */
function getEnvValue(key: string): string | undefined {
  const value = process.env[key];
  return value;
}

/**
 * Safely parse number from environment variable with fallback
 */
function parseNumberWithFallback(
  value: string | undefined,
  fallback: number,
  varName?: string,
): number {
  if (!value) return fallback;

  const parsed = Number(value);
  if (isNaN(parsed)) {
    console.warn(`Invalid ${varName || 'value'}: ${value}, using default ${fallback}`);
    return fallback;
  }

  return parsed;
}

/**
 * Create configuration with environment variable overrides and validation
 */
export function createAppConfig(): AppConfig {
  const rawConfig = {
    server: {
      nodeEnv: getEnvValue('NODE_ENV'),
      logLevel: getEnvValue('LOG_LEVEL'),
      port: getEnvValue('PORT'),
      host: getEnvValue('HOST'),
    },
    mcp: {
      name: getEnvValue('MCP_SERVER_NAME'),
      version: getPackageVersion(),
      storePath: getEnvValue('MCP_STORE_PATH'),
      maxSessions: parseNumberWithFallback(
        getEnvValue('MAX_SESSIONS'),
        DEFAULT_CONFIG.MAX_SESSIONS,
        'MAX_SESSIONS',
      ),
      enableMetrics: true,
      enableEvents: true,
    },
    session: {
      store: 'memory' as const,
      ttl: getEnvValue('SESSION_TTL'),
      maxSessions: parseNumberWithFallback(
        getEnvValue('MAX_SESSIONS'),
        DEFAULT_CONFIG.MAX_SESSIONS,
        'MAX_SESSIONS',
      ),
      persistencePath: getEnvValue('MCP_STORE_PATH') || './data/sessions.db',
      persistenceInterval: 60000,
      cleanupInterval: DEFAULT_CONFIG.CACHE_TTL * 1000,
    },
    services: {
      docker: {
        socketPath: getEnvValue('DOCKER_HOST') || getEnvValue('DOCKER_SOCKET'),
        host: 'localhost',
        port: getEnvValue('DOCKER_PORT'),
        registry: getEnvValue('DOCKER_REGISTRY'),
        timeout: getEnvValue('DOCKER_TIMEOUT'),
        buildArgs: {},
      },
      kubernetes: {
        namespace: getEnvValue('KUBE_NAMESPACE') || getEnvValue('K8S_NAMESPACE'),
        kubeconfig: getEnvValue('KUBECONFIG'),
        timeout: getEnvValue('K8S_TIMEOUT'),
      },
    },
    workspace: {
      workspaceDir: getEnvValue('WORKSPACE_DIR') || process.cwd(),
      tempDir: getEnvValue('TEMP_DIR') || os.tmpdir(),
      maxFileSize: DEFAULT_CONFIG.MAX_FILE_SIZE,
    },
    logging: {
      level: getEnvValue('LOG_LEVEL'),
    },
    workflow: {
      mode: 'interactive' as const,
    },
    cache: {
      ttl: DEFAULT_CONFIG.CACHE_TTL,
      maxSize: DEFAULT_CONFIG.CACHE_MAX_SIZE,
    },
    security: {
      scanTimeout: DEFAULT_CONFIG.SCAN_TIMEOUT,
      failOnCritical: getEnvValue('FAIL_ON_CRITICAL') === 'true',
    },
  };

  /**
   * Postcondition: Config is fully validated and type-safe
   * Failure Mode: Throws on invalid configuration to fail fast
   */
  const result = AppConfigSchema.safeParse(rawConfig);

  if (!result.success) {
    throw new Error(`Configuration validation failed: ${result.error.message}`);
  }

  return result.data;
}

/**
 * Export the application configuration
 * Creates configuration with environment variable overrides
 */
export const appConfig = createAppConfig();
