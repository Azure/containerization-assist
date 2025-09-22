/**
 * Policy Schemas and Types
 * Type definitions and Zod schemas for policy structures
 */

import { z } from 'zod';

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
 * Typed environment defaults for data-driven constraint generation
 */
export interface EnvironmentDefaults {
  allowedBaseImages?: string[];
  registries?: {
    allowed?: string[];
    blocked?: string[];
  };
  security?: {
    scanners?: {
      required?: boolean;
      tools?: string[];
    };
    nonRootUser?: boolean;
    minimizeSize?: boolean;
  };
  resources?: {
    limits?: {
      cpu?: string;
      memory?: string;
    };
    requests?: {
      cpu?: string;
      memory?: string;
    };
  };
  naming?: {
    pattern?: string;
    examples?: string[];
  };
}

/**
 * Policy structure (v2.0)
 */
export interface Policy {
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
      defaults?: EnvironmentDefaults;
      overrides?: EnvironmentOverride[];
    }
  >;
  cache?: CacheConfig;
}

// ============================================================================
// Schema Validation
// ============================================================================

export const RegexMatcherSchema = z.object({
  kind: z.literal('regex'),
  pattern: z.string(),
  flags: z.string().optional(),
  count_threshold: z.number().optional(),
});

export const FunctionMatcherSchema = z.object({
  kind: z.literal('function'),
  name: z.enum(['hasPattern', 'fileExists', 'largerThan', 'hasVulnerabilities']),
  args: z.array(z.unknown()),
});

export const MatcherSchema = z.discriminatedUnion('kind', [
  RegexMatcherSchema,
  FunctionMatcherSchema,
]);

export const PolicyRuleSchema = z.object({
  id: z.string(),
  category: z.enum(['quality', 'security', 'performance', 'compliance']).optional(),
  priority: z.number(),
  conditions: z.array(MatcherSchema),
  actions: z.record(z.unknown()),
  description: z.string().optional(),
});

export const EnvironmentOverrideSchema = z.object({
  rule_id: z.string(),
  actions: z.record(z.unknown()).optional(),
  priority: z.number().optional(),
  enabled: z.boolean().optional(),
});

export const EnvironmentDefaultsSchema = z.object({
  allowedBaseImages: z.array(z.string()).optional(),
  registries: z
    .object({
      allowed: z.array(z.string()).optional(),
      blocked: z.array(z.string()).optional(),
    })
    .optional(),
  security: z
    .object({
      scanners: z
        .object({
          required: z.boolean().optional(),
          tools: z.array(z.string()).optional(),
        })
        .optional(),
      nonRootUser: z.boolean().optional(),
      minimizeSize: z.boolean().optional(),
    })
    .optional(),
  resources: z
    .object({
      limits: z
        .object({
          cpu: z.string().optional(),
          memory: z.string().optional(),
        })
        .optional(),
      requests: z
        .object({
          cpu: z.string().optional(),
          memory: z.string().optional(),
        })
        .optional(),
    })
    .optional(),
  naming: z
    .object({
      pattern: z.string().optional(),
      examples: z.array(z.string()).optional(),
    })
    .optional(),
});

export const PolicySchema = z.object({
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
        defaults: EnvironmentDefaultsSchema.optional(),
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
