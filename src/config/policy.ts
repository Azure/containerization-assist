/**
 * Unified Policy Module - Single source of truth for policy management
 *
 * Consolidates policy-lite.ts and policy-normalizer.ts into a unified module
 * with stricter types, compile-time safety, and environment resolution at load time.
 */

import { readFileSync } from 'node:fs';
import { parse as yamlParse } from 'yaml';
import { z } from 'zod';

// Core types with discriminated unions for type safety
export type Category =
  | 'security'
  | 'performance'
  | 'quality'
  | 'maintainability'
  | 'efficiency'
  | 'penalties';

export type RegexMatcher = {
  kind: 'regex';
  pattern: string;
  flags?: string;
  count_threshold?: number;
  comparison?: 'greater_than' | 'greater_than_or_equal' | 'equal' | 'less_than';
};

export type FunctionMatcher = {
  kind: 'function';
  function: string;
};

// Discriminated union for type safety
export type Matcher = RegexMatcher | FunctionMatcher;

// Rule item interface
export interface RuleItem {
  name: string;
  matcher?: {
    type?: string;
    function?: string;
    pattern?: string;
    regex?: string;
    flags?: string;
    count_threshold?: number;
    comparison?: string;
  };
  points: number;
  weight: number;
  description?: string;
}

// Category-based rules as they appear in YAML
export interface CategoryRules {
  base_score: number;
  max_score: number;
  timeout_ms: number;
  security?: RuleItem[];
  performance?: RuleItem[];
  quality?: RuleItem[];
  maintainability?: RuleItem[];
  efficiency?: RuleItem[];
  penalties?: RuleItem[];
}

// Flattened rules for internal use
export interface PolicyRulesFlat {
  base_score: number;
  max_score: number;
  timeout_ms: number;
  matchers: Array<{
    name: string;
    category: string;
    points: number;
    weight: number;
    description?: string;
    [key: string]: any;
  }>;
}

// Raw policy structure from YAML
export interface UnifiedPolicyRaw {
  version: string;
  metadata: {
    description: string;
    created: string;
    author: string;
    migration_from?: string;
  };
  weights: {
    global_categories: Record<string, number>;
    content_types: Record<string, Record<string, number>>;
  };
  rules: Record<string, CategoryRules>;
  strategies: Record<string, string[]>;
  strategy_selection: Record<
    string,
    {
      conditions: Array<{
        key: string;
        strategy_index: number;
        value: any;
      }>;
      default_strategy_index: number;
    }
  >;
  env_overrides: Record<string, any>;
  tool_defaults: Record<string, any>;
  global_penalties?: any[];
  schema_version: string;
}

// Policy interface with resolved rules
export interface UnifiedPolicy {
  raw: UnifiedPolicyRaw;
  rules: Record<string, PolicyRulesFlat>;
}

// Zod schemas for validation
const RuleItemSchema = z.object({
  name: z.string(),
  matcher: z
    .object({
      type: z.string().optional(),
      function: z.string().optional(),
      pattern: z.string().optional(),
      regex: z.string().optional(),
      flags: z.string().optional(),
      count_threshold: z.number().optional(),
      comparison: z.string().optional(),
    })
    .optional(),
  points: z.number(),
  weight: z.number(),
  description: z.string().optional(),
});

const CategoryRulesSchema = z.object({
  base_score: z.number(),
  max_score: z.number(),
  timeout_ms: z.number(),
  security: z.array(RuleItemSchema).optional(),
  performance: z.array(RuleItemSchema).optional(),
  quality: z.array(RuleItemSchema).optional(),
  maintainability: z.array(RuleItemSchema).optional(),
  efficiency: z.array(RuleItemSchema).optional(),
  penalties: z.array(RuleItemSchema).optional(),
});

