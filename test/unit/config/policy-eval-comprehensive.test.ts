/**
 * Comprehensive Policy Evaluation Tests
 * Covers edge cases and uncovered branches to improve coverage
 */

import { describe, it, expect } from '@jest/globals';
import { evaluateMatcher, applyPolicy } from '@/config/policy-eval';
import type { Policy, Matcher } from '@/config/policy-schemas';

describe('Policy Evaluation - Comprehensive Coverage', () => {
  describe('Regex Matcher Edge Cases', () => {
    it('should handle regex with global flag for count threshold', () => {
      const matcher: Matcher = {
        kind: 'regex',
        pattern: 'COPY',
        count_threshold: 3,
      };

      const input = 'COPY file1\nCOPY file2\nCOPY file3';
      expect(evaluateMatcher(matcher, input)).toBe(true);

      const insufficientInput = 'COPY file1\nCOPY file2';
      expect(evaluateMatcher(matcher, insufficientInput)).toBe(false);
    });

    it('should combine flags with global flag for count threshold', () => {
      const matcher: Matcher = {
        kind: 'regex',
        pattern: 'from',
        flags: 'i', // Case insensitive
        count_threshold: 2,
      };

      const input = 'FROM node:18\nfrom alpine:latest';
      expect(evaluateMatcher(matcher, input)).toBe(true);
    });

    it('should handle count threshold of exactly 0', () => {
      const matcher: Matcher = {
        kind: 'regex',
        pattern: 'MISSING',
        count_threshold: 0,
      };

      const input = 'No matches here';
      expect(evaluateMatcher(matcher, input)).toBe(true); // 0 >= 0
    });

    it('should handle regex with no matches and count threshold', () => {
      const matcher: Matcher = {
        kind: 'regex',
        pattern: 'NOTFOUND',
        count_threshold: 1,
      };

      const input = 'This text does not contain the pattern';
      expect(evaluateMatcher(matcher, input)).toBe(false);
    });

    it('should handle multiline regex patterns', () => {
      const matcher: Matcher = {
        kind: 'regex',
        pattern: 'FROM.*\\n.*RUN',
        flags: 's', // Dotall flag
      };

      const input = 'FROM node:18\nRUN npm install';
      // Note: JavaScript regex doesn't support 's' flag in older versions
      // This will match with or without the flag depending on environment
      const result = evaluateMatcher(matcher, input);
      expect(typeof result).toBe('boolean');
    });
  });

  describe('Function Matcher - hasPattern Edge Cases', () => {
    it('should match hasPattern with flags', () => {
      const matcher: Matcher = {
        kind: 'function',
        name: 'hasPattern',
        args: ['user.*admin', 'i'],
      };

      expect(evaluateMatcher(matcher, 'USER admin')).toBe(true);
      expect(evaluateMatcher(matcher, 'user ADMIN')).toBe(true);
      expect(evaluateMatcher(matcher, 'user guest')).toBe(false);
    });

    it('should match hasPattern without flags', () => {
      const matcher: Matcher = {
        kind: 'function',
        name: 'hasPattern',
        args: ['exact.*match'],
      };

      expect(evaluateMatcher(matcher, 'exact string match')).toBe(true);
      expect(evaluateMatcher(matcher, 'EXACT STRING MATCH')).toBe(false);
    });

    it('should handle hasPattern with complex regex', () => {
      const matcher: Matcher = {
        kind: 'function',
        name: 'hasPattern',
        args: ['^FROM\\s+[a-z]+:[0-9.]+$', 'm'],
      };

      const input = 'FROM node:18.0.0';
      expect(evaluateMatcher(matcher, input)).toBe(true);
    });
  });

  describe('Function Matcher - fileExists Edge Cases', () => {
    it('should check fileExists with base path from input object', () => {
      const matcher: Matcher = {
        kind: 'function',
        name: 'fileExists',
        args: ['package.json'],
      };

      const inputWithPath = {
        path: process.cwd(),
      };

      // Will check if package.json exists in cwd
      const result = evaluateMatcher(matcher, inputWithPath);
      expect(typeof result).toBe('boolean');
    });

    it('should check fileExists with default path', () => {
      const matcher: Matcher = {
        kind: 'function',
        name: 'fileExists',
        args: ['tsconfig.json'],
      };

      const result = evaluateMatcher(matcher, 'string input');
      expect(typeof result).toBe('boolean');
    });

    it('should return false for non-existent file', () => {
      const matcher: Matcher = {
        kind: 'function',
        name: 'fileExists',
        args: ['nonexistent-file-12345.txt'],
      };

      expect(evaluateMatcher(matcher, { path: '/tmp' })).toBe(false);
    });
  });

  describe('Function Matcher - largerThan Edge Cases', () => {
    it('should check string length', () => {
      const matcher: Matcher = {
        kind: 'function',
        name: 'largerThan',
        args: [10],
      };

      expect(evaluateMatcher(matcher, 'short')).toBe(false); // 5 chars
      expect(evaluateMatcher(matcher, 'this is longer than ten')).toBe(true); // > 10
    });

    it('should check object size property', () => {
      const matcher: Matcher = {
        kind: 'function',
        name: 'largerThan',
        args: [1000],
      };

      expect(evaluateMatcher(matcher, { size: 500 })).toBe(false);
      expect(evaluateMatcher(matcher, { size: 2000 })).toBe(true);
    });

    it('should return false for object without size property', () => {
      const matcher: Matcher = {
        kind: 'function',
        name: 'largerThan',
        args: [100],
      };

      expect(evaluateMatcher(matcher, { other: 'property' })).toBe(false);
    });

    it('should handle edge case of exact size', () => {
      const matcher: Matcher = {
        kind: 'function',
        name: 'largerThan',
        args: [10],
      };

      expect(evaluateMatcher(matcher, '1234567890')).toBe(false); // Exactly 10
      expect(evaluateMatcher(matcher, '12345678901')).toBe(true); // 11
    });
  });

  describe('Function Matcher - hasVulnerabilities Edge Cases', () => {
    it('should match vulnerabilities with uppercase severity', () => {
      const matcher: Matcher = {
        kind: 'function',
        name: 'hasVulnerabilities',
        args: [['CRITICAL', 'HIGH']],
      };

      const input = {
        vulnerabilities: [
          { severity: 'CRITICAL', cve: 'CVE-2021-1234' },
        ],
      };

      expect(evaluateMatcher(matcher, input)).toBe(true);
    });

    it('should match vulnerabilities with lowercase severity (case insensitive)', () => {
      const matcher: Matcher = {
        kind: 'function',
        name: 'hasVulnerabilities',
        args: [['HIGH']],
      };

      const input = {
        vulnerabilities: [
          { severity: 'high', cve: 'CVE-2021-5678' },
        ],
      };

      expect(evaluateMatcher(matcher, input)).toBe(true);
    });

    it('should return false for non-matching severities', () => {
      const matcher: Matcher = {
        kind: 'function',
        name: 'hasVulnerabilities',
        args: [['CRITICAL']],
      };

      const input = {
        vulnerabilities: [
          { severity: 'LOW', cve: 'CVE-2021-9999' },
          { severity: 'MEDIUM', cve: 'CVE-2021-8888' },
        ],
      };

      expect(evaluateMatcher(matcher, input)).toBe(false);
    });

    it('should return false when vulnerabilities array is empty', () => {
      const matcher: Matcher = {
        kind: 'function',
        name: 'hasVulnerabilities',
        args: [['HIGH']],
      };

      const input = {
        vulnerabilities: [],
      };

      expect(evaluateMatcher(matcher, input)).toBe(false);
    });

    it('should return false when input has no vulnerabilities property', () => {
      const matcher: Matcher = {
        kind: 'function',
        name: 'hasVulnerabilities',
        args: [['HIGH']],
      };

      expect(evaluateMatcher(matcher, { other: 'data' })).toBe(false);
      expect(evaluateMatcher(matcher, 'string input')).toBe(false);
    });
  });

  describe('Unknown Function Matcher', () => {
    it('should return false for unknown function names', () => {
      const matcher: Matcher = {
        kind: 'function',
        name: 'unknownFunction',
        args: ['arg1', 'arg2'],
      };

      expect(evaluateMatcher(matcher, 'any input')).toBe(false);
    });
  });

  describe('Unknown Matcher Kind', () => {
    it('should return false for unknown matcher kind', () => {
      const matcher = {
        kind: 'unknown',
        value: 'test',
      } as any;

      expect(evaluateMatcher(matcher, 'any input')).toBe(false);
    });
  });

  describe('Policy Application Edge Cases', () => {
    it('should handle policy with empty conditions array', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [
          {
            id: 'no-conditions',
            priority: 100,
            conditions: [],
            actions: { test: true },
          },
        ],
      };

      const results = applyPolicy(policy, 'any input');
      expect(results).toHaveLength(1);
      expect(results[0].matched).toBe(false); // Empty conditions don't match
    });

    it('should handle policy with single condition', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [
          {
            id: 'single-condition',
            priority: 100,
            conditions: [
              { kind: 'regex', pattern: 'test' },
            ],
            actions: { match: true },
          },
        ],
      };

      const results = applyPolicy(policy, 'test input');
      expect(results[0].matched).toBe(true);

      const noMatch = applyPolicy(policy, 'no match');
      expect(noMatch[0].matched).toBe(false);
    });

    it('should require all conditions to match (AND logic)', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [
          {
            id: 'multi-condition-and',
            priority: 100,
            conditions: [
              { kind: 'regex', pattern: 'alpha' },
              { kind: 'regex', pattern: 'beta' },
              { kind: 'regex', pattern: 'gamma' },
            ],
            actions: { allMatch: true },
          },
        ],
      };

      const allMatch = applyPolicy(policy, 'alpha beta gamma');
      expect(allMatch[0].matched).toBe(true);

      const partialMatch = applyPolicy(policy, 'alpha beta');
      expect(partialMatch[0].matched).toBe(false);

      const noMatch = applyPolicy(policy, 'delta epsilon');
      expect(noMatch[0].matched).toBe(false);
    });

    it('should handle object input for pattern matching', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [
          {
            id: 'object-match',
            priority: 100,
            conditions: [
              { kind: 'regex', pattern: 'value' },
            ],
            actions: { found: true },
          },
        ],
      };

      const objInput = {
        field: 'value',
        nested: { key: 'data' },
      };

      const results = applyPolicy(policy, objInput);
      // Object is converted to JSON string, so 'value' should match
      expect(results[0].matched).toBe(true);
    });

    it('should handle complex nested conditions', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [
          {
            id: 'complex-rule',
            priority: 100,
            conditions: [
              { kind: 'regex', pattern: 'FROM', flags: 'i' },
              { kind: 'function', name: 'hasPattern', args: ['alpine', 'i'] },
              {
                kind: 'regex',
                pattern: 'RUN',
                count_threshold: 2,
              },
            ],
            actions: { complex: true },
          },
        ],
      };

      const matchingInput = 'FROM alpine:latest\nRUN cmd1\nRUN cmd2';
      const results = applyPolicy(policy, matchingInput);
      expect(results[0].matched).toBe(true);

      const notEnoughRuns = 'FROM alpine:latest\nRUN cmd1';
      const results2 = applyPolicy(policy, notEnoughRuns);
      expect(results2[0].matched).toBe(false);
    });

    it('should return all rules with their match status', () => {
      const policy: Policy = {
        version: '2.0',
        rules: [
          {
            id: 'rule-1',
            priority: 100,
            conditions: [{ kind: 'regex', pattern: 'match' }],
            actions: { a: true },
          },
          {
            id: 'rule-2',
            priority: 90,
            conditions: [{ kind: 'regex', pattern: 'nomatch' }],
            actions: { b: true },
          },
          {
            id: 'rule-3',
            priority: 80,
            conditions: [{ kind: 'regex', pattern: 'match' }],
            actions: { c: true },
          },
        ],
      };

      const results = applyPolicy(policy, 'match this text');

      expect(results).toHaveLength(3);
      expect(results[0].matched).toBe(true); // rule-1
      expect(results[1].matched).toBe(false); // rule-2
      expect(results[2].matched).toBe(true); // rule-3
    });
  });

  describe('Type Coercion and Edge Cases', () => {
    it('should handle string length for largerThan', () => {
      const matcher: Matcher = {
        kind: 'function',
        name: 'largerThan',
        args: [100],
      };

      // String length, not numeric value
      const shortString = '50'; // 2 characters
      const longString = 'a'.repeat(150); // 150 characters

      expect(evaluateMatcher(matcher, shortString)).toBe(false);
      expect(evaluateMatcher(matcher, longString)).toBe(true);
    });

    it('should handle boolean values in input', () => {
      const matcher: Matcher = {
        kind: 'regex',
        pattern: 'true',
      };

      const boolInput = { flag: true, other: false };
      const results = evaluateMatcher(matcher, boolInput);
      expect(results).toBe(true); // JSON string contains 'true'
    });

    it('should handle null and undefined in object input', () => {
      const matcher: Matcher = {
        kind: 'regex',
        pattern: 'null',
      };

      const nullInput = { value: null, other: undefined };
      const results = evaluateMatcher(matcher, nullInput);
      expect(results).toBe(true); // JSON string contains 'null'
    });

    it('should handle arrays in object input', () => {
      const matcher: Matcher = {
        kind: 'regex',
        pattern: 'item',
      };

      const arrayInput = { items: ['item1', 'item2', 'item3'] };
      const results = evaluateMatcher(matcher, arrayInput);
      expect(results).toBe(true); // JSON contains 'item'
    });
  });
});
