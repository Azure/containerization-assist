/**
 * Merged Policy Evaluator
 *
 * Combines results from multiple policy evaluators (Rego and CEL).
 * Implements unified RegoEvaluator interface for transparent multi-policy support.
 *
 * **Use Cases:**
 * - Evaluate built-in Rego policies + custom CEL policies together
 * - Combine multiple custom policies (Rego or CEL)
 * - Merge organizational and team-level policies
 *
 * @module config/policy-merged-evaluator
 */

import type { Logger } from 'pino';
import type { Result } from '@types';
import { Success } from '@types';
import type {
  RegoEvaluator,
  RegoPolicyResult,
  RegoPolicyViolation,
  CategorizedViolations,
} from './policy-rego';

/**
 * Evaluator that merges results from multiple policy evaluators
 *
 * Evaluates all policies in parallel and combines the results,
 * sorting by priority and de-duplicating by rule name.
 *
 * **Behavior:**
 * - Evaluates all policies concurrently for performance
 * - Merges violations, warnings, and suggestions
 * - Sorts results by priority (higher first)
 * - De-duplicates violations by rule name (keeps first occurrence)
 * - Handles policy evaluation failures gracefully
 *
 * **Configuration Queries:**
 * - Queries policies in order until one returns non-null config
 * - CEL policies always return null for queryConfig
 * - Rego policies may return configuration objects
 *
 * @example
 * ```typescript
 * const regoEvaluator = await loadRegoPolicy('built-in.rego', logger);
 * const celEvaluator = await loadCelPolicy('custom.yaml', logger);
 *
 * const merged = createMergedEvaluator([regoEvaluator.value, celEvaluator.value], logger);
 *
 * const result = await merged.evaluate('FROM alpine');
 * // result.violations contains violations from both policies
 * ```
 */
export class MergedPolicyEvaluator implements RegoEvaluator {
  constructor(
    private evaluators: RegoEvaluator[],
    private logger: Logger,
    public policyPaths: string[]
  ) {
    this.logger.info(
      {
        evaluatorCount: evaluators.length,
        policyPaths,
      },
      'Created merged policy evaluator'
    );
  }

  /**
   * Evaluate all policies and merge results
   *
   * Runs all evaluators in parallel and combines their results.
   * If any evaluator fails, it's logged as an error and reported as a blocking violation.
   *
   * @param input - String content or object to evaluate
   * @returns Promise resolving to merged policy result
   */
  async evaluate(input: string | Record<string, unknown>): Promise<RegoPolicyResult> {
    this.logger.debug(
      { evaluatorCount: this.evaluators.length },
      'Evaluating merged policies'
    );

    const allViolations: RegoPolicyViolation[] = [];
    const allWarnings: RegoPolicyViolation[] = [];
    const allSuggestions: RegoPolicyViolation[] = [];

    // Evaluate all policies in parallel for performance
    const results = await Promise.all(
      this.evaluators.map((evaluator) =>
        evaluator
          .evaluate(input)
          .catch((error) => {
            const message = error?.message || String(error);
            this.logger.error(
              {
                error: message,
                policyPaths: evaluator.policyPaths,
              },
              'Policy evaluation failed'
            );

            // Return error as blocking violation
            return {
              allow: false,
              violations: [
                {
                  rule: 'policy-evaluation-error',
                  category: 'system',
                  severity: 'block' as const,
                  message: `Policy evaluation failed: ${message}`,
                  description: `Policy: ${evaluator.policyPaths.join(', ')}`,
                },
              ],
              warnings: [],
              suggestions: [],
            } as RegoPolicyResult;
          })
      )
    );

    // Merge results from all evaluators
    for (const result of results) {
      allViolations.push(...result.violations);
      allWarnings.push(...result.warnings);
      allSuggestions.push(...result.suggestions);
    }

    // Sort by priority (higher priority first)
    const sortByPriority = (a: RegoPolicyViolation, b: RegoPolicyViolation) =>
      (b.priority ?? 50) - (a.priority ?? 50);

    allViolations.sort(sortByPriority);
    allWarnings.sort(sortByPriority);
    allSuggestions.sort(sortByPriority);

    // De-duplicate violations by rule name (keep first occurrence)
    const deduplicateByRule = (violations: RegoPolicyViolation[]) => {
      const seen = new Set<string>();
      return violations.filter((v) => {
        if (seen.has(v.rule)) {
          this.logger.debug({ rule: v.rule }, 'Skipping duplicate violation');
          return false;
        }
        seen.add(v.rule);
        return true;
      });
    };

    const uniqueViolations = deduplicateByRule(allViolations);
    const uniqueWarnings = deduplicateByRule(allWarnings);
    const uniqueSuggestions = deduplicateByRule(allSuggestions);

    const allow = uniqueViolations.length === 0;

    this.logger.info(
      {
        allow,
        violations: uniqueViolations.length,
        warnings: uniqueWarnings.length,
        suggestions: uniqueSuggestions.length,
        evaluatorCount: this.evaluators.length,
      },
      'Merged policy evaluation completed'
    );

    return {
      allow,
      violations: uniqueViolations,
      warnings: uniqueWarnings,
      suggestions: uniqueSuggestions,
      summary: {
        total_violations: uniqueViolations.length,
        total_warnings: uniqueWarnings.length,
        total_suggestions: uniqueSuggestions.length,
      },
    };
  }

