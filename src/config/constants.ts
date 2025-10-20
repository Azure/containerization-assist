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
  cache: 300000, // 5 minutes
  cacheCleanup: 300000, // 5 minutes
  docker: 30000, // 30 seconds
  dockerBuild: 300000, // 5 minutes
  kubernetes: 30000, // 30 seconds
  'scan-image': 300000, // 5 minutes
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
 * Get default port for a given language
 */
export function getDefaultPort(language: string): number {
  const key = language.toLowerCase() as keyof typeof DEFAULT_PORTS;
  const ports = DEFAULT_PORTS[key] || DEFAULT_PORTS.default;
  return ports[0];
}
