/**
 * Unified Application Configuration
 *
 * Single source of truth for all configuration with Zod validation.
 * Consolidates environment variables, constants, and defaults.
 */

import { z } from 'zod';
import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * Invariant: These constants define system-wide limits and defaults
 * Rationale: Centralized constants prevent magic numbers across codebase
 */
const CONSTANTS = {
  MCP: {
    NAME: 'containerization-assist',
    DEFAULT_SESSION_TTL: '24h',
  },
  TIMEOUTS: {
    DOCKER: 60000, // 60s
    KUBERNETES: 30000, // 30s - match test expectation
    SCAN: 300000, // 5min
    SAMPLING: 30000, // 30s
  },
  LIMITS: {
    MAX_FILE_SIZE: 10 * 1024 * 1024, // 10MB
    CACHE_TTL: 3600, // 1 hour
    CACHE_MAX_SIZE: 100,
    MAX_SESSIONS_DEFAULT: 100,
    SESSION_TTL: 86400, // 24h in seconds
  },
  DEFAULTS: {
    HOST: '0.0.0.0',
    PORT: 3000,
    DOCKER_SOCKET: '/var/run/docker.sock',
    DOCKER_REGISTRY: 'docker.io',
    K8S_NAMESPACE: 'default',
    KUBECONFIG: '~/.kube/config',
  },
  ORCHESTRATOR: {
    DEFAULT_CANDIDATES: 3,
    MAX_CANDIDATES: 5,
    EARLY_STOP_THRESHOLD: 90,
    TIEBREAK_MARGIN: 5,
    SCAN_THRESHOLDS: {
      CRITICAL: 0,
      HIGH: 2,
      MEDIUM: 10,
    },
    BUILD_SIZE_LIMITS: {
      SANITY_FACTOR: 1.25,
      REJECT_FACTOR: 2.5,
    },
    SAMPLING_WEIGHTS: {
      DOCKERFILE: {
        BUILD: 30,
        SIZE: 30,
        SECURITY: 25,
        SPEED: 15,
      },
      K8S: {
        VALIDATION: 20,
        SECURITY: 20,
        RESOURCES: 20,
        BEST_PRACTICES: 20,
      },
    },
  },
} as const;

const NodeEnvSchema = z.enum(['development', 'production', 'test']).default('development');
const LogLevelSchema = z.enum(['error', 'warn', 'info', 'debug', 'trace']).default('info');
const WorkflowModeSchema = z.enum(['interactive', 'auto', 'batch']).default('interactive');
const StoreTypeSchema = z.enum(['memory', 'file', 'redis']).default('memory');
const AppConfigSchema = z.object({
  server: z.object({
    nodeEnv: NodeEnvSchema,
    logLevel: LogLevelSchema,
    port: z.coerce.number().int().min(1024).max(65535).default(CONSTANTS.DEFAULTS.PORT),
    host: z.string().min(1).default(CONSTANTS.DEFAULTS.HOST),
  }),
  mcp: z.object({
    name: z.string().default(CONSTANTS.MCP.NAME),
    version: z.string(),
    storePath: z.string().default('./data/sessions.db'),
    sessionTTL: z.string().default(CONSTANTS.MCP.DEFAULT_SESSION_TTL),
    maxSessions: z.coerce.number().int().positive().default(CONSTANTS.LIMITS.MAX_SESSIONS_DEFAULT),
    enableMetrics: z.boolean().default(true),
    enableEvents: z.boolean().default(true),
  }),
  session: z.object({
    store: StoreTypeSchema,
    ttl: z.coerce.number().int().positive().default(CONSTANTS.LIMITS.SESSION_TTL),
    maxSessions: z.coerce.number().int().positive().default(1000),
    persistencePath: z.string().default('./data/sessions.db'),
    persistenceInterval: z.coerce.number().int().positive().default(60000),
    cleanupInterval: z.coerce
      .number()
      .int()
      .positive()
      .default(CONSTANTS.LIMITS.CACHE_TTL * 1000),
  }),
  docker: z.object({
    socketPath: z.string().default(CONSTANTS.DEFAULTS.DOCKER_SOCKET),
    host: z.string().default('localhost'),
    port: z.coerce.number().int().min(1).max(65535).default(2375),
    registry: z.string().default(CONSTANTS.DEFAULTS.DOCKER_REGISTRY),
    timeout: z.coerce.number().int().positive().default(CONSTANTS.TIMEOUTS.DOCKER),
    buildArgs: z.record(z.string()).default({}),
  }),
  kubernetes: z.object({
    namespace: z.string().default(CONSTANTS.DEFAULTS.K8S_NAMESPACE),
    kubeconfig: z.string().default(CONSTANTS.DEFAULTS.KUBECONFIG),
    timeout: z.coerce.number().int().positive().default(CONSTANTS.TIMEOUTS.KUBERNETES),
  }),
  workspace: z.object({
    workspaceDir: z.string().default(() => process.cwd()),
    tempDir: z.string().default('/tmp'),
    cleanupOnExit: z.boolean().default(true),
    maxFileSize: z.coerce.number().int().positive().default(CONSTANTS.LIMITS.MAX_FILE_SIZE),
  }),
  logging: z.object({
    level: LogLevelSchema,
    format: z.enum(['json', 'text']).default('json'),
  }),
  workflow: z.object({
    mode: WorkflowModeSchema,
  }),
  cache: z.object({
    ttl: z.coerce.number().int().positive().default(CONSTANTS.LIMITS.CACHE_TTL),
    maxSize: z.coerce.number().int().positive().default(CONSTANTS.LIMITS.CACHE_MAX_SIZE),
  }),
  security: z.object({
    scanTimeout: z.coerce.number().int().positive().default(CONSTANTS.TIMEOUTS.SCAN),
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
      sessionTTL: getEnvValue('SESSION_TTL'),
      maxSessions: parseNumberWithFallback(
        getEnvValue('MAX_SESSIONS'),
        CONSTANTS.LIMITS.MAX_SESSIONS_DEFAULT,
        'MAX_SESSIONS',
      ),
      enableMetrics: true,
      enableEvents: true,
    },
    session: {
      store: 'memory' as const,
      ttl: getEnvValue('SESSION_TTL'),
      maxSessions: parseNumberWithFallback(getEnvValue('MAX_SESSIONS'), 1000, 'MAX_SESSIONS'),
      persistencePath: getEnvValue('MCP_STORE_PATH') || './data/sessions.db',
      persistenceInterval: 60000,
      cleanupInterval: CONSTANTS.LIMITS.CACHE_TTL * 1000,
    },
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
    workspace: {
      workspaceDir: getEnvValue('WORKSPACE_DIR') || process.cwd(),
      tempDir: '/tmp',
      cleanupOnExit: true,
      maxFileSize: CONSTANTS.LIMITS.MAX_FILE_SIZE,
    },
    logging: {
      level: getEnvValue('LOG_LEVEL'),
      format: getEnvValue('LOG_FORMAT'),
    },
    workflow: {
      mode: 'interactive' as const,
    },
    cache: {
      ttl: CONSTANTS.LIMITS.CACHE_TTL,
      maxSize: CONSTANTS.LIMITS.CACHE_MAX_SIZE,
    },
    security: {
      scanTimeout: CONSTANTS.TIMEOUTS.SCAN,
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
