/**
 * Configuration-Driven Sampling System
 *
 * Provides configuration loading, validation, and environment resolution
 * for sampling rules and strategies.
 */

import { readFileSync } from 'fs';
import { join } from 'path';
import { ValidationResult } from '../validation/core-types';
import { load as yamlLoad } from 'js-yaml';
import { z } from 'zod';
import { Result, Success, Failure } from '../types';
import { extractErrorMessage } from './error-utils';

// Configuration Schema Definitions
const MatcherSchema = z.union([
  z.object({
    type: z.literal('regex'),
    pattern: z.string(),
    flags: z.string().optional(),
    count_threshold: z.number().optional(),
    comparison: z
      .enum(['greater_than', 'less_than', 'equal', 'greater_than_or_equal', 'less_than_or_equal'])
      .optional(),
  }),
  z.object({
    type: z.literal('function'),
    function: z.string(),
    threshold: z.number().optional(),
    comparison: z
      .enum(['greater_than', 'less_than', 'equal', 'greater_than_or_equal', 'less_than_or_equal'])
      .optional(),
  }),
]);

const ScoringRuleSchema = z.object({
  name: z.string(),
  matcher: MatcherSchema,
  points: z.number(),
  weight: z.number(),
  category: z.string(),
  description: z.string(),
});

const PenaltyRuleSchema = z.object({
  name: z.string(),
  matcher: MatcherSchema,
  points: z.number().negative(), // Must be negative for penalties
  description: z.string(),
});

const ScoringProfileSchema = z.object({
  name: z.string(),
  version: z.string(),
  metadata: z.object({
    description: z.string(),
    created: z.string(),
    author: z.string(),
  }),
  base_score: z.number(),
  max_score: z.number(),
  timeout_ms: z.number(),
  category_weights: z.record(z.string(), z.number()),
  rules: z.record(z.string(), z.array(ScoringRuleSchema)),
  penalties: z.array(PenaltyRuleSchema).optional(),
});

const StrategyConditionSchema = z.object({
  key: z.string(),
  value: z.union([z.string(), z.number(), z.boolean()]),
  strategy_index: z.number(),
});

const StrategySelectionSchema = z.object({
  conditions: z.array(StrategyConditionSchema),
  default_strategy_index: z.number(),
});

const StrategiesConfigSchema = z.object({
  version: z.string(),
  strategies: z.record(z.string(), z.array(z.string())),
  selection_rules: z.record(z.string(), StrategySelectionSchema),
});

const EnvironmentOverrideSchema = z.object({
  environment: z.string(),
  overrides: z.object({
    scoring: z
      .record(
        z.string(),
        z.object({
          rules: z
            .record(
              z.string(),
              z.array(
                z.object({
                  name: z.string(),
                  points: z.number().optional(),
                  weight: z.number().optional(),
                }),
              ),
            )
            .optional(),
          category_weights: z.record(z.string(), z.number()).optional(),
        }),
      )
      .optional(),
    sampling: z
      .object({
        max_candidates: z.number().optional(),
        timeout_ms: z.number().optional(),
        early_stop_threshold: z.number().optional(),
      })
      .optional(),
    strategies: z
      .record(
        z.string(),
        z.object({
          default_strategy_index: z.number().optional(),
          conditions: z.array(StrategyConditionSchema).optional(),
        }),
      )
      .optional(),
  }),
});

// TypeScript interfaces derived from schemas
export type ScoringMatcher = z.infer<typeof MatcherSchema>;
export type ScoringRule = z.infer<typeof ScoringRuleSchema>;
export type PenaltyRule = z.infer<typeof PenaltyRuleSchema>;
export type ScoringProfile = z.infer<typeof ScoringProfileSchema>;
export type StrategyCondition = z.infer<typeof StrategyConditionSchema>;
export type StrategySelection = z.infer<typeof StrategySelectionSchema>;
export type StrategiesConfig = z.infer<typeof StrategiesConfigSchema>;
export type EnvironmentOverride = z.infer<typeof EnvironmentOverrideSchema>;

export interface SamplingConfiguration {
  version: string;
  scoring: Record<string, ScoringProfile>;
  strategies: StrategiesConfig;
  environments: Record<string, EnvironmentOverride>;
}

// ValidationResult now imported from canonical source

// ============================================================================
// FUNCTIONAL APPROACH - State and Pure Functions
// ============================================================================

