/**
 * Strategy Resolution Engine
 *
 * Resolves sampling strategies based on configuration and context
 */

import type { ToolContext } from '@mcp/context';
import { ConfigurationManager } from './sampling-config';

export interface StrategyContext {
  environment: string;
  contentType: string;
  hasDockerfile?: boolean;
  complexity?: 'low' | 'medium' | 'high';
  scaling_required?: boolean;
  [key: string]: any;
}

export class StrategyResolver {
  constructor(private configManager: ConfigurationManager) {}

  /**
   * Resolve strategy for content generation based on context
   */
  resolveStrategy(contentType: string, context: ToolContext & StrategyContext): string[] {
    const config = this.configManager.resolveForEnvironment(context.environment);
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
    const strategyIndex = this.resolveStrategyIndex(selectionRules, context);
    const defaultIndex = selectionRules.default_strategy_index || 0;
    return [
      strategies[strategyIndex] || strategies[defaultIndex] || strategies[0] || 'Generate content',
    ];
  }

  /**
   * Resolve specific strategy index based on conditions
   */
  private resolveStrategyIndex(selectionRules: any, context: StrategyContext): number {
    // Check each condition
    for (const condition of selectionRules.conditions) {
      const contextValue = context[condition.key];

      if (contextValue !== undefined && this.matchesCondition(contextValue, condition.value)) {
        return condition.strategy_index;
      }
    }

    // Return default if no conditions match
    return selectionRules.default_strategy_index;
  }

  /**
   * Check if context value matches condition
   */
  private matchesCondition(contextValue: any, conditionValue: any): boolean {
    if (typeof contextValue === typeof conditionValue) {
      return contextValue === conditionValue;
    }

    // Handle type coercion for strings/numbers/booleans
    return String(contextValue) === String(conditionValue);
  }

  /**
   * Get all available strategies for a content type
   */
  getAllStrategies(contentType: string, environment: string = 'development'): string[] {
    const config = this.configManager.resolveForEnvironment(environment);
    return config.strategies.strategies[contentType] || [];
  }

  /**
   * Get strategy with variable substitution
   */
  getFormattedStrategy(strategy: string, variables: Record<string, string>): string {
    let formatted = strategy;

    for (const [key, value] of Object.entries(variables)) {
      formatted = formatted.replace(new RegExp(`\\{${key}\\}`, 'g'), value);
    }

    return formatted;
  }
}

/**
 * Create strategy resolver instance
 */
export function createStrategyResolver(configManager: ConfigurationManager): StrategyResolver {
  return new StrategyResolver(configManager);
}
