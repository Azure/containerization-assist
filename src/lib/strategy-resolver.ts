/**
 * Strategy Resolution Engine
 *
 * Resolves sampling strategies based on configuration and context
 */

import type { ToolContext } from '@/mcp/context';
import {
  ConfigurationManagerInterface,
  SamplingConfiguration,
  StrategySelection,
} from './sampling-config';

export interface StrategyContext {
  environment: string;
  contentType: string;
  hasDockerfile?: boolean;
  complexity?: 'low' | 'medium' | 'high';
  scaling_required?: boolean;
  [key: string]: any;
}

// ============================================================================
// FUNCTIONAL APPROACH - Pure Functions
// ============================================================================

/**
 * Resolve strategy for content generation based on context (pure function)
 */
export function resolveStrategyPure(
  config: SamplingConfiguration,
  contentType: string,
  context: ToolContext & StrategyContext,
): string[] {
  const strategies = config.strategies.strategies[contentType];

  if (!strategies || strategies.length === 0) {
    // Fallback to generic strategies
    return (
      config.strategies.strategies.generic || [
        'Generate {contentType} content',
        'Create well-structured {contentType} following best practices',
      ]
    );
  }

  const selectionRules = config.strategies.selection_rules[contentType];
  if (!selectionRules) {
    return strategies;
  }

  // Apply conditional strategy selection
  const strategyIndex = resolveStrategyIndexPure(selectionRules, context);
  const defaultIndex = selectionRules.default_strategy_index || 0;
  return [
    strategies[strategyIndex] || strategies[defaultIndex] || strategies[0] || 'Generate content',
  ];
}

/**
 * Resolve specific strategy index based on conditions (pure function)
 */
function resolveStrategyIndexPure(
  selectionRules: StrategySelection,
  context: StrategyContext,
): number {
  // Check each condition
  for (const condition of selectionRules.conditions) {
    const contextValue = context[condition.key];

    if (contextValue !== undefined && matchesConditionPure(contextValue, condition.value)) {
      return condition.strategy_index;
    }
  }

  // Return default if no conditions match
  return selectionRules.default_strategy_index;
}

/**
 * Check if context value matches condition (pure function)
 */
function matchesConditionPure(contextValue: any, conditionValue: any): boolean {
  if (typeof contextValue === typeof conditionValue) {
    return contextValue === conditionValue;
  }

  // Handle type coercion for strings/numbers/booleans
  return String(contextValue) === String(conditionValue);
}

/**
 * Get all available strategies for a content type (pure function)
 */
export function getAllStrategiesPure(config: SamplingConfiguration, contentType: string): string[] {
  return config.strategies.strategies[contentType] || [];
}

/**
 * Get strategy with variable substitution (pure function)
 */
export function getFormattedStrategyPure(
  strategy: string,
  variables: Record<string, string>,
): string {
  let formatted = strategy;

  for (const [key, value] of Object.entries(variables)) {
    formatted = formatted.replace(new RegExp(`\\{${key}\\}`, 'g'), value);
  }

  return formatted;
}

/**
 * Factory function to create a strategy resolver with closure-based config
 */
export interface StrategyResolverInterface {
  resolveStrategy(contentType: string, context: ToolContext & StrategyContext): string[];
  getAllStrategies(contentType: string, environment?: string): string[];
  getFormattedStrategy(strategy: string, variables: Record<string, string>): string;
}

export function createStrategyResolver(
  configManager: ConfigurationManagerInterface,
): StrategyResolverInterface {
  return {
    resolveStrategy(contentType: string, context: ToolContext & StrategyContext): string[] {
      const config = configManager.resolveForEnvironment(context.environment);
      return resolveStrategyPure(config, contentType, context);
    },

    getAllStrategies(contentType: string, environment: string = 'development'): string[] {
      const config = configManager.resolveForEnvironment(environment);
      return getAllStrategiesPure(config, contentType);
    },

    getFormattedStrategy(strategy: string, variables: Record<string, string>): string {
      return getFormattedStrategyPure(strategy, variables);
    },
  };
}
