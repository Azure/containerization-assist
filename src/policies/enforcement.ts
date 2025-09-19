/**
 * Policy Enforcement Engine - Ensures policies are always applied
 */

import type { Logger } from 'pino';
import { Result, Success, Failure } from '@types';
import { Policy } from './schema';
import type { EffectiveConfig } from '@/pksp/loader';

export interface PolicyViolation {
  policyId: string;
  field: string;
  reason: string;
  severity: 'error' | 'warning';
  originalValue?: unknown;
  enforcedValue?: unknown;
}

export interface PolicyEnforcementResult {
  config: EffectiveConfig;
  violations: PolicyViolation[];
  enforced: boolean;
}

export interface ValidationContext {
  tool?: string;
  user?: string;
  environment?: string;
  parameters?: Record<string, unknown>;
}

/**
 * Policy Enforcement Engine
 */
export class PolicyEngine {
  private violations: PolicyViolation[] = [];
  private logger?: Logger;

  constructor(logger?: Logger) {
    if (logger) {
      this.logger = logger.child({ component: 'PolicyEngine' });
    }
  }

  /**
   * Enforce policies on a configuration
   */
  enforce(
    config: EffectiveConfig,
    policies: Policy[],
    context?: ValidationContext,
  ): PolicyEnforcementResult {
    this.violations = [];
    let enforcedConfig = { ...config };

    for (const policy of policies) {
      enforcedConfig = this.enforcePolicy(enforcedConfig, policy, context);
    }

    const result: PolicyEnforcementResult = {
      config: enforcedConfig,
      violations: [...this.violations],
      enforced: this.violations.length > 0,
    };

    // Log violations
    if (this.violations.length > 0) {
      this.logger?.warn(
        { violations: this.violations, context },
        'Policy violations detected and enforced',
      );
    }

    return result;
  }

  /**
   * Validate constraints without enforcement
   */
  validateConstraints(
    params: Record<string, unknown>,
    policies: Policy[],
    context?: ValidationContext,
  ): Result<PolicyViolation[]> {
    const violations: PolicyViolation[] = [];

    for (const policy of policies) {
      // Check forbidden patterns
      if (policy.security?.forbiddenPatterns) {
        const paramStr = JSON.stringify(params);
        for (const pattern of policy.security.forbiddenPatterns) {
          const regex = new RegExp(pattern, 'gi');
          if (regex.test(paramStr)) {
            violations.push({
              policyId: policy.id,
              field: 'parameters',
              reason: `Contains forbidden pattern: ${pattern}`,
              severity: 'error',
            });
          }
        }
      }

      // Check tool-specific policies
      if (context?.tool && policy.toolPolicies?.[context.tool]) {
        const toolPolicy = policy.toolPolicies[context.tool];

        // Validate against tool-specific constraints
        for (const [key, constraint] of Object.entries(toolPolicy || {})) {
          const value = params[key];
          if (!this.validateConstraint(value, constraint)) {
            violations.push({
              policyId: policy.id,
              field: key,
              reason: `Violates tool-specific constraint`,
              severity: 'error',
              originalValue: value,
            });
          }
        }
      }
    }

    if (violations.length > 0) {
      return Failure(JSON.stringify(violations));
    }

    return Success(violations);
  }

  /**
   * Track policy violations for metrics
   */
  trackViolations(violations: PolicyViolation[]): void {
    // In production, send to telemetry system
    this.logger?.info(
      { violationCount: violations.length, violations },
      'Tracking policy violations',
    );
  }

  /**
   * Get policy recommendations for a context
   */
  getRecommendations(context: ValidationContext, policies: Policy[]): string[] {
    const recommendations: string[] = [];

    for (const policy of policies) {
      // Check if security scan is required
      if (policy.security?.requireSecurityScan && context?.tool?.includes('build')) {
        recommendations.push('Security scan is required for build operations');
      }

      // Check cost limits
      if (policy.limits?.maxCost && policy.limits.maxCost < 0.1) {
        recommendations.push('Cost limits are very restrictive, consider caching');
      }

      // Check timeout limits
      if (policy.limits?.maxTimeMs && policy.limits.maxTimeMs < 30000) {
        recommendations.push('Timeout limits may be too short for complex operations');
      }
    }

    return recommendations;
  }

  // Private methods

