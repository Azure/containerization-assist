/**
 * Policy Validation Tests
 * Tests for the policy system
 */

import { describe, it, expect } from '@jest/globals';
import * as path from 'node:path';
import * as fs from 'node:fs';
import {
  loadPolicy,
  validatePolicy,
  resolveEnvironment,
  createDefaultPolicy,
} from '@/config/policy-io';
import {
  evaluateMatcher,
  applyPolicy,
  getRuleWeights,
  selectStrategy,
} from '@/config/policy-eval';
import type {
  Policy,
  Matcher,
  PolicyRule,
} from '@/config/policy-schemas';

describe('Policy System', () => {
  describe('Policy Validation', () => {
    it('should validate a correct v2.0 policy', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [
          {
            id: 'test-rule',
            priority: 100,
            conditions: [
              {
                kind: 'regex',
                pattern: 'test.*pattern',
              },
            ],
            actions: {
              test: true,
            },
          },
        ],
      };

      const result = validatePolicy(policy);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.version).toBe('2.0');
        expect(result.value.rules).toHaveLength(1);
      }
    });

    it('should reject invalid policy structure', () => {
      const invalidPolicy = {
        version: '2.0',
        rules: [
          {
            id: 'test-rule',
            // Missing required 'priority' field
            conditions: [],
            actions: {},
          },
        ],
      };

      const result = validatePolicy(invalidPolicy);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('priority');
      }
    });

    it('should validate matcher types', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [
          {
            id: 'regex-matcher',
            priority: 100,
            conditions: [
              {
                kind: 'regex',
                pattern: '^FROM',
                flags: 'i',
                count_threshold: 2,
              },
            ],
            actions: {},
          },
          {
            id: 'function-matcher',
            priority: 90,
            conditions: [
              {
                kind: 'function',
                name: 'fileExists',
                args: ['package.json'],
              },
            ],
            actions: {},
          },
        ],
      };

      const result = validatePolicy(policy);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.rules[0].conditions[0].kind).toBe('regex');
        expect(result.value.rules[1].conditions[0].kind).toBe('function');
      }
    });

    it('should validate cache configuration', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [],
        cache: {
          enabled: true,
          ttl: 600,
        },
      };

      const result = validatePolicy(policy);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.cache?.enabled).toBe(true);
        expect(result.value.cache?.ttl).toBe(600);
      }
    });
  });

  describe('Environment Resolution', () => {
    it('should apply environment overrides', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [
          {
            id: 'security-rule',
            priority: 100,
            conditions: [],
            actions: {
              block: false,
            },
          },
        ],
        environments: {
          production: {
            overrides: [
              {
                rule_id: 'security-rule',
                priority: 200,
                actions: {
                  block: true,
                },
              },
            ],
          },
        },
      };

      const resolved = resolveEnvironment(policy, 'production');
      expect(resolved.rules[0].priority).toBe(200);
      expect(resolved.rules[0].actions.block).toBe(true);
    });

    it('should disable rules when enabled is false', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [
          {
            id: 'rule-1',
            priority: 100,
            conditions: [],
            actions: {},
          },
          {
            id: 'rule-2',
            priority: 90,
            conditions: [],
            actions: {},
          },
        ],
        environments: {
          test: {
            overrides: [
              {
                rule_id: 'rule-1',
                enabled: false,
              },
            ],
          },
        },
      };

      const resolved = resolveEnvironment(policy, 'test');
      expect(resolved.rules).toHaveLength(1);
      expect(resolved.rules[0].id).toBe('rule-2');
    });

    it('should sort rules by priority after resolution', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [
          {
            id: 'low-priority',
            priority: 50,
            conditions: [],
            actions: {},
          },
          {
            id: 'high-priority',
            priority: 100,
            conditions: [],
            actions: {},
          },
          {
            id: 'medium-priority',
            priority: 75,
            conditions: [],
            actions: {},
          },
        ],
      };

      const resolved = resolveEnvironment(policy, 'development');
      expect(resolved.rules[0].id).toBe('high-priority');
      expect(resolved.rules[1].id).toBe('medium-priority');
      expect(resolved.rules[2].id).toBe('low-priority');
    });
  });

  describe('Matcher Evaluation', () => {
    describe('Regex Matcher', () => {
      it('should match regex patterns', () => {
        const matcher: Matcher = {
          kind: 'regex',
          pattern: 'FROM.*alpine',
        };

        expect(evaluateMatcher(matcher, 'FROM node:alpine')).toBe(true);
        expect(evaluateMatcher(matcher, 'FROM ubuntu:latest')).toBe(false);
      });

      it('should respect regex flags', () => {
        const matcher: Matcher = {
          kind: 'regex',
          pattern: 'from.*alpine',
          flags: 'i',
        };

        expect(evaluateMatcher(matcher, 'FROM node:alpine')).toBe(true);
        expect(evaluateMatcher(matcher, 'from node:alpine')).toBe(true);
      });

      it('should handle count threshold', () => {
        const matcher: Matcher = {
          kind: 'regex',
          pattern: 'RUN',
          count_threshold: 3,
        };

        const dockerfileWith2Runs = 'FROM node\nRUN npm install\nRUN npm build';
        const dockerfileWith3Runs = 'FROM node\nRUN npm install\nRUN npm build\nRUN npm test';

        expect(evaluateMatcher(matcher, dockerfileWith2Runs)).toBe(false);
        expect(evaluateMatcher(matcher, dockerfileWith3Runs)).toBe(true);
      });
    });

    describe('Function Matcher', () => {
      it('should evaluate hasPattern function', () => {
        const matcher: Matcher = {
          kind: 'function',
          name: 'hasPattern',
          args: ['USER.*root', 'i'],
        };

        expect(evaluateMatcher(matcher, 'USER root')).toBe(true);
        expect(evaluateMatcher(matcher, 'user ROOT')).toBe(true);
        expect(evaluateMatcher(matcher, 'USER app')).toBe(false);
      });

      it('should evaluate largerThan function', () => {
        const matcher: Matcher = {
          kind: 'function',
          name: 'largerThan',
          args: [100],
        };

        expect(evaluateMatcher(matcher, 'a'.repeat(101))).toBe(true);
        expect(evaluateMatcher(matcher, 'a'.repeat(100))).toBe(false);
        expect(evaluateMatcher(matcher, { size: 150 })).toBe(true);
        expect(evaluateMatcher(matcher, { size: 50 })).toBe(false);
      });

      it('should evaluate hasVulnerabilities function', () => {
        const matcher: Matcher = {
          kind: 'function',
          name: 'hasVulnerabilities',
          args: [['HIGH', 'CRITICAL']],
        };

        const withHighVuln = {
          vulnerabilities: [
            { severity: 'HIGH', cve: 'CVE-2021-1234' },
            { severity: 'LOW', cve: 'CVE-2021-5678' },
          ],
        };

        const withLowVuln = {
          vulnerabilities: [
            { severity: 'LOW', cve: 'CVE-2021-5678' },
            { severity: 'MEDIUM', cve: 'CVE-2021-9012' },
          ],
        };

        expect(evaluateMatcher(matcher, withHighVuln)).toBe(true);
        expect(evaluateMatcher(matcher, withLowVuln)).toBe(false);
      });
    });
  });

  describe('Policy Application', () => {
    it('should apply all matching rules', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [
          {
            id: 'rule-1',
            priority: 100,
            conditions: [
              {
                kind: 'regex',
                pattern: 'alpine',
              },
            ],
            actions: {
              scan: true,
            },
          },
          {
            id: 'rule-2',
            priority: 90,
            conditions: [
              {
                kind: 'regex',
                pattern: ':latest',
              },
            ],
            actions: {
              warn: true,
            },
          },
        ],
      };

      const input = 'FROM alpine:latest';
      const results = applyPolicy(policy, input);

      expect(results).toHaveLength(2);
      expect(results[0].rule.id).toBe('rule-1');
      expect(results[0].matched).toBe(true);
      expect(results[1].rule.id).toBe('rule-2');
      expect(results[1].matched).toBe(true);
    });

    it('should require all conditions to match (AND logic)', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [
          {
            id: 'multi-condition',
            priority: 100,
            conditions: [
              {
                kind: 'regex',
                pattern: 'FROM',
              },
              {
                kind: 'regex',
                pattern: 'alpine',
              },
            ],
            actions: {},
          },
        ],
      };

      const matchingInput = 'FROM alpine:3.14';
      const partialMatch = 'FROM ubuntu:20.04';

      const matchingResults = applyPolicy(policy, matchingInput);
      const partialResults = applyPolicy(policy, partialMatch);

      expect(matchingResults[0].matched).toBe(true);
      expect(partialResults[0].matched).toBe(false);
    });
  });

  describe('Helper Functions', () => {
    it('should get rule weights', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [
          { id: 'rule-1', priority: 100, conditions: [], actions: {} },
          { id: 'rule-2', priority: 50, conditions: [], actions: {} },
          { id: 'rule-3', priority: 75, conditions: [], actions: {} },
        ],
      };

      const weights = getRuleWeights(policy);
      expect(weights.get('rule-1')).toBe(100);
      expect(weights.get('rule-2')).toBe(50);
      expect(weights.get('rule-3')).toBe(75);
    });

    it('should select best strategy based on weights', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [
          { id: 'strategy-1', priority: 100, conditions: [], actions: {} },
          { id: 'strategy-2', priority: 150, conditions: [], actions: {} },
        ],
      };

      const candidates = [
        { id: 'strategy-1', score: 80 },
        { id: 'strategy-2', score: 70 },
      ];

      const selected = selectStrategy(policy, candidates);
      expect(selected).toBe('strategy-2'); // Higher weight compensates for lower score
    });

    it('should create default policy', () => {
      const policy = createDefaultPolicy();
      expect(policy.version).toBe('2.0');
      expect(policy.rules.length).toBeGreaterThan(0);
      expect(policy.defaults?.enforcement).toBe('advisory');
    });
  });

  describe('Policy Loading', () => {
    const testPolicyPath = path.join(process.cwd(), 'config', 'policy.yaml');

    it('should load policy from YAML file', () => {
      // Only run if policy file exists
      if (fs.existsSync(testPolicyPath)) {
        const result = loadPolicy(testPolicyPath);
        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.version).toBe('2.0');
          expect(result.value.rules.length).toBeGreaterThan(0);
        }
      }
    });

    it('should apply environment when loading', () => {
      if (fs.existsSync(testPolicyPath)) {
        const result = loadPolicy(testPolicyPath, 'production');
        expect(result.ok).toBe(true);
        if (result.ok) {
          // Check that production overrides were applied
          const securityRule = result.value.rules.find(r => r.id === 'security-scanning');
          if (securityRule) {
            expect(securityRule.priority).toBeGreaterThanOrEqual(100);
          }
        }
      }
    });

    it('should handle missing policy file', () => {
      const result = loadPolicy('/non/existent/policy.yaml');
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('not found');
      }
    });
  });
});