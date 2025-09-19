/**
 * A/B Testing Framework for Prompts
 * Enables controlled experiments to compare prompt effectiveness
 */

import { z } from 'zod';
import type { Logger } from 'pino';
import { Result, Success, Failure } from '@types';

/**
 * A/B Test Configuration Schema
 */
export const ABTestSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string(),
  active: z.boolean(),
  startDate: z.string().datetime(),
  endDate: z.string().datetime().optional(),

  // Test configuration
  variants: z
    .array(
      z.object({
        id: z.string(),
        name: z.string(),
        promptId: z.string(),
        promptVersion: z.string().optional(),
        weight: z.number().min(0).max(1), // Traffic percentage (0.0 to 1.0)
      }),
    )
    .min(2),

  // Metrics to track
  metrics: z.array(z.string()),

  // Target audience
  audience: z
    .object({
      tools: z.array(z.string()).optional(), // Specific tools
      userSegments: z.array(z.string()).optional(), // User segments
      percentage: z.number().min(0).max(1).default(1), // % of eligible traffic
    })
    .optional(),

  // Success criteria
  successCriteria: z.object({
    primaryMetric: z.string(),
    minimumSampleSize: z.number().positive().default(100),
    minimumEffectSize: z.number().positive().default(0.05), // 5% improvement
    confidenceLevel: z.number().min(0).max(1).default(0.95), // 95% confidence
  }),
});

export type ABTest = z.infer<typeof ABTestSchema>;

/**
 * Test execution result
 */
export interface TestExecution {
  testId: string;
  variantId: string;
  userId?: string;
  tool: string;
  timestamp: number;
  metrics: Record<string, number | boolean>;
  success: boolean;
  latencyMs: number;
  cost?: number;
}

/**
 * Test results analysis
 */
export interface TestResults {
  testId: string;
  startDate: string;
  endDate?: string;
  totalExecutions: number;

  variants: Array<{
    id: string;
    name: string;
    executions: number;
    metrics: Record<
      string,
      {
        mean: number;
        stdDev: number;
        count: number;
      }
    >;
    successRate: number;
    avgLatencyMs: number;
    avgCost: number;
  }>;

  // Statistical analysis
  analysis: {
    hasSignificantResults: boolean;
    confidence: number;
    pValue: number;
    effectSize: number;
    recommendedAction: 'continue' | 'stop_promote_winner' | 'stop_inconclusive' | 'extend_test';
    winner?: string; // Variant ID
  };
}

/**
 * A/B Testing Manager
 */
export class ABTestManager {
  private tests: Map<string, ABTest> = new Map();
  private executions: Map<string, TestExecution[]> = new Map();
  private logger?: Logger;

  constructor(logger?: Logger) {
    if (logger) {
      this.logger = logger.child({ component: 'ABTestManager' });
    }
  }

  /**
   * Create a new A/B test
   */
  createTest(testConfig: ABTest): Result<void> {
    try {
      // Validate test configuration
      const validation = ABTestSchema.safeParse(testConfig);
      if (!validation.success) {
        return Failure(`Invalid test configuration: ${validation.error.message}`);
      }

      const test = validation.data;

      // Validate variants weights sum to 1
      const totalWeight = test.variants.reduce((sum, v) => sum + v.weight, 0);
      if (Math.abs(totalWeight - 1.0) > 0.001) {
        return Failure(`Variant weights must sum to 1.0, got ${totalWeight}`);
      }

      // Check for duplicate variant IDs
      const variantIds = test.variants.map((v) => v.id);
      if (new Set(variantIds).size !== variantIds.length) {
        return Failure('Variant IDs must be unique');
      }

      this.tests.set(test.id, test);
      this.executions.set(test.id, []);

      this.logger?.info({ testId: test.id, variants: test.variants.length }, 'A/B test created');

      return Success(undefined);
    } catch (error) {
      return Failure(`Failed to create test: ${error}`);
    }
  }

  /**
   * Get variant for execution (traffic splitting)
   */
  getVariant(
    testId: string,
    context: {
      tool: string;
      userId?: string;
      sessionId?: string;
    },
  ): Result<{ variantId: string; promptId: string; promptVersion?: string }> {
    const test = this.tests.get(testId);
    if (!test) {
      return Failure(`Test not found: ${testId}`);
    }

    if (!test.active) {
      return Failure(`Test is not active: ${testId}`);
    }

    // Check if test has ended
    if (test.endDate && new Date() > new Date(test.endDate)) {
      return Failure(`Test has ended: ${testId}`);
    }

    // Check audience targeting
    if (test.audience) {
      if (test.audience.tools && !test.audience.tools.includes(context.tool)) {
        return Failure(`Tool ${context.tool} not in test audience`);
      }

      // Random sampling for audience percentage
      if (test.audience.percentage < 1.0) {
        const hash = this.hashString(context.userId || context.sessionId || 'anonymous');
        const sample = (hash % 100) / 100;
        if (sample >= test.audience.percentage) {
          return Failure('User not selected for test audience');
        }
      }
    }

    // Select variant based on weights and consistent hashing
    const hash = this.hashString(`${testId}:${context.userId || context.sessionId || 'anonymous'}`);
    const sample = (hash % 1000) / 1000; // 0.0 to 0.999

    let cumulative = 0;
    for (const variant of test.variants) {
      cumulative += variant.weight;
      if (sample < cumulative) {
        this.logger?.debug(
          {
            testId,
            variantId: variant.id,
            tool: context.tool,
            sample,
          },
          'Variant selected',
        );

        return Success({
          variantId: variant.id,
          promptId: variant.promptId,
          promptVersion: variant.promptVersion,
        });
      }
    }

    // Fallback to last variant (should not happen if weights sum to 1)
    const lastVariant = test.variants[test.variants.length - 1];
    if (!lastVariant) {
      return Failure('No variants available in test');
    }
    return Success({
      variantId: lastVariant.id,
      promptId: lastVariant.promptId,
      promptVersion: lastVariant.promptVersion,
    });
  }