const UnifiedPolicySchemaRaw = z.object({
  version: z.string(),
  metadata: z.object({
    description: z.string(),
    created: z.string(),
    author: z.string(),
    migration_from: z.string().optional(),
  }),
  weights: z.object({
    global_categories: z.record(z.number()),
    content_types: z.record(z.record(z.number())),
  }),
  rules: z.record(CategoryRulesSchema),
  strategies: z.record(z.array(z.string())),
  strategy_selection: z.record(
    z.object({
      conditions: z.array(
        z.object({
          key: z.string(),
          strategy_index: z.number(),
          value: z.any(),
        }),
      ),
      default_strategy_index: z.number(),
    }),
  ),
  env_overrides: z.record(z.any()),
  tool_defaults: z.record(z.any()),
  global_penalties: z.array(z.any()).optional(),
  schema_version: z.string(),
});

// Cache for resolved policies by environment
const policyCache = new Map<string, UnifiedPolicy>();

/**
 * Transform category-based rules to flat matchers array
 * Single implementation for consistent rule flattening
 */
export function flattenCategoryRules(categoryRules: CategoryRules): PolicyRulesFlat {
  const {
    base_score,
    max_score,
    timeout_ms,
    security,
    performance,
    quality,
    maintainability,
    efficiency,
    penalties,
  } = categoryRules;

  const matchers: PolicyRulesFlat['matchers'] = [];

  // Flatten each category into the matchers array
  const categories = {
    security,
    performance,
    quality,
    maintainability,
    efficiency,
    penalties,
  };

  Object.entries(categories).forEach(([category, rules]) => {
    if (rules && Array.isArray(rules)) {
      rules.forEach((rule) => {
        matchers.push({
          ...rule.matcher,
          name: rule.name,
          points: rule.points,
          weight: rule.weight,
          description: rule.description,
          category, // Add category for reference
        });
      });
    }
  });

  return {
    base_score,
    max_score,
    timeout_ms,
    matchers,
  };
}

/**
 * Apply environment-specific rule overrides
 */
export function applyEnvironmentOverrides(
  baseRules: PolicyRulesFlat,
  envRules?: Partial<CategoryRules>,
): PolicyRulesFlat {
  if (!envRules) return baseRules;

  // If environment provides category rules, flatten and merge
  if (envRules.security || envRules.performance || envRules.quality) {
    const envFlattened = flattenCategoryRules({
      base_score: envRules.base_score ?? baseRules.base_score,
      max_score: envRules.max_score ?? baseRules.max_score,
      timeout_ms: envRules.timeout_ms ?? baseRules.timeout_ms,
      security: envRules.security,
      performance: envRules.performance,
      quality: envRules.quality,
      maintainability: envRules.maintainability,
      efficiency: envRules.efficiency,
      penalties: envRules.penalties,
    });

    // Merge by replacing rules with same name
    const ruleMap = new Map<string, any>();

    // Start with base rules
    baseRules.matchers.forEach((rule) => {
      if (rule.name) ruleMap.set(rule.name, rule);
    });

    // Override with environment rules
    envFlattened.matchers.forEach((rule) => {
      if (rule.name) ruleMap.set(rule.name, rule);
    });

    return {
      base_score: envFlattened.base_score,
      max_score: envFlattened.max_score,
      timeout_ms: envFlattened.timeout_ms,
      matchers: Array.from(ruleMap.values()),
    };
  }

  // Simple override of scores only
  return {
    ...baseRules,
    base_score: envRules.base_score ?? baseRules.base_score,
    max_score: envRules.max_score ?? baseRules.max_score,
    timeout_ms: envRules.timeout_ms ?? baseRules.timeout_ms,
  };
}

/**
 * Load and resolve policy with environment overrides at load time
 * Returns fully resolved UnifiedPolicy object with all environment merging completed
 */
