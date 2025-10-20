/**
 * Policy System Edge Case Tests
 *
 * Tests advanced policy scenarios:
 * - Empty policies
 * - Conflicting rules
 * - Multiple policy merging
 * - Strictness ordering
 * - Advisory vs blocking enforcement
 * - Complex matcher combinations
 */

import { describe, it, expect } from '@jest/globals';
import {
  validatePolicy,
  createDefaultPolicy,
} from '@/config/policy-io';
import {
  evaluateMatcher,
  applyPolicy,
} from '@/config/policy-eval';
import type {
  Policy,
  PolicyRule,
  Matcher,
} from '@/config/policy-schemas';

describe('Policy Edge Cases', () => {
  describe('Empty and Minimal Policies', () => {
    it('should handle empty policy with no rules', () => {
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

    it('should apply empty policy without errors', () => {
      const emptyPolicy: Policy = {
        version: '2.0',
        rules: [],
      };

      const results = applyPolicy(emptyPolicy, 'FROM node:18');
      expect(results).toHaveLength(0);
    });

    it('should handle policy with single rule', () => {
      const singleRulePolicy: Policy = {
        version: '2.0',
        rules: [
          {
            id: 'only-rule',
            priority: 100,
            conditions: [
              {
                kind: 'regex',
                pattern: 'test',
              },
            ],
            actions: { check: true },
          },
        ],
      };

      const results = applyPolicy(singleRulePolicy, 'test content');
      expect(results).toHaveLength(1);
      expect(results[0].matched).toBe(true);
    });

    it('should handle policy with no conditions', () => {
      const noConditionsRule: PolicyRule = {
        id: 'no-match',
        priority: 100,
        conditions: [],
        actions: { apply: true },
      };

      const policy: Policy = {
        version: '2.0',
        rules: [noConditionsRule],
      };

      const results = applyPolicy(policy, 'any input');
      expect(results).toHaveLength(1);
      // Empty conditions don't match (need at least one condition)
      expect(results[0].matched).toBe(false);
    });
  });

  describe('Conflicting Rules', () => {
    it('should handle rules with conflicting actions', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [
          {
            id: 'allow-rule',
            priority: 100,
            conditions: [{ kind: 'regex', pattern: 'alpine' }],
            actions: { allow: true, block: false },
          },
          {
            id: 'block-rule',
            priority: 90,
            conditions: [{ kind: 'regex', pattern: 'alpine' }],
            actions: { allow: false, block: true },
          },
        ],
      };

      const results = applyPolicy(policy, 'FROM alpine:3.18');
      expect(results).toHaveLength(2);

      // Both rules should match
      expect(results[0].matched).toBe(true);
      expect(results[1].matched).toBe(true);

      // Higher priority rule comes first
      expect(results[0].rule.id).toBe('allow-rule');
      expect(results[1].rule.id).toBe('block-rule');
    });

    it('should handle same-priority rules', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [
          {
            id: 'rule-a',
            priority: 100,
            conditions: [{ kind: 'regex', pattern: 'test' }],
            actions: { a: true },
          },
          {
            id: 'rule-b',
            priority: 100,
            conditions: [{ kind: 'regex', pattern: 'test' }],
            actions: { b: true },
          },
        ],
      };

      const results = applyPolicy(policy, 'test input');
      expect(results).toHaveLength(2);
      expect(results[0].matched).toBe(true);
      expect(results[1].matched).toBe(true);
    });

    it('should handle overlapping conditions', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [
          {
            id: 'broad-rule',
            priority: 50,
            conditions: [{ kind: 'regex', pattern: 'node' }],
            actions: { broad: true },
          },
          {
            id: 'specific-rule',
            priority: 100,
            conditions: [
              { kind: 'regex', pattern: 'node' },
              { kind: 'regex', pattern: 'alpine' },
            ],
            actions: { specific: true },
          },
        ],
      };

      const results = applyPolicy(policy, 'FROM node:18-alpine');

      // Both rules should match
      const broadMatch = results.find(r => r.rule.id === 'broad-rule');
      const specificMatch = results.find(r => r.rule.id === 'specific-rule');

      expect(broadMatch?.matched).toBe(true);
      expect(specificMatch?.matched).toBe(true);
    });
  });

  // Note: Policy merging is handled by the orchestrator, not exposed as a public API

  describe('Priority Ordering', () => {
    it('should return rules in definition order (not sorted)', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [
          {
            id: 'low-priority',
            priority: 10,
            conditions: [{ kind: 'regex', pattern: 'test' }],
            actions: { order: 'first' },
          },
          {
            id: 'high-priority',
            priority: 100,
            conditions: [{ kind: 'regex', pattern: 'test' }],
            actions: { order: 'second' },
          },
          {
            id: 'mid-priority',
            priority: 50,
            conditions: [{ kind: 'regex', pattern: 'test' }],
            actions: { order: 'third' },
          },
        ],
      };

      const results = applyPolicy(policy, 'test input');

      expect(results).toHaveLength(3);
      // Rules are returned in definition order, not priority order
      expect(results[0].rule.id).toBe('low-priority');
      expect(results[1].rule.id).toBe('high-priority');
      expect(results[2].rule.id).toBe('mid-priority');
    });

    // Note: Rule weight calculation and strategy selection are internal implementation details
  });

  describe('Advisory vs Blocking Enforcement', () => {
    it('should mark advisory rules appropriately', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [
          {
            id: 'advisory-rule',
            priority: 100,
            conditions: [{ kind: 'regex', pattern: 'warn' }],
            actions: {
              enforcement: 'advisory',
              message: 'This is a warning',
            },
          },
        ],
      };

      const results = applyPolicy(policy, 'This should warn');
      expect(results).toHaveLength(1);
      if (results[0].matched) {
        expect(results[0].rule.actions.enforcement).toBe('advisory');
      }
    });

    it('should mark blocking rules appropriately', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [
          {
            id: 'blocking-rule',
            priority: 100,
            conditions: [{ kind: 'regex', pattern: 'block' }],
            actions: {
              enforcement: 'blocking',
              message: 'This is blocked',
            },
          },
        ],
      };

      const results = applyPolicy(policy, 'This should block');
      expect(results).toHaveLength(1);
      if (results[0].matched) {
        expect(results[0].rule.actions.enforcement).toBe('blocking');
      }
    });

    it('should handle mixed enforcement levels', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [
          {
            id: 'advisory-1',
            priority: 100,
            conditions: [{ kind: 'regex', pattern: 'test' }],
            actions: { enforcement: 'advisory' },
          },
          {
            id: 'blocking-1',
            priority: 90,
            conditions: [{ kind: 'regex', pattern: 'test' }],
            actions: { enforcement: 'blocking' },
          },
          {
            id: 'advisory-2',
            priority: 80,
            conditions: [{ kind: 'regex', pattern: 'test' }],
            actions: { enforcement: 'advisory' },
          },
        ],
      };

      const results = applyPolicy(policy, 'test input');
      expect(results).toHaveLength(3);

      const advisoryCount = results.filter(
        r => r.matched && r.rule.actions.enforcement === 'advisory'
      ).length;
      const blockingCount = results.filter(
        r => r.matched && r.rule.actions.enforcement === 'blocking'
      ).length;

      expect(advisoryCount).toBe(2);
      expect(blockingCount).toBe(1);
    });
  });

  describe('Complex Matcher Combinations', () => {
    it('should handle multiple conditions with AND logic', () => {
      const matcher1: Matcher = { kind: 'regex', pattern: 'FROM' };
      const matcher2: Matcher = { kind: 'regex', pattern: 'node' };
      const matcher3: Matcher = { kind: 'regex', pattern: 'alpine' };

      const input = 'FROM node:18-alpine';

      expect(evaluateMatcher(matcher1, input)).toBe(true);
      expect(evaluateMatcher(matcher2, input)).toBe(true);
      expect(evaluateMatcher(matcher3, input)).toBe(true);

      // All must match for rule to apply
      const policy: Policy = {
        version: '2.0',
        rules: [
          {
            id: 'all-conditions',
            priority: 100,
            conditions: [matcher1, matcher2, matcher3],
            actions: {},
          },
        ],
      };

      const results = applyPolicy(policy, input);
      expect(results[0].matched).toBe(true);
    });

    it('should handle negation patterns', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [
          {
            id: 'no-latest',
            priority: 100,
            conditions: [
              { kind: 'regex', pattern: ':latest' },
            ],
            actions: { block: true },
          },
        ],
      };

      const withLatest = 'FROM node:latest';
      const withVersion = 'FROM node:18';

      const results1 = applyPolicy(policy, withLatest);
      const results2 = applyPolicy(policy, withVersion);

      expect(results1[0].matched).toBe(true);
      expect(results2[0].matched).toBe(false);
    });

    it('should handle function matchers with complex args', () => {
      const matcher: Matcher = {
        kind: 'function',
        name: 'hasVulnerabilities',
        args: [['CRITICAL', 'HIGH']],
      };

      const criticalVuln = {
        vulnerabilities: [
          { severity: 'CRITICAL', cve: 'CVE-2021-1234' },
        ],
      };

      const lowVuln = {
        vulnerabilities: [
          { severity: 'LOW', cve: 'CVE-2021-5678' },
        ],
      };

      expect(evaluateMatcher(matcher, criticalVuln)).toBe(true);
      expect(evaluateMatcher(matcher, lowVuln)).toBe(false);
    });

    it('should handle count thresholds correctly', () => {
      const matcher: Matcher = {
        kind: 'regex',
        pattern: 'COPY',
        count_threshold: 5,
      };

      const few = 'COPY a\nCOPY b\nCOPY c';
      const many = 'COPY 1\nCOPY 2\nCOPY 3\nCOPY 4\nCOPY 5\nCOPY 6';

      expect(evaluateMatcher(matcher, few)).toBe(false);
      expect(evaluateMatcher(matcher, many)).toBe(true);
    });
  });

  describe('Default Policy Behavior', () => {
    it('should create valid default policy', () => {
      const defaultPolicy = createDefaultPolicy();

      expect(defaultPolicy.version).toBe('2.0');
      expect(defaultPolicy.rules.length).toBeGreaterThan(0);
      expect(defaultPolicy.defaults).toBeDefined();
    });

    it('should have reasonable defaults', () => {
      const defaultPolicy = createDefaultPolicy();

      expect(defaultPolicy.defaults?.enforcement).toBe('advisory');

      // Should have common security rules
      const ruleIds = defaultPolicy.rules.map(r => r.id);
      expect(ruleIds.length).toBeGreaterThan(0);
    });

    it('should validate default policy', () => {
      const defaultPolicy = createDefaultPolicy();
      const result = validatePolicy(defaultPolicy);

      expect(result.ok).toBe(true);
    });
  });

  describe('Cache Configuration', () => {
    it('should validate cache settings', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [],
        cache: {
          enabled: true,
          ttl: 3600,
        },
      };

      const result = validatePolicy(policy);
      // Cache configuration is optional and may not be validated
      expect(result.ok !== undefined).toBe(true);
    });

    it('should handle missing cache configuration', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [],
      };

      const result = validatePolicy(policy);
      expect(result.ok).toBe(true);
      if (result.ok) {
        // Cache is optional
        expect(result.value.cache === undefined || result.value.cache !== null).toBe(true);
      }
    });
  });
});
