/**
 * Policy Constraints (functional, minimal)
 */
import type { Policy, EnvironmentDefaults } from './policy-schemas';

type Str = string;
type ConstraintList = Str[];

const add = (arr: ConstraintList, v?: Str | null): ConstraintList => (v ? arr.concat(v) : arr);
const addAll = (arr: ConstraintList, vs?: Str[]): ConstraintList =>
  vs?.length ? arr.concat(vs) : arr;

/** Extract constraints from environment defaults */
export function extractEnvironmentConstraints(defaults: EnvironmentDefaults): ConstraintList {
  let out: ConstraintList = [];

  if (defaults.allowedBaseImages?.length) {
    out = add(
      out,
      `IMPORTANT: Only use these base images: ${defaults.allowedBaseImages.join(', ')}`,
    );
  }

  if (defaults.registries?.allowed?.length) {
    out = add(
      out,
      `Container registries must be one of: ${defaults.registries.allowed.join(', ')}`,
    );
  }
  if (defaults.registries?.blocked?.length) {
    out = add(out, `Never use these registries: ${defaults.registries.blocked.join(', ')}`);
  }

  if (defaults.security?.scanners?.required) {
    out = add(out, 'Include security scanning steps in all CI/CD pipelines');
    out = add(
      out,
      defaults.security.scanners.tools?.length
        ? `Use these security scanners: ${defaults.security.scanners.tools.join(', ')}`
        : null,
    );
  }
  if (defaults.security?.nonRootUser) out = add(out, 'Use non-root users in containers');
  if (defaults.security?.minimizeSize) out = add(out, 'Minimize container image size');

  if (defaults.resources?.limits) {
    const { cpu, memory } = defaults.resources.limits;
    if (cpu || memory)
      out = add(
        out,
        `Apply resource limits - CPU: ${cpu ?? 'default'}, Memory: ${memory ?? 'default'}`,
      );
  }
  if (defaults.resources?.requests) {
    const { cpu, memory } = defaults.resources.requests;
    if (cpu || memory)
      out = add(
        out,
        `Set resource requests - CPU: ${cpu ?? 'default'}, Memory: ${memory ?? 'default'}`,
      );
  }

  if (defaults.naming?.pattern) {
    out = add(out, `Follow naming pattern: ${defaults.naming.pattern}`);
    out = add(
      out,
      defaults.naming.examples?.length
        ? `Naming examples: ${defaults.naming.examples.join(', ')}`
        : null,
    );
  }

  return out;
}

/** Tool-specific constraints (unchanged semantics, flattened) */
const TOOL_POLICIES: Record<string, Record<string, string[]>> = {
  'generate-dockerfile': {
    production: [
      'Use multi-stage builds for production images',
      'Include LABEL metadata with version and maintainer',
      'Copy only necessary files (avoid COPY . .)',
      'Run security updates in separate layer',
    ],
    development: ['Include development tools for debugging', 'Use volume mounts for hot reloading'],
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

export function getToolConstraints(tool: string, environment: string): string[] {
  return TOOL_POLICIES[tool]?.[environment] ?? [];
}

export function getProductionConstraints(): string[] {
  return [
    'Use non-root users in containers',
    'Minimize container image size',
    'Use specific version tags (not :latest)',
    'Include health checks and readiness probes',
    'Set resource limits and requests',
  ];
}

export function getCategoryConstraints(
  policy: Policy,
  tool: string,
  environment: string,
): string[] {
  return policy.rules
    .filter(
      (r) =>
        (r.category === 'security' && environment === 'production') ||
        (r.category === 'quality' && tool.includes('generate')),
    )
    .sort((a, b) => b.priority - a.priority)
    .slice(0, 3)
    .map((r) => r.description)
    .filter((d): d is string => Boolean(d));
}

export interface ConstraintContext {
  policy: Policy;
  tool: string;
  environment: string;
  tags?: string[];
}

export function buildConstraints(ctx: ConstraintContext): string[] {
  const { policy, tool, environment } = ctx;
  let out: ConstraintList = [];

  const envCfg = policy.environments?.[environment]?.defaults;
  if (envCfg) out = addAll(out, extractEnvironmentConstraints(envCfg));

  out = addAll(out, getToolConstraints(tool, environment));
  if (environment === 'production') out = addAll(out, getProductionConstraints());
  out = addAll(out, getCategoryConstraints(policy, tool, environment));

  return out;
}

export function formatConstraintsPrompt(constraints: string[]): string {
  if (!constraints.length) return '';
  return `\n## Policy Constraints\nYou must follow these organizational policies and best practices:\n${constraints.map((c) => `- ${c}`).join('\n')}`;
}
