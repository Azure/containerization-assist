/**
 * Config Resolver - Simplified policy resolution using unified policy module
 *
 * Provides core resolution logic with simplified interface.
 * Uses the new unified policy module for all policy operations.
 */

import type { Logger } from 'pino';
import { z } from 'zod';
import { config as baseConfig } from './index';
import {
  loadPolicy,
  validatePolicyFile,
  getRuleWeights as getPolicyRuleWeights,
  getPolicyRules,
  selectStrategy,
  getToolDefaults as getPolicyToolDefaults,
  type UnifiedPolicy,
  type PolicyRulesFlat as PolicyRules,
} from './policy';

// Logger is now passed as parameter instead of global

// Legacy schemas for backward compatibility
const LegacyMatcherSchema = z.object({
  pattern: z.string().optional(),
  regex: z.string().optional(),
  weight: z.number(),
  description: z.string().optional(),
  type: z.string().optional(),
  instruction: z.string().optional(),
  resource: z.string().optional(),
  field: z.string().optional(),
  file_pattern: z.string().optional(),
  content_pattern: z.string().optional(),
});

// Legacy policy schemas for backward compatibility

const LegacyPolicyRulesSchema = z.object({
  base_score: z.number(),
  max_score: z.number(),
  timeout_ms: z.number(),
  matchers: z.array(LegacyMatcherSchema),
});

// Legacy policy schema for backward compatibility
const _LegacyPolicySchema = z.object({
  rules: z.record(LegacyPolicyRulesSchema),
  strategies: z.record(z.array(z.string())).optional(),
});

// Type exports for backward compatibility
export type LegacyPolicy = z.infer<typeof _LegacyPolicySchema>;
export type { PolicyRules };
export type { UnifiedPolicy };

// Policy schema removed - using the one from ./policy module

// Strategy schema
const _StrategySchema = z.object({
  model: z.string().optional(),
  maxTokens: z.number().optional(),
  temperature: z.number().min(0).max(2).optional(),
  timeout: z.number().optional(),
});

// Type exports are now imported from ./policy module
// PolicyRules type is imported from ./policy module

export type Strategy = z.infer<typeof _StrategySchema>;

// Config cache removed - now using stateless functions

/**
 * Load configuration with optional policy file
 * Simplified interface using the unified policy module
 */
export function loadConfig(options?: {
  policyFile?: string;
  environment?: string;
  logger?: Logger;
}): typeof baseConfig & {
  unifiedPolicy?: UnifiedPolicy;
} {
  const logger = options?.logger?.child({ component: 'ConfigResolver' });
  const environment = options?.environment || 'development';

  const merged: typeof baseConfig & { unifiedPolicy?: UnifiedPolicy } = {
    ...baseConfig,
  };

  // Load policy file if provided
  if (options?.policyFile) {
    try {
      const unifiedPolicy = loadPolicy(options.policyFile, environment);
      merged.unifiedPolicy = unifiedPolicy;

      logger?.info(
        {
          policyFile: options.policyFile,
          environment,
          version: unifiedPolicy.raw.version,
        },
        'Loaded unified policy',
      );
    } catch (error) {
      logger?.warn({ error, file: options.policyFile }, 'Failed to load policy file');
    }
  }

  return merged;
}

/**
 * Get resolved config with policy enforcement
 * @deprecated Use loadConfig() instead for stateless operation
 */
export function getConfig(policyFile?: string): typeof baseConfig & {
  unifiedPolicy?: UnifiedPolicy;
  legacyPolicy?: LegacyPolicy;
  strategies?: Record<string, Strategy>;
} {
  return loadConfig({ policyFile });
}

/**
 * Initialize config resolver with optional policy file
 * @deprecated Use loadConfig() instead
 */
export function initializeResolver(options?: {
  policyFile?: string;
  environment?: string;
  logger?: Logger;
}): void {
  // For backward compatibility, just call loadConfig
  loadConfig(options);
}

/**
 * Get effective config for a specific tool (simplified)
 */
export function getEffectiveConfig(
  _toolName: string, // Currently unused, kept for backward compatibility
  options: {
    contentType?: string;
    environment?: string;
  } = {},
): {
  model: string;
  weights?: Record<string, number>;
  rules?: PolicyRules;
} {
  const { contentType = 'generic', environment = 'development' } = options;
  const cfg = getConfig();
  const unifiedPolicy = cfg.unifiedPolicy;

  const effectiveModel = cfg.ai?.defaultModel || 'claude-3-haiku';
  let weights: Record<string, number> | undefined;
  let rules: PolicyRules | undefined;

  if (unifiedPolicy) {
    // Use the new policy module functions
    weights = getPolicyRuleWeights(unifiedPolicy, contentType, environment);
    rules = getPolicyRules(unifiedPolicy, contentType);
  }

  return {
    model: effectiveModel,
    weights,
    rules,
  };
}

/**
 * Get config cache info
 */
export function getCacheInfo(): { hash: string; age: number } | null {
  // No longer using cache, return null
  return null;
}

/**
 * Clear config cache (for testing)
 */
export function clearCache(): void {
  // No-op since we removed the cache
  // No-op since we removed the cache
}

/**
 * Get rule weights for a specific content type and environment (simplified)
 */
export function getRuleWeights(
  contentType: string,
  environment = 'development',
): Record<string, number> | null {
  const cfg = getConfig();
  const unifiedPolicy = cfg.unifiedPolicy;

  if (!unifiedPolicy) {
    return null;
  }

  // Get policy data
  return getPolicyRuleWeights(unifiedPolicy, contentType, environment);
}

/**
 * Get strategy selection for a tool and context (simplified)
 */
export function getStrategySelection(
  contentType: string,
  context: Record<string, any> = {},
): { strategy: string; index: number } | null {
  const cfg = getConfig();
  const unifiedPolicy = cfg.unifiedPolicy;

  if (!unifiedPolicy) {
    return null;
  }

  // Get policy data
  return selectStrategy(unifiedPolicy, contentType, context);
}

/**
 * Get tool defaults for a specific tool (simplified)
 */
export function getToolDefaults(
  toolName: string,
  environment = 'development',
): Record<string, any> | null {
  const cfg = getConfig();
  const unifiedPolicy = cfg.unifiedPolicy;

  if (!unifiedPolicy) {
    return null;
  }

  // Get policy data
  return getPolicyToolDefaults(unifiedPolicy, toolName, environment);
}

/**
 * Check if unified policy format is being used
 */
export function isUnifiedPolicyEnabled(): boolean {
  const cfg = getConfig();
  return !!cfg.unifiedPolicy;
}

/**
 * Validate a policy file (simplified)
 */
export function validatePolicy(
  policyPath: string,
): { ok: true; value: any } | { ok: false; error: string } {
  const result = validatePolicyFile(policyPath);
  return result.valid
    ? { ok: true, value: null }
    : { ok: false, error: result.error || 'Unknown error' };
}
