/**
 * Policy-Prompt Integration Module
 * Applies policy constraints to AI prompts for consistent behavior
 */

import { loadPolicy } from '@/config/policy-io';
import { buildConstraints } from '@/config/policy-constraints';
import type { Policy } from '@/config/policy-schemas';
import { createLogger } from '@/lib/logger';

const logger = createLogger().child({ module: 'policy-prompt' });

export interface PolicyPromptContext {
  /** Tool being executed */
  tool: string;
  /** Target environment */
  environment?: string;
  /** Additional context for policy filtering */
  tags?: string[];
}

/**
 * Get policy instance using built-in cache
 */
function getPolicy(path: string, environment?: string): Policy | null {
  const result = loadPolicy(path, environment);
  if (result.ok) {
    return result.value;
  }
  logger.debug({ path, environment, error: result.error }, 'Failed to load policy');
  return null;
}

/**
 * Build policy constraints for the prompt engine.
 * Returns an array of constraint strings to be included in the system message.
 *
 * @param context - Context for building constraints
 * @returns Array of constraint strings
 */
export function buildPolicyConstraints(context: { tool: string; environment: string }): string[] {
  try {
    // Try to load policy from correct default location
    const policyPath = process.env.POLICY_FILE || 'config/policy.yaml';
    const policy = getPolicy(policyPath, context.environment);

    if (!policy) {
      logger.debug('No policy loaded, returning empty constraints');
      return [];
    }

    // Build constraints using data-driven approach
    const constraints = buildConstraints({
      policy,
      tool: context.tool,
      environment: context.environment,
    });

    logger.debug(
      {
        tool: context.tool,
        environment: context.environment,
        constraintCount: constraints.length,
      },
      'Built policy constraints for prompt engine',
    );

    return constraints;
  } catch (error) {
    logger.warn({ error }, 'Failed to build policy constraints');
    return [];
  }
}

/**
 * Get policy summary for logging/debugging
 */
export function getPolicySummary(environment?: string): string {
  const policyPath = process.env.POLICY_FILE || 'config/policy.yaml';
  const env = environment || 'development';
  const policy = getPolicy(policyPath, env);

  if (!policy) {
    return 'No policy loaded';
  }

  const envConfig = policy.environments?.[env];
  const summary = [
    `Environment: ${env}`,
    `Enforcement: ${policy.defaults?.enforcement || 'advisory'}`,
    `Rules: ${policy.rules.length}`,
  ];

  if (envConfig?.defaults) {
    const defaults = envConfig.defaults;
    if (defaults.allowedBaseImages?.length) {
      summary.push(`Base Images: ${defaults.allowedBaseImages.length} allowed`);
    }
    if (defaults.registries?.allowed?.length) {
      summary.push(`Registries: ${defaults.registries.allowed.length} allowed`);
    }
  }

  return summary.join(', ');
}
