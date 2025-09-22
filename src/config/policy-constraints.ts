/**
 * Policy Constraints Module
 * Data-driven constraint extraction from policies
 */

import type { Policy, EnvironmentDefaults } from './policy-schemas';

/**
 * Constraint builder for composing policy constraints
 */
export class ConstraintBuilder {
  private constraints: string[] = [];

  add(constraint: string | null | undefined): this {
    if (constraint) {
      this.constraints.push(constraint);
    }
    return this;
  }

  addAll(constraints: string[]): this {
    this.constraints.push(...constraints);
    return this;
  }

  build(): string[] {
    return [...this.constraints];
  }
}

/**
 * Extract constraints from environment defaults
 */
export function extractEnvironmentConstraints(defaults: EnvironmentDefaults): string[] {
  const builder = new ConstraintBuilder();

  // Base image constraints
  if (defaults.allowedBaseImages?.length) {
    builder.add(`IMPORTANT: Only use these base images: ${defaults.allowedBaseImages.join(', ')}`);
  }

  // Registry constraints
  if (defaults.registries?.allowed?.length) {
    builder.add(`Container registries must be one of: ${defaults.registries.allowed.join(', ')}`);
  }
  if (defaults.registries?.blocked?.length) {
    builder.add(`Never use these registries: ${defaults.registries.blocked.join(', ')}`);
  }

  // Security constraints
  if (defaults.security?.scanners?.required) {
    builder.add('Include security scanning steps in all CI/CD pipelines');
    if (defaults.security.scanners.tools?.length) {
      builder.add(`Use these security scanners: ${defaults.security.scanners.tools.join(', ')}`);
    }
  }
  if (defaults.security?.nonRootUser) {
    builder.add('Use non-root users in containers');
  }
  if (defaults.security?.minimizeSize) {
    builder.add('Minimize container image size');
  }

  // Resource constraints
  if (defaults.resources?.limits) {
    const { cpu, memory } = defaults.resources.limits;
    if (cpu || memory) {
      builder.add(
        `Apply resource limits - CPU: ${cpu || 'default'}, Memory: ${memory || 'default'}`,
      );
    }
  }
  if (defaults.resources?.requests) {
    const { cpu, memory } = defaults.resources.requests;
    if (cpu || memory) {
      builder.add(
        `Set resource requests - CPU: ${cpu || 'default'}, Memory: ${memory || 'default'}`,
      );
    }
  }

  // Naming constraints
  if (defaults.naming?.pattern) {
    builder.add(`Follow naming pattern: ${defaults.naming.pattern}`);
    if (defaults.naming.examples?.length) {
      builder.add(`Naming examples: ${defaults.naming.examples.join(', ')}`);
    }
  }

  return builder.build();
}

/**
 * Get tool-specific constraints
 */
export function getToolConstraints(tool: string, environment: string): string[] {
  const toolPolicies: Record<string, Record<string, string[]>> = {
    'generate-dockerfile': {
      production: [
        'Use multi-stage builds for production images',
        'Include LABEL metadata with version and maintainer',
        'Copy only necessary files (avoid COPY . .)',
        'Run security updates in separate layer',
      ],
      development: [
        'Include development tools for debugging',
        'Use volume mounts for hot reloading',
      ],
    },
    'generate-k8s-manifests': {
      production: [
        'Include namespace definitions',
        'Set resource quotas and limits',
        'Use ConfigMaps for configuration',
        'Include NetworkPolicies for security',
      ],
      development: ['Use NodePort services for easy access', 'Include debug containers if needed'],
    },
    'generate-helm-charts': {
      production: [
        'Use semantic versioning for chart versions',
        'Include comprehensive values.yaml with defaults',
        'Add NOTES.txt for post-install instructions',
        'Include helpers for common labels and selectors',
      ],
      development: [
        'Include values files for local development',
        'Add debug hooks for troubleshooting',
      ],
    },
    'fix-dockerfile': {
      production: [
        'Pin base image versions',
        'Combine RUN commands to reduce layers',
        'Order layers from least to most frequently changing',
        'Remove package manager caches',
      ],
      development: [],
    },
    scan: {
      production: [
        'Report HIGH and CRITICAL vulnerabilities',
        'Include remediation suggestions',
        'Check for outdated base images',
      ],
      development: ['Report all vulnerability levels', 'Include detailed dependency trees'],
    },
  };

  return toolPolicies[tool]?.[environment] || [];
}

/**
 * Get production-specific constraints
 */
export function getProductionConstraints(): string[] {
  return [
    'Use non-root users in containers',
    'Minimize container image size',
    'Use specific version tags (not :latest)',
    'Include health checks and readiness probes',
    'Set resource limits and requests',
  ];
}

/**
 * Extract category-based constraints from policy rules
 */
export function getCategoryConstraints(
  policy: Policy,
  tool: string,
  environment: string,
): string[] {
  return policy.rules
    .filter((rule) => {
      // Check if rule applies to this tool/category
      if (rule.category === 'security' && environment === 'production') {
        return true;
      }
      if (rule.category === 'quality' && tool.includes('generate')) {
        return true;
      }
      return false;
    })
    .sort((a, b) => b.priority - a.priority)
    .slice(0, 3) // Take top 3 relevant rules
    .map((rule) => rule.description)
    .filter((desc): desc is string => Boolean(desc));
}

/**
 * Build complete constraint set for a tool/environment combination
 */
export interface ConstraintContext {
  policy: Policy;
  tool: string;
  environment: string;
  tags?: string[];
}

export function buildConstraints(context: ConstraintContext): string[] {
  const builder = new ConstraintBuilder();
  const { policy, tool, environment } = context;

  // Extract environment-specific defaults
  const envConfig = policy.environments?.[environment];
  if (envConfig?.defaults) {
    builder.addAll(extractEnvironmentConstraints(envConfig.defaults));
  }

  // Add tool-specific constraints
  builder.addAll(getToolConstraints(tool, environment));

  // Add production constraints if applicable
  if (environment === 'production') {
    builder.addAll(getProductionConstraints());
  }

  // Add category-based rules from policy
  builder.addAll(getCategoryConstraints(policy, tool, environment));

  return builder.build();
}

/**
 * Format constraints as a prompt section
 */
export function formatConstraintsPrompt(constraints: string[]): string {
  if (constraints.length === 0) {
    return '';
  }

  return `
## Policy Constraints
You must follow these organizational policies and best practices:
${constraints.map((c) => `- ${c}`).join('\n')}`;
}
