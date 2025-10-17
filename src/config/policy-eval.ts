/**
 * Policy Evaluation Module
 * Handles rule evaluation and policy application
 */

import * as fs from 'node:fs';
import * as path from 'node:path';
import type { Policy, PolicyRule, Matcher, FunctionMatcher } from './policy-schemas';

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

/**
 * Evaluate a matcher against input data
 */
export function evaluateMatcher(
  matcher: Matcher,
  input: string | Record<string, unknown>,
): boolean {
  switch (matcher.kind) {
    case 'regex': {
      const text = typeof input === 'string' ? input : JSON.stringify(input);

      if (matcher.count_threshold !== undefined) {
        // Only use 'g' flag when counting matches
        const flags = matcher.flags ? `${matcher.flags}g` : 'g';
        const regex = new RegExp(matcher.pattern, flags);
        const matches = text.match(regex);
        return (matches?.length || 0) >= matcher.count_threshold;
      }

      // Use provided flags or default to none
      const regex = new RegExp(matcher.pattern, matcher.flags || '');
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

/**
 * Apply policy rules to input and return matching actions
 */
export function applyPolicy(
  policy: Policy,
  input: string | Record<string, unknown>,
): Array<{ rule: PolicyRule; matched: boolean }> {
  const results: Array<{ rule: PolicyRule; matched: boolean }> = [];

  for (const rule of policy.rules) {
    // Type assertion is safe here because:
    // 1. Policy has been validated by PolicySchema via loadPolicy()
    // 2. Zod uses z.unknown() for conditions since it can't express discriminated unions
    // 3. Runtime validation ensures conditions are well-formed Matcher objects
    const typedRule = rule as PolicyRule;

    // All conditions must match (AND logic)
    const matched =
      typedRule.conditions.length > 0 &&
      typedRule.conditions.every((condition) => evaluateMatcher(condition, input));

    results.push({ rule: typedRule, matched });
  }

  return results;
}
