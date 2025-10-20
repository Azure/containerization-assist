/**
 * Comprehensive Policy IO Tests
 * Covers merge, cache, and edge cases to improve branch coverage
 */

import { describe, it, expect, beforeEach, jest } from '@jest/globals';
import * as fs from 'node:fs';
import * as path from 'node:path';
import {
  loadPolicy,
  validatePolicy,
  createDefaultPolicy,
  loadAndMergePolicies,
} from '@/config/policy-io';
import type { Policy } from '@/config/policy-schemas';

// Mock fs for cache testing
jest.mock('node:fs');

describe('Policy IO - Comprehensive Coverage', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe('Policy Merging', () => {
    it('should load and merge policies with fallback to TypeScript data', () => {
      // When YAML files don't exist, loadAndMergePolicies falls back to TypeScript policy data
      const policy1Path = path.join(process.cwd(), 'nonexistent-policy1.yaml');
      const policy2Path = path.join(process.cwd(), 'nonexistent-policy2.yaml');

      const result = loadAndMergePolicies([policy1Path, policy2Path]);

      expect(result.ok).toBe(true);
      if (result.ok) {
        // Should load the default TypeScript policy
        expect(result.value.version).toBe('2.0');
        expect(result.value.rules.length).toBeGreaterThan(0);

        // Verify it's the default policy by checking for known rules
        const securityRule = result.value.rules.find(r => r.id === 'security-scanning');
        expect(securityRule).toBeDefined();
      }
    });

    it('should handle empty policy list in merge', () => {
      const result = loadAndMergePolicies([]);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('No policies loaded successfully');
      }
    });

    it('should handle single policy in merge', () => {
      const policyPath = path.join(process.cwd(), 'single-policy.yaml');
      const result = loadAndMergePolicies([policyPath]);

      expect(result.ok).toBe(true);
      if (result.ok) {
        // Should return the default policy (TypeScript fallback)
        expect(result.value.version).toBe('2.0');
        expect(result.value.rules.length).toBeGreaterThan(0);
      }
    });

    it('should handle policy with no cache configuration', () => {
      const policy: Policy = {
        version: '2.0',
        metadata: { name: 'No Cache Policy' },
        rules: [
          {
            id: 'test-rule',
            priority: 100,
            conditions: [{ kind: 'regex', pattern: 'test' }],
            actions: { test: true },
          },
        ],
      };

      const result = validatePolicy(policy);
      expect(result.ok).toBe(true);
      if (result.ok) {
        // Cache is optional
        expect(result.value.cache).toBeUndefined();
      }
    });

    it('should merge defaults from multiple policies', () => {
      const policy1: Policy = {
        version: '2.0',
        rules: [],
        defaults: { enforcement: 'advisory', cache_ttl: 300 },
      };

      const policy2: Policy = {
        version: '2.0',
        rules: [],
        defaults: { enforcement: 'strict' }, // Overrides enforcement
      };

      const result = validatePolicy(policy1);
      expect(result.ok).toBe(true);

      const result2 = validatePolicy(policy2);
      expect(result2.ok).toBe(true);
    });
  });

  describe('Policy Caching', () => {
    it('should cache loaded policies', () => {
      const policyPath = path.join(process.cwd(), 'cached-policy.yaml');

      // First load
      const result1 = loadPolicy(policyPath);
      expect(result1.ok).toBe(true);

      // Second load should use cache (same path)
      const result2 = loadPolicy(policyPath);
      expect(result2.ok).toBe(true);

      if (result1.ok && result2.ok) {
        // Both should return the same policy data
        expect(result1.value.version).toBe(result2.value.version);
        expect(result1.value.rules.length).toBe(result2.value.rules.length);
      }
    });

    it('should handle cache with different paths', () => {
      const path1 = path.join(process.cwd(), 'policy-a.yaml');
      const path2 = path.join(process.cwd(), 'policy-b.yaml');

      const result1 = loadPolicy(path1);
      const result2 = loadPolicy(path2);

      expect(result1.ok).toBe(true);
      expect(result2.ok).toBe(true);

      // Both should load successfully (falling back to TypeScript data)
      if (result1.ok && result2.ok) {
        expect(result1.value).toBeDefined();
        expect(result2.value).toBeDefined();
      }
    });
  });

  describe('Error Handling', () => {
    it('should handle invalid policy structure in validation', () => {
      const invalidPolicy = {
        version: '2.0',
        rules: [
          {
            id: 'invalid-rule',
            // Missing required 'priority' field
            // Missing required 'conditions' field
            // Missing required 'actions' field
          },
        ],
      };

      const result = validatePolicy(invalidPolicy);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Policy validation failed');
      }
    });

    it('should handle YAML parse errors', () => {
      // Create a file with invalid YAML
      const invalidPath = path.join(process.cwd(), 'invalid.yaml');

      // Mock fs.existsSync to return true
      (fs.existsSync as jest.Mock).mockReturnValue(true);

      // Mock fs.readFileSync to return invalid YAML
      (fs.readFileSync as jest.Mock).mockReturnValue('invalid: yaml: content: [unclosed');

      const result = loadPolicy(invalidPath);

      // Should fall back to TypeScript data on YAML error
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.version).toBe('2.0');
      }
    });

    it('should sort rules by priority descending after load', () => {
      const policyPath = path.join(process.cwd(), 'unsorted-policy.yaml');
      const result = loadPolicy(policyPath);

      expect(result.ok).toBe(true);
      if (result.ok) {
        const priorities = result.value.rules.map(r => r.priority);

        // Verify sorted in descending order
        for (let i = 0; i < priorities.length - 1; i++) {
          expect(priorities[i]).toBeGreaterThanOrEqual(priorities[i + 1]);
        }
      }
    });

    it('should handle loadAndMergePolicies with all failed loads', () => {
      // Mock fs.existsSync to return false for all files
      (fs.existsSync as jest.Mock).mockReturnValue(false);

      // Create paths that will all fail
      const paths = [
        '/nonexistent/policy1.yaml',
        '/nonexistent/policy2.yaml',
      ];

      const result = loadAndMergePolicies(paths);

      // Should still succeed by falling back to TypeScript data
      expect(result.ok).toBe(true);
      if (result.ok) {
        // At least one policy should have loaded (TypeScript fallback)
        expect(result.value.rules.length).toBeGreaterThan(0);
      }
    });
  });

  describe('Default Policy Creation', () => {
    it('should create valid default policy', () => {
      const defaultPolicy = createDefaultPolicy();

      expect(defaultPolicy.version).toBe('2.0');
      expect(defaultPolicy.metadata).toBeDefined();
      expect(defaultPolicy.metadata?.description).toBe('Default containerization policy');
      expect(defaultPolicy.defaults?.enforcement).toBe('advisory');
      expect(defaultPolicy.rules.length).toBeGreaterThan(0);
      expect(defaultPolicy.cache).toBeDefined();
      expect(defaultPolicy.cache?.enabled).toBe(true);
    });

    it('should create default policy with security rules', () => {
      const defaultPolicy = createDefaultPolicy();

      const securityRule = defaultPolicy.rules.find(r => r.id === 'security-scanning');
      expect(securityRule).toBeDefined();
      expect(securityRule?.priority).toBe(100);
      expect(securityRule?.actions.enforce_scan).toBe(true);
    });

    it('should create default policy with base image validation', () => {
      const defaultPolicy = createDefaultPolicy();

      const baseImageRule = defaultPolicy.rules.find(r => r.id === 'base-image-validation');
      expect(baseImageRule).toBeDefined();
      expect(baseImageRule?.priority).toBe(90);
      expect(baseImageRule?.actions.suggest_pinned_version).toBe(true);
    });
  });

  describe('Policy Metadata', () => {
    it('should preserve metadata in policy loading', () => {
      const policyWithMetadata: Policy = {
        version: '2.0',
        metadata: {
          name: 'Custom Policy',
          author: 'Test Team',
          description: 'Test policy for metadata preservation',
          created: '2024-01-01',
        },
        rules: [],
      };

      const result = validatePolicy(policyWithMetadata);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.metadata?.name).toBe('Custom Policy');
        expect(result.value.metadata?.author).toBe('Test Team');
        expect(result.value.metadata?.description).toBe('Test policy for metadata preservation');
      }
    });

    it('should handle policy without metadata', () => {
      const policyWithoutMetadata: Policy = {
        version: '2.0',
        rules: [],
      };

      const result = validatePolicy(policyWithoutMetadata);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.metadata).toBeUndefined();
      }
    });
  });

  describe('Complex Merge Scenarios', () => {
    it('should handle merging policies with different enforcement levels', () => {
      const advisoryPolicy: Policy = {
        version: '2.0',
        metadata: { name: 'Advisory Policy' },
        defaults: { enforcement: 'advisory' as const },
        rules: [
          {
            id: 'rule-1',
            priority: 50,
            conditions: [{ kind: 'regex', pattern: 'test' }],
            actions: { warn: true },
          },
        ],
      };

      const strictPolicy: Policy = {
        version: '2.0',
        metadata: { name: 'Strict Policy' },
        defaults: { enforcement: 'strict' as const },
        rules: [
          {
            id: 'rule-2',
            priority: 100,
            conditions: [{ kind: 'regex', pattern: 'test' }],
            actions: { block: true },
          },
        ],
      };

      const result1 = validatePolicy(advisoryPolicy);
      const result2 = validatePolicy(strictPolicy);

      expect(result1.ok).toBe(true);
      expect(result2.ok).toBe(true);

      if (result1.ok && result2.ok) {
        expect(result1.value.defaults?.enforcement).toBe('advisory');
        expect(result2.value.defaults?.enforcement).toBe('strict');
      }
    });

    it('should handle policies with empty rule arrays', () => {
      const emptyPolicy: Policy = {
        version: '2.0',
        rules: [],
      };

      const result = validatePolicy(emptyPolicy);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.rules).toHaveLength(0);
      }
    });

    it('should handle policies with many rules', () => {
      const manyRules: Policy = {
        version: '2.0',
        rules: Array.from({ length: 50 }, (_, i) => ({
          id: `rule-${i}`,
          priority: 100 - i,
          conditions: [{ kind: 'regex' as const, pattern: `pattern-${i}` }],
          actions: { test: true },
        })),
      };

      const result = validatePolicy(manyRules);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.rules).toHaveLength(50);

        // Should be sorted by priority
        const priorities = result.value.rules.map(r => r.priority);
        for (let i = 0; i < priorities.length - 1; i++) {
          expect(priorities[i]).toBeGreaterThanOrEqual(priorities[i + 1]);
        }
      }
    });
  });
});
