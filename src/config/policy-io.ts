/**
 * Policy IO - Rego Policy Loading
 *
 * This module provides loading and caching for Rego policies.
 * YAML policy support has been removed in favor of industry-standard OPA Rego.
 */
import * as path from 'node:path';
import { type Result, Success, Failure } from '@/types';
import { extractErrorMessage, ERROR_MESSAGES } from '@/lib/errors';
import { createLogger } from '@/lib/logger';
import { loadRegoPolicy, loadAndMergeRegoPolicies, type RegoEvaluator } from './policy-rego';

const log = createLogger().child({ module: 'policy-io' });

// Simple cache without TTL - single-user CLI tool loads policies once
const regoCache = new Map<string, RegoEvaluator>();

/**
 * Load Rego policy from file
 *
 * @param file - Path to .rego policy file
 * @returns Promise<Result<RegoEvaluator>>
 */
export async function loadPolicy(file: string): Promise<Result<RegoEvaluator>> {
  try {
    // Validate .rego extension
    if (!file.endsWith('.rego')) {
      return Failure('Only .rego policy files are supported', {
        message: 'Invalid policy file format',
        hint: `File: ${file}`,
        resolution: 'Provide a .rego policy file. YAML policies are no longer supported.',
      });
    }

    // Check cache first
    const cached = regoCache.get(path.resolve(file));
    if (cached) {
      log.debug({ file }, 'Using cached Rego policy');
      return Success(cached);
    }

    // Load and compile Rego policy
    const result = await loadRegoPolicy(file, log);
    if (result.ok) {
      regoCache.set(path.resolve(file), result.value);
    }
    return result;
  } catch (err) {
    return Failure(ERROR_MESSAGES.POLICY_LOAD_FAILED(extractErrorMessage(err)));
  }
}

/**
 * Load and merge multiple Rego policy files
 *
 * @param policyPaths - Array of .rego policy file paths
 * @returns Promise<Result<RegoEvaluator>>
 */
export async function loadAndMergePolicies(
  policyPaths: string[],
): Promise<Result<RegoEvaluator>> {
  // Validate all paths are .rego files
  const nonRegoPaths = policyPaths.filter((p) => !p.endsWith('.rego'));
  if (nonRegoPaths.length > 0) {
    return Failure('All policy files must be .rego format', {
      message: 'Invalid policy file formats detected',
      hint: `Non-Rego files: ${nonRegoPaths.join(', ')}`,
      resolution: 'Convert YAML policies to Rego format. YAML is no longer supported.',
    });
  }

  return loadAndMergeRegoPolicies(policyPaths, log);
}

/**
 * Clear the policy cache
 * Useful for testing or when policies are updated
 */
export function clearPolicyCache(): void {
  regoCache.clear();
  log.debug('Policy cache cleared');
}