/**
 * Configuration state interface
 */
export interface ConfigurationState {
  config: SamplingConfiguration | null;
  configPath: string;
}

/**
 * Load configuration from YAML files (pure function)
 */
export async function loadConfigurationPure(
  state: ConfigurationState,
): Promise<Result<SamplingConfiguration>> {
  try {
    // Load scoring profiles
    const scoring: Record<string, ScoringProfile> = {};

    try {
      const dockerfileYaml = readFileSync(
        join(state.configPath, 'scoring', 'dockerfile.yml'),
        'utf8',
      );
      const dockerfileConfig = yamlLoad(dockerfileYaml);
      const parsed = ScoringProfileSchema.safeParse(dockerfileConfig);
      if (!parsed.success) {
        return Failure(`Invalid dockerfile scoring config: ${parsed.error.message}`);
      }
      scoring.dockerfile = parsed.data;
    } catch (error) {
      return Failure(`Failed to load dockerfile scoring config: ${extractErrorMessage(error)}`);
    }

    // Load strategies
    let strategies: StrategiesConfig;
    try {
      const strategiesYaml = readFileSync(join(state.configPath, 'strategies.yml'), 'utf8');
      const strategiesConfig = yamlLoad(strategiesYaml);
      const parsed = StrategiesConfigSchema.safeParse(strategiesConfig);
      if (!parsed.success) {
        return Failure(`Invalid strategies config: ${parsed.error.message}`);
      }
      strategies = parsed.data;
    } catch (error) {
      return Failure(`Failed to load strategies config: ${extractErrorMessage(error)}`);
    }

    // Load environment overrides
    const environments: Record<string, EnvironmentOverride> = {};

    // Load production environment
    try {
      const prodYaml = readFileSync(
        join(state.configPath, 'environments', 'production.yml'),
        'utf8',
      );
      const prodConfig = yamlLoad(prodYaml);
      const parsed = EnvironmentOverrideSchema.safeParse(prodConfig);
      if (!parsed.success) {
        return Failure(`Invalid production environment config: ${parsed.error.message}`);
      }
      environments.production = parsed.data;
    } catch (error) {
      return Failure(`Failed to load production environment config: ${extractErrorMessage(error)}`);
    }

    // Load development environment
    try {
      const devYaml = readFileSync(
        join(state.configPath, 'environments', 'development.yml'),
        'utf8',
      );
      const devConfig = yamlLoad(devYaml);
      const parsed = EnvironmentOverrideSchema.safeParse(devConfig);
      if (!parsed.success) {
        return Failure(`Invalid development environment config: ${parsed.error.message}`);
      }
      environments.development = parsed.data;
    } catch (error) {
      return Failure(
        `Failed to load development environment config: ${extractErrorMessage(error)}`,
      );
    }

    // Assemble final configuration
    const config: SamplingConfiguration = {
      version: '1.0.0',
      scoring,
      strategies,
      environments,
    };

    return Success(config);
  } catch (error) {
    return Failure(`Failed to load configuration: ${extractErrorMessage(error)}`);
  }
}

/**
 * Validate configuration structure (pure function)
 */
export async function validateConfigurationPure(
  config: SamplingConfiguration,
): Promise<ValidationResult> {
  const errors: string[] = [];

  // Basic structure validation
  if (!config.scoring || Object.keys(config.scoring).length === 0) {
    errors.push('Configuration must include at least one scoring profile');
  }

  if (!config.strategies) {
    errors.push('Configuration must include strategies');
  }

  // Validate scoring profiles
  for (const [profileName, profile] of Object.entries(config.scoring || {})) {
    const result = ScoringProfileSchema.safeParse(profile);
    if (!result.success) {
      errors.push(`Invalid scoring profile '${profileName}': ${result.error.message}`);
    }
  }

  // Validate strategies
  if (config.strategies) {
    const result = StrategiesConfigSchema.safeParse(config.strategies);
    if (!result.success) {
      errors.push(`Invalid strategies config: ${result.error.message}`);
    }
  }

  // Validate environment overrides
  for (const [envName, envConfig] of Object.entries(config.environments || {})) {
    const result = EnvironmentOverrideSchema.safeParse(envConfig);
    if (!result.success) {
      errors.push(`Invalid environment override '${envName}': ${result.error.message}`);
    }
  }

  return {
    isValid: errors.length === 0,
    errors,
  };
}

