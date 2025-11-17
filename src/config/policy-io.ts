/**
 * Policy IO - Unified Policy Loading (Rego + CEL)
 *
 * This module provides loading and caching for both Rego and CEL policies.
 *
 * **Supported Formats:**
 * - `.rego` files - Rego policies (built-in and user custom)
 * - `.yaml`, `.yml` files - CEL policies (user custom only)
 *
 * **Policy Loading:**
 * - Auto-detects format by file extension
 * - Caches compiled policies for performance
 * - Supports merging multiple policies (mixed formats)
 */
import * as path from 'node:path';
import { type Result, Success, Failure } from '@/types';
import { extractErrorMessage, ERROR_MESSAGES } from '@/lib/errors';
import { createLogger } from '@/lib/logger';
import { loadRegoPolicy, loadAndMergeRegoPolicies, type RegoEvaluator } from './policy-rego';
import { loadCelPolicy } from './policy-cel';
import { createMergedEvaluator } from './policy-merged-evaluator';

const log = createLogger().child({ module: 'policy-io' });

// Simple cache without TTL - single-user CLI tool loads policies once
const policyCache = new Map<string, RegoEvaluator>();

/**
 * Load policy from file (auto-detects format)
 *
 * Supports:
 * - `.rego` files → Rego policies
 * - `.yaml`, `.yml` files → CEL policies
 *
 * @param file - Path to policy file
 * @returns Promise<Result<RegoEvaluator>>
 *
 * @example
 * ```typescript
 * // Load Rego policy
 * const rego = await loadPolicy('policies/security.rego');
 *
 * // Load CEL policy
 * const cel = await loadPolicy('policies.user/custom.yaml');
 * ```
 */
export async function loadPolicy(file: string): Promise<Result<RegoEvaluator>> {
  try {
    const resolvedPath = path.resolve(file);

    // Check cache first
    const cached = policyCache.get(resolvedPath);
    if (cached) {
      log.debug({ file }, 'Using cached policy');
      return Success(cached);
    }

    // Detect format by extension
    let result: Result<RegoEvaluator>;

    if (file.endsWith('.rego')) {
      log.debug({ file }, 'Loading Rego policy');
      result = await loadRegoPolicy(file, log);
    } else if (file.endsWith('.yaml') || file.endsWith('.yml')) {
      log.debug({ file }, 'Loading CEL policy');
      result = await loadCelPolicy(file, log);
    } else {
      return Failure(`Unsupported policy file format: ${file}`, {
        message: 'Unknown policy file extension',
        hint: `File: ${file}`,
        resolution: 'Policy files must end with .rego, .yaml, or .yml',
      });
    }

    // Cache successful loads
    if (result.ok) {
      policyCache.set(resolvedPath, result.value);
    }

    return result;
  } catch (err) {
    return Failure(ERROR_MESSAGES.POLICY_LOAD_FAILED(extractErrorMessage(err)));
  }
}

/**
 * Load and merge multiple policy files (mixed formats supported)
 *
 * Supports mixing Rego (.rego) and CEL (.yaml, .yml) policies.
 * All policies are evaluated and their results are merged.
 *
 * @param policyPaths - Array of policy file paths (.rego, .yaml, .yml)
 * @returns Promise<Result<RegoEvaluator>>
 *
 * @example
 * ```typescript
 * // Mix Rego and CEL policies
 * const result = await loadAndMergePolicies([
 *   'policies/security.rego',
 *   'policies.user/custom.yaml'
 * ]);
 * ```
 */
export async function loadAndMergePolicies(
  policyPaths: string[],
): Promise<Result<RegoEvaluator>> {
  if (policyPaths.length === 0) {
    return Failure('No policy paths provided');
  }

  log.info(
    { policyCount: policyPaths.length, policyPaths },
    'Loading and merging multiple policies'
  );

  // Validate file extensions
  const validExtensions = ['.rego', '.yaml', '.yml'];
  const invalidPaths = policyPaths.filter(
    (p) => !validExtensions.some((ext) => p.endsWith(ext))
  );

  if (invalidPaths.length > 0) {
    return Failure('Invalid policy file formats detected', {
      message: 'All policy files must be .rego, .yaml, or .yml',
      hint: `Invalid files: ${invalidPaths.join(', ')}`,
      resolution: 'Ensure all policy files have valid extensions',
    });
  }

  // Separate Rego and CEL policies
  const regoPolicies = policyPaths.filter((p) => p.endsWith('.rego'));
  const celPolicies = policyPaths.filter((p) => p.endsWith('.yaml') || p.endsWith('.yml'));

  // If all Rego, use optimized Rego-specific merger
  if (celPolicies.length === 0) {
    return loadAndMergeRegoPolicies(regoPolicies, log);
  }

  // Mixed or all CEL: load each policy individually and merge
  const evaluators: RegoEvaluator[] = [];

  for (const policyPath of policyPaths) {
    const result = await loadPolicy(policyPath);
    if (!result.ok) {
      return result; // Return first error
    }
    evaluators.push(result.value);
  }

  // If only one policy, return it directly
  if (evaluators.length === 1) {
    return Success(evaluators[0]);
  }

  // Multiple policies: create merged evaluator
  const merged = createMergedEvaluator(evaluators, log);
  return Success(merged);
}

/**
 * Clear the policy cache
 *
 * Clears all cached policy evaluators (both Rego and CEL).
 * Useful for testing or when policies are updated.
 */
export function clearPolicyCache(): void {
  policyCache.clear();
  log.debug('Policy cache cleared');
}
