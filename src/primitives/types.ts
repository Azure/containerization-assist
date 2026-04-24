/**
 * Shared types for MCP primitives.
 * Primitives are small, strongly-typed MCP tools that wrap library code
 * (knowledge matcher, policy evaluator) for use by skills and the SDK.
 */

import type { KnowledgeCategory } from '@/knowledge/types';

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

export type ContentType = 'dockerfile' | 'k8s-manifest' | 'compose';
