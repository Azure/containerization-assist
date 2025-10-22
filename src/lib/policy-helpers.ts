/**
 * Shared Policy Validation Utilities
 *
 * Common functions and types for policy validation across tools.
 * This module provides shared utilities for Rego policy evaluation and violation mapping.
 */

import type { Logger } from 'pino';
import { applyPolicy } from '@/config/policy-eval';
import type { RegoEvaluator, RegoPolicyViolation } from '@/config/policy-rego';

/**
 * Policy violation from organizational policy validation
 */
export interface PolicyViolation {
  ruleId: string;
  category: string;
  priority?: number;
  message: string;
  severity: 'block' | 'warn' | 'suggest';
  description?: string;
}

/**
 * Policy validation result
 */
export interface PolicyValidationResult {
  passed: boolean;
  violations: PolicyViolation[];
  warnings: PolicyViolation[];
  suggestions: PolicyViolation[];
  summary: {
    totalRules: number;
    matchedRules: number;
    blockingViolations: number;
    warnings: number;
    suggestions: number;
  };
}

/**
 * Convert RegoPolicyViolation to PolicyViolation format
 *
 * @param regoViolation - Violation from Rego evaluation
 * @returns Standardized PolicyViolation
 */
export function mapRegoPolicyViolation(regoViolation: RegoPolicyViolation): PolicyViolation {
  const violation: PolicyViolation = {
    ruleId: regoViolation.rule,
    category: regoViolation.category,
    message: regoViolation.message,
    severity: regoViolation.severity,
  };

  // Only add optional fields if they have values
  if (regoViolation.priority !== undefined) {
    violation.priority = regoViolation.priority;
  }
  if (regoViolation.description !== undefined) {
    violation.description = regoViolation.description;
  }

  return violation;
}

/**
 * Validate content against Rego policy
 *
 * Generic utility for evaluating content against a Rego policy evaluator.
 * Used by tools like generate-dockerfile, fix-dockerfile, and generate-k8s-manifests.
 *
 * @param content - Content to validate (Dockerfile text, K8s manifest, etc.)
 * @param policyEvaluator - Rego policy evaluator
 * @param logger - Logger instance for diagnostics
 * @param contentType - Description of content type for logging (e.g., "Dockerfile", "K8s manifest")
 * @returns PolicyValidationResult
 *
 * @example
 * ```typescript
 * const result = await validateContentAgainstPolicy(
 *   dockerfileContent,
 *   policyEvaluator,
 *   logger,
 *   'Dockerfile'
 * );
 * if (!result.passed) {
 *   console.log('Violations:', result.violations);
 * }
 * ```
 */
export async function validateContentAgainstPolicy(
  content: string,
  policyEvaluator: RegoEvaluator,
  logger: Logger,
  contentType: string = 'content',
): Promise<PolicyValidationResult> {
  logger.debug({ contentType, contentLength: content.length }, 'Validating content against policy');

  // Apply Rego policy to the content
  const policyResult = await applyPolicy(policyEvaluator, content);

  // Map Rego violations to PolicyViolation format
  const violations: PolicyViolation[] = policyResult.violations.map(mapRegoPolicyViolation);
  const warnings: PolicyViolation[] = policyResult.warnings.map(mapRegoPolicyViolation);
  const suggestions: PolicyViolation[] = policyResult.suggestions.map(mapRegoPolicyViolation);

  const passed = violations.length === 0;

  const totalRules =
    (policyResult.summary?.total_violations || 0) +
    (policyResult.summary?.total_warnings || 0) +
    (policyResult.summary?.total_suggestions || 0);

  logger.info(
    {
      contentType,
      totalRules,
      matchedRules: totalRules,
      passed,
      violations: violations.length,
      warnings: warnings.length,
      suggestions: suggestions.length,
    },
    `Policy validation completed for ${contentType}`,
  );

  return {
    passed,
    violations,
    warnings,
    suggestions,
    summary: {
      totalRules,
      matchedRules: totalRules,
      blockingViolations: violations.length,
      warnings: warnings.length,
      suggestions: suggestions.length,
    },
  };
}