  /**
   * Record test execution result
   */
  recordExecution(execution: TestExecution): Result<void> {
    const test = this.tests.get(execution.testId);
    if (!test) {
      return Failure(`Test not found: ${execution.testId}`);
    }

    // Validate variant exists
    const variant = test.variants.find((v) => v.id === execution.variantId);
    if (!variant) {
      return Failure(`Variant not found: ${execution.variantId}`);
    }

    // Store execution
    const executions = this.executions.get(execution.testId) || [];
    executions.push(execution);
    this.executions.set(execution.testId, executions);

    this.logger?.debug(
      {
        testId: execution.testId,
        variantId: execution.variantId,
        success: execution.success,
        metrics: execution.metrics,
      },
      'Test execution recorded',
    );

    return Success(undefined);
  }

  /**
   * Get test results and analysis
   */
  getResults(testId: string): Result<TestResults> {
    const test = this.tests.get(testId);
    if (!test) {
      return Failure(`Test not found: ${testId}`);
    }

    const executions = this.executions.get(testId) || [];

    try {
      // Calculate variant statistics
      const variantStats = test.variants.map((variant) => {
        const variantExecutions = executions.filter((e) => e.variantId === variant.id);

        const metrics: Record<string, { mean: number; stdDev: number; count: number }> = {};
        for (const metricName of test.metrics) {
          const values = variantExecutions
            .map((e) => e.metrics[metricName])
            .filter((v) => typeof v === 'number') as number[];

          if (values.length > 0) {
            const mean = values.reduce((sum, v) => sum + v, 0) / values.length;
            const variance =
              values.reduce((sum, v) => sum + Math.pow(v - mean, 2), 0) / values.length;
            metrics[metricName] = {
              mean,
              stdDev: Math.sqrt(variance),
              count: values.length,
            };
          }
        }

        const successCount = variantExecutions.filter((e) => e.success).length;
        const totalLatency = variantExecutions.reduce((sum, e) => sum + e.latencyMs, 0);
        const totalCost = variantExecutions.reduce((sum, e) => sum + (e.cost || 0), 0);

        return {
          id: variant.id,
          name: variant.name,
          executions: variantExecutions.length,
          metrics,
          successRate: variantExecutions.length > 0 ? successCount / variantExecutions.length : 0,
          avgLatencyMs: variantExecutions.length > 0 ? totalLatency / variantExecutions.length : 0,
          avgCost: variantExecutions.length > 0 ? totalCost / variantExecutions.length : 0,
        };
      });

      // Statistical analysis
      const analysis = this.performStatisticalAnalysis(test, variantStats);

      const results: TestResults = {
        testId,
        startDate: test.startDate,
        endDate: test.endDate,
        totalExecutions: executions.length,
        variants: variantStats,
        analysis,
      };

      return Success(results);
    } catch (error) {
      return Failure(`Failed to analyze results: ${error}`);
    }
  }

