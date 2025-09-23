/**
 * Policy Configuration
 * TypeScript version of the policy configuration
 */

import type { Policy } from '../src/config/policy-schemas';

export const policyConfig: Policy = {
  version: '2.0',

  metadata: {
    created: '2024-01-15',
    author: 'containerization-assist-team',
    description: 'Unified containerization policy for security, quality, and compliance',
  },

  defaults: {
    cache_ttl: 300,
    enforcement: 'advisory' as const,
  },

  rules: [
    // Security Rules (Priority 100-199)
    {
      id: 'security-scanning',
      category: 'security',
      priority: 100,
      description: 'Enforce security scanning for sensitive base images',
      conditions: [
        {
          kind: 'regex',
          pattern: 'FROM .*(alpine|distroless|scratch)',
        },
      ],
      actions: {
        enforce_scan: true,
        block_on_critical: true,
        scan_type: 'trivy',
      },
    },
    {
      id: 'vulnerability-prevention',
      category: 'security',
      priority: 95,
      description: 'Prevent deployment of images with known vulnerabilities',
      conditions: [
        {
          kind: 'function',
          name: 'hasVulnerabilities',
          args: [['HIGH', 'CRITICAL']],
        },
      ],
      actions: {
        block_deployment: true,
        require_approval: true,
        notify_security: true,
      },
    },

    // Quality Rules (Priority 80-99)
    {
      id: 'base-image-validation',
      category: 'quality',
      priority: 90,
      description: 'Ensure base images use pinned versions',
      conditions: [
        {
          kind: 'function',
          name: 'hasPattern',
          args: ['FROM.*:latest'],
        },
      ],
      actions: {
        suggest_pinned_version: true,
        severity: 'warning',
      },
    },
    {
      id: 'layer-optimization',
      category: 'quality',
      priority: 85,
      description: 'Optimize Docker layers for better caching',
      conditions: [
        {
          kind: 'regex',
          pattern: 'RUN.*&&.*&&.*&&.*&&',
          count_threshold: 3,
        },
      ],
      actions: {
        suggest_layer_split: true,
        max_layers: 50,
      },
    },
    {
      id: 'non-root-user',
      category: 'security',
      priority: 88,
      description: 'Enforce non-root user in containers',
      conditions: [
        {
          kind: 'regex',
          pattern: 'USER root$',
          flags: 'm',
        },
      ],
      actions: {
        require_non_root: true,
        suggest_user: 'app',
        severity: 'error',
      },
    },

    // Performance Rules (Priority 60-79)
    {
      id: 'multi-stage-builds',
      category: 'performance',
      priority: 75,
      description: 'Recommend multi-stage builds for compiled languages',
      conditions: [
        {
          kind: 'function',
          name: 'hasPattern',
          args: ['(golang|rust|java|dotnet|node)'],
        },
      ],
      actions: {
        suggest_multistage: true,
        provide_template: true,
      },
    },
    {
      id: 'cache-mount-optimization',
      category: 'performance',
      priority: 70,
      description: 'Use BuildKit cache mounts for package managers',
      conditions: [
        {
          kind: 'regex',
          pattern: '(npm install|pip install|go mod download|cargo build)',
        },
      ],
      actions: {
        suggest_cache_mount: true,
        buildkit_required: true,
      },
    },

    // Compliance Rules (Priority 40-59)
    {
      id: 'label-standards',
      category: 'compliance',
      priority: 50,
      description: 'Ensure required labels are present',
      conditions: [
        {
          kind: 'regex',
          pattern: '^LABEL',
          flags: 'm',
        },
      ],
      actions: {
        required_labels: [
          'org.opencontainers.image.source',
          'org.opencontainers.image.version',
          'org.opencontainers.image.description',
        ],
        validate_oci_compliance: true,
      },
    },
    {
      id: 'secret-prevention',
      category: 'security',
      priority: 92,
      description: 'Prevent secrets from being included in images',
      conditions: [
        {
          kind: 'regex',
          pattern: '(API_KEY|SECRET|PASSWORD|TOKEN)\\s*=',
          flags: 'i',
        },
      ],
      actions: {
        block_build: true,
        severity: 'critical',
        suggest_secret_manager: true,
      },
    },
  ],

  // Environment-specific overrides
  environments: {
    development: {
      defaults: {
        cache_ttl: 600,
        enforcement: 'lenient' as const,
      },
      overrides: [
        {
          rule_id: 'security-scanning',
          actions: {
            block_on_critical: false,
            block_on_high: false,
          },
        },
        {
          rule_id: 'vulnerability-prevention',
          enabled: false,
        },
      ],
    },

    staging: {
      defaults: {
        cache_ttl: 300,
        enforcement: 'advisory' as const,
      },
      overrides: [
        {
          rule_id: 'security-scanning',
          actions: {
            block_on_critical: true,
            block_on_high: false,
          },
        },
        {
          rule_id: 'base-image-validation',
          priority: 95,
        },
      ],
    },

    production: {
      defaults: {
        cache_ttl: 120,
        enforcement: 'strict' as const,
      },
      overrides: [
        {
          rule_id: 'security-scanning',
          priority: 200,
          actions: {
            block_on_critical: true,
            block_on_high: true,
            block_on_medium: true,
          },
        },
        {
          rule_id: 'vulnerability-prevention',
          actions: {
            block_deployment: true,
            block_on_medium: true,
          },
        },
        {
          rule_id: 'non-root-user',
          actions: {
            require_non_root: true,
            severity: 'critical',
          },
        },
        {
          rule_id: 'secret-prevention',
          priority: 195,
          actions: {
            block_build: true,
            audit_log: true,
          },
        },
      ],
    },
  },

  // Cache configuration
  cache: {
    enabled: true,
    ttl: 300,
  },
};