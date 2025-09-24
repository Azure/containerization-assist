/**
 * Unified environment module - single source of truth for environment handling
 * Eliminates duplication between environmentFull and environmentBasic
 */

import { z } from 'zod';

/**
 * Standard environment types supported across all tools
 */
export type Environment = 'development' | 'staging' | 'production' | 'testing';

/**
 * Zod schema for environment validation
 */
export const environmentSchema = z
  .enum(['development', 'staging', 'production', 'testing'])
  .describe('Target environment');

/**
 * Default environment if none specified
 */
export const DEFAULT_ENVIRONMENT: Environment = 'development';

/**
 * Parse environment string to typed Environment
 * Falls back to DEFAULT_ENVIRONMENT for invalid values
 */
export function parseEnvironment(value?: string): Environment {
  if (!value) {
    return DEFAULT_ENVIRONMENT;
  }

  const result = environmentSchema.safeParse(value);
  if (result.success) {
    return result.data;
  }

  // Log warning for invalid environment values
  console.warn(`Invalid environment value: "${value}". Using default: ${DEFAULT_ENVIRONMENT}`);
  return DEFAULT_ENVIRONMENT;
}

/**
 * Helper to parse boolean values from various sources
 * Handles string values like "true", "false", "1", "0"
 */
export function parseBool(value: unknown, fallback = false): boolean {
  if (typeof value === 'boolean') {
    return value;
  }

  if (typeof value === 'string') {
    const normalized = value.toLowerCase().trim();
    if (normalized === 'true' || normalized === '1' || normalized === 'yes') {
      return true;
    }
    if (normalized === 'false' || normalized === '0' || normalized === 'no') {
      return false;
    }
  }

  if (typeof value === 'number') {
    return value !== 0;
  }

  return fallback;
}

/**
 * Environment-specific configuration helper
 */
export function isProductionEnvironment(env: Environment): boolean {
  return env === 'production';
}

export function isDevelopmentEnvironment(env: Environment): boolean {
  return env === 'development';
}

export function isTestingEnvironment(env: Environment): boolean {
  return env === 'testing';
}

export function isStagingEnvironment(env: Environment): boolean {
  return env === 'staging';
}

/**
 * Get environment from process.env with fallback
 */
export function getEnvironmentFromEnv(
  envVarName = 'NODE_ENV',
  fallback: Environment = DEFAULT_ENVIRONMENT,
): Environment {
  const value = process.env[envVarName];
  return value ? parseEnvironment(value) : fallback;
}
