/**
 * Policy IO Module
 * Handles loading, validation, migration, and caching of policies
 */

import * as fs from 'node:fs';
import * as yaml from 'js-yaml';
import { z } from 'zod';
import { type Result, Success, Failure } from '@/types';
import { extractErrorMessage } from '@/lib/error-utils';
import { createLogger } from '@/lib/logger';
import { type Policy, PolicySchema } from './policy-schemas';

const logger = createLogger().child({ module: 'policy-io' });

// ============================================================================
// Cache Implementation
// ============================================================================

interface CacheEntry<T> {
  value: T;
  expires: number;
}

class SimpleCache<T> {
  private cache = new Map<string, CacheEntry<T>>();

  get(key: string): T | null {
    const entry = this.cache.get(key);
    if (!entry) return null;

    if (Date.now() > entry.expires) {
      this.cache.delete(key);
      return null;
    }

    return entry.value;
  }

  set(key: string, value: T, ttlMs: number): void {
    this.cache.set(key, {
      value,
      expires: Date.now() + ttlMs,
    });
  }

  clear(): void {
    this.cache.clear();
  }

  refresh(key: string): void {
    this.cache.delete(key);
  }
}

const policyCache = new SimpleCache<Policy>();

// ============================================================================
// Core Functions
// ============================================================================

/**
 * Load and resolve policy from YAML file with caching
 */
export function loadPolicy(filePath: string, environment?: string): Result<Policy> {
  try {
    // Check cache first
    const cacheKey = `${filePath}:${environment || 'default'}`;
    const cached = policyCache.get(cacheKey);
    if (cached) {
      logger.debug({ filePath, environment }, 'Using cached policy');
      return Success(cached);
    }

    // Load YAML file
    if (!fs.existsSync(filePath)) {
      return Failure(`Policy file not found: ${filePath}`);
    }

    const content = fs.readFileSync(filePath, 'utf-8');
    const policy = yaml.load(content) as Policy;

    // Resolve environment overrides
    const resolved = environment ? resolveEnvironment(policy, environment) : policy;

    // Validate final policy
    const validationResult = validatePolicy(resolved);
    if (!validationResult.ok) {
      return validationResult;
    }

    // Cache the loaded policy (default 5 minutes)
    const ttl = resolved.cache?.ttl || 300;
    policyCache.set(cacheKey, validationResult.value, ttl * 1000);

    return validationResult;
  } catch (error) {
    return Failure(`Failed to load policy: ${extractErrorMessage(error)}`);
  }
}

/**
 * Refresh policy cache for a specific file/environment
 */
export function refreshPolicy(filePath: string, environment?: string): void {
  const cacheKey = `${filePath}:${environment || 'default'}`;
  policyCache.refresh(cacheKey);
  logger.debug({ filePath, environment }, 'Refreshed policy cache');
}

/**
 * Clear entire policy cache
 */
export function clearPolicyCache(): void {
  policyCache.clear();
  logger.info('Cleared policy cache');
}

/**
 * Validate policy against schema
 */
export function validatePolicy(policy: unknown): Result<Policy> {
  try {
    const validated = PolicySchema.parse(policy) as Policy;
    return Success(validated);
  } catch (error) {
    if (error instanceof z.ZodError) {
      const issues = error.issues.map((i) => `${i.path.join('.')}: ${i.message}`).join(', ');
      return Failure(`Policy validation failed: ${issues}`);
    }
    return Failure(`Policy validation error: ${String(error)}`);
  }
}

/**
 * Resolve environment-specific overrides at load time
 */
export function resolveEnvironment(policy: Policy, environment: string): Policy {
  const envConfig = policy.environments?.[environment];

  // Deep clone the policy
  const resolved: Policy = JSON.parse(JSON.stringify(policy));

  if (envConfig) {
    // Apply environment defaults
    if (envConfig.defaults) {
      resolved.defaults = {
        ...resolved.defaults,
        ...(envConfig.defaults as any),
      };
    }

    // Apply rule overrides
    if (envConfig.overrides) {
      for (const override of envConfig.overrides) {
        const rule = resolved.rules.find((r) => r.id === override.rule_id);
        if (rule) {
          if (override.enabled === false) {
            // Remove disabled rules
            resolved.rules = resolved.rules.filter((r) => r.id !== override.rule_id);
          } else {
            // Apply overrides
            if (override.priority !== undefined) {
              rule.priority = override.priority;
            }
            if (override.actions) {
              rule.actions = { ...rule.actions, ...override.actions };
            }
          }
        }
      }
    }
  }

  // Sort rules by priority for consistent evaluation order
  resolved.rules.sort((a, b) => b.priority - a.priority);

  return resolved;
}

/**
 * Create default policy if none exists
 */
export function createDefaultPolicy(): Policy {
  return {
    version: '2.0',
    metadata: {
      created: new Date().toISOString(),
      description: 'Default containerization policy',
    },
    defaults: {
      enforcement: 'advisory',
      cache_ttl: 300,
    },
    rules: [
      {
        id: 'security-scanning',
        category: 'security',
        priority: 100,
        conditions: [
          {
            kind: 'regex',
            pattern: 'FROM .*(alpine|distroless)',
          },
        ],
        actions: {
          enforce_scan: true,
          block_on_critical: true,
        },
      },
      {
        id: 'base-image-validation',
        category: 'quality',
        priority: 90,
        conditions: [
          {
            kind: 'function',
            name: 'hasPattern',
            args: ['FROM.*:latest'],
          },
        ],
        actions: {
          suggest_pinned_version: true,
        },
      },
    ],
    cache: {
      enabled: true,
      ttl: 300,
    },
  };
}
