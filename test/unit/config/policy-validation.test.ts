/**
 * Policy Validation Tests
 * Tests for the policy system
 */

import { describe, it, expect } from '@jest/globals';
import * as path from 'node:path';
import * as fs from 'node:fs';
import { loadPolicy, validatePolicy, createDefaultPolicy } from '@/config/policy-io';
import {
  evaluateMatcher,
  applyPolicy,
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


    it('should always return the TypeScript config', () => {
      // Now that we use TypeScript config, loadPolicy always succeeds
      const result = loadPolicy('/any/path/policy.yaml');
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBeDefined();
        expect(result.value.version).toBe('2.0');
      }
    });
  });

  describe('Error Messages', () => {
    it('should provide helpful validation error messages', () => {
      const invalidPolicy = {
        version: '2.0',
        rules: [
          {
            id: 'test',
            // Missing priority
            conditions: [],
            actions: {},
          },
        ],
      };

      const result = validatePolicy(invalidPolicy);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Policy validation failed');
        expect(result.error).toContain('policies/');
      }
    });
  });
});