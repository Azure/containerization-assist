/**
 * Policy Schema Fix Tests
 * Validates that the category-based YAML structure is correctly parsed and transformed
 */

import { describe, it, expect, beforeAll } from '@jest/globals';
import { join } from 'node:path';
import {
  loadPolicy,
  getRuleWeights,
  getPolicyRules,
  validatePolicyFile,
} from '@config/policy';

const POLICY_PATH = join(__dirname, '../../../config/policy.yaml');

describe('Policy Schema Fix - Category to Matchers Transformation', () => {
  let policy: any;

  beforeAll(() => {
    try {
      policy = loadPolicy(POLICY_PATH, 'production'); // Use production to get base values
    } catch (error) {
      console.error('Failed to load policy:', error);
      throw error;
    }
  });

  describe('Category-based Rules Flattening', () => {
    it('should successfully load and transform the policy', () => {
      expect(policy).toBeDefined();
      expect(policy.raw.version).toBe('2.0.0');
    });

    it('should correctly flatten category rules to matchers array', () => {
      const dockerRules = policy.rules.dockerfile;

      // Should have base properties
      expect(dockerRules.base_score).toBe(30);
      expect(dockerRules.max_score).toBe(100);
      expect(dockerRules.timeout_ms).toBe(2000);

      // Should have flattened matchers array, not category objects
      expect(dockerRules.matchers).toBeDefined();
      expect(Array.isArray(dockerRules.matchers)).toBe(true);

      // Should NOT have category properties at the rule level
      expect(dockerRules.security).toBeUndefined();
      expect(dockerRules.performance).toBeUndefined();
      expect(dockerRules.quality).toBeUndefined();
    });

    it('should preserve all rules from all categories', () => {
      const dockerRules = policy.rules.dockerfile;

      // Check for specific rules from different categories
      const securityRules = dockerRules.matchers.filter((m: any) => m.category === 'security');
      const performanceRules = dockerRules.matchers.filter((m: any) => m.category === 'performance');
      const qualityRules = dockerRules.matchers.filter((m: any) => m.category === 'quality');

      expect(securityRules.length).toBeGreaterThan(0);
      expect(performanceRules.length).toBeGreaterThan(0);
      expect(qualityRules.length).toBeGreaterThan(0);

      // Verify specific rules exist
      expect(dockerRules.matchers.find((m: any) => m.name === 'non_root_user')).toBeDefined();
      expect(dockerRules.matchers.find((m: any) => m.name === 'multistage_build')).toBeDefined();
      expect(dockerRules.matchers.find((m: any) => m.name === 'proper_workdir')).toBeDefined();
    });

    it('should add category field to each matcher', () => {
      const dockerRules = policy.rules.dockerfile;

      // Every matcher should have a category field
      dockerRules.matchers.forEach((matcher: any) => {
        expect(matcher.category).toBeDefined();
        expect(['security', 'performance', 'quality', 'maintainability', 'efficiency', 'penalties'])
          .toContain(matcher.category);
      });
    });

    it('should preserve all matcher properties during transformation', () => {
      const dockerRules = policy.rules.dockerfile;

      // Check a specific rule with all properties
      const nonRootRule = dockerRules.matchers.find((m: any) => m.name === 'non_root_user');
      expect(nonRootRule).toMatchObject({
        name: 'non_root_user',
        type: 'function',
        function: 'hasNonRootUser',
        points: 25,
        weight: 1.3,
        description: 'Runs container as non-root user',
        category: 'security'
      });

      // Check a regex-based rule
      const distrolessRule = dockerRules.matchers.find((m: any) => m.name === 'distroless_base');
      expect(distrolessRule).toMatchObject({
        name: 'distroless_base',
        type: 'regex',
        pattern: 'FROM.*distroless',
        flags: 'i',
        points: 25,
        weight: 1.1,
        category: 'security'
      });
    });
  });

  describe('Rule Weight Calculation with Fixed Schema', () => {
    it('should calculate weights correctly using category and rule weights', () => {
      // Test development environment (which has overrides)
      const devWeights = getRuleWeights(policy, 'dockerfile', 'development');

      // In development environment:
      // non_root_user: security category (1.0) * rule weight (1.3) = 1.3 (actual policy values)
      expect(devWeights['non_root_user']).toBeCloseTo(1.3, 2);

      // multistage_build: performance category (0.9) * rule weight (1.0) = 0.9
      expect(devWeights['multistage_build']).toBeCloseTo(0.9, 2);

      // Test production environment (no overrides, uses base values)
      const prodWeights = getRuleWeights(policy, 'dockerfile', 'production');

      // In production environment:
      // non_root_user: security category (1.4) * rule weight (1.3) = 1.82 (actual policy values)
      expect(prodWeights['non_root_user']).toBeCloseTo(1.82, 2);

      // proper_workdir: quality category (1.0) * rule weight (0.8) = 0.8
      expect(devWeights['proper_workdir']).toBeCloseTo(0.8, 2);
    });
  });

  describe('All Content Types', () => {
    it('should correctly transform kubernetes rules', () => {
      const k8sRules = policy.rules.kubernetes;

      expect(k8sRules).toBeDefined();
      expect(k8sRules.matchers).toBeDefined();
      expect(Array.isArray(k8sRules.matchers)).toBe(true);

      // Should NOT have category properties
      expect(k8sRules.security).toBeUndefined();
      expect(k8sRules.performance).toBeUndefined();

      // Should have matchers with category field
      if (k8sRules.matchers.length > 0) {
        k8sRules.matchers.forEach((matcher: any) => {
          expect(matcher.category).toBeDefined();
        });
      }
    });

    it('should correctly transform generic rules', () => {
      const genericRules = policy.rules.generic;

      expect(genericRules).toBeDefined();
      expect(genericRules.matchers).toBeDefined();
      expect(Array.isArray(genericRules.matchers)).toBe(true);

      // Should NOT have category properties
      expect(genericRules.security).toBeUndefined();
      expect(genericRules.performance).toBeUndefined();
    });
  });

  describe('Policy Validation', () => {
    it('should validate the actual policy.yaml file', () => {
      const result = validatePolicyFile(POLICY_PATH);
      expect(result.valid).toBe(true);
      expect(result.error).toBeUndefined();
    });

    it('should detect when rules are not properly structured', () => {
      // Test with an invalid structure
      const invalidPath = '/tmp/test-invalid-policy.yaml';
      const fs = require('node:fs');

      // Write an invalid policy with old structure
      const invalidPolicy = `
version: "2.0.0"
metadata:
  description: "Test"
  created: "2024-01-01"
  author: "test"
weights:
  global_categories:
    security: 1.0
  content_types:
    dockerfile:
      security: 1.0
rules:
  dockerfile:
    base_score: 30
    max_score: 100
    timeout_ms: 2000
    matchers:  # Wrong: expecting category objects, not flat matchers
      - name: "test"
        points: 10
strategies:
  dockerfile: ["default"]
strategy_selection:
  dockerfile:
    default_strategy_index: 0
env_overrides: {}
tool_defaults: {}
schema_version: "2.0.0"
`;

      fs.writeFileSync(invalidPath, invalidPolicy, 'utf8');

      const result = validatePolicyFile(invalidPath);
      expect(result.valid).toBe(false);
      expect(result.error).toBeDefined();

      // Clean up
      fs.unlinkSync(invalidPath);
    });
  });

  describe('Backwards Compatibility', () => {
    it('should handle empty categories gracefully', () => {
      const rules = getPolicyRules(policy, 'dockerfile');

      // Even if a category had no rules, should still work
      expect(rules).toBeDefined();
      expect(rules?.matchers).toBeDefined();
      expect(Array.isArray(rules?.matchers)).toBe(true);
    });

    it('should handle missing optional fields', () => {
      const dockerRules = policy.rules.dockerfile;

      // Some matchers might not have all optional fields
      dockerRules.matchers.forEach((matcher: any) => {
        expect(matcher.name).toBeDefined();
        expect(matcher.points).toBeDefined();
        expect(matcher.weight).toBeDefined();
        expect(matcher.category).toBeDefined();
        // description is optional
      });
    });
  });
});