/**
 * Shared types for MCP primitives.
 * Primitives are small, strongly-typed MCP tools that wrap library code
 * (knowledge matcher, policy evaluator) for use by skills and the SDK.
 */

import type { KnowledgeCategory } from '@/knowledge/types';
import type { RegoPolicyViolation } from '@/config/policy-rego';

export interface KnowledgeMatchOut {
  id: string;
  category: KnowledgeCategory;
  severity: 'high' | 'medium' | 'low' | 'required';
  title: string;
  guidance: string;
  tags: string[];
  score: number;
}

export interface QueryKnowledgeOut {
  matches: KnowledgeMatchOut[];
  totalMatched: number;
}

export type PolicyViolationSeverity = 'block' | 'warn' | 'suggest';

export interface PolicyFinding {
  rule: string;
  severity: PolicyViolationSeverity;
  message: string;
  category: string;
  priority?: number;
  /** Optional remediation guidance provided by some policy rules. */
  hint?: string;
  /** Content-type-specific location reference (e.g. "L14" for Dockerfile, "spec.containers[0]" for K8s). */
  path?: string;
}

export interface ValidatePolicyOut {
  /** True when there are no blocking violations. */
  passed: boolean;
  /** Blocking findings (severity === 'block'). */
  violations: PolicyFinding[];
  /** Non-blocking warnings (severity === 'warn'). */
  warnings: PolicyFinding[];
  /** Optional improvements (severity === 'suggest'). */
  suggestions: PolicyFinding[];
}

/**
 * Map a raw RegoPolicyViolation to a PolicyFinding.
 *
 * Severity is taken from the CALLER's bucket (violations → 'block', warnings → 'warn',
 * suggestions → 'suggest') rather than from the violation's own `severity` field. This
 * matches the project-wide convention in `validateContentAgainstPolicy`
 * (src/lib/policy-helpers.ts) and trusts the three result arrays to be the source of truth.
 * If a policy author accidentally tags a rule with a severity inconsistent with the bucket
 * it emits into, we silently use the bucket. This is intentional.
 */
export function toFinding(
  r: RegoPolicyViolation,
  severity: PolicyFinding['severity'],
): PolicyFinding {
  return {
    rule: r.rule,
    severity,
    message: r.message,
    category: r.category,
    ...(r.priority !== undefined && { priority: r.priority }),
    ...(r.description && { hint: r.description }),
  };
}
