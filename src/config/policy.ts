/**
 * Unified Policy Module
 * Consolidates policy management with compile-time type safety
 */

import { z } from 'zod';
import * as fs from 'node:fs';
import * as path from 'node:path';
import * as yaml from 'js-yaml';
import { type Result, Success, Failure } from '@/types/index';
import { extractErrorMessage } from '@/lib/error-utils';

// ============================================================================
// Type Definitions
// ============================================================================

/**
 * Discriminated union for matcher types
 */
export type Matcher = RegexMatcher | FunctionMatcher;

export interface RegexMatcher {
  kind: 'regex';
  pattern: string;
  flags?: string;
  count_threshold?: number;
}

export interface FunctionMatcher {
  kind: 'function';
  name: 'hasPattern' | 'fileExists' | 'largerThan' | 'hasVulnerabilities';
  args: unknown[];
}

/**
 * Policy rule with priority and actions
 */
export interface PolicyRule {
  id: string;
  category?: 'quality' | 'security' | 'performance' | 'compliance';
  priority: number;
  conditions: Matcher[];
  actions: Record<string, unknown>;
  description?: string;
}

/**
 * Environment-specific override
 */
export interface EnvironmentOverride {
  rule_id: string;
  actions?: Record<string, unknown>;
  priority?: number;
  enabled?: boolean;
}

/**
 * Cache configuration
 */
export interface CacheConfig {
  enabled: boolean;
  ttl: number;
}

/**
 * Unified policy structure (v2.0)
 */
export interface UnifiedPolicy {
  version: '2.0';
  metadata?: {
    created?: string;
    author?: string;
    description?: string;
  };
  defaults?: {
    cache_ttl?: number;
    enforcement?: 'strict' | 'lenient' | 'advisory';
  };
  rules: PolicyRule[];
  environments?: Record<
    string,
    {
      defaults?: Record<string, unknown>;
      overrides?: EnvironmentOverride[];
    }
  >;
  cache?: CacheConfig;
}

/**
 * Legacy v1.0 policy structure (for migration)
 */
export interface LegacyPolicy {
  version: '1.0';
  rules: Array<{
    id: string;
    priority?: number;
    matchers?: Array<{
      pattern?: string;
      function?: string;
      args?: unknown[];
    }>;
    actions?: Record<string, unknown>;
  }>;
}

// ============================================================================
// Schema Validation
// ============================================================================

const RegexMatcherSchema = z.object({
  kind: z.literal('regex'),
  pattern: z.string(),
  flags: z.string().optional(),
  count_threshold: z.number().optional(),
});

const FunctionMatcherSchema = z.object({
  kind: z.literal('function'),
  name: z.enum(['hasPattern', 'fileExists', 'largerThan', 'hasVulnerabilities']),
  args: z.array(z.unknown()),
});

const MatcherSchema = z.discriminatedUnion('kind', [RegexMatcherSchema, FunctionMatcherSchema]);

const PolicyRuleSchema = z.object({
  id: z.string(),
  category: z.enum(['quality', 'security', 'performance', 'compliance']).optional(),
  priority: z.number(),
  conditions: z.array(MatcherSchema),
  actions: z.record(z.unknown()),
  description: z.string().optional(),
});

const EnvironmentOverrideSchema = z.object({
  rule_id: z.string(),
  actions: z.record(z.unknown()).optional(),
  priority: z.number().optional(),
  enabled: z.boolean().optional(),
});

const UnifiedPolicySchema = z.object({
  version: z.literal('2.0'),
  metadata: z
    .object({
      created: z.string().optional(),
      author: z.string().optional(),
      description: z.string().optional(),
    })
    .optional(),
  defaults: z
    .object({
      cache_ttl: z.number().optional(),
      enforcement: z.enum(['strict', 'lenient', 'advisory']).optional(),
    })
    .optional(),
  rules: z.array(PolicyRuleSchema),
  environments: z
    .record(
      z.object({
        defaults: z.record(z.unknown()).optional(),
        overrides: z.array(EnvironmentOverrideSchema).optional(),
      }),
    )
    .optional(),
  cache: z
    .object({
      enabled: z.boolean(),
      ttl: z.number(),
    })
    .optional(),
});

// ============================================================================
// Core Functions
// ============================================================================

/**
 * Load and resolve policy from YAML file
 */
export function loadPolicy(filePath: string, environment?: string): Result<UnifiedPolicy> {
  try {
    // Load YAML file
    if (!fs.existsSync(filePath)) {
      return Failure(`Policy file not found: ${filePath}`);
    }

    const content = fs.readFileSync(filePath, 'utf-8');
    const raw = yaml.load(content);

    // Check version and migrate if needed
    const policy = isLegacyPolicy(raw) ? migrateV1ToV2(raw) : (raw as UnifiedPolicy);

    // Resolve environment overrides
    const resolved = environment ? resolveEnvironment(policy, environment) : policy;

    // Validate final policy
    return validatePolicy(resolved);
  } catch (error) {
    return Failure(`Failed to load policy: ${extractErrorMessage(error)}`);
  }
}

/**
 * Validate policy against schema
 */
