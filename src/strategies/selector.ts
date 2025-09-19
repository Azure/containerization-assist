/**
 * Context-Aware Strategy Selection with Cost Optimization
 * Intelligently selects the optimal strategy based on context, cost, and performance
 */

import type { Logger } from 'pino';
import { Result, Success, Failure } from '@types';
import type { Strategy } from './schema';

/**
 * Strategy selection context with enhanced metadata
 */
export interface StrategySelectionContext {
  tool: string;
  // Cost considerations
  budget?: {
    maxCost: number;
    currency: 'USD';
  };

  // Performance requirements
  performance?: {
    maxLatencyMs: number;
    preferAccuracy: boolean;
    preferSpeed: boolean;
  };

  // Historical data
  history?: {
    previousExecutions: number;
    avgSuccessRate: number;
    avgCostUsd: number;
    avgLatencyMs: number;
  };

  // Task complexity indicators
  complexity?: {
    inputSize: 'small' | 'medium' | 'large';
    taskType: 'simple' | 'complex' | 'experimental';
    requiresMultiStep: boolean;
  };

  // User preferences
  preferences?: {
    prioritizeCost: boolean;
    prioritizeSpeed: boolean;
    prioritizeAccuracy: boolean;
  };
}

/**
 * Strategy scoring criteria
 */
interface StrategyScore {
  strategy: Strategy;
  score: number;
  breakdown: {
    costScore: number;
    performanceScore: number;
    compatibilityScore: number;
    historyScore: number;
  };
  estimatedCost: number;
  estimatedLatencyMs: number;
}

/**
 * Intelligent Strategy Selector
 */
export class StrategySelector {
  private logger?: Logger;
  private costHistory: Map<string, Array<{ cost: number; timestamp: number }>> = new Map();
  private performanceHistory: Map<
    string,
    Array<{ latencyMs: number; success: boolean; timestamp: number }>
  > = new Map();

  constructor(logger?: Logger) {
    if (logger) {
      this.logger = logger.child({ component: 'StrategySelector' });
    }
  }

  /**
   * Select optimal strategy based on context
   */
  selectOptimalStrategy(
    context: StrategySelectionContext,
    availableStrategies: Strategy[],
  ): Result<Strategy> {
    if (availableStrategies.length === 0) {
      return Failure('No strategies available for selection');
    }

    if (availableStrategies.length === 1) {
      const firstStrategy = availableStrategies[0];
      if (!firstStrategy) {
        return Failure('Strategy array is corrupted');
      }
      return Success(firstStrategy);
    }

    try {
      // Score all available strategies
      const scoredStrategies = availableStrategies.map((strategy) =>
        this.scoreStrategy(strategy, context),
      );

      // Sort by score (descending)
      scoredStrategies.sort((a, b) => b.score - a.score);

      const selected = scoredStrategies[0];
      if (!selected) {
        return Failure('No valid strategy found after scoring');
      }

      this.logger?.info(
        {
          tool: context.tool,
          selectedStrategy: selected.strategy.id,
          score: selected.score,
          estimatedCost: selected.estimatedCost,
          estimatedLatencyMs: selected.estimatedLatencyMs,
          breakdown: selected.breakdown,
        },
        'Strategy selected',
      );

      return Success(selected.strategy);
    } catch (error) {
      return Failure(`Failed to select strategy: ${error}`);
    }
  }

