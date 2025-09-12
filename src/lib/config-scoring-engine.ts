/**
 * Evaluates content using configurable scoring rules with category weighting.
 * Trade-off: Runtime rule evaluation over compile-time for flexibility in scoring strategies.
 */

import type { ScoringProfile, ScoringRule, ScoringMatcher, PenaltyRule } from './sampling-config';
import { SCORING_FUNCTIONS, type ScoringFunctionName } from './scoring-functions';
import { Result, Success, Failure } from '../types';
import { extractErrorMessage } from './error-utils';
import { createLogger } from './logger';

export interface ConfigScoreResult {
  total: number;
  breakdown: Record<string, number>;
  categoryScores: Record<string, number>;
  matchedRules: string[];
  appliedPenalties: string[];
}

export class ConfigScoringEngine {
  private readonly logger = createLogger({ name: 'ConfigScoringEngine' });

  constructor(private debug: boolean = false) {}

  /**
   * Score content using a configuration profile
   */
  score(content: string, profile: ScoringProfile): Result<ConfigScoreResult> {
    try {
      let totalScore = profile.base_score;
      const breakdown: Record<string, number> = {};
      const categoryScores: Record<string, number> = {};
      const matchedRules: string[] = [];
      const appliedPenalties: string[] = [];

      // Invariant: All categories must be initialized to zero before rule evaluation
      for (const category of Object.keys(profile.category_weights)) {
        categoryScores[category] = 0;
      }

      // Process rules by category
      for (const [categoryName, rules] of Object.entries(profile.rules)) {
        let categoryScore = 0;

        for (const rule of rules) {
          const ruleResult = this.evaluateRule(content, rule);

          if (ruleResult.ok && ruleResult.value) {
            const weightedScore = rule.points * rule.weight;
            const categoryWeight = profile.category_weights[rule.category] || 1.0;
            const finalScore = weightedScore * categoryWeight;

            categoryScore += finalScore;
            breakdown[rule.name] = finalScore;
            matchedRules.push(rule.name);

            if (this.debug) {
              this.logger.info(
                `Rule ${rule.name}: ${rule.points} * ${rule.weight} * ${categoryWeight} = ${finalScore}`,
              );
            }
          }
        }

        categoryScores[categoryName] = categoryScore;
        totalScore += categoryScore;
      }

      // Penalties applied after positive scores to allow negative adjustments
      if (profile.penalties) {
        for (const penalty of profile.penalties) {
          const penaltyResult = this.evaluatePenaltyRule(content, penalty);

          if (penaltyResult.ok && penaltyResult.value) {
            totalScore += penalty.points; // Precondition: penalty.points must be negative
            breakdown[penalty.name] = penalty.points;
            appliedPenalties.push(penalty.name);

            if (this.debug) {
              this.logger.info(`Penalty ${penalty.name}: ${penalty.points}`);
            }
          }
        }
      }

      // Postcondition: Score must be within [0, max_score] bounds
      totalScore = Math.max(0, Math.min(totalScore, profile.max_score));

      return Success({
        total: Math.round(totalScore),
        breakdown,
        categoryScores,
        matchedRules,
        appliedPenalties,
      });
    } catch (error) {
      return Failure(`Scoring failed: ${extractErrorMessage(error)}`);
    }
  }

  private evaluateRule(content: string, rule: ScoringRule): Result<boolean> {
    try {
      return this.evaluateMatcher(content, rule.matcher);
    } catch (error) {
      if (this.debug) {
        this.logger.warn({ rule: rule.name, error }, 'Rule evaluation failed');
      }
      return Success(false);
    }
  }

  private evaluatePenaltyRule(content: string, penalty: PenaltyRule): Result<boolean> {
    try {
      return this.evaluateMatcher(content, penalty.matcher);
    } catch (error) {
      if (this.debug) {
        this.logger.warn({ penalty: penalty.name, error }, 'Penalty evaluation failed');
      }
      return Success(false);
    }
  }

  private evaluateMatcher(content: string, matcher: ScoringMatcher): Result<boolean> {
    try {
      if (matcher.type === 'regex') {
        return this.evaluateRegexMatcher(content, matcher);
      } else if (matcher.type === 'function') {
        return this.evaluateFunctionMatcher(content, matcher);
      }

      return Failure(`Unknown matcher type: ${(matcher as any).type}`);
    } catch (error) {
      return Failure(`Matcher evaluation failed: ${extractErrorMessage(error)}`);
    }
  }

  private evaluateRegexMatcher(content: string, matcher: any): Result<boolean> {
    try {
      const flags = matcher.flags || 'gm';
      const regex = new RegExp(matcher.pattern, flags);
      const matches = content.match(regex);
      const matchCount = matches ? matches.length : 0;

      // Count threshold evaluation for quantitative pattern matching
      if (matcher.count_threshold !== undefined && matcher.comparison) {
        return Success(this.compareValues(matchCount, matcher.count_threshold, matcher.comparison));
      }

      // Fallback: Binary presence check when no threshold specified
      return Success(matchCount > 0);
    } catch (error) {
      return Failure(`Regex evaluation failed: ${extractErrorMessage(error)}`);
    }
  }

  private evaluateFunctionMatcher(content: string, matcher: any): Result<boolean> {
    try {
      const functionName = matcher.function as ScoringFunctionName;
      const scoringFunction = SCORING_FUNCTIONS[functionName];

      if (!scoringFunction) {
        return Failure(`Unknown scoring function: ${functionName}`);
      }

      // Dynamic function invocation with optional threshold parameter
      let result: any;
      try {
        if (matcher.threshold !== undefined) {
          result = (scoringFunction as any)(content, matcher.threshold);
        } else {
          result = (scoringFunction as any)(content);
        }
      } catch (error) {
        if (this.debug) {
          this.logger.error({ functionName, error }, `Error evaluating function "${functionName}"`);
        }
        return Success(false);
      }

      // Numeric result comparison for threshold-based functions
      if (typeof result === 'number' && matcher.threshold !== undefined && matcher.comparison) {
        return Success(this.compareValues(result, matcher.threshold, matcher.comparison));
      }

      // Direct boolean coercion for non-threshold functions
      return Success(Boolean(result));
    } catch (error) {
      return Failure(`Function evaluation failed: ${extractErrorMessage(error)}`);
    }
  }

  private compareValues(actual: number, expected: number, comparison: string): boolean {
    switch (comparison) {
      case 'greater_than':
        return actual > expected;
      case 'less_than':
        return actual < expected;
      case 'equal':
        return actual === expected;
      case 'greater_than_or_equal':
        return actual >= expected;
      case 'less_than_or_equal':
        return actual <= expected;
      default:
        return false;
    }
  }
}

export function createConfigScoringEngine(debug: boolean = false): ConfigScoringEngine {
  return new ConfigScoringEngine(debug);
}
