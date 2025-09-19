/**
 * Pure Policy Resolver - Lightweight policy resolution without global state
 *
 * This module provides pure functions for loading and working with policies
 * without any global mutable state or side effects.
 */

import { readFileSync } from 'node:fs';
import { parse as yamlParse } from 'yaml';
import { z } from 'zod';

// Import the schemas from resolver
import type { PolicyRules } from './resolver';
// Import the normalizer functions
import {
  flattenCategoryRules,
  type CategoryRules as NormalizerCategoryRules,
} from './policy-normalizer';

// Re-define minimal schemas needed for pure policy loading
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
          value: z.any(),
          strategy_index: z.number(),
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

export type UnifiedPolicy = z.infer<typeof UnifiedPolicySchemaRaw> & {
  rules: Record<string, PolicyRules>;
};

/**
 * Transform entire unified policy from raw YAML structure to flat matchers
 */
function transformUnifiedPolicy(rawPolicy: z.infer<typeof UnifiedPolicySchemaRaw>): UnifiedPolicy {
  const transformedRules: Record<string, PolicyRules> = {};

  // Transform each content type's rules
  Object.entries(rawPolicy.rules).forEach(([contentType, rules]) => {
    transformedRules[contentType] = flattenCategoryRules(rules as NormalizerCategoryRules);
  });

  return {
    ...rawPolicy,
    rules: transformedRules,
  };
}

/**
 * Load unified policy from file path
 * Pure function - no global state
 */
export function loadUnifiedPolicy(path: string): UnifiedPolicy {
  const raw = readFileSync(path, 'utf8');
  const parsed = yamlParse(raw);
  const rawPolicy = UnifiedPolicySchemaRaw.parse(parsed);
  return transformUnifiedPolicy(rawPolicy);
}

/**
 * Get rule weights for a specific content type
 * Pure function - no global state
 */
export function getRuleWeights(
  policy: UnifiedPolicy,
  contentType: string,
  environment = 'development',
): Record<string, number> {
  const weights: Record<string, number> = {};

  // Get global category weights
  const globalWeights = policy.weights.global_categories || {};

  // Get content-type specific weight adjustments
  const contentWeights = policy.weights.content_types[contentType] || {};

  // Apply environment-specific overrides if they exist
  const envOverrides = policy.env_overrides?.[environment] || {};
  const envWeights = envOverrides.weights?.content_types?.[contentType] || {};

  // Merge weights (env overrides > content-specific > global)
  const finalWeights = {
    ...globalWeights,
    ...contentWeights,
    ...envWeights,
  };

  // Get rules with environment overrides applied
  const rules = getPolicyRules(policy, contentType, environment);
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
 * Select strategy based on context and environment
 * Pure function - no global state
 */
export function selectStrategy(
  policy: UnifiedPolicy,
  contentType: string,
  context: Record<string, unknown> = {},
  _environment = 'development',
): { strategy: string; index: number } {
  // Get strategy selection rules for the content type
  const selection = policy.strategy_selection[contentType];
  if (!selection) {
    // Fallback to default strategy
    const strategies = policy.strategies[contentType] || ['default'];
    return { strategy: strategies[0] || 'default', index: 0 };
  }

  // Check conditions to select strategy
  for (const condition of selection.conditions) {
    if (context[condition.key] === condition.value) {
      const strategies = policy.strategies[contentType] || ['default'];
      const strategy = strategies[condition.strategy_index];
      if (strategy) {
        return { strategy, index: condition.strategy_index };
      }
    }
  }

  // Use default strategy index
  const strategies = policy.strategies[contentType] || ['default'];
  const defaultIndex = selection.default_strategy_index || 0;
  const strategy = strategies[defaultIndex] || 'default';
  return { strategy, index: defaultIndex };
}

/**
 * Get policy rules for a content type
 * Pure function - no global state
 */
export function getPolicyRules(
  policy: UnifiedPolicy,
  contentType: string,
  environment = 'development',
): PolicyRules | undefined {
  const rules = policy.rules[contentType];
  if (!rules) {
    return undefined;
  }

  // Apply environment-specific overrides if they exist
  const envOverrides = policy.env_overrides?.[environment];
  if (envOverrides?.rules?.[contentType]) {
    // Keep original logic since env rules structure may not match CategoryRules perfectly
    const envCategoryRules = envOverrides.rules[contentType];
    const transformedEnvRules = flattenCategoryRules({
      base_score: envCategoryRules.base_score || rules.base_score,
      max_score: envCategoryRules.max_score || rules.max_score,
      timeout_ms: envCategoryRules.timeout_ms || rules.timeout_ms,
      security: envCategoryRules.security,
      performance: envCategoryRules.performance,
      quality: envCategoryRules.quality,
      maintainability: envCategoryRules.maintainability,
      efficiency: envCategoryRules.efficiency,
    } as z.infer<typeof CategoryRulesSchema>);

    // Merge environment rules by overriding rules with the same name
    const baseMatchers = rules.matchers || [];
    const envMatchers = transformedEnvRules.matchers || [];

    // Create a map of environment rules by name for easy lookup
    const envRuleMap = new Map<string, any>();
    envMatchers.forEach((envRule: any) => {
      if (envRule.name) {
        envRuleMap.set(envRule.name, envRule);
      }
    });

    // Override base rules with environment rules where names match
    const mergedMatchers = baseMatchers.map((baseRule: any) => {
      if (baseRule.name && envRuleMap.has(baseRule.name)) {
        return { ...baseRule, ...envRuleMap.get(baseRule.name) };
      }
      return baseRule;
    });

    // Add any environment rules that don't override existing ones
    envMatchers.forEach((envRule: any) => {
      if (envRule.name && !baseMatchers.find((base: any) => base.name === envRule.name)) {
        mergedMatchers.push(envRule);
      }
    });

    return {
      ...rules,
      base_score: envCategoryRules.base_score || rules.base_score,
      max_score: envCategoryRules.max_score || rules.max_score,
      timeout_ms: envCategoryRules.timeout_ms || rules.timeout_ms,
      matchers: mergedMatchers,
    };
  }

  return rules;
}

/**
 * Validate policy file without loading it into global state
 * Pure function - returns validation result
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

/**
 * Get all available content types from policy
 * Pure function - no global state
 */
export function getContentTypes(policy: UnifiedPolicy): string[] {
  return Object.keys(policy.rules);
}

/**
 * Get tool defaults for a specific tool
 * Pure function - no global state
 */
export function getToolDefaults(
  policy: UnifiedPolicy,
  toolName: string,
  environment = 'development',
): Record<string, unknown> {
  // Start with base tool defaults
  const baseDefaults = policy.tool_defaults[toolName] || {};

  // Apply environment-specific overrides
  const envOverrides = policy.env_overrides?.[environment];
  const envDefaults = envOverrides?.tool_defaults?.[toolName] || {};

  return {
    ...baseDefaults,
    ...envDefaults,
  };
}