  /**
   * Score a strategy based on context
   */
  private scoreStrategy(strategy: Strategy, context: StrategySelectionContext): StrategyScore {
    let score = 0;
    const breakdown = {
      costScore: 0,
      performanceScore: 0,
      compatibilityScore: 0,
      historyScore: 0,
    };

    // Estimate cost and latency
    const estimatedCost = this.estimateCost(strategy, context);
    const estimatedLatencyMs = this.estimateLatency(strategy, context);

    // Cost scoring (30% weight)
    breakdown.costScore = this.scoreCost(strategy, context, estimatedCost);

    // Performance scoring (25% weight)
    breakdown.performanceScore = this.scorePerformance(strategy, context, estimatedLatencyMs);

    // Compatibility scoring (25% weight)
    breakdown.compatibilityScore = this.scoreCompatibility(strategy, context);

    // Historical performance scoring (20% weight)
    breakdown.historyScore = this.scoreHistory(strategy, context);

    // Calculate weighted score
    score =
      breakdown.costScore * 0.3 +
      breakdown.performanceScore * 0.25 +
      breakdown.compatibilityScore * 0.25 +
      breakdown.historyScore * 0.2;

    // Apply user preference adjustments
    if (context.preferences) {
      if (context.preferences.prioritizeCost) {
        score += breakdown.costScore * 0.1;
      }
      if (context.preferences.prioritizeSpeed) {
        score += breakdown.performanceScore * 0.1;
      }
      if (context.preferences.prioritizeAccuracy) {
        score += breakdown.historyScore * 0.1;
      }
    }

    return {
      strategy,
      score: Math.max(0, Math.min(100, score)),
      breakdown,
      estimatedCost,
      estimatedLatencyMs,
    };
  }

  /**
   * Score strategy based on cost considerations
   */
  private scoreCost(
    strategy: Strategy,
    context: StrategySelectionContext,
    estimatedCost: number,
  ): number {
    let score = 50; // Base score

    // Budget constraint scoring
    if (context.budget?.maxCost) {
      if (estimatedCost <= context.budget.maxCost * 0.5) {
        score += 30; // Well under budget
      } else if (estimatedCost <= context.budget.maxCost * 0.8) {
        score += 15; // Reasonable cost
      } else if (estimatedCost <= context.budget.maxCost) {
        score += 5; // At budget limit
      } else {
        score -= 50; // Over budget - heavily penalize
      }
    }

    // Strategy cost limits
    if (strategy.cost?.maxUsd) {
      if (estimatedCost <= strategy.cost.maxUsd * 0.5) {
        score += 10;
      } else if (estimatedCost > strategy.cost.maxUsd) {
        score -= 20;
      }
    }

    return Math.max(0, Math.min(100, score));
  }

  /**
   * Score strategy based on performance requirements
   */
  private scorePerformance(
    strategy: Strategy,
    context: StrategySelectionContext,
    estimatedLatencyMs: number,
  ): number {
    let score = 50; // Base score

    // Latency requirements
    if (context.performance?.maxLatencyMs) {
      if (estimatedLatencyMs <= context.performance.maxLatencyMs * 0.5) {
        score += 30; // Much faster than required
      } else if (estimatedLatencyMs <= context.performance.maxLatencyMs * 0.8) {
        score += 15; // Reasonably fast
      } else if (estimatedLatencyMs <= context.performance.maxLatencyMs) {
        score += 5; // At limit
      } else {
        score -= 40; // Too slow
      }
    }

    // Strategy timeout constraints
    if (strategy.timeouts?.totalMs) {
      if (estimatedLatencyMs <= strategy.timeouts.totalMs * 0.5) {
        score += 10;
      } else if (estimatedLatencyMs > strategy.timeouts.totalMs) {
        score -= 20;
      }
    }

    // Performance preferences
    if (context.performance?.preferSpeed && strategy.parameters?.temperature) {
      // Lower temperature generally means faster but less creative responses
      score += (1 - strategy.parameters.temperature) * 10;
    }

    if (context.performance?.preferAccuracy && strategy.parameters?.temperature) {
      // Moderate temperature often provides good accuracy
      const optimalTemp = 0.3;
      const tempDiff = Math.abs(strategy.parameters.temperature - optimalTemp);
      score += Math.max(0, 10 - tempDiff * 20);
    }

    return Math.max(0, Math.min(100, score));
  }

