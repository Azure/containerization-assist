/**
 * Policy Evaluation Module
 * Handles rule evaluation and policy application
 */

import * as fs from 'node:fs';
import * as path from 'node:path';
import type { Policy, PolicyRule, Matcher, FunctionMatcher } from './policy-schemas';

/**
 * Convert input to searchable text for pattern matching
 */
function toSearchableText(input: string | Record<string, unknown>): string {
  return typeof input === 'string' ? input : JSON.stringify(input);
}

/**
 * Evaluate a function matcher against input data
 */
function evaluateFunctionMatcher(
  matcher: FunctionMatcher,
  input: string | Record<string, unknown>,
): boolean {
  const text = toSearchableText(input);

  switch (matcher.name) {
    case 'hasPattern': {
      const [pattern, flags] = matcher.args as [string, string?];
      return new RegExp(pattern, flags).test(text);
    }

    case 'fileExists': {
      const [filePath] = matcher.args as [string];
      const basePath = typeof input === 'object' && 'path' in input ? String(input.path) : '.';
      // Prevent path traversal attacks
      const resolvedBase = path.resolve(basePath);
      const resolvedTarget = path.resolve(basePath, filePath);
      if (!resolvedTarget.startsWith(resolvedBase + path.sep) && resolvedTarget !== resolvedBase) {
        // Path traversal detected, do not allow
        return false;
      }
      return fs.existsSync(resolvedTarget);
    }

    case 'largerThan': {
      const [size] = matcher.args as [number];
      if (typeof input === 'string') return input.length > size;
      if (typeof input === 'object' && 'size' in input) {
        return Number(input.size) > size;
      }
      return false;
    }

    case 'hasVulnerabilities': {
      const [severities] = matcher.args as [string[]];
      if (typeof input === 'object' && 'vulnerabilities' in input) {
        const vulns = input.vulnerabilities as Array<{ severity: string }>;
        return vulns.some((v) => severities.includes(v.severity.toUpperCase()));
      }
      return false;
    }

    default:
      return false;
  }
}

/**
 * Evaluate a matcher against input data
 */
export function evaluateMatcher(
  matcher: Matcher,
  input: string | Record<string, unknown>,
): boolean {
  switch (matcher.kind) {
    case 'regex': {
      const text = toSearchableText(input);

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
      return evaluateFunctionMatcher(matcher, input);
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