export function validatePolicy(policy: unknown): Result<UnifiedPolicy> {
  try {
    const validated = UnifiedPolicySchema.parse(policy) as UnifiedPolicy;
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
export function resolveEnvironment(policy: UnifiedPolicy, environment: string): UnifiedPolicy {
  const envConfig = policy.environments?.[environment];

  // Deep clone the policy
  const resolved: UnifiedPolicy = JSON.parse(JSON.stringify(policy));

  if (envConfig) {
    // Apply environment defaults
    if (envConfig.defaults) {
      resolved.defaults = {
        ...resolved.defaults,
        ...envConfig.defaults,
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
 * Check if policy is legacy v1.0 format
 */
function isLegacyPolicy(policy: unknown): policy is LegacyPolicy {
  return (
    typeof policy === 'object' &&
    policy !== null &&
    'version' in policy &&
    'rules' in policy &&
    (policy as { version: unknown }).version === '1.0'
  );
}

/**
 * Migrate v1.0 policy to v2.0 format
 */
export function migrateV1ToV2(legacy: LegacyPolicy): UnifiedPolicy {
  const migrated: UnifiedPolicy = {
    version: '2.0',
    metadata: {
      created: new Date().toISOString(),
      description: 'Migrated from v1.0 policy',
    },
    rules: [],
  };

  // Migrate rules
  for (const oldRule of legacy.rules) {
    const conditions: Matcher[] = [];

    // Convert old matchers to new format
    if (oldRule.matchers) {
      for (const matcher of oldRule.matchers) {
        if (matcher.pattern) {
          conditions.push({
            kind: 'regex',
            pattern: matcher.pattern,
          });
        } else if (matcher.function) {
          conditions.push({
            kind: 'function',
            name: matcher.function as any,
            args: matcher.args || [],
          });
        }
      }
    }

    migrated.rules.push({
      id: oldRule.id,
      priority: oldRule.priority || 50,
      conditions,
      actions: oldRule.actions || {},
    });
  }

  return migrated;
}

// ============================================================================
// Rule Evaluation
// ============================================================================

/**
 * Evaluate a matcher against input data
 */
export function evaluateMatcher(
  matcher: Matcher,
  input: string | Record<string, unknown>,
): boolean {
  switch (matcher.kind) {
    case 'regex': {
      const regex = new RegExp(matcher.pattern, matcher.flags || 'g');
      const text = typeof input === 'string' ? input : JSON.stringify(input);

      if (matcher.count_threshold !== undefined) {
        const matches = text.match(regex);
        return (matches?.length || 0) >= matcher.count_threshold;
      }
      return regex.test(text);
    }

    case 'function': {
      return evaluateFunction(matcher, input);
    }

    default:
      return false;
  }
}

/**
 * Evaluate a function matcher
 */
function evaluateFunction(
  matcher: FunctionMatcher,
  input: string | Record<string, unknown>,
): boolean {
  switch (matcher.name) {
    case 'hasPattern': {
      const [pattern, flags] = matcher.args as [string, string?];
      const regex = new RegExp(pattern, flags);
      const text = typeof input === 'string' ? input : JSON.stringify(input);
      return regex.test(text);
    }

    case 'fileExists': {
      const [filePath] = matcher.args as [string];
      const basePath = typeof input === 'object' && input.path ? String(input.path) : '.';
      return fs.existsSync(path.join(basePath, filePath));
    }

    case 'largerThan': {
      const [size] = matcher.args as [number];
      if (typeof input === 'string') {
        return input.length > size;
      }
      if (typeof input === 'object' && input.size !== undefined) {
        return Number(input.size) > size;
      }
      return false;
    }

    case 'hasVulnerabilities': {
      const [severities] = matcher.args as [string[]];
      if (typeof input === 'object' && Array.isArray(input.vulnerabilities)) {
        return input.vulnerabilities.some((v) =>
          severities.includes(String(v.severity).toUpperCase()),
        );
      }
      return false;
    }

    default:
      return false;
  }
}

/**
 * Apply policy rules to input and return matching actions
 */
export function applyPolicy(
  policy: UnifiedPolicy,
  input: string | Record<string, unknown>,
): Array<{ rule: PolicyRule; matched: boolean }> {
  const results: Array<{ rule: PolicyRule; matched: boolean }> = [];

  for (const rule of policy.rules) {
    // All conditions must match (AND logic)
    const matched =
      rule.conditions.length > 0 &&
      rule.conditions.every((condition) => evaluateMatcher(condition, input));

    results.push({ rule, matched });
  }

  return results;
}

// ============================================================================
// Helper Functions
// ============================================================================

/**
 * Get all rule weights for optimization algorithms
 */
export function getRuleWeights(policy: UnifiedPolicy): Map<string, number> {
  const weights = new Map<string, number>();

  for (const rule of policy.rules) {
    weights.set(rule.id, rule.priority);
  }

  return weights;
}

/**
 * Select best strategy based on policy rules
 */
export function selectStrategy(
  policy: UnifiedPolicy,
  candidates: Array<{ id: string; score: number }>,
): string | null {
  // Sort candidates by score
  const sorted = [...candidates].sort((a, b) => b.score - a.score);

  if (sorted.length === 0) {
    return null;
  }

  // Apply policy rules to filter/adjust candidates
  const weights = getRuleWeights(policy);

  for (const candidate of sorted) {
    const weight = weights.get(candidate.id) || 100;
    candidate.score *= weight / 100;
  }

  // Re-sort after weight adjustment
  sorted.sort((a, b) => b.score - a.score);

  return sorted[0]?.id || null;
}

/**
 * Create default policy if none exists
 */
export function createDefaultPolicy(): UnifiedPolicy {
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

// ============================================================================
// Exports
// ============================================================================