  /**
   * Score strategy based on tool compatibility
   */
  private scoreCompatibility(strategy: Strategy, context: StrategySelectionContext): number {
    let score = 50; // Base score

    // Check if strategy has specific tool configurations
    if (strategy.selectionRules?.toolChain?.includes(context.tool)) {
      score += 20;
    }

    // Task complexity matching
    if (context.complexity) {
      const maxTokens = strategy.parameters?.maxTokens || 4096;

      switch (context.complexity.inputSize) {
        case 'small':
          if (maxTokens >= 2048) score += 10;
          break;
        case 'medium':
          if (maxTokens >= 4096) score += 10;
          break;
        case 'large':
          if (maxTokens >= 8192) score += 10;
          break;
      }

      switch (context.complexity.taskType) {
        case 'simple':
          // Prefer lower temperature for simple tasks
          if ((strategy.parameters?.temperature || 0.3) <= 0.4) score += 15;
          break;
        case 'complex':
          // Prefer moderate temperature for complex tasks
          if (
            (strategy.parameters?.temperature || 0.3) >= 0.2 &&
            (strategy.parameters?.temperature || 0.3) <= 0.7
          )
            score += 15;
          break;
        case 'experimental':
          // Prefer higher temperature for experimental tasks
          if ((strategy.parameters?.temperature || 0.3) >= 0.5) score += 15;
          break;
      }
    }

    return Math.max(0, Math.min(100, score));
  }

  /**
   * Score strategy based on historical performance
   */
  private scoreHistory(strategy: Strategy, context: StrategySelectionContext): number {
    let score = 50; // Base score with no history

    const historyKey = `${strategy.id}:${context.tool}`;
    const performanceHist = this.performanceHistory.get(historyKey) || [];

    if (performanceHist.length === 0) {
      return score; // No history available
    }

    // Calculate recent success rate (last 10 executions)
    const recent = performanceHist.slice(-10);
    const successRate = recent.filter((p) => p.success).length / recent.length;
    score += (successRate - 0.5) * 40; // -20 to +20 based on success rate

    // Consider average latency performance
    const avgLatency = recent.reduce((sum, p) => sum + p.latencyMs, 0) / recent.length;
    if (context.performance?.maxLatencyMs) {
      const latencyRatio = avgLatency / context.performance.maxLatencyMs;
      if (latencyRatio <= 0.5) {
        score += 10; // Consistently fast
      } else if (latencyRatio > 1.2) {
        score -= 15; // Consistently slow
      }
    }

    // Use context history if available
    if (context.history) {
      const contextSuccessRate = context.history.avgSuccessRate;
      score += (contextSuccessRate - 0.5) * 20;
    }

    return Math.max(0, Math.min(100, score));
  }

  /**
   * Estimate cost for strategy execution
   */
  private estimateCost(strategy: Strategy, context: StrategySelectionContext): number {
    // Base cost estimation based on token usage
    const maxTokens = strategy.parameters?.maxTokens || 4096;
    const baseTokenCost = 0.00002; // Rough estimate per token
    let estimatedCost = maxTokens * baseTokenCost;

    // Adjust based on complexity
    if (context.complexity) {
      switch (context.complexity.taskType) {
        case 'simple':
          estimatedCost *= 0.7;
          break;
        case 'complex':
          estimatedCost *= 1.3;
          break;
        case 'experimental':
          estimatedCost *= 1.8;
          break;
      }

      if (context.complexity.requiresMultiStep) {
        estimatedCost *= 1.5;
      }
    }

    // Consider historical cost data
    const historyKey = `${strategy.id}:${context.tool}`;
    const costHist = this.costHistory.get(historyKey) || [];
    if (costHist.length > 0) {
      const avgHistoricalCost = costHist.reduce((sum, c) => sum + c.cost, 0) / costHist.length;
      estimatedCost = (estimatedCost + avgHistoricalCost) / 2; // Blend estimate with history
    }

    return estimatedCost;
  }

  /**
   * Estimate latency for strategy execution
   */
  private estimateLatency(strategy: Strategy, context: StrategySelectionContext): number {
    // Base latency estimation
    const maxTokens = strategy.parameters?.maxTokens || 4096;
    const baseLatency = Math.max(5000, maxTokens * 2); // Rough estimate: 2ms per token, min 5s

    let estimatedLatency = baseLatency;

    // Adjust based on complexity
    if (context.complexity) {
      switch (context.complexity.inputSize) {
        case 'small':
          estimatedLatency *= 0.8;
          break;
        case 'medium':
          estimatedLatency *= 1.0;
          break;
        case 'large':
          estimatedLatency *= 1.4;
          break;
      }

      if (context.complexity.requiresMultiStep) {
        estimatedLatency *= 1.8;
      }
    }

    // Consider historical performance data
    const historyKey = `${strategy.id}:${context.tool}`;
    const perfHist = this.performanceHistory.get(historyKey) || [];
    if (perfHist.length > 0) {
      const avgHistoricalLatency =
        perfHist.reduce((sum, p) => sum + p.latencyMs, 0) / perfHist.length;
      estimatedLatency = (estimatedLatency + avgHistoricalLatency) / 2; // Blend estimate with history
    }

    return estimatedLatency;
  }

