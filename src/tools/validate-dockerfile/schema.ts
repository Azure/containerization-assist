import { z } from 'zod';

export const validateImageSchema = z.object({
  path: z.string().optional().describe('Path to Dockerfile to validate'),
  dockerfile: z
    .string()
    .optional()
    .describe('Dockerfile content to validate (alternative to path)'),
  policyPath: z
    .string()
    .optional()
    .describe('Optional path to specific policy file to use (defaults to all policies in policies/)'),
});

export interface PolicyViolation {
  ruleId: string;
  category: string | undefined;
  priority: number;
  severity: 'block' | 'warn' | 'suggest';
  message: string;
}

export interface ValidateImageResult {
  success: boolean;
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
