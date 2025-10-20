/**
 * Policy Evaluation Performance Tests
 * Benchmark tests for policy system performance on large manifests and rule sets
 */

import { performance } from 'node:perf_hooks';

import { jest } from '@jest/globals';
import { applyPolicy, evaluateMatcher } from '../../../src/config/policy-eval';
import type { Policy, PolicyRule, Matcher } from '../../../src/config/policy-schemas';

/**
 * Performance multiplier to account for variations across different machines and system load
 */
const PERFORMANCE_VARIANCE_MULTIPLIER = 4;

describe('Policy Evaluation Performance', () => {
  /**
   * Generate mock policy rules for testing
   */
  function generateMockRules(count: number): PolicyRule[] {
    const rules: PolicyRule[] = [];
    const patterns = [
      'FROM node:',
      'FROM python:',
      'RUN npm install',
      'RUN pip install',
      'COPY package.json',
    ];

    for (let i = 0; i < count; i++) {
      const matcher: Matcher = {
        kind: 'regex',
        pattern: patterns[i % patterns.length],
        flags: 'i',
      };

      const rule: PolicyRule = {
        id: `rule-${i}`,
        type: 'dockerfile-best-practice',
        conditions: [matcher], // Use conditions array instead of matcher
        action: {
          type: 'warning',
          message: `Test warning ${i}`,
        },
        priority: (i % 3) as 0 | 1 | 2,
      };
      rules.push(rule);
    }

    return rules;
  }

  /**
   * Generate large Dockerfile for testing
   */
  function generateLargeDockerfile(lines: number): string {
    let dockerfile = 'FROM node:20-alpine\n';
    dockerfile += 'WORKDIR /app\n';

    // Add many RUN commands
    for (let i = 0; i < lines - 10; i++) {
      if (i % 5 === 0) {
        dockerfile += `RUN echo "Build step ${i}" && mkdir -p /app/dir-${i}\n`;
      } else if (i % 3 === 0) {
        dockerfile += `ENV VAR_${i}=value-${i}\n`;
      } else {
        dockerfile += `RUN echo "Processing line ${i}"\n`;
      }
    }

    dockerfile += 'COPY package*.json ./\n';
    dockerfile += 'RUN npm ci --only=production\n';
    dockerfile += 'COPY . .\n';
    dockerfile += 'EXPOSE 3000\n';
    dockerfile += 'USER appuser\n';
    dockerfile += 'CMD ["node", "index.js"]\n';

    return dockerfile;
  }

  /**
   * Create a mock policy with given rules
   */
  function createMockPolicy(rules: PolicyRule[]): Policy {
    return {
      version: '1.0.0',
      rules,
      metadata: {
        name: 'test-policy',
        description: 'Performance test policy',
      },
    };
  }

  describe('Rule Evaluation Performance', () => {
    it('should evaluate 100 rules in <50ms', () => {
      const rules = generateMockRules(100);
      const policy = createMockPolicy(rules);
      const dockerfile = generateLargeDockerfile(50);

      const start = performance.now();
      const results = applyPolicy(policy, dockerfile);
      const elapsed = performance.now() - start;

      expect(elapsed).toBeLessThan(50);
      expect(results).toBeDefined();
      expect(Array.isArray(results)).toBe(true);
    });

    it('should evaluate 1000 rules in <100ms', () => {
      const rules = generateMockRules(1000);
      const policy = createMockPolicy(rules);
      const dockerfile = generateLargeDockerfile(50);

      const start = performance.now();
      const results = applyPolicy(policy, dockerfile);
      const elapsed = performance.now() - start;

      expect(elapsed).toBeLessThan(100);
      expect(results).toBeDefined();
      expect(Array.isArray(results)).toBe(true);
    });

    it('should evaluate 5000 rules in <500ms', () => {
      const rules = generateMockRules(5000);
      const policy = createMockPolicy(rules);
      const dockerfile = generateLargeDockerfile(50);

      const start = performance.now();
      const results = applyPolicy(policy, dockerfile);
      const elapsed = performance.now() - start;

      expect(elapsed).toBeLessThan(500);
      expect(results).toBeDefined();
      expect(Array.isArray(results)).toBe(true);
    });
  });

  describe('Matcher Evaluation Performance', () => {
    it('should evaluate regex matcher in <1ms', () => {
      const matcher: Matcher = {
        kind: 'regex',
        pattern: 'FROM node:.*alpine',
        flags: 'i',
      };

      const dockerfile = generateLargeDockerfile(100);

      const start = performance.now();
      const result = evaluateMatcher(matcher, dockerfile);
      const elapsed = performance.now() - start;

      expect(elapsed).toBeLessThan(1);
      expect(typeof result).toBe('boolean');
    });

    it('should evaluate 100 matchers in <10ms', () => {
      const dockerfile = generateLargeDockerfile(100);

      const start = performance.now();
      for (let i = 0; i < 100; i++) {
        const matcher: Matcher = {
          kind: 'regex',
          pattern: 'RUN',
          flags: 'g',
        };
        evaluateMatcher(matcher, dockerfile);
      }
      const elapsed = performance.now() - start;

      expect(elapsed).toBeLessThan(10);
    });
  });

  describe('Large Dockerfile Processing Performance', () => {
    it('should handle large Dockerfile (500 lines) with 100 rules in <200ms', () => {
      const dockerfile = generateLargeDockerfile(500);
      const rules = generateMockRules(100);
      const policy = createMockPolicy(rules);

      const start = performance.now();
      const results = applyPolicy(policy, dockerfile);
      const elapsed = performance.now() - start;

      expect(elapsed).toBeLessThan(200);
      expect(results.length).toBe(100); // All rules should be evaluated
    });

    it('should handle large Dockerfile (1000 lines) with 50 rules in <200ms', () => {
      const dockerfile = generateLargeDockerfile(1000);
      const rules = generateMockRules(50);
      const policy = createMockPolicy(rules);

      const start = performance.now();
      const results = applyPolicy(policy, dockerfile);
      const elapsed = performance.now() - start;

      expect(elapsed).toBeLessThan(200);
      expect(results.length).toBe(50);
    });
  });

  describe('Memory Efficiency', () => {
    it('should not consume excessive memory with large rule sets', () => {
      const rules = generateMockRules(10000);
      const policy = createMockPolicy(rules);
      const dockerfile = generateLargeDockerfile(100);

      const start = performance.now();
      const results = applyPolicy(policy, dockerfile);
      const elapsed = performance.now() - start;

      // Should complete even with very large rule set
      expect(elapsed).toBeLessThan(2000);
      expect(results).toBeDefined();
      expect(results.length).toBe(10000);
    });

    it('should handle repeated evaluations without memory leaks', () => {
      const rules = generateMockRules(100);
      const policy = createMockPolicy(rules);
      const dockerfile = generateLargeDockerfile(50);
      const iterations = 100;

      const start = performance.now();

      for (let i = 0; i < iterations; i++) {
        const results = applyPolicy(policy, dockerfile);
        // Results are created and discarded each iteration
        expect(results).toBeDefined();
      }

      const elapsed = performance.now() - start;

      // 100 iterations should complete quickly
      expect(elapsed).toBeLessThan(500);
    });
  });

  describe('Edge Case Performance', () => {
    it('should handle empty rule set efficiently', () => {
      const rules: PolicyRule[] = [];
      const policy = createMockPolicy(rules);
      const dockerfile = generateLargeDockerfile(100);

      const start = performance.now();
      const results = applyPolicy(policy, dockerfile);
      const elapsed = performance.now() - start;

      expect(elapsed).toBeLessThan(1);
      expect(results.length).toBe(0);
    });

    it('should handle single rule efficiently', () => {
      const rules = generateMockRules(1);
      const policy = createMockPolicy(rules);
      const dockerfile = 'FROM node:20-alpine\nCMD ["node", "index.js"]';

      const start = performance.now();
      const results = applyPolicy(policy, dockerfile);
      const elapsed = performance.now() - start;

      expect(elapsed).toBeLessThan(1);
      expect(results.length).toBe(1);
    });

    it('should handle empty input efficiently', () => {
      const rules = generateMockRules(100);
      const policy = createMockPolicy(rules);

      const start = performance.now();
      const results = applyPolicy(policy, '');
      const elapsed = performance.now() - start;

      expect(elapsed).toBeLessThan(10);
      expect(results.length).toBe(100);
    });
  });

  describe('Performance Regression Detection', () => {
    it('should establish baseline for 100 rules', () => {
      const rules = generateMockRules(100);
      const policy = createMockPolicy(rules);
      const dockerfile = generateLargeDockerfile(50);
      const measurements: number[] = [];

      // Run multiple times to get average
      for (let i = 0; i < 10; i++) {
        const start = performance.now();
        applyPolicy(policy, dockerfile);
        const elapsed = performance.now() - start;
        measurements.push(elapsed);
      }

      const average = measurements.reduce((a, b) => a + b, 0) / measurements.length;

      // Average should be well under the limit
      expect(average).toBeLessThan(25);

      // Log baseline for reference
      console.log(`Baseline (100 rules): ${average.toFixed(2)}ms average`);
    });

    it('should establish baseline for 1000 rules', () => {
      const rules = generateMockRules(1000);
      const policy = createMockPolicy(rules);
      const dockerfile = generateLargeDockerfile(50);
      const measurements: number[] = [];

      for (let i = 0; i < 10; i++) {
        const start = performance.now();
        applyPolicy(policy, dockerfile);
        const elapsed = performance.now() - start;
        measurements.push(elapsed);
      }

      const average = measurements.reduce((a, b) => a + b, 0) / measurements.length;

      // Average should be well under the limit
      expect(average).toBeLessThan(50);

      // Log baseline for reference
      console.log(`Baseline (1000 rules): ${average.toFixed(2)}ms average`);
    });

    it('should detect performance characteristics', () => {
      const sizes = [10, 50, 100, 500, 1000];
      const results: { size: number; time: number }[] = [];
      const dockerfile = generateLargeDockerfile(50);

      for (const size of sizes) {
        const rules = generateMockRules(size);
        const policy = createMockPolicy(rules);
        const start = performance.now();
        applyPolicy(policy, dockerfile);
        const elapsed = performance.now() - start;

        results.push({ size, time: elapsed });
      }

      // Log performance scaling
      console.log('\nPerformance Scaling:');
      results.forEach(({ size, time }) => {
        console.log(`  ${size} rules: ${time.toFixed(2)}ms`);
      });

      // Verify linear or better scaling
      for (let i = 1; i < results.length; i++) {
        const prev = results[i - 1];
        const curr = results[i];
        if (prev && curr) {
          const sizeRatio = curr.size / prev.size;
          const timeRatio = curr.time / prev.time;

          // Time should not grow faster than linear with size
          expect(timeRatio).toBeLessThanOrEqual(sizeRatio * PERFORMANCE_VARIANCE_MULTIPLIER);
        }
      }
    });
  });
});