  /**
   * Record actual execution results for future optimization
   */
  recordExecution(
    strategy: Strategy,
    context: StrategySelectionContext,
    result: {
      success: boolean;
      actualCost: number;
      actualLatencyMs: number;
    },
  ): void {
    const historyKey = `${strategy.id}:${context.tool}`;
    const timestamp = Date.now();

    // Record cost
    if (!this.costHistory.has(historyKey)) {
      this.costHistory.set(historyKey, []);
    }
    const costHist = this.costHistory.get(historyKey);
    if (!costHist) {
      throw new Error(`Failed to initialize cost history for ${historyKey}`);
    }
    costHist.push({ cost: result.actualCost, timestamp });

    // Keep only last 50 entries
    if (costHist.length > 50) {
      costHist.splice(0, costHist.length - 50);
    }

    // Record performance
    if (!this.performanceHistory.has(historyKey)) {
      this.performanceHistory.set(historyKey, []);
    }
    const perfHist = this.performanceHistory.get(historyKey);
    if (!perfHist) {
      throw new Error(`Failed to initialize performance history for ${historyKey}`);
    }
    perfHist.push({
      latencyMs: result.actualLatencyMs,
      success: result.success,
      timestamp,
    });

    // Keep only last 50 entries
    if (perfHist.length > 50) {
      perfHist.splice(0, perfHist.length - 50);
    }

    this.logger?.debug(
      {
        strategy: strategy.id,
        tool: context.tool,
        result,
      },
      'Recorded execution result for strategy optimization',
    );
  }

  /**
   * Get strategy recommendations for a tool
   */
  getRecommendations(
    context: StrategySelectionContext,
    availableStrategies: Strategy[],
  ): Array<{
    strategy: Strategy;
    reason: string;
    confidence: number;
  }> {
    const scoredStrategies = availableStrategies.map((strategy) =>
      this.scoreStrategy(strategy, context),
    );

    return scoredStrategies
      .sort((a, b) => b.score - a.score)
      .slice(0, 3) // Top 3 recommendations
      .map((scored) => ({
        strategy: scored.strategy,
        reason: this.generateRecommendationReason(scored),
        confidence: scored.score,
      }));
  }

  /**
   * Generate human-readable reason for strategy recommendation
   */
  private generateRecommendationReason(scored: StrategyScore): string {
    const { breakdown } = scored;
    const reasons: string[] = [];

    if (breakdown.costScore > 70) {
      reasons.push('excellent cost efficiency');
    } else if (breakdown.costScore > 50) {
      reasons.push('good cost balance');
    }

    if (breakdown.performanceScore > 70) {
      reasons.push('fast execution');
    } else if (breakdown.performanceScore > 50) {
      reasons.push('reasonable performance');
    }

    if (breakdown.historyScore > 70) {
      reasons.push('proven track record');
    } else if (breakdown.historyScore > 50) {
      reasons.push('reliable performance');
    }

    if (breakdown.compatibilityScore > 70) {
      reasons.push('excellent tool compatibility');
    }

    if (reasons.length === 0) {
      return 'default recommendation based on available options';
    }

    return `Recommended for ${reasons.slice(0, 2).join(' and ')}`;
  }
}

/**
 * Global strategy selector instance
 */
let selectorInstance: StrategySelector | null = null;

/**
 * Get or create strategy selector instance
 */
export function getStrategySelector(logger?: Logger): StrategySelector {
  if (!selectorInstance) {
    selectorInstance = new StrategySelector(logger);
  }
  return selectorInstance;
}

/**
 * Convenience function for optimal strategy selection
 */
export function selectOptimalStrategy(
  context: StrategySelectionContext,
  available: Strategy[],
  logger?: Logger,
): Result<Strategy> {
  return getStrategySelector(logger).selectOptimalStrategy(context, available);
}
