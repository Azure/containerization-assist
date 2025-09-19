import { z } from 'zod';

/**
 * Policy Schema - Defines hard limits and compliance requirements
 * Policies enforce WHAT can be done (limits, security, compliance)
 * Policies always override strategies
 */
export const PolicySchema = z.object({
  version: z.string(),
  id: z.string(),
  description: z.string().optional(),

  // Hard limits that cannot be exceeded
  limits: z
    .object({
      maxTokens: z.number().positive().optional(),
      maxCost: z.number().positive().optional(),
      maxTimeMs: z.number().positive().optional(),
      maxRetries: z.number().positive().optional(),
    })
    .optional(),

  // Security constraints
  security: z
    .object({
      forbiddenPatterns: z.array(z.string()).optional(),
      requireSecurityScan: z.boolean().optional(),
      enforceNonRootUser: z.boolean().optional(),
      preventPrivilegedContainers: z.boolean().optional(),
    })
    .optional(),

  // Performance requirements
  performance: z
    .object({
      maxLatencyMs: z.number().positive().optional(),
      maxMemoryMB: z.number().positive().optional(),
      requireCaching: z.boolean().optional(),
      requireOptimization: z.boolean().optional(),
    })
    .optional(),

  // Quality standards
  quality: z
    .object({
      minTestCoverage: z.number().min(0).max(100).optional(),
      requireLinting: z.boolean().optional(),
      requireDocumentation: z.boolean().optional(),
      enforceNamingConventions: z.boolean().optional(),
    })
    .optional(),

  // Compliance requirements
  compliance: z
    .object({
      dataResidency: z.string().optional(),
      retentionDays: z.number().positive().optional(),
      auditLogging: z.boolean().optional(),
      gdprCompliant: z.boolean().optional(),
    })
    .optional(),

  // Tool-specific policies
  toolPolicies: z
    .record(z.record(z.union([z.string(), z.number(), z.boolean(), z.array(z.string())])))
    .optional(),

  // Cost optimization
  costOptimization: z
    .object({
      preferCheaperModels: z.boolean().optional(),
      cacheExpensiveOperations: z.boolean().optional(),
      batchRequests: z.boolean().optional(),
    })
    .optional(),

  // Governance
  governance: z
    .object({
      changeApproval: z.boolean().optional(),
      requireJustification: z.boolean().optional(),
      trackUsage: z.boolean().optional(),
      enforceQuotas: z.boolean().optional(),
    })
    .optional(),
});

export type Policy = z.infer<typeof PolicySchema>;

/**
 * Validate a policy configuration
 */
export function validatePolicy(data: unknown): Policy {
  return PolicySchema.parse(data);
}

/**
 * Safely validate a policy configuration
 */
export function safeValidatePolicy(
  data: unknown,
): { success: true; data: Policy } | { success: false; error: z.ZodError } {
  const result = PolicySchema.safeParse(data);
  if (result.success) {
    return { success: true, data: result.data };
  } else {
    return { success: false, error: result.error };
  }
}

/**
 * Merge multiple policies with proper precedence
 * Later policies override earlier ones
 */
export function mergePolicies(policies: Policy[]): Policy {
  if (policies.length === 0) {
    throw new Error('Cannot merge empty policy array');
  }

  const merged: Policy = {
    version: policies[policies.length - 1]?.version || '1.0.0',
    id: 'merged',
    description: 'Merged policy from multiple sources',
  };

  for (const policy of policies) {
    // Merge limits (take most restrictive)
    if (policy.limits) {
      merged.limits = {
        ...merged.limits,
        ...policy.limits,
        maxTokens:
          Math.min(merged.limits?.maxTokens ?? Infinity, policy.limits.maxTokens ?? Infinity) ||
          undefined,
        maxCost:
          Math.min(merged.limits?.maxCost ?? Infinity, policy.limits.maxCost ?? Infinity) ||
          undefined,
        maxTimeMs:
          Math.min(merged.limits?.maxTimeMs ?? Infinity, policy.limits.maxTimeMs ?? Infinity) ||
          undefined,
      };
    }

    // Merge security (combine patterns, OR booleans)
    if (policy.security) {
      merged.security = {
        ...merged.security,
        ...policy.security,
        forbiddenPatterns: [
          ...(merged.security?.forbiddenPatterns ?? []),
          ...(policy.security.forbiddenPatterns ?? []),
        ],
      };
    }

    // Merge other sections (later wins)
    if (policy.performance) merged.performance = { ...merged.performance, ...policy.performance };
    if (policy.quality) merged.quality = { ...merged.quality, ...policy.quality };
    if (policy.compliance) merged.compliance = { ...merged.compliance, ...policy.compliance };
    if (policy.toolPolicies)
      merged.toolPolicies = { ...merged.toolPolicies, ...policy.toolPolicies };
    if (policy.costOptimization)
      merged.costOptimization = { ...merged.costOptimization, ...policy.costOptimization };
    if (policy.governance) merged.governance = { ...merged.governance, ...policy.governance };
  }

  return merged;
}
