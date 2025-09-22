/**
 * Policy Evaluation Module
 * Handles rule evaluation and policy application
 */

import * as fs from 'node:fs';
import * as path from 'node:path';
import type { Policy, PolicyRule, Matcher, FunctionMatcher } from './policy-schemas';

// ============================================================================
// Function Evaluators
// ============================================================================

type FunctionEvaluator = (args: unknown[], input: string | Record<string, unknown>) => boolean;

const functionEvaluators: Record<FunctionMatcher['name'], FunctionEvaluator> = {
  hasPattern: (args, input) => {
    const [pattern, flags] = args as [string, string?];
    const regex = new RegExp(pattern, flags);
    const text = typeof input === 'string' ? input : JSON.stringify(input);
    return regex.test(text);
  },

  fileExists: (args, input) => {
    const [filePath] = args as [string];
    const basePath = typeof input === 'object' && input.path ? String(input.path) : '.';
    return fs.existsSync(path.join(basePath, filePath));
  },

  largerThan: (args, input) => {
    const [size] = args as [number];
    if (typeof input === 'string') {
      return input.length > size;
    }
    if (typeof input === 'object' && input.size !== undefined) {
      return Number(input.size) > size;
    }
    return false;
  },

  hasVulnerabilities: (args, input) => {
    const [severities] = args as [string[]];
    if (typeof input === 'object' && Array.isArray(input.vulnerabilities)) {
      return input.vulnerabilities.some((v) =>
        severities.includes(String(v.severity).toUpperCase()),
      );
    }
    return false;
  },
};

// ============================================================================
// Matcher Evaluation
// ============================================================================

/**
 * Evaluate a matcher against input data
 */
export function evaluateMatcher(
  matcher: Matcher,
  input: string | Record<string, unknown>,
): boolean {
  switch (matcher.kind) {
    case 'regex': {
      const regex = new RegExp(matcher.pattern, matcher.flags || 'g');
      const text = typeof input === 'string' ? input : JSON.stringify(input);

      if (matcher.count_threshold !== undefined) {
        const matches = text.match(regex);
        return (matches?.length || 0) >= matcher.count_threshold;
      }
      return regex.test(text);
    }

    case 'function': {
      const evaluator = functionEvaluators[matcher.name];
      return evaluator ? evaluator(matcher.args, input) : false;
    }

    default:
      return false;
  }
}

// ============================================================================
// Policy Application
// ============================================================================

/**
 * Apply policy rules to input and return matching actions
 */
export function applyPolicy(
  policy: Policy,
  input: string | Record<string, unknown>,
): Array<{ rule: PolicyRule; matched: boolean }> {
  const results: Array<{ rule: PolicyRule; matched: boolean }> = [];

  for (const rule of policy.rules) {
    // All conditions must match (AND logic)
    const matched =
      rule.conditions.length > 0 &&
      rule.conditions.every((condition) => evaluateMatcher(condition, input));

    results.push({ rule, matched });
  }

  return results;
}

/**
 * Get all rule weights for optimization algorithms
 */
export function getRuleWeights(policy: Policy): Map<string, number> {
  const weights = new Map<string, number>();

  for (const rule of policy.rules) {
    weights.set(rule.id, rule.priority);
  }

  return weights;
}

/**
 * Select best strategy based on policy rules
 */
export function selectStrategy(
  policy: Policy,
  candidates: Array<{ id: string; score: number }>,
): string | null {
  // Sort candidates by score
  const sorted = [...candidates].sort((a, b) => b.score - a.score);

  if (sorted.length === 0) {
    return null;
  }

  // Apply policy rules to filter/adjust candidates
  const weights = getRuleWeights(policy);

  for (const candidate of sorted) {
    const weight = weights.get(candidate.id) || 100;
    candidate.score *= weight / 100;
  }

  // Re-sort after weight adjustment
  sorted.sort((a, b) => b.score - a.score);

  return sorted[0]?.id || null;
}
