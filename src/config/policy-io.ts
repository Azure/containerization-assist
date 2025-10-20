/**
 * Policy IO - lean cache + strict validation
 */
import * as fs from 'node:fs';
import * as path from 'node:path';
import * as yaml from 'js-yaml';
import { z } from 'zod';
import { type Result, Success, Failure } from '@/types';
import { extractErrorMessage, ERROR_MESSAGES } from '@/lib/errors';
import { createLogger } from '@/lib/logger';
import { type Policy, PolicySchema } from './policy-schemas';
import { policyData } from './policy-data';

const log = createLogger().child({ module: 'policy-io' });

// Simple cache without TTL - single-user CLI tool loads policies once
const policyCache = new Map<string, Policy>();

/** Validate policy via Zod and return Result */
export function validatePolicy(p: unknown): Result<Policy> {
  try {
    return Success(PolicySchema.parse(p));
  } catch (e) {
    if (e instanceof z.ZodError) {
      const issues = e.issues.map((i) => `${i.path.join('.')}: ${i.message}`).join(', ');
      return Failure(ERROR_MESSAGES.POLICY_VALIDATION_FAILED(issues));
    }
    return Failure(ERROR_MESSAGES.POLICY_VALIDATION_FAILED(String(e)));
  }
}

/** Load + cache policy */
export function loadPolicy(file: string): Result<Policy> {
  try {
    // Simple cache lookup
    const cached = policyCache.get(path.resolve(file));
    if (cached) {
      log.debug({ file }, 'Using cached policy');
      return Success(cached);
    }

    let base: Result<Policy>;

    // Try to load from YAML file if it exists
    if (fs.existsSync(file)) {
      try {
        const content = fs.readFileSync(file, 'utf8');
        const parsed = yaml.load(content);
        base = validatePolicy(parsed);
        if (base.ok) {
          log.debug({ file }, 'Loaded policy from YAML file');
        }
      } catch (yamlError) {
        log.warn(
          { file, error: yamlError },
          'Failed to load YAML file, falling back to TypeScript data',
        );
        base = validatePolicy(policyData);
      }
    } else {
      // Fall back to TypeScript policy data
      log.debug({ file }, 'Policy file not found, using default TypeScript data');
      base = validatePolicy(policyData);
    }

    if (!base.ok) return base;

    // Sort rules by priority descending
    base.value.rules.sort((a, b) => b.priority - a.priority);

    // Cache the loaded policy
    policyCache.set(path.resolve(file), base.value);
    log.debug(
      { file, rulesCount: base.value.rules.length },
      'Policy loaded and cached successfully',
    );
    return base;
  } catch (err) {
    return Failure(ERROR_MESSAGES.POLICY_LOAD_FAILED(extractErrorMessage(err)));
  }
}

/** Create a tiny default when none exists (unchanged behavior semantics) */
export function createDefaultPolicy(): Policy {
  return {
    version: '2.0',
    metadata: {
      created: new Date().toISOString(),
      description: 'Default containerization policy',
    },
    defaults: { enforcement: 'advisory', cache_ttl: 300 },
    rules: [
      {
        id: 'security-scanning',
        category: 'security',
        priority: 100,
        conditions: [{ kind: 'regex', pattern: 'FROM .*(alpine|distroless)' }],
        actions: { enforce_scan: true, block_on_critical: true },
      },
      {
        id: 'base-image-validation',
        category: 'quality',
        priority: 90,
        conditions: [{ kind: 'function', name: 'hasPattern', args: ['FROM.*:latest'] }],
        actions: { suggest_pinned_version: true },
      },
    ],
    cache: { enabled: true, ttl: 300 },
  };
}

/**
 * Calculate policy strictness score based on rule priorities
 * Higher score = stricter policy (should override less strict ones)
 */
function calculatePolicyStrictness(policy: Policy): number {
  if (policy.rules.length === 0) return 0;

  // Use max priority as the strictness metric
  // This ensures policies with the highest-priority rules take precedence
  return policy.rules.reduce((max, r) => Math.max(max, r.priority), 0);
}

/**
 * Merge multiple policies into a single policy
 * Policies are merged in order of strictness (least strict first)
 * so that stricter policies override less strict ones for rules with the same ID
 */
function mergePolicies(policies: Policy[]): Result<Policy> {
  if (policies.length === 0) {
    return Failure('Cannot merge empty policy list');
  }

  // Sort policies by strictness (ascending) so stricter policies come last and override
  const sortedPolicies = [...policies].sort(
    (a, b) => calculatePolicyStrictness(a) - calculatePolicyStrictness(b),
  );

  if (sortedPolicies.length === 1) {
    const singlePolicy = sortedPolicies[0];
    if (!singlePolicy) {
      return Failure('Unexpected: sorted policies array is empty after validation');
    }
    return Success(singlePolicy);
  }

  const firstPolicy = sortedPolicies[0];
  if (!firstPolicy) {
    return Failure('Unexpected: sorted policies array is empty after validation');
  }

  // Start with the first policy as base
  const merged: Policy = {
    version: '2.0',
    metadata: {
      ...firstPolicy.metadata,
      name: 'Merged Policy',
      description: `Combined policy from ${sortedPolicies.length} sources`,
    },
    defaults: {},
    rules: [],
  };

  // Add cache if first policy has it
  if (firstPolicy.cache) {
    merged.cache = firstPolicy.cache;
  }

  // Merge defaults (later policies override earlier ones)
  for (const policy of sortedPolicies) {
    if (policy.defaults) {
      merged.defaults = { ...merged.defaults, ...policy.defaults };
    }
  }

  // Merge rules using a Map to handle duplicates
  // Rules from stricter policies (later in sorted list) override earlier ones
  const rulesMap = new Map<string, (typeof merged.rules)[0]>();

  for (const policy of sortedPolicies) {
    for (const rule of policy.rules) {
      rulesMap.set(rule.id, rule);
    }
  }

  // Convert back to array and sort by priority descending
  merged.rules = Array.from(rulesMap.values()).sort((a, b) => b.priority - a.priority);

  return Success(merged);
}

/**
 * Load and merge multiple policy files into a single policy
 * Returns Failure if no policies can be loaded successfully
 */
export function loadAndMergePolicies(policyPaths: string[]): Result<Policy> {
  const policies: Policy[] = [];

  for (const policyPath of policyPaths) {
    const result = loadPolicy(policyPath);
    if (result.ok) {
      policies.push(result.value);
      log.debug({ policyPath }, 'Policy loaded successfully');
    } else {
      log.warn({ policyPath, error: result.error }, 'Failed to load policy');
    }
  }

  if (policies.length === 0) {
    return Failure('No policies loaded successfully');
  }

  return mergePolicies(policies);
}
