/**
 * Policy Evaluation Module - Rego Only
 *
 * Handles Rego policy evaluation using OPA.
 * YAML policy support has been removed in favor of industry-standard OPA Rego.
 */

import type { RegoEvaluator, RegoPolicyResult } from './policy-rego';

/**
 * Apply Rego policy to input and return evaluation result
 *
 * @param evaluator - Rego policy evaluator
 * @param input - Content to evaluate (Dockerfile text, K8s manifest, etc.)
 * @returns Promise<RegoPolicyResult>
 *
 * @example
 * ```typescript
 * const result = await applyPolicy(evaluator, dockerfileContent);
 * if (!result.allow) {
 *   console.log('Violations:', result.violations);
 * }
 * ```
 */
export async function applyPolicy(
  evaluator: RegoEvaluator,
  input: string | Record<string, unknown>,
): Promise<RegoPolicyResult> {
  return evaluator.evaluate(input);
}