export function loadPolicy(path: string, environment = 'development'): UnifiedPolicy {
  const cacheKey = `${path}:${environment}`;

  // Return cached policy if available
  if (policyCache.has(cacheKey)) {
    return policyCache.get(cacheKey) as UnifiedPolicy;
  }

  // Load and parse raw policy
  const raw = readFileSync(path, 'utf8');
  const parsed = yamlParse(raw);
  const rawPolicy = UnifiedPolicySchemaRaw.parse(parsed);

  // Transform rules with environment resolution
  const resolvedRules: Record<string, PolicyRulesFlat> = {};

  Object.entries(rawPolicy.rules).forEach(([contentType, rules]) => {
    // Start with base rules
    const baseFlattened = flattenCategoryRules(rules);

    // Apply environment overrides if they exist
    const envOverrides = rawPolicy.env_overrides?.[environment];
    const envRules = envOverrides?.rules?.[contentType];

    const finalRules = applyEnvironmentOverrides(baseFlattened, envRules);
    resolvedRules[contentType] = finalRules;
  });

  // Create resolved policy
  const resolvedPolicy: UnifiedPolicy = {
    raw: rawPolicy as UnifiedPolicyRaw,
    rules: resolvedRules,
  };

  // Cache the resolved policy
  policyCache.set(cacheKey, resolvedPolicy);

  return resolvedPolicy;
}

/**
 * Get rule weights for a specific content type with environment already resolved
 */
export function getRuleWeights(
  policy: UnifiedPolicy,
  contentType: string,
  environment = 'development',
): Record<string, number> {
  const weights: Record<string, number> = {};

  // Get global category weights
  const globalWeights = policy.raw.weights.global_categories || {};

  // Get content-type specific weight adjustments
  const contentWeights = policy.raw.weights.content_types[contentType] || {};

  // Apply environment-specific overrides if they exist
  const envOverrides = policy.raw.env_overrides?.[environment] || {};
  const envWeights = envOverrides.weights?.content_types?.[contentType] || {};

  // Merge weights (env overrides > content-specific > global)
  const finalWeights = {
    ...globalWeights,
    ...contentWeights,
    ...envWeights,
  };

  // Get already resolved rules
  const rules = policy.rules[contentType];
  if (rules?.matchers) {
    rules.matchers.forEach((matcher: any) => {
      if (matcher.name) {
        // Rule weight * category weight
        const categoryWeight = finalWeights[matcher.category] || 1.0;
        const ruleWeight = matcher.weight || 1.0;
        weights[matcher.name] = categoryWeight * ruleWeight;
      }
    });
  }

  return weights;
}

/**
 * Select strategy based on context (environment already resolved)
 */
export function selectStrategy(
  policy: UnifiedPolicy,
  contentType: string,
  context: Record<string, unknown> = {},
): { strategy: string; index: number } {
  // Get strategy selection rules for the content type
  const selection = policy.raw.strategy_selection[contentType];
  if (!selection) {
    // Fallback to default strategy
    const strategies = policy.raw.strategies[contentType] || ['default'];
    return { strategy: strategies[0] || 'default', index: 0 };
  }

  // Check conditions to select strategy
  for (const condition of selection.conditions) {
    if (context[condition.key] === condition.value) {
      const strategies = policy.raw.strategies[contentType] || ['default'];
      const strategy = strategies[condition.strategy_index];
      if (strategy) {
        return { strategy, index: condition.strategy_index };
      }
    }
  }

  // Use default strategy index
  const strategies = policy.raw.strategies[contentType] || ['default'];
  const defaultIndex = selection.default_strategy_index || 0;
  const strategy = strategies[defaultIndex] || 'default';
  return { strategy, index: defaultIndex };
}

/**
 * Get policy rules for a content type (already resolved with environment)
 */
export function getPolicyRules(
  policy: UnifiedPolicy,
  contentType: string,
): PolicyRulesFlat | undefined {
  return policy.rules[contentType];
}

/**
 * Validate policy file without loading it into memory
 */
