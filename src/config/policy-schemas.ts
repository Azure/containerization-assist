/**
 * Policy Schemas and Types
 * Type definitions and Zod schemas for policy structures
 */

import { z } from 'zod';

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

const PolicyRuleSchema = z.object({
  id: z.string(),
  category: z.enum(['quality', 'security', 'performance', 'compliance']).optional(),
  priority: z.number(),
  conditions: z.array(z.unknown()),
  actions: z.record(z.string(), z.unknown()),
  description: z.string().optional(),
});

// Defaults schema that includes both base and environment-specific fields
const UnifiedDefaultsSchema = z.object({
  cache_ttl: z.number().optional(),
  enforcement: z.enum(['strict', 'lenient', 'advisory']).optional(),
  // Include all environment-specific fields as optional
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
      name: z.string().optional(),
      created: z.string().optional(),
      author: z.string().optional(),
      description: z.string().optional(),
      category: z.string().optional(),
    })
    .optional(),
  defaults: UnifiedDefaultsSchema.optional(),
  rules: z.array(PolicyRuleSchema),
  cache: z
    .object({
      enabled: z.boolean(),
      ttl: z.number(),
    })
    .optional(),
});

/**
 * Policy rule with priority and actions
 * Derived from PolicyRuleSchema but with properly typed conditions
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
 * Policy structure
 * Derived from PolicySchema
 */
export type Policy = z.infer<typeof PolicySchema>;
