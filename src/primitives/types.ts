// src/primitives/types.ts
/**
 * Shared types for MCP primitives.
 * Primitives are small, strongly-typed MCP tools that wrap library code
 * (knowledge matcher, policy evaluator) for use by skills and the SDK.
 */

export interface KnowledgeMatchOut {
  id: string;
  category: 'security' | 'performance' | 'best-practices' | 'optimization';
  severity: 'high' | 'medium' | 'low';
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
  hint?: string;
  path?: string;
}

export interface ValidatePolicyOut {
  passed: boolean;
  violations: PolicyFinding[]; // severity === 'block'
  warnings: PolicyFinding[]; // severity === 'warn'
  suggestions: PolicyFinding[]; // severity === 'suggest'
}

export type ContentType = 'dockerfile' | 'k8s-manifest' | 'compose';
