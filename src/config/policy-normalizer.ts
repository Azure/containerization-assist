/**
 * Policy Normalizer - Single source of truth for policy transformations
 * Eliminates duplication between policy-lite.ts and resolver.ts
 */

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

// Flattened rules for internal use
export interface PolicyRules {
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

/**
 * Transform category-based rules to flat matchers array
 * Single implementation used by both policy-lite and resolver
 */
export function flattenCategoryRules(categoryRules: CategoryRules): PolicyRules {
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

  const matchers: PolicyRules['matchers'] = [];

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
  baseRules: PolicyRules,
  envRules?: Partial<CategoryRules>,
): PolicyRules {
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
