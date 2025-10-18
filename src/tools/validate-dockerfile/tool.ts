/**
 * Validate Dockerfile Tool
 *
 * Validates Dockerfile content against organizational policies defined in policies/*.yaml
 * Applies policy rules and returns violations, warnings, and suggestions.
 */

import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { getToolLogger } from '@/lib/tool-helpers';
import {
  validateImageSchema,
  type ValidateImageResult,
  type PolicyViolation,
} from './schema';
import { existsSync, readdirSync } from 'node:fs';
import { readFile } from 'node:fs/promises';
import nodePath from 'node:path';
import type { z } from 'zod';
import { loadPolicy } from '@/config/policy-io';
import { applyPolicy } from '@/config/policy-eval';
import type { Policy } from '@/config/policy-schemas';
import { createLogger } from '@/lib/logger';

const name = 'validate-dockerfile';
const description = 'Validate Dockerfile against organizational policies';
const version = '2.0.0';

/**
 * Get default policy paths from policies/ directory
 */
function getDefaultPolicyPaths(): string[] {
  const logger = createLogger({ name: 'policy-discovery' });
  try {
    const policiesDir = nodePath.join(process.cwd(), 'policies');

    if (!existsSync(policiesDir)) {
      logger.debug({ policiesDir }, 'Policies directory not found');
      return [];
    }

    const files = readdirSync(policiesDir);
    return files
      .filter((f: string) => f.endsWith('.yaml') || f.endsWith('.yml'))
      .sort((a: string, b: string) => a.localeCompare(b, undefined, { numeric: true }))
      .map((f: string) => nodePath.join(policiesDir, f));
  } catch (error) {
    logger.warn(
      { error, cwd: process.cwd() },
      'Failed to read policies directory - using no policies',
    );
    return [];
  }
}

/**
 * Merge multiple policies into a single unified policy
 */
function mergePolicies(policies: Policy[]): Policy {
  if (policies.length === 0) {
    throw new Error('Cannot merge empty policy list');
  }

  if (policies.length === 1) {
    const singlePolicy = policies[0];
    if (!singlePolicy) {
      throw new Error('Unexpected: policy array is empty');
    }
    return singlePolicy;
  }

  // Merge all policies - later policies override earlier ones for rules with same ID
  const ruleMap = new Map<string, Policy['rules'][0]>();
  let mergedDefaults = {};

  for (const policy of policies) {
    // Merge defaults (later overrides earlier)
    mergedDefaults = { ...mergedDefaults, ...policy.defaults };

    // Merge rules by ID (later overrides earlier)
    for (const rule of policy.rules) {
      ruleMap.set(rule.id, rule);
    }
  }

  const merged: Policy = {
    version: '2.0',
    metadata: {
      name: 'Merged Policies',
      description: `Merged from ${policies.length} policy files`,
    },
    defaults: mergedDefaults,
    rules: Array.from(ruleMap.values()).sort((a, b) => b.priority - a.priority),
  };

  return merged;
}

/**
 * Classify matched rules by severity based on actions
 */
function classifyViolation(
  ruleId: string,
  category: string | undefined,
  priority: number,
  actions: Record<string, unknown>,
  description?: string,
): PolicyViolation | null {
  // Check for blocking actions
  if (actions.block === true || actions.block_deployment === true || actions.block_build === true) {
    return {
      ruleId,
      category,
      priority,
      severity: 'block',
      message: (actions.message as string) || description || `Rule ${ruleId} violated`,
    };
  }

  // Check for warning actions
  if (actions.warn === true || actions.require_approval === true) {
    return {
      ruleId,
      category,
      priority,
      severity: 'warn',
      message: (actions.message as string) || description || `Rule ${ruleId} triggered warning`,
    };
  }

  // Check for suggestion actions
  if (actions.suggest === true || actions.suggest_pinned_version === true) {
    return {
      ruleId,
      category,
      priority,
      severity: 'suggest',
      message: (actions.message as string) || description || `Rule ${ruleId} suggests improvement`,
    };
  }

  // Rule matched but no actionable severity
  return null;
}