  private enforcePolicy(
    config: EffectiveConfig,
    policy: Policy,
    context?: ValidationContext,
  ): EffectiveConfig {
    const enforced = { ...config };

    // Enforce token limits
    if (policy.limits?.maxTokens && config.maxTokens) {
      if (config.maxTokens > policy.limits.maxTokens) {
        this.violations.push({
          policyId: policy.id,
          field: 'maxTokens',
          reason: `Exceeds policy limit of ${policy.limits.maxTokens}`,
          severity: 'warning',
          originalValue: config.maxTokens,
          enforcedValue: policy.limits.maxTokens,
        });
        enforced.maxTokens = policy.limits.maxTokens;
      }
    }

    // Enforce timeout limits
    if (policy.limits?.maxTimeMs && config.timeoutMs) {
      if (config.timeoutMs > policy.limits.maxTimeMs) {
        this.violations.push({
          policyId: policy.id,
          field: 'timeoutMs',
          reason: `Exceeds policy limit of ${policy.limits.maxTimeMs}ms`,
          severity: 'warning',
          originalValue: config.timeoutMs,
          enforcedValue: policy.limits.maxTimeMs,
        });
        enforced.timeoutMs = policy.limits.maxTimeMs;
      }
    }

    // Enforce security requirements
    if (policy.security) {
      // Merge forbidden patterns
      if (policy.security.forbiddenPatterns) {
        enforced.security.forbiddenPatterns = [
          ...new Set([
            ...enforced.security.forbiddenPatterns,
            ...policy.security.forbiddenPatterns,
          ]),
        ];
      }

      // Enforce security scan requirement
      if (policy.security.requireSecurityScan) {
        enforced.security.requireSecurityScan = true;
      }
    }

    // Apply tool-specific policies
    if (context?.tool && policy.toolPolicies?.[context.tool]) {
      enforced.toolConfig = {
        ...enforced.toolConfig,
        ...policy.toolPolicies[context.tool],
      };
    }

    return enforced;
  }

  private validateConstraint(value: unknown, constraint: unknown): boolean {
    // Simple validation - can be extended
    if (typeof constraint === 'object' && constraint !== null) {
      const c = constraint as Record<string, unknown>;

      if ('min' in c && typeof value === 'number') {
        if (value < (c.min as number)) return false;
      }

      if ('max' in c && typeof value === 'number') {
        if (value > (c.max as number)) return false;
      }

      if ('enum' in c && Array.isArray(c.enum)) {
        if (!c.enum.includes(value)) return false;
      }

      if ('pattern' in c && typeof value === 'string') {
        const regex = new RegExp(c.pattern as string);
        if (!regex.test(value)) return false;
      }
    }

    return true;
  }
}

/**
 * Create a singleton policy engine
 */
let engineInstance: PolicyEngine | null = null;

export function getPolicyEngine(logger?: Logger): PolicyEngine {
  if (!engineInstance) {
    engineInstance = new PolicyEngine(logger);
  }
  return engineInstance;
}

/**
 * Check if a value would violate policies
 */
export function wouldViolatePolicy(value: unknown, field: string, policies: Policy[]): boolean {
  for (const policy of policies) {
    // Check limits
    if (field === 'maxTokens' && policy.limits?.maxTokens) {
      if (typeof value === 'number' && value > policy.limits.maxTokens) {
        return true;
      }
    }

    if (field === 'timeoutMs' && policy.limits?.maxTimeMs) {
      if (typeof value === 'number' && value > policy.limits.maxTimeMs) {
        return true;
      }
    }

    if (field === 'cost' && policy.limits?.maxCost) {
      if (typeof value === 'number' && value > policy.limits.maxCost) {
        return true;
      }
    }
  }

  return false;
}

/**
 * Get effective limit for a field across all policies
 */
export function getEffectiveLimit(field: string, policies: Policy[]): number | undefined {
  let limit: number | undefined;

  for (const policy of policies) {
    let policyLimit: number | undefined;

    if (field === 'maxTokens') {
      policyLimit = policy.limits?.maxTokens;
    } else if (field === 'timeoutMs') {
      policyLimit = policy.limits?.maxTimeMs;
    } else if (field === 'cost') {
      policyLimit = policy.limits?.maxCost;
    }

    if (policyLimit !== undefined) {
      limit = limit === undefined ? policyLimit : Math.min(limit, policyLimit);
    }
  }

  return limit;
}
