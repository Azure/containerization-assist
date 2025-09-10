/**
 * Centralized Configuration Defaults
 *
 * Single source of truth for all default values used throughout the application.
 * Eliminates hardcoded values and makes configuration more maintainable.
 */

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
  csharp: [5000, 5001],
  dotnet: [5000, 5001],
  default: [3000, 8080],
} as const;

/**
 * Default timeout values in milliseconds
 */
export const DEFAULT_TIMEOUTS = {
  cache: 300000, // 5 minutes
  cacheCleanup: 300000, // 5 minutes
  docker: 30000, // 30 seconds
  dockerBuild: 300000, // 5 minutes
  kubernetes: 30000, // 30 seconds
  sampling: 30000, // 30 seconds
  scan: 300000, // 5 minutes
  deployment: 180000, // 3 minutes
  deploymentPoll: 5000, // 5 seconds (between deployment status checks)
  verification: 60000, // 1 minute
  healthCheck: 5000, // 5 seconds (between health checks)
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
 * Default container configuration
 */
export const DEFAULT_CONTAINER = {
  healthCheckPath: '/health',
  maxImageSize: '2GB',
  buildTimeLimit: 600000, // 10 minutes
} as const;

/**
 * Get default port for a given language
 */
export function getDefaultPort(language: string): number {
  const key = language.toLowerCase() as keyof typeof DEFAULT_PORTS;
  const ports = DEFAULT_PORTS[key] || DEFAULT_PORTS.default;
  return ports[0];
}