/**
 * Resolve configuration for specific environment (pure function)
 */
export function resolveForEnvironmentPure(
  config: SamplingConfiguration,
  environment: string = 'development',
): SamplingConfiguration {
  // Deep clone the base config to avoid mutating the original
  const baseConfig = {
    ...config,
    scoring: { ...config.scoring },
  };
  const envOverride = config.environments[environment];

  if (!envOverride) {
    return baseConfig;
  }

  // Apply environment overrides
  const resolvedConfig = {
    ...baseConfig,
    scoring: { ...baseConfig.scoring },
  };

  // Apply scoring overrides
  if (envOverride.overrides.scoring) {
    for (const [profileName, overrides] of Object.entries(envOverride.overrides.scoring)) {
      if (resolvedConfig.scoring[profileName]) {
        const profile = { ...resolvedConfig.scoring[profileName] };

        // Apply category weight overrides
        if (overrides.category_weights) {
          profile.category_weights = {
            ...profile.category_weights,
            ...overrides.category_weights,
          };
        }

        // Apply rule overrides
        if (overrides.rules) {
          for (const [categoryName, ruleOverrides] of Object.entries(overrides.rules)) {
            if (profile.rules?.[categoryName] && ruleOverrides) {
              const existingRules = profile.rules[categoryName];
              if (existingRules) {
                profile.rules[categoryName] = existingRules.map((rule) => {
                  const override = ruleOverrides?.find((ro) => ro.name === rule.name);
                  if (override) {
                    return {
                      ...rule,
                      ...(override.points !== undefined && { points: override.points }),
                      ...(override.weight !== undefined && { weight: override.weight }),
                    };
                  }
                  return rule;
                });
              }
            }
          }
        }

        resolvedConfig.scoring[profileName] = profile as ScoringProfile;
      }
    }
  }

  // Apply strategy overrides
  if (envOverride.overrides.strategies) {
    const strategies = { ...resolvedConfig.strategies };
    strategies.selection_rules = { ...strategies.selection_rules };

    for (const [strategyType, overrides] of Object.entries(envOverride.overrides.strategies)) {
      if (strategies.selection_rules[strategyType]) {
        const selection = { ...strategies.selection_rules[strategyType] };

        if (overrides.default_strategy_index !== undefined) {
          selection.default_strategy_index = overrides.default_strategy_index;
        }

        if (overrides.conditions) {
          selection.conditions = [...(selection.conditions || []), ...overrides.conditions];
        }

        strategies.selection_rules[strategyType] = {
          conditions: selection.conditions || [],
          default_strategy_index: selection.default_strategy_index || 0,
        };
      }
    }

    resolvedConfig.strategies = strategies;
  }

  return resolvedConfig;
}

/**
 * Factory function to create a configuration manager with closure-based state
 * This is the preferred approach for new code
 */
export interface ConfigurationManagerInterface {
  loadConfiguration: () => Promise<Result<void>>;
  validateConfiguration: (config: SamplingConfiguration) => Promise<ValidationResult>;
  resolveForEnvironment: (environment?: string) => SamplingConfiguration;
  getConfiguration: () => SamplingConfiguration;
  getScoringProfile: (profileName: string) => ScoringProfile | undefined;
}

export function createConfigurationManager(configPath?: string): ConfigurationManagerInterface {
  const state: ConfigurationState = {
    config: null,
    configPath: configPath || join(process.cwd(), 'config', 'sampling'),
  };

  return {
    async loadConfiguration(): Promise<Result<void>> {
      const result = await loadConfigurationPure(state);
      if (result.ok) {
        state.config = result.value;
        return Success(undefined);
      }
      return result;
    },

    async validateConfiguration(config: SamplingConfiguration): Promise<ValidationResult> {
      return validateConfigurationPure(config);
    },

    resolveForEnvironment(environment?: string): SamplingConfiguration {
      if (!state.config) {
        throw new Error('Configuration not loaded. Call loadConfiguration() first.');
      }
      return resolveForEnvironmentPure(state.config, environment);
    },

    getConfiguration(): SamplingConfiguration {
      if (!state.config) {
        throw new Error('Configuration not loaded. Call loadConfiguration() first.');
      }
      return state.config;
    },

    getScoringProfile(profileName: string): ScoringProfile | undefined {
      return state.config?.scoring[profileName];
    },
  };
}
