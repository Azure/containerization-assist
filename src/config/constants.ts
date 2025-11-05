/**
 * Application Constants and Defaults
 *
 * Consolidated configuration values for the entire application.
 * This file merges environment schemas, default values, and application constants.
 */

import { z } from 'zod';

/**
 * Environment Schema
 * Zod schema for environment validation across the application.
 */
export const environmentSchema = z
  .enum(['development', 'staging', 'production', 'testing'])
  .describe('Target environment');

export type Environment = z.infer<typeof environmentSchema>;

/**
 * Default ports by programming language/framework
 */
export const DEFAULT_PORTS = {
  javascript: [3000, 8080],
  typescript: [3000, 8080],
  python: [8000, 5000],
  java: [8080, 8081],
  go: [8080, 3000],
  rust: [8080, 3000],
  ruby: [3000, 9292],
  php: [8080, 80],
  csharp: [8080, 5000],
  dotnet: [8080, 5000],
  default: [3000, 8080],
} as const;

/**
 * Default timeout values in milliseconds
 */
export const DEFAULT_TIMEOUTS = {
  /** Cache expiration timeout: 5 minutes. */
  cache: 300_000,
  /** Cache cleanup interval: 5 minutes. */
  cacheCleanup: 300_000,
  /** Docker operations timeout: 30 seconds. */
  docker: 30_000,
  /** Docker build timeout: 5 minutes. */
  dockerBuild: 300_000,
  /** Kubernetes operations timeout: 30 seconds. */
  kubernetes: 30_000,
  /** Security scan timeout: 5 minutes. */
  'scan-image': 300_000,
  /** Deployment timeout: 3 minutes. */
  deployment: 180_000,
  /** Deployment status poll interval: 5 seconds. */
  deploymentPoll: 5_000,
  /** Deployment verification timeout: 1 minute. */
  verification: 60_000,
  /** Health check timeout: 5 seconds. */
  healthCheck: 5_000,
  /** Trivy version check timeout: 15 seconds. */
  trivyVersionCheck: 15_000,
  /** Cluster stabilization wait: 5 seconds. */
  clusterStabilization: 5_000,
} as const;

/**
 * Default network configuration
 */
export const DEFAULT_NETWORK = {
  host: 'localhost',
  loopback: '127.0.0.1',
  dockerHost: '0.0.0.0',
} as const;

/**
 * Validation limits
 */
export const LIMITS = {
  /** Maximum Dockerfile size (bytes): 1MB */
  MAX_DOCKERFILE_SIZE: 1_048_576,
  /** Maximum manifest size (bytes): 10MB */
  MAX_MANIFEST_SIZE: 10_485_760,
  /** Maximum log lines to retain */
  MAX_LOG_LINES: 1000,
  /** Maximum buffer size for scan results: 10MB */
  MAX_SCAN_BUFFER: 10 * 1024 * 1024,
  /** Maximum characters for AI prompt context */
  MAX_PROMPT_CHARS: 5000,
  /** Maximum snippets for AI prompt context */
  MAX_PROMPT_SNIPPETS: 25,
} as const;

/**
 * Retry configuration
 */
export const RETRY = {
  /** Default max retry attempts */
  MAX_ATTEMPTS: 3,
  /** Initial backoff delay (ms) */
  INITIAL_DELAY: 1000,
  /** Maximum backoff delay (ms) */
  MAX_DELAY: 10_000,
  /** Backoff multiplier */
  MULTIPLIER: 2,
} as const;

/**
 * Docker-related constants
 */
export const DOCKER = {
  /** Default Dockerfile name */
  DEFAULT_DOCKERFILE: 'Dockerfile',
  /** Default build context */
  DEFAULT_CONTEXT: '.',
  /** Default registry */
  DEFAULT_REGISTRY: 'docker.io',
  /** Standard registry port for both external and internal access (default starting point, actual port may vary) */
  REGISTRY_PORT: 6000,
  /** Registry port search range start */
  REGISTRY_PORT_START: 6000,
  /** Registry port search range end */
  REGISTRY_PORT_END: 6100,
  /** Registry container name */
  REGISTRY_CONTAINER_NAME: 'ca-registry',
  /** Registry host for external access */
  REGISTRY_HOST: 'localhost',
  /** Registry internal port (inside container) */
  REGISTRY_INTERNAL_PORT: 5000,
} as const;

/**
 * Kubernetes constants
 */
export const KUBERNETES = {
  /** Default namespace */
  DEFAULT_NAMESPACE: 'default',
  /** Deployment check interval (ms) */
  DEPLOYMENT_CHECK_INTERVAL: 5000,
  /** Max deployment checks */
  MAX_DEPLOYMENT_CHECKS: 60,
  /** Wait timeout in seconds */
  WAIT_TIMEOUT_SECONDS: 300,
  /** Default replicas */
  DEFAULT_REPLICAS: 1,
  /** Default environment */
  DEFAULT_ENVIRONMENT: 'development' as const,
  /** Default cluster */
  DEFAULT_CLUSTER: 'default',
  /** Default HTTP port */
  DEFAULT_HTTP_PORT: 80,
  /** Default HTTPS port */
  DEFAULT_HTTPS_PORT: 443,
  /** Pending LoadBalancer URL placeholder */
  PENDING_LB_URL: 'http://pending-loadbalancer',
  /** Default ingress host */
  DEFAULT_INGRESS_HOST: 'app.example.com',
} as const;

/**
 * Framework-specific default ports
 */
export const FRAMEWORK_PORTS = {
  // JavaScript/TypeScript
  next: 3000,
  react: 3000,
  angular: 4200,
  nuxt: 3000,
  vue: 3000,
  // Python
  django: 8000,
  flask: 5000,
  fastapi: 8000,
  // .NET
  'aspnet-core': 5000,
  'aspnet-core-https': 5001,
} as const;

/**
 * Get default port for a given language
 */
export function getDefaultPort(language: string): number {
  const key = language.toLowerCase() as keyof typeof DEFAULT_PORTS;
  const ports = DEFAULT_PORTS[key] || DEFAULT_PORTS.default;
  return ports[0];
}

/**
 * Get default port for a given framework
 */
export function getFrameworkPort(framework: string): number | undefined {
  const key = framework.toLowerCase() as keyof typeof FRAMEWORK_PORTS;
  return FRAMEWORK_PORTS[key];
}