export function validatePolicyFile(path: string): { valid: boolean; error?: string } {
  try {
    const raw = readFileSync(path, 'utf8');
    const parsed = yamlParse(raw);
    UnifiedPolicySchemaRaw.parse(parsed);
    return { valid: true };
  } catch (error) {
    return {
      valid: false,
      error: error instanceof Error ? error.message : 'Unknown error',
    };
  }
}

// Legacy policy schema for backward compatibility
const LegacyPolicySchema = z.object({
  maxTokens: z.number().max(100000).optional(),
  maxCost: z.number().max(100).optional(),
  forbiddenModels: z.array(z.string()).optional(),
  allowedModels: z.array(z.string()).optional(),
  timeoutMs: z.number().max(600000).optional(),
});

/**
 * Validate policy data in memory (for testing)
 */
export function validatePolicyData(
  policyData: unknown,
): { ok: true; value: any } | { ok: false; error: string } {
  try {
    // Try current policy schema first
    if (policyData && typeof policyData === 'object' && 'schema_version' in policyData) {
      const validated = UnifiedPolicySchemaRaw.parse(policyData);
      return { ok: true, value: validated };
    }

    // Try legacy policy schema
    if (policyData && typeof policyData === 'object') {
      // Check if it has any legacy policy fields
      const hasLegacyFields = [
        'maxTokens',
        'maxCost',
        'forbiddenModels',
        'allowedModels',
        'timeoutMs',
      ].some((field) => field in policyData);

      if (hasLegacyFields || Object.keys(policyData).length === 0) {
        const validated = LegacyPolicySchema.parse(policyData);
        return { ok: true, value: validated };
      }
    }

    // If no schema_version is detected, try current format for better error messages
    const validated = UnifiedPolicySchemaRaw.parse(policyData);
    return { ok: true, value: validated };
  } catch (error) {
    let errorMessage = 'Policy validation failed';
    if (error instanceof Error) {
      // Try to parse Zod errors for better formatting
      try {
        const zodErrors = JSON.parse(error.message);
        if (Array.isArray(zodErrors) && zodErrors.length > 0) {
          const firstError = zodErrors[0];
          if (firstError.path && firstError.message) {
            errorMessage = `Policy validation failed: ${firstError.path.join('.')}: ${firstError.message}`;
          } else {
            errorMessage = `Policy validation failed: ${firstError.message || error.message}`;
          }
        } else {
          errorMessage = `Policy validation failed: ${error.message}`;
        }
      } catch {
        // Not a JSON error, use original message
        errorMessage = `Policy validation failed: ${error.message}`;
      }
    }
    return {
      ok: false,
      error: errorMessage,
    };
  }
}

/**
 * Get all available content types from policy
 */
export function getContentTypes(policy: UnifiedPolicy): string[] {
  return Object.keys(policy.rules);
}

/**
 * Get tool defaults for a specific tool with environment already resolved
 */
export function getToolDefaults(
  policy: UnifiedPolicy,
  toolName: string,
  environment = 'development',
): Record<string, unknown> {
  // Start with base tool defaults
  const baseDefaults = policy.raw.tool_defaults[toolName] || {};

  // Apply environment-specific overrides
  const envOverrides = policy.raw.env_overrides?.[environment];
  const envDefaults = envOverrides?.tool_defaults?.[toolName] || {};

  return {
    ...baseDefaults,
    ...envDefaults,
  };
}

/**
 * Clear policy cache (useful for testing)
 */
export function clearPolicyCache(): void {
  policyCache.clear();
}

/**
 * Get available strategies for a content type
 */
export function getAvailableStrategies(policy: UnifiedPolicy, contentType: string): string[] {
  return policy.raw.strategies[contentType] || [];
}

/**
 * Get policy metadata
 */
export function getPolicyMetadata(policy: UnifiedPolicy): UnifiedPolicyRaw['metadata'] {
  return policy.raw.metadata;
}
