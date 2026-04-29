import type { RegoEvaluator, RegoPolicyResult } from '@/config/policy-rego';

/**
 * Build a stub RegoEvaluator for unit-testing primitives that consume policies.
 *
 * Pass either a RegoPolicyResult (returned directly) or an async thunk that
 * throws (to exercise the catch-and-Failure path).
 */
export function makeMockEvaluator(
  result: RegoPolicyResult | (() => Promise<never>),
): RegoEvaluator {
  return {
    evaluate: typeof result === 'function' ? result : async () => result,
  } as unknown as RegoEvaluator;
}