async function handleValidateDockerfile(
  input: z.infer<typeof validateImageSchema>,
  ctx: ToolContext,
): Promise<Result<ValidateImageResult>> {
  const logger = getToolLogger(ctx, 'validate-dockerfile');
  const { path, dockerfile: inputDockerfile, policyPath } = input;

  let content = inputDockerfile || '';

  // Read Dockerfile from path if provided
  if (path) {
    const dockerfilePath = nodePath.isAbsolute(path) ? path : nodePath.resolve(process.cwd(), path);
    try {
      content = await readFile(dockerfilePath, 'utf-8');
    } catch (error) {
      return Failure(`Failed to read Dockerfile at ${dockerfilePath}: ${error}`);
    }
  }

  if (!content) {
    return Failure('Either path or dockerfile content is required');
  }

  // Load policies
  const policyPaths = policyPath ? [policyPath] : getDefaultPolicyPaths();

  if (policyPaths.length === 0) {
    logger.warn('No policy files found - validation will pass without checks');
    return Success({
      success: true,
      passed: true,
      violations: [],
      warnings: [],
      suggestions: [],
      summary: {
        totalRules: 0,
        matchedRules: 0,
        blockingViolations: 0,
        warnings: 0,
        suggestions: 0,
      },
    });
  }

  const policies: Policy[] = [];
  for (const policyFile of policyPaths) {
    const policyResult = loadPolicy(policyFile);
    if (policyResult.ok) {
      policies.push(policyResult.value);
      logger.debug({ policyFile, rulesCount: policyResult.value.rules.length }, 'Loaded policy');
    } else {
      logger.warn({ policyFile, error: policyResult.error }, 'Failed to load policy file');
    }
  }

  if (policies.length === 0) {
    return Failure('No valid policies could be loaded');
  }

  // Merge all policies
  const mergedPolicy = mergePolicies(policies);

  logger.info(
    {
      policiesLoaded: policies.length,
      totalRules: mergedPolicy.rules.length,
    },
    'Validating Dockerfile against policies',
  );

  // Apply policy to Dockerfile content
  const policyResults = applyPolicy(mergedPolicy, content);

  // Classify matched rules
  const violations: PolicyViolation[] = [];
  const warnings: PolicyViolation[] = [];
  const suggestions: PolicyViolation[] = [];

  let matchedRulesCount = 0;

  for (const result of policyResults) {
    if (!result.matched) continue;

    matchedRulesCount++;
    const violation = classifyViolation(
      result.rule.id,
      result.rule.category,
      result.rule.priority,
      result.rule.actions,
      result.rule.description,
    );

    if (!violation) continue;

    switch (violation.severity) {
      case 'block':
        violations.push(violation);
        break;
      case 'warn':
        warnings.push(violation);
        break;
      case 'suggest':
        suggestions.push(violation);
        break;
    }
  }

  const passed = violations.length === 0;

  const resultData: ValidateImageResult = {
    success: true,
    passed,
    violations,
    warnings,
    suggestions,
    summary: {
      totalRules: mergedPolicy.rules.length,
      matchedRules: matchedRulesCount,
      blockingViolations: violations.length,
      warnings: warnings.length,
      suggestions: suggestions.length,
    },
  };

  logger.info(
    {
      totalRules: mergedPolicy.rules.length,
      matchedRules: matchedRulesCount,
      passed,
      violations: violations.length,
      warnings: warnings.length,
      suggestions: suggestions.length,
    },
    'Dockerfile validation completed',
  );

  return Success(resultData);
}

import { tool } from '@/types/tool';

export default tool({
  name,
  description,
  category: 'docker',
  version,
  schema: validateImageSchema,
  metadata: {
    knowledgeEnhanced: false,
  },
  chainHints: {
    success:
      'Dockerfile validated against policies. If violations exist, fix them before building. Continue by building the Dockerfile with build-image, then proceed with generate-k8s-manifests.',
    failure:
      'Dockerfile validation failed. Review the policy violations and update the Dockerfile to comply with organizational policies.',
  },
  handler: handleValidateDockerfile,
});