  /**
   * Evaluate with Result wrapper
   *
   * Wraps evaluate() to work with Result pattern.
   *
   * @param result - Result containing content to evaluate
   * @param packageName - Package name (unused, for interface compatibility)
   * @returns Promise resolving to Result containing categorized violations
   */
  async evaluatePolicy<T>(
    result: Result<T>,
    packageName?: string
  ): Promise<Result<CategorizedViolations>> {
    if (!result.ok) {
      return result as Result<never>;
    }

    const policyResult = await this.evaluate(
      result.value as string | Record<string, unknown>
    );

    return Success({
      blocking: policyResult.violations,
      warnings: policyResult.warnings,
      suggestions: policyResult.suggestions,
    });
  }

  /**
   * Query first evaluator that returns non-null config
   *
   * Queries evaluators in order until one returns a configuration object.
   *
   * **Note:** CEL evaluators always return null. This will query
   * Rego evaluators in order until one returns a config.
   *
   * @param packageName - Package name to query
   * @param input - Input data for the query
   * @returns Promise resolving to configuration object or null
   *
   * @example
   * ```typescript
   * const config = await merged.queryConfig('containerization.generation_config', {});
   * if (config) {
   *   console.log('Base image:', config.baseImage);
   * }
   * ```
   */
  async queryConfig<T = unknown>(
    packageName: string,
    input: Record<string, unknown>
  ): Promise<T | null> {
    this.logger.debug(
      {
        packageName,
        evaluatorCount: this.evaluators.length,
      },
      'Querying merged policies for config'
    );

    // Try each evaluator in order
    for (const evaluator of this.evaluators) {
      try {
        const config = await evaluator.queryConfig<T>(packageName, input);
        if (config !== null) {
          this.logger.debug(
            {
              packageName,
              policyPaths: evaluator.policyPaths,
            },
            'Config found in policy'
          );
          return config;
        }
      } catch (error) {
        const message = error?.message || String(error);
        this.logger.warn(
          {
            error: message,
            policyPaths: evaluator.policyPaths,
          },
          'Config query failed, trying next policy'
        );
        // Continue to next evaluator
      }
    }

    this.logger.debug({ packageName }, 'No config found in any policy');
    return null;
  }

  /**
   * Clean up all evaluators
   *
   * Closes all underlying evaluators and releases their resources.
   */
  close(): void {
    this.logger.debug('Closing merged policy evaluator');

    for (const evaluator of this.evaluators) {
      try {
        evaluator.close();
      } catch (error) {
        const message = error?.message || String(error);
        this.logger.warn(
          {
            error: message,
            policyPaths: evaluator.policyPaths,
          },
          'Error closing evaluator'
        );
      }
    }
  }
}

/**
 * Create merged evaluator from multiple evaluators
 *
 * Factory function that combines multiple policy evaluators into a single
 * merged evaluator.
 *
 * @param evaluators - Array of policy evaluators to merge
 * @param logger - Logger instance
 * @returns Merged policy evaluator
 *
 * @example
 * ```typescript
 * const evaluators = [
 *   await loadRegoPolicy('built-in.rego', logger),
 *   await loadCelPolicy('custom.yaml', logger),
 * ];
 *
 * const merged = createMergedEvaluator(
 *   evaluators.filter(r => r.ok).map(r => r.value),
 *   logger
 * );
 * ```
 */
export function createMergedEvaluator(
  evaluators: RegoEvaluator[],
  logger: Logger
): RegoEvaluator {
  const allPaths = evaluators.flatMap((e) => e.policyPaths);
  return new MergedPolicyEvaluator(evaluators, logger, allPaths);
}