  /**
   * Perform statistical analysis on test results
   */
  private performStatisticalAnalysis(
    test: ABTest,
    variantStats: TestResults['variants'],
  ): TestResults['analysis'] {
    const primaryMetric = test.successCriteria.primaryMetric;
    const minimumSampleSize = test.successCriteria.minimumSampleSize;
    const minimumEffectSize = test.successCriteria.minimumEffectSize;
    const confidenceLevel = test.successCriteria.confidenceLevel;

    // Check if we have enough data
    const hasMinimumSample = variantStats.every((v) => v.executions >= minimumSampleSize);

    if (!hasMinimumSample) {
      return {
        hasSignificantResults: false,
        confidence: 0,
        pValue: 1,
        effectSize: 0,
        recommendedAction: 'continue',
      };
    }

    // For simplicity, compare first two variants for primary metric
    if (variantStats.length < 2) {
      return {
        hasSignificantResults: false,
        confidence: 0,
        pValue: 1,
        effectSize: 0,
        recommendedAction: 'continue',
      };
    }

    const variant1 = variantStats[0];
    const variant2 = variantStats[1];

    if (!variant1 || !variant2) {
      return {
        hasSignificantResults: false,
        confidence: 0,
        pValue: 1,
        effectSize: 0,
        recommendedAction: 'extend_test' as const,
      };
    }

    // Get primary metric values
    const metric1 = variant1.metrics[primaryMetric];
    const metric2 = variant2.metrics[primaryMetric];

    if (!metric1 || !metric2) {
      return {
        hasSignificantResults: false,
        confidence: 0,
        pValue: 1,
        effectSize: 0,
        recommendedAction: 'continue',
      };
    }

    // Calculate effect size (Cohen's d)
    const pooledStdDev = Math.sqrt(
      ((metric1.count - 1) * Math.pow(metric1.stdDev, 2) +
        (metric2.count - 1) * Math.pow(metric2.stdDev, 2)) /
        (metric1.count + metric2.count - 2),
    );

    const effectSize = Math.abs(metric1.mean - metric2.mean) / pooledStdDev;

    // T-test calculation
    const standardError = pooledStdDev * Math.sqrt(1 / metric1.count + 1 / metric2.count);
    const tStatistic = Math.abs(metric1.mean - metric2.mean) / standardError;
    const degreesOfFreedom = metric1.count + metric2.count - 2;

    // P-value estimation (approximation)
    const pValue = this.estimatePValue(tStatistic, degreesOfFreedom);
    const confidence = 1 - pValue;

    const hasSignificantResults = pValue < 1 - confidenceLevel && effectSize >= minimumEffectSize;

    let recommendedAction: TestResults['analysis']['recommendedAction'] = 'continue';
    let winner: string | undefined;

    if (hasSignificantResults) {
      winner = metric1.mean > metric2.mean ? variant1.id : variant2.id;
      recommendedAction = 'stop_promote_winner';
    } else if (effectSize < minimumEffectSize / 2) {
      recommendedAction = 'stop_inconclusive';
    } else {
      recommendedAction = 'extend_test';
    }

    return {
      hasSignificantResults,
      confidence,
      pValue,
      effectSize,
      recommendedAction,
      winner,
    };
  }

  /**
   * Simple hash function for consistent user assignment
   */
  private hashString(str: string): number {
    let hash = 0;
    for (let i = 0; i < str.length; i++) {
      const char = str.charCodeAt(i);
      hash = (hash << 5) - hash + char;
      hash = hash & hash; // Convert to 32bit integer
    }
    return Math.abs(hash);
  }

  /**
   * Simplified p-value estimation
   */
  private estimatePValue(tStatistic: number, _degreesOfFreedom: number): number {
    // Approximation - would use proper statistical library in production
    if (tStatistic > 2.576) return 0.01; // 99% confidence
    if (tStatistic > 1.96) return 0.05; // 95% confidence
    if (tStatistic > 1.645) return 0.1; // 90% confidence
    return 0.5; // Not significant
  }

  /**
   * List all active tests
   */
  getActiveTests(): ABTest[] {
    const now = new Date();
    return Array.from(this.tests.values()).filter(
      (test) =>
        test.active &&
        new Date(test.startDate) <= now &&
        (!test.endDate || new Date(test.endDate) > now),
    );
  }

  /**
   * Stop a test
   */
  stopTest(testId: string): Result<void> {
    const test = this.tests.get(testId);
    if (!test) {
      return Failure(`Test not found: ${testId}`);
    }

    test.active = false;
    test.endDate = new Date().toISOString();

    this.logger?.info({ testId }, 'A/B test stopped');
    return Success(undefined);
  }

  /**
   * Get test by ID
   */
  getTest(testId: string): ABTest | null {
    return this.tests.get(testId) || null;
  }

  /**
   * Update test configuration
   */
  updateTest(testId: string, updates: Partial<ABTest>): Result<void> {
    const test = this.tests.get(testId);
    if (!test) {
      return Failure(`Test not found: ${testId}`);
    }

    // Don't allow certain updates while test is active
    if (test.active && (updates.variants || updates.audience)) {
      return Failure('Cannot modify variants or audience while test is active');
    }

    Object.assign(test, updates);

    this.logger?.info({ testId, updates }, 'A/B test updated');
    return Success(undefined);
  }
}

/**
 * Global A/B test manager instance
 */
let testManagerInstance: ABTestManager | null = null;

/**
 * Get or create A/B test manager instance
 */
export function getABTestManager(logger?: Logger): ABTestManager {
  if (!testManagerInstance) {
    testManagerInstance = new ABTestManager(logger);
  }
  return testManagerInstance;
}

/**
 * Enhanced prompt API integration with A/B testing
 */
export interface ABTestingOptions {
  testId?: string;
  userId?: string;
  sessionId?: string;
  recordMetrics?: boolean;
}

/**
 * A/B testing result with prompt content
 */
export interface ABTestPromptResult {
  content: string;
  metadata: {
    testId?: string;
    variantId?: string;
    promptId: string;
    promptVersion?: string;
  };
}
