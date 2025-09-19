/**
 * Policy Configuration Migration Tests
 *
 * Comprehensive test suite to ensure:
 * - Unified policy configuration maintains backward compatibility
 * - Migration script works correctly
 * - Config resolver handles both old and new formats
 * - All existing behavior is preserved
 */

import { describe, it, expect, beforeEach, afterEach } from '@jest/globals';
import { readFileSync, writeFileSync, existsSync, mkdirSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { parse as yamlParse, stringify as yamlStringify } from 'yaml';

// Import the functions we're testing
import {
  initializeResolver,
  getConfig,
  getEffectiveConfig,
  getRuleWeights,
  getStrategySelection,
  getToolDefaults,
  isUnifiedPolicyEnabled,
  clearCache,
} from '../../src/config/resolver';
import { PolicyMigrator } from '../../scripts/migrate-policy';

describe('Policy Configuration Migration', () => {
  let tempDir: string;
  let originalConfigDir: string;

  beforeEach(() => {
    // Create a temporary directory for test files
    tempDir = join(tmpdir(), `policy-test-${Date.now()}`);
    mkdirSync(tempDir, { recursive: true });

    // Store original config directory
    originalConfigDir = process.cwd();

    // Clear any existing config cache
    clearCache();
  });

  afterEach(() => {
    // Clean up temp directory
    if (existsSync(tempDir)) {
      rmSync(tempDir, { recursive: true, force: true });
    }

    // Clear config cache
    clearCache();
  });

  describe('Legacy Policy Format Support', () => {
    it('should handle legacy JSON policy format', () => {
      const legacyPolicy = {
        maxTokens: 8192,
        maxCost: 10.0,
        forbiddenModels: ['gpt-4'],
        allowedModels: ['claude-3-haiku', 'claude-3-sonnet'],
        timeoutMs: 30000,
      };

      const policyFile = join(tempDir, 'legacy-policy.json');
      writeFileSync(policyFile, JSON.stringify(legacyPolicy, null, 2));

      initializeResolver({ policyFile });

      const config = getConfig();
      expect(config.legacyPolicy).toBeDefined();
      expect(config.legacyPolicy?.maxTokens).toBe(8192);
      expect(config.legacyPolicy?.forbiddenModels).toContain('gpt-4');
    });

    it('should apply legacy policy constraints', () => {
      const legacyPolicy = {
        maxTokens: 4000,
        forbiddenModels: ['gpt-4'],
        allowedModels: ['claude-3-haiku'],
        timeoutMs: 20000,
      };

      const policyFile = join(tempDir, 'legacy-policy.json');
      writeFileSync(policyFile, JSON.stringify(legacyPolicy, null, 2));

      initializeResolver({ policyFile });

      const effectiveConfig = getEffectiveConfig('test-tool', {
        toolStrategy: {
          model: 'gpt-4', // This should be blocked
          maxTokens: 8000, // This should be clamped
          timeout: 40000, // This should be clamped
        },
      });

      expect(effectiveConfig.model).toBe('claude-3-haiku'); // Fallback to allowed model
      expect(effectiveConfig.parameters.maxTokens).toBe(4000); // Clamped
      expect(effectiveConfig.timeouts.sampling).toBe(20000); // Clamped
    });
  });

  describe('Unified Policy Format Support', () => {
    it('should handle unified YAML policy format', () => {
      const unifiedPolicy = {
        version: '2.0.0',
        metadata: {
          description: 'Test unified policy',
          created: '2024-09-18',
          author: 'test',
        },
        weights: {
          global_categories: {
            security: 1.2,
            performance: 1.1,
            quality: 1.0,
          },
          content_types: {
            dockerfile: {
              security: 1.3,
              performance: 1.2,
            },
          },
        },
        rules: {
          dockerfile: {
            base_score: 30,
            max_score: 100,
            timeout_ms: 2000,
            security: [
              {
                name: 'test_rule',
                matcher: { type: 'function', function: 'testFunction' },
                points: 25,
                weight: 1.3,
                description: 'Test security rule',
              },
            ],
          },
        },
        strategies: {
          dockerfile: ['basic', 'intermediate', 'advanced'],
        },
        strategy_selection: {
          dockerfile: {
            conditions: [
              { key: 'complexity', value: 'high', strategy_index: 2 },
            ],
            default_strategy_index: 1,
          },
        },
        env_overrides: {
          development: {
            weights: {
              content_types: {
                dockerfile: {
                  security: 1.0, // Less strict in dev
                },
              },
            },
          },
        },
        tool_defaults: {
          'generate-dockerfile': {
            content_type: 'dockerfile',
            environment: 'development',
          },
        },
        global_penalties: [],
        schema_version: '2.0.0',
      };

      const policyFile = join(tempDir, 'unified-policy.yaml');
      writeFileSync(policyFile, yamlStringify(unifiedPolicy));

      initializeResolver({ policyFile });

      const config = getConfig();
      expect(config.unifiedPolicy).toBeDefined();
      expect(config.unifiedPolicy?.version).toBe('2.0.0');
      expect(isUnifiedPolicyEnabled()).toBe(true);
    });

    it('should resolve rule weights correctly', () => {
      const unifiedPolicy = {
        version: '2.0.0',
        metadata: { description: 'Test', created: '2024-09-18', author: 'test' },
        weights: {
          global_categories: { security: 1.2, performance: 1.1, quality: 1.0 },
          content_types: {
            dockerfile: { security: 1.3, performance: 1.2 },
          },
        },
        rules: {},
        strategies: {},
        strategy_selection: {},
        env_overrides: {
          production: {
            weights: {
              content_types: {
                dockerfile: { security: 1.5 }, // Higher in production
              },
            },
          },
        },
        tool_defaults: {},
        global_penalties: [],
        schema_version: '2.0.0',
      };

      const policyFile = join(tempDir, 'unified-policy.yaml');
      writeFileSync(policyFile, yamlStringify(unifiedPolicy));

      initializeResolver({ policyFile });

      // Test development weights
      const devWeights = getRuleWeights('dockerfile', 'development');
      expect(devWeights).toEqual({
        security: 1.3, // Content-type specific
        performance: 1.2, // Content-type specific
        quality: 1.0, // Global fallback
      });

      // Test production weights (with override)
      const prodWeights = getRuleWeights('dockerfile', 'production');
      expect(prodWeights).toEqual({
        security: 1.5, // Environment override
        performance: 1.2, // Content-type specific
        quality: 1.0, // Global fallback
      });
    });

    it('should select strategies correctly based on context', () => {
      const unifiedPolicy = {
        version: '2.0.0',
        metadata: { description: 'Test', created: '2024-09-18', author: 'test' },
        weights: { global_categories: {}, content_types: {} },
        rules: {},
        strategies: {
          dockerfile: ['basic', 'intermediate', 'advanced'],
        },
        strategy_selection: {
          dockerfile: {
            conditions: [
              { key: 'complexity', value: 'high', strategy_index: 2 },
              { key: 'environment', value: 'production', strategy_index: 2 },
            ],
            default_strategy_index: 0,
          },
        },
        env_overrides: {},
        tool_defaults: {},
        global_penalties: [],
        schema_version: '2.0.0',
      };

      const policyFile = join(tempDir, 'unified-policy.yaml');
      writeFileSync(policyFile, yamlStringify(unifiedPolicy));

      initializeResolver({ policyFile });

      // Test default strategy
      const defaultStrategy = getStrategySelection('dockerfile', {});
      expect(defaultStrategy).toEqual({
        strategy: 'basic',
        index: 0,
      });

      // Test high complexity strategy
      const complexStrategy = getStrategySelection('dockerfile', { complexity: 'high' });
      expect(complexStrategy).toEqual({
        strategy: 'advanced',
        index: 2,
      });

      // Test production strategy
      const prodStrategy = getStrategySelection('dockerfile', { environment: 'production' });
      expect(prodStrategy).toEqual({
        strategy: 'advanced',
        index: 2,
      });
    });

    it('should get tool defaults correctly', () => {
      const unifiedPolicy = {
        version: '2.0.0',
        metadata: { description: 'Test', created: '2024-09-18', author: 'test' },
        weights: { global_categories: {}, content_types: {} },
        rules: {},
        strategies: {},
        strategy_selection: {},
        env_overrides: {},
        tool_defaults: {
          'generate-dockerfile': {
            content_type: 'dockerfile',
            environment: 'development',
            strategy_override: null,
          },
          'fix-dockerfile': {
            content_type: 'dockerfile',
            environment: 'production',
            strategy_override: 2,
          },
        },
        global_penalties: [],
        schema_version: '2.0.0',
      };

      const policyFile = join(tempDir, 'unified-policy.yaml');
      writeFileSync(policyFile, yamlStringify(unifiedPolicy));

      initializeResolver({ policyFile });

      const dockerfileDefaults = getToolDefaults('generate-dockerfile');
      expect(dockerfileDefaults).toEqual({
        content_type: 'dockerfile',
        environment: 'development',
        strategy_override: null,
      });

      const fixDefaults = getToolDefaults('fix-dockerfile');
      expect(fixDefaults).toEqual({
        content_type: 'dockerfile',
        environment: 'production',
        strategy_override: 2,
      });

      const unknownDefaults = getToolDefaults('unknown-tool');
      expect(unknownDefaults).toBeNull();
    });
  });

  describe('getEffectiveConfig with Unified Policy', () => {
    it('should resolve effective config with unified policy', () => {
      const unifiedPolicy = {
        version: '2.0.0',
        metadata: { description: 'Test', created: '2024-09-18', author: 'test' },
        weights: {
          global_categories: { security: 1.2, performance: 1.1 },
          content_types: {
            dockerfile: { security: 1.3, performance: 1.2 },
          },
        },
        rules: {
          dockerfile: {
            base_score: 30,
            max_score: 100,
            timeout_ms: 2000,
          },
        },
        strategies: {},
        strategy_selection: {},
        env_overrides: {
          production: {
            sampling: { timeout_ms: 5000 },
          },
        },
        tool_defaults: {
          'generate-dockerfile': {
            content_type: 'dockerfile',
            environment: 'development',
          },
        },
        global_penalties: [],
        schema_version: '2.0.0',
      };

      const policyFile = join(tempDir, 'unified-policy.yaml');
      writeFileSync(policyFile, yamlStringify(unifiedPolicy));

      initializeResolver({ policyFile });

      const effectiveConfig = getEffectiveConfig('generate-dockerfile', {
        contentType: 'dockerfile',
        environment: 'production',
      });

      expect(effectiveConfig.weights).toEqual({
        security: 1.3,
        performance: 1.2,
      });
      expect(effectiveConfig.rules).toEqual({
        base_score: 30,
        max_score: 100,
        timeout_ms: 2000,
      });
      expect(effectiveConfig.timeouts.sampling).toBe(5000); // Environment override applied
    });
  });

  describe('Migration Script', () => {
    it('should migrate old configuration to unified format', async () => {
      // Create mock old configuration files
      const scoringDir = join(tempDir, 'config', 'sampling', 'scoring');
      const envDir = join(tempDir, 'config', 'sampling', 'environments');
      mkdirSync(scoringDir, { recursive: true });
      mkdirSync(envDir, { recursive: true });

      // Mock dockerfile.yml
      const dockerfileConfig = {
        name: 'dockerfile',
        version: '1.0.0',
        base_score: 30,
        max_score: 100,
        timeout_ms: 2000,
        category_weights: {
          security: 1.2,
          performance: 1.1,
          quality: 1.0,
        },
        rules: {
          security: [
            {
              name: 'non_root_user',
              matcher: { type: 'function', function: 'hasNonRootUser' },
              points: 25,
              weight: 1.3,
              description: 'Runs container as non-root user',
            },
          ],
        },
      };

      writeFileSync(join(scoringDir, 'dockerfile.yml'), yamlStringify(dockerfileConfig));

      // Mock strategies.yml
      const strategiesConfig = {
        version: '1.0.0',
        strategies: {
          dockerfile: ['basic', 'intermediate', 'advanced'],
        },
        selection_rules: {
          dockerfile: {
            conditions: [{ key: 'complexity', value: 'high', strategy_index: 2 }],
            default_strategy_index: 1,
          },
        },
      };

      writeFileSync(join(tempDir, 'config', 'sampling', 'strategies.yml'), yamlStringify(strategiesConfig));

      // Mock development.yml
      const devConfig = {
        environment: 'development',
        overrides: {
          scoring: {
            dockerfile: {
              category_weights: {
                security: 1.0, // Less strict in dev
              },
            },
          },
        },
      };

      writeFileSync(join(envDir, 'development.yml'), yamlStringify(devConfig));

      // Run migration
      const migrator = new PolicyMigrator(
        join(tempDir, 'config'),
        join(tempDir, 'config', 'policy.yaml')
      );

      await migrator.migrate({ dryRun: false, createBackup: false, validate: true });

      // Verify policy was created
      const unifiedPolicyPath = join(tempDir, 'config', 'policy.yaml');
      expect(existsSync(unifiedPolicyPath)).toBe(true);

      const unifiedPolicy = yamlParse(readFileSync(unifiedPolicyPath, 'utf8'));
      expect(unifiedPolicy.version).toBe('2.0.0');
      expect(unifiedPolicy.weights.content_types.dockerfile.security).toBe(1.2);
      expect(unifiedPolicy.rules.dockerfile.base_score).toBe(30);
      expect(unifiedPolicy.strategies.dockerfile).toEqual(['basic', 'intermediate', 'advanced']);
      expect(unifiedPolicy.env_overrides.development).toBeDefined();
    });

    it('should validate migrated configuration', async () => {
      // Create a minimal valid old configuration
      const configDir = join(tempDir, 'config');
      mkdirSync(join(configDir, 'sampling', 'scoring'), { recursive: true });
      mkdirSync(join(configDir, 'sampling', 'environments'), { recursive: true });

      const genericConfig = {
        name: 'generic',
        version: '1.0.0',
        base_score: 40,
        max_score: 100,
        timeout_ms: 1500,
        category_weights: { quality: 1.0 },
        rules: { quality: [] },
      };

      writeFileSync(
        join(configDir, 'sampling', 'scoring', 'generic.yml'),
        yamlStringify(genericConfig)
      );

      const strategiesConfig = {
        version: '1.0.0',
        strategies: { generic: ['basic'] },
        selection_rules: { generic: { conditions: [], default_strategy_index: 0 } },
      };

      writeFileSync(
        join(configDir, 'sampling', 'strategies.yml'),
        yamlStringify(strategiesConfig)
      );

      const migrator = new PolicyMigrator(configDir, join(configDir, 'policy.yaml'));

      // Should not throw
      expect(async () => {
        await migrator.migrate({ dryRun: false, createBackup: false, validate: true });
      }).not.toThrow();
    });
  });

  describe('Backward Compatibility', () => {
    it('should maintain existing behavior when no policy is provided', () => {
      initializeResolver();

      const config = getConfig();
      expect(config.unifiedPolicy).toBeUndefined();
      expect(config.legacyPolicy).toBeUndefined();
      expect(isUnifiedPolicyEnabled()).toBe(false);

      // Should return default effective config
      const effectiveConfig = getEffectiveConfig('test-tool');
      expect(effectiveConfig.model).toBeDefined();
      expect(effectiveConfig.parameters).toBeDefined();
      expect(effectiveConfig.timeouts).toBeDefined();
    });

    it('should handle missing policy file gracefully', () => {
      const nonExistentFile = join(tempDir, 'nonexistent.yaml');

      // Should not throw
      expect(() => {
        initializeResolver({ policyFile: nonExistentFile });
      }).not.toThrow();

      const config = getConfig();
      expect(config.unifiedPolicy).toBeUndefined();
      expect(config.legacyPolicy).toBeUndefined();
    });

    it('should handle malformed policy file gracefully', () => {
      const malformedFile = join(tempDir, 'malformed.yaml');
      writeFileSync(malformedFile, 'invalid: yaml: content: [');

      // Should not throw
      expect(() => {
        initializeResolver({ policyFile: malformedFile });
      }).not.toThrow();

      const config = getConfig();
      expect(config.unifiedPolicy).toBeUndefined();
      expect(config.legacyPolicy).toBeUndefined();
    });
  });

  describe('Performance and Caching', () => {
    it('should cache configuration and return same instance', () => {
      const unifiedPolicy = {
        version: '2.0.0',
        metadata: { description: 'Test', created: '2024-09-18', author: 'test' },
        weights: { global_categories: {}, content_types: {} },
        rules: {},
        strategies: {},
        strategy_selection: {},
        env_overrides: {},
        tool_defaults: {},
        global_penalties: [],
        schema_version: '2.0.0',
      };

      const policyFile = join(tempDir, 'unified-policy.yaml');
      writeFileSync(policyFile, yamlStringify(unifiedPolicy));

      initializeResolver({ policyFile });

      const config1 = getConfig();
      const config2 = getConfig();

      expect(config1).toBe(config2); // Same instance
    });

    it('should reinitialize when cache is cleared', () => {
      const unifiedPolicy = {
        version: '2.0.0',
        metadata: { description: 'Test', created: '2024-09-18', author: 'test' },
        weights: { global_categories: {}, content_types: {} },
        rules: {},
        strategies: {},
        strategy_selection: {},
        env_overrides: {},
        tool_defaults: {},
        global_penalties: [],
        schema_version: '2.0.0',
      };

      const policyFile = join(tempDir, 'unified-policy.yaml');
      writeFileSync(policyFile, yamlStringify(unifiedPolicy));

      initializeResolver({ policyFile });
      const config1 = getConfig();

      clearCache();
      const config2 = getConfig(); // Should reinitialize with defaults

      expect(config1).not.toBe(config2);
    });
  });
});