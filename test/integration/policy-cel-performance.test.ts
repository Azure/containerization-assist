/**
 * CEL Policy Performance Tests
 *
 * Validates that CEL policy evaluation meets performance requirements.
 *
 * **Performance Targets (from Phase 3.3):**
 * - Simple rule: <10ms
 * - 10 rules: <50ms
 * - No performance regression vs. Rego
 */

import { describe, it, expect, beforeAll } from '@jest/globals';
import { mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { createLogger } from '@/lib/logger';
import { loadPolicy, loadAndMergePolicies } from '@/config/policy-io';
import { loadRegoPolicy } from '@/config/policy-rego';
import type { RegoEvaluator } from '@/config/policy-rego';

describe('CEL Policy Performance', () => {
  let testDir: string;
  let logger: ReturnType<typeof createLogger>;
  let celEvaluator: RegoEvaluator;
  let celEvaluatorMultiple: RegoEvaluator;
  let regoEvaluator: RegoEvaluator;

  beforeAll(async () => {
    testDir = join(__dirname, 'perf-test-' + Date.now());
    mkdirSync(testDir, { recursive: true });
    logger = createLogger({ name: 'perf-test', level: 'silent' });

    // Create simple CEL policy (1 rule)
    const simpleCelFile = join(testDir, 'simple-cel.yaml');
    writeFileSync(
      simpleCelFile,
      `apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: simple-perf
spec:
  rules:
    - name: require-user
      category: security
      severity: block
      condition: '!input.content.contains("USER")'
      message: "Must specify USER directive"
`
    );

    // Create complex CEL policy (10 rules)
    const complexCelFile = join(testDir, 'complex-cel.yaml');
    writeFileSync(
      complexCelFile,
      `apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: complex-perf
spec:
  rules:
    - name: rule1
      category: security
      severity: block
      condition: '!input.content.contains("USER")'
      message: "Rule 1"
    - name: rule2
      category: security
      severity: block
      condition: '!input.content.contains("HEALTHCHECK")'
      message: "Rule 2"
    - name: rule3
      category: optimization
      severity: warn
      condition: '!input.content.contains("AS builder")'
      message: "Rule 3"
    - name: rule4
      category: security
      severity: block
      condition: 'input.content.contains(":latest")'
      message: "Rule 4"
    - name: rule5
      category: best-practices
      severity: suggest
      condition: '!input.content.contains("EXPOSE")'
      message: "Rule 5"
    - name: rule6
      category: security
      severity: warn
      condition: '!input.content.contains("WORKDIR")'
      message: "Rule 6"
    - name: rule7
      category: optimization
      severity: suggest
      condition: '!input.content.contains("COPY")'
      message: "Rule 7"
    - name: rule8
      category: security
      severity: block
      condition: 'input.content.contains("root")'
      message: "Rule 8"
    - name: rule9
      category: best-practices
      severity: warn
      condition: '!input.content.contains("ENV")'
      message: "Rule 9"
    - name: rule10
      category: optimization
      severity: suggest
      condition: '!input.content.contains("RUN apt-get")'
      message: "Rule 10"
`
    );

    // Create equivalent Rego policy for comparison
    const regoFile = join(testDir, 'simple-rego.rego');
    writeFileSync(
      regoFile,
      `package containerization.perf

violations contains result if {
  not regex.match("USER", input.content)
  result := {
    "rule": "require-user",
    "category": "security",
    "severity": "block",
    "message": "Must specify USER directive"
  }
}
`
    );

    // Load policies
    const celResult = await loadPolicy(simpleCelFile);
    const celComplexResult = await loadPolicy(complexCelFile);
    const regoResult = await loadRegoPolicy(regoFile, logger);

    if (!celResult.ok) throw new Error('Failed to load simple CEL policy');
    if (!celComplexResult.ok) throw new Error('Failed to load complex CEL policy');
    if (!regoResult.ok) throw new Error('Failed to load Rego policy');

    celEvaluator = celResult.value;
    celEvaluatorMultiple = celComplexResult.value;
    regoEvaluator = regoResult.value;

    return () => {
      rmSync(testDir, { recursive: true, force: true });
    };
  });

  describe('Simple Rule Performance', () => {
    it('should evaluate simple CEL rule in <10ms', async () => {
      const testContent = 'FROM alpine:3.20\nRUN echo hello\nEXPOSE 8080';
      const iterations = 100;
      const start = performance.now();

      for (let i = 0; i < iterations; i++) {
        await celEvaluator.evaluate(testContent);
      }

      const end = performance.now();
      const avgTime = (end - start) / iterations;

      expect(avgTime).toBeLessThan(10);

      // Log for visibility
      console.log(`    Average time per evaluation: ${avgTime.toFixed(3)}ms (target: <10ms)`);
    });

    it('should have comparable performance to Rego for simple rules', async () => {
      const testContent = 'FROM alpine:3.20\nRUN echo hello\nEXPOSE 8080';
      const iterations = 100;

      // Measure CEL
      const celStart = performance.now();
      for (let i = 0; i < iterations; i++) {
        await celEvaluator.evaluate(testContent);
      }
      const celTime = (performance.now() - celStart) / iterations;

      // Measure Rego
      const regoStart = performance.now();
      for (let i = 0; i < iterations; i++) {
        await regoEvaluator.evaluate(testContent);
      }
      const regoTime = (performance.now() - regoStart) / iterations;

      // CEL should be within 3x of Rego (generous threshold for mocked tests)
      const ratio = celTime / regoTime;
      expect(ratio).toBeLessThan(3);

      console.log(`    CEL: ${celTime.toFixed(3)}ms, Rego: ${regoTime.toFixed(3)}ms (ratio: ${ratio.toFixed(2)}x)`);
    });
  });

  describe('Multiple Rules Performance', () => {
    it('should evaluate 10 CEL rules in <50ms', async () => {
      const testContent = `FROM alpine:3.20
USER appuser
WORKDIR /app
COPY package.json .
RUN npm install
ENV NODE_ENV=production
EXPOSE 8080
HEALTHCHECK CMD curl -f http://localhost:8080 || exit 1
`;
      const iterations = 50;
      const start = performance.now();

      for (let i = 0; i < iterations; i++) {
        await celEvaluatorMultiple.evaluate(testContent);
      }

      const end = performance.now();
      const avgTime = (end - start) / iterations;

      expect(avgTime).toBeLessThan(50);

      console.log(`    Average time for 10 rules: ${avgTime.toFixed(3)}ms (target: <50ms)`);
    });
  });

  describe('Compilation Performance', () => {
    it('should compile CEL policy quickly', async () => {
      const policyFile = join(testDir, 'compile-test.yaml');
      writeFileSync(
        policyFile,
        `apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: compile-test
spec:
  rules:
    - name: test-rule
      category: test
      severity: block
      condition: 'input.content.contains("FROM")'
      message: "Test"
`
      );

      const start = performance.now();
      const result = await loadPolicy(policyFile);
      const loadTime = performance.now() - start;

      expect(result.ok).toBe(true);
      expect(loadTime).toBeLessThan(100); // Should load in <100ms

      console.log(`    Policy load/compile time: ${loadTime.toFixed(3)}ms (target: <100ms)`);
    });
  });

  describe('Memory Efficiency', () => {
    it('should not leak memory with repeated evaluations', async () => {
      const testContent = 'FROM alpine:3.20\nRUN echo hello';
      const iterations = 1000;

      // Force GC if available (Node with --expose-gc flag)
      if (global.gc) {
        global.gc();
      }

      const memBefore = process.memoryUsage().heapUsed;

      for (let i = 0; i < iterations; i++) {
        await celEvaluator.evaluate(testContent);
      }

      if (global.gc) {
        global.gc();
      }

      const memAfter = process.memoryUsage().heapUsed;
      const memIncrease = (memAfter - memBefore) / 1024 / 1024; // MB

      // Memory should not increase significantly (<5MB for 1000 evaluations)
      expect(memIncrease).toBeLessThan(5);

      console.log(`    Memory increase after ${iterations} evaluations: ${memIncrease.toFixed(2)}MB`);
    });
  });

  describe('Concurrent Evaluation', () => {
    it('should handle concurrent evaluations efficiently', async () => {
      const testContent = 'FROM alpine:3.20\nRUN echo hello';
      const concurrency = 10;
      const iterations = 10;

      const start = performance.now();

      const promises = Array.from({ length: concurrency }, async () => {
        for (let i = 0; i < iterations; i++) {
          await celEvaluator.evaluate(testContent);
        }
      });

      await Promise.all(promises);

      const totalTime = performance.now() - start;
      const avgTimePerEval = totalTime / (concurrency * iterations);

      expect(avgTimePerEval).toBeLessThan(10);

      console.log(`    Concurrent avg time: ${avgTimePerEval.toFixed(3)}ms (${concurrency}x${iterations} evaluations)`);
    });
  });
});
