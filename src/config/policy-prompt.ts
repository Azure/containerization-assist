/**
 * Policy-Prompt Integration Module
 * Applies policy constraints to AI prompts for consistent behavior
 */

import { loadPolicy, type UnifiedPolicy } from '@/config/policy';
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
 * Apply policy constraints to an AI prompt
 * Adds environment-specific rules and organizational constraints
 */
export function applyPolicyConstraints(prompt: string, context: PolicyPromptContext): string {
  try {
    // Try to load policy from default location
    const policyPath = process.env.POLICY_FILE || 'policy.yaml';
    const policyResult = loadPolicy(policyPath);
    if (!policyResult.ok) {
      logger.debug('No policy loaded, using unconstrained prompt');
      return prompt;
    }

    const policy = policyResult.value;
    const environment = context.environment || 'development';
    const constraints: string[] = [];

    // Get environment-specific configuration
    const envConfig = policy.environments?.[environment];
    const envDefaults = envConfig?.defaults || {};

    // Apply environment defaults as constraints
    const allowedBaseImages = (envDefaults as any).allowedBaseImages;
    if (Array.isArray(allowedBaseImages) && allowedBaseImages.length > 0) {
      constraints.push(`IMPORTANT: Only use these base images: ${allowedBaseImages.join(', ')}`);
    }

    const registries = (envDefaults as any).registries;
    if (registries?.allowed && Array.isArray(registries.allowed) && registries.allowed.length > 0) {
      constraints.push(`Container registries must be one of: ${registries.allowed.join(', ')}`);
    }

    const security = (envDefaults as any).security;
    if (security?.scanners?.required) {
      constraints.push('Include security scanning steps in all CI/CD pipelines');
      if (Array.isArray(security.scanners.tools) && security.scanners.tools.length > 0) {
        constraints.push(`Use these security scanners: ${security.scanners.tools.join(', ')}`);
      }
    }

    const resources = (envDefaults as any).resources;
    if (resources?.limits) {
      const limits = resources.limits;
      if (limits.cpu || limits.memory) {
        constraints.push(
          `Apply resource limits - CPU: ${limits.cpu || 'default'}, Memory: ${limits.memory || 'default'}`,
        );
      }
    }

    const naming = (envDefaults as any).naming;
    if (naming?.pattern) {
      constraints.push(`Follow naming pattern: ${naming.pattern}`);
      if (Array.isArray(naming.examples) && naming.examples.length > 0) {
        constraints.push(`Naming examples: ${naming.examples.join(', ')}`);
      }
    }

    // Add tool-specific policy rules
    const toolRules = getToolSpecificRules(policy, context.tool, environment);
    constraints.push(...toolRules);

    // Add security best practices for production
    if (environment === 'production') {
      constraints.push(
        'Use non-root users in containers',
        'Minimize container image size',
        'Use specific version tags (not :latest)',
        'Include health checks and readiness probes',
        'Set resource limits and requests',
      );
    }

    // If no constraints, return original prompt
    if (constraints.length === 0) {
      return prompt;
    }

    // Append constraints to prompt
    const constrainedPrompt = `${prompt}

## Policy Constraints
You must follow these organizational policies and best practices:
${constraints.map((c) => `- ${c}`).join('\n')}`;

    logger.debug(
      {
        tool: context.tool,
        environment,
        constraintCount: constraints.length,
      },
      'Applied policy constraints to prompt',
    );

    return constrainedPrompt;
  } catch (error) {
    logger.warn({ error }, 'Failed to apply policy constraints, using unconstrained prompt');
    return prompt;
  }
}

/**
 * Get tool-specific policy rules
 */
function getToolSpecificRules(policy: UnifiedPolicy, tool: string, environment: string): string[] {
  const rules: string[] = [];

  // Tool-specific policies based on common patterns
  const toolPolicies: Record<string, string[]> = {
    'generate-dockerfile': [
      'Use multi-stage builds for production images',
      'Include LABEL metadata with version and maintainer',
      'Copy only necessary files (avoid COPY . .)',
      'Run security updates in separate layer',
    ],
    'generate-k8s-manifests': [
      'Include namespace definitions',
      'Set resource quotas and limits',
      'Use ConfigMaps for configuration',
      'Include NetworkPolicies for security',
    ],
    'generate-helm-charts': [
      'Use semantic versioning for chart versions',
      'Include comprehensive values.yaml with defaults',
      'Add NOTES.txt for post-install instructions',
      'Include helpers for common labels and selectors',
    ],
    'fix-dockerfile': [
      'Pin base image versions',
      'Combine RUN commands to reduce layers',
      'Order layers from least to most frequently changing',
      'Remove package manager caches',
    ],
    scan: [
      'Report HIGH and CRITICAL vulnerabilities',
      'Include remediation suggestions',
      'Check for outdated base images',
    ],
  };

  const toolRules = toolPolicies[tool];
  if (toolRules && environment === 'production') {
    rules.push(...toolRules);
  }

  // Apply category-based rules from policy
  const categoryRules = policy.rules
    .filter((rule: any) => {
      // Check if rule applies to this tool/category
      if (rule.category === 'security' && environment === 'production') {
        return true;
      }
      if (rule.category === 'quality' && tool.includes('generate')) {
        return true;
      }
      return false;
    })
    .sort((a: any, b: any) => b.priority - a.priority)
    .slice(0, 3) // Take top 3 relevant rules
    .map((rule: any) => rule.description)
    .filter((desc: any): desc is string => Boolean(desc));

  rules.push(...categoryRules);

  return rules;
}

/**
 * Get policy summary for logging/debugging
 */
export function getPolicySummary(environment?: string): string {
  const policyPath = process.env.POLICY_FILE || 'policy.yaml';
  const policyResult = loadPolicy(policyPath);
  if (!policyResult.ok) {
    return 'No policy loaded';
  }

  const policy = policyResult.value;
  const env = environment || 'development';
  const envConfig = policy.environments?.[env];

  const summary = [
    `Environment: ${env}`,
    `Enforcement: ${policy.defaults?.enforcement || 'advisory'}`,
    `Rules: ${policy.rules.length}`,
  ];

  if (envConfig?.defaults) {
    const defaults = envConfig.defaults as any;
    const allowedBaseImages = defaults.allowedBaseImages;
    if (Array.isArray(allowedBaseImages) && allowedBaseImages.length > 0) {
      summary.push(`Base Images: ${allowedBaseImages.length} allowed`);
    }
    const registries = defaults.registries;
    if (registries?.allowed && Array.isArray(registries.allowed) && registries.allowed.length > 0) {
      summary.push(`Registries: ${registries.allowed.length} allowed`);
    }
  }

  return summary.join(', ');
}
