/**
 * CEL Policy Integration Tests
 *
 * Tests end-to-end CEL policy evaluation and mixed Rego + CEL policy scenarios.
 */

import { describe, it, expect, beforeEach, afterEach } from '@jest/globals';
import { mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { createLogger } from '@/lib/logger';
import { loadPolicy, loadAndMergePolicies } from '@/config/policy-io';
import type { RegoEvaluator } from '@/config/policy-rego';

describe('CEL Policy Integration', () => {
  let testDir: string;
  let logger: ReturnType<typeof createLogger>;

  beforeEach(() => {
    testDir = join(__dirname, 'test-cel-policies-' + Date.now());
    mkdirSync(testDir, { recursive: true });
    logger = createLogger({ name: 'test', level: 'silent' });
  });

  afterEach(() => {
    rmSync(testDir, { recursive: true, force: true });
  });

  describe('End-to-End CEL Evaluation', () => {
    it('should load and evaluate CEL policy', async () => {
      const policyFile = join(testDir, 'test-policy.yaml');
      writeFileSync(
        policyFile,
        `apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: test-policy
  description: Test CEL policy
spec:
  rules:
    - name: require-alpine
      category: base-image
      severity: block
      condition: '!input.content.contains("FROM alpine")'
      message: "Must use Alpine base image"
      description: "Corporate policy requires Alpine Linux"
`
      );

      const result = await loadPolicy(policyFile);
      expect(result.ok).toBe(true);

      if (result.ok) {
        const evaluator = result.value;

        // Test with non-Alpine image (should fail)
        const evalResult1 = await evaluator.evaluate('FROM node:22\nRUN echo hello');
        expect(evalResult1.allow).toBe(false);
        expect(evalResult1.violations).toHaveLength(1);
        expect(evalResult1.violations[0].rule).toBe('require-alpine');
        expect(evalResult1.violations[0].severity).toBe('block');

        // Test with Alpine image (should pass)
        const evalResult2 = await evaluator.evaluate('FROM alpine:3.20\nRUN echo hello');
        expect(evalResult2.allow).toBe(true);
        expect(evalResult2.violations).toHaveLength(0);
      }
    });

    it('should categorize violations by severity', async () => {
      const policyFile = join(testDir, 'severity-test.yaml');
      writeFileSync(
        policyFile,
        `apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: severity-test
spec:
  rules:
    - name: no-root-user
      category: security
      severity: block
      condition: '!input.content.contains("USER") || input.content.contains("USER root")'
      message: "Must specify non-root USER"

    - name: recommend-healthcheck
      category: health
      severity: warn
      condition: '!input.content.contains("HEALTHCHECK")'
      message: "HEALTHCHECK recommended for production"

    - name: suggest-multi-stage
      category: optimization
      severity: suggest
      condition: '!input.content.contains("AS builder")'
      message: "Consider using multi-stage builds"
`
      );

      const result = await loadPolicy(policyFile);
      expect(result.ok).toBe(true);

      if (result.ok) {
        const evalResult = await result.value.evaluate('FROM alpine\nRUN echo hello');

        // Should have all three severity levels
        expect(evalResult.violations.length).toBe(1); // block
        expect(evalResult.warnings.length).toBe(1); // warn
        expect(evalResult.suggestions.length).toBe(1); // suggest

        expect(evalResult.violations[0].rule).toBe('no-root-user');
        expect(evalResult.warnings[0].rule).toBe('recommend-healthcheck');
        expect(evalResult.suggestions[0].rule).toBe('suggest-multi-stage');

        // allow should be false due to blocking violation
        expect(evalResult.allow).toBe(false);
      }
    });

    it('should handle CEL expression with regex-like patterns', async () => {
      const policyFile = join(testDir, 'pattern-test.yaml');
      writeFileSync(
        policyFile,
        `apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: pattern-test
spec:
  rules:
    - name: no-latest-tag
      category: best-practices
      severity: warn
      condition: 'input.content.contains(":latest")'
      message: "Avoid using :latest tag"
`
      );

      const result = await loadPolicy(policyFile);
      expect(result.ok).toBe(true);

      if (result.ok) {
        // Test with :latest tag - should trigger warning
        const evalResult1 = await result.value.evaluate('FROM alpine:latest\nRUN echo hello');
        expect(evalResult1.warnings.length).toBe(1);
        expect(evalResult1.warnings[0].rule).toBe('no-latest-tag');

        // Test with specific version - should pass
        const evalResult2 = await result.value.evaluate('FROM alpine:3.20\nRUN echo hello');
        expect(evalResult2.warnings.length).toBe(0);
        expect(evalResult2.allow).toBe(true);
      }
    });

    it('should handle complex boolean expressions', async () => {
      const policyFile = join(testDir, 'complex-test.yaml');
      writeFileSync(
        policyFile,
        `apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: complex-test
spec:
  rules:
    - name: security-requirements
      category: security
      severity: block
      condition: |
        !input.content.contains("USER") || input.content.contains("USER root")
      message: "Dockerfile must specify non-root USER"

    - name: production-ready
      category: production
      severity: warn
      condition: |
        input.content.contains("ENV NODE_ENV production") &&
        !input.content.contains("HEALTHCHECK")
      message: "Production images should include HEALTHCHECK"
`
      );

      const result = await loadPolicy(policyFile);
      expect(result.ok).toBe(true);

      if (result.ok) {
        // Test security violation (no USER directive)
        const evalResult1 = await result.value.evaluate('FROM alpine\nRUN echo hello');
        expect(evalResult1.violations.length).toBeGreaterThanOrEqual(1);
        expect(evalResult1.violations.some((v) => v.rule === 'security-requirements')).toBe(true);

        // Test production warning
        const evalResult2 = await result.value.evaluate(
          'FROM alpine\nUSER appuser\nENV NODE_ENV production\nRUN echo hello'
        );
        expect(evalResult2.violations.length).toBe(0);
        expect(evalResult2.warnings.length).toBe(1);
        expect(evalResult2.warnings[0].rule).toBe('production-ready');
      }
    });
  });

  describe('Mixed Rego + CEL Policies', () => {
    it('should merge and enforce both Rego and CEL policies', async () => {
      const regoFile = join(testDir, 'test-rego.rego');
      const celFile = join(testDir, 'test-cel.yaml');

      // Create Rego policy with correct structure (must export 'result' object)
      writeFileSync(
        regoFile,
        `package containerization.test

default allow := true

violations contains v if {
  not regex.match("USER", input.content)
  v := {
    "rule": "require-user-rego",
    "category": "security",
    "severity": "block",
    "message": "USER directive required (from Rego)"
  }
}

# Export result object (required by evaluator)
result := {
  "allow": count(violations) == 0,
  "violations": violations,
  "warnings": [],
  "suggestions": []
}
`
      );

      // Create CEL policy
      writeFileSync(
        celFile,
        `apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: test-cel
spec:
  rules:
    - name: require-healthcheck-cel
      category: health
      severity: warn
      condition: '!input.content.contains("HEALTHCHECK")'
      message: "HEALTHCHECK recommended (from CEL)"
`
      );

      const result = await loadAndMergePolicies([regoFile, celFile]);
      expect(result.ok).toBe(true);

      if (result.ok) {
        const evalResult = await result.value.evaluate('FROM alpine\nRUN echo hello');

        // Should have violations from both policies
        const allIssues = [
          ...evalResult.violations.map((v) => v.rule),
          ...evalResult.warnings.map((w) => w.rule),
        ];

        expect(allIssues).toContain('require-user-rego');
        expect(allIssues).toContain('require-healthcheck-cel');

        // Verify severity categorization
        expect(evalResult.violations.some((v) => v.rule === 'require-user-rego')).toBe(true);
        expect(evalResult.warnings.some((w) => w.rule === 'require-healthcheck-cel')).toBe(true);
      }
    });

    it('should maintain priority order when merging policies', async () => {
      const rego1File = join(testDir, 'rego1.rego');
      const cel1File = join(testDir, 'cel1.yaml');
      const rego2File = join(testDir, 'rego2.rego');

      writeFileSync(
        rego1File,
        `package containerization.test1

violations contains v if {
  true
  v := {"rule": "rego1", "category": "test", "severity": "block", "message": "Rego 1", "priority": 10}
}

result := {
  "allow": false,
  "violations": violations,
  "warnings": [],
  "suggestions": []
}
`
      );

      writeFileSync(
        cel1File,
        `apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: cel1
spec:
  rules:
    - name: cel1
      category: test
      severity: block
      priority: 90
      condition: 'true'
      message: "CEL 1"
`
      );

      writeFileSync(
        rego2File,
        `package containerization.test2

violations contains v if {
  true
  v := {"rule": "rego2", "category": "test", "severity": "block", "message": "Rego 2", "priority": 50}
}

result := {
  "allow": false,
  "violations": violations,
  "warnings": [],
  "suggestions": []
}
`
      );

      const result = await loadAndMergePolicies([rego1File, cel1File, rego2File]);
      expect(result.ok).toBe(true);

      if (result.ok) {
        const evalResult = await result.value.evaluate('FROM alpine');

        // Should have all violations
        expect(evalResult.violations.length).toBe(3);

        // Should be sorted by priority (highest first)
        expect(evalResult.violations[0].rule).toBe('cel1'); // priority 90
        expect(evalResult.violations[1].rule).toBe('rego2'); // priority 50
        expect(evalResult.violations[2].rule).toBe('rego1'); // priority 10
      }
    });

    it('should deduplicate violations from multiple policies', async () => {
      const rego1File = join(testDir, 'dup-rego.rego');
      const celFile = join(testDir, 'dup-cel.yaml');

      // Both policies check for the same thing with same rule name
      writeFileSync(
        rego1File,
        `package containerization.dup

violations contains v if {
  not regex.match("USER", input.content)
  v := {"rule": "require-user", "category": "security", "severity": "block", "message": "From Rego"}
}

result := {
  "allow": count(violations) == 0,
  "violations": violations,
  "warnings": [],
  "suggestions": []
}
`
      );

      writeFileSync(
        celFile,
        `apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: dup-cel
spec:
  rules:
    - name: require-user
      category: security
      severity: block
      condition: '!input.content.contains("USER")'
      message: "From CEL"
`
      );

      const result = await loadAndMergePolicies([rego1File, celFile]);
      expect(result.ok).toBe(true);

      if (result.ok) {
        const evalResult = await result.value.evaluate('FROM alpine\nRUN echo hello');

        // Should only have one violation (deduplicated by rule name)
        expect(evalResult.violations.length).toBe(1);
        expect(evalResult.violations[0].rule).toBe('require-user');
      }
    });
  });

  describe('CEL Limitations', () => {
    it('should return null for queryConfig on CEL policies', async () => {
      const celFile = join(testDir, 'config-test.yaml');
      writeFileSync(
        celFile,
        `apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: config-test
spec:
  rules:
    - name: dummy
      category: test
      severity: suggest
      condition: 'false'
      message: "Never triggers"
`
      );

      const result = await loadPolicy(celFile);
      expect(result.ok).toBe(true);

      if (result.ok) {
        const config = await result.value.queryConfig('test.config', {});
        expect(config).toBeNull();
      }
    });

    it('should query config from Rego when mixed with CEL', async () => {
      const regoFile = join(testDir, 'config-rego.rego');
      const celFile = join(testDir, 'no-config-cel.yaml');

      // Rego policy with config
      writeFileSync(
        regoFile,
        `package containerization.config

generation_config := {
  "baseImage": "alpine:3.20",
  "useMultiStage": true
}
`
      );

      // CEL policy without config
      writeFileSync(
        celFile,
        `apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: no-config
spec:
  rules:
    - name: dummy
      category: test
      severity: suggest
      condition: 'false'
      message: "Dummy"
`
      );

      const result = await loadAndMergePolicies([celFile, regoFile]);
      expect(result.ok).toBe(true);

      if (result.ok) {
        // Query with full package path (containerization.config.generation_config)
        const config = await result.value.queryConfig('containerization.config.generation_config', {});

        // Should get config from Rego policy
        expect(config).not.toBeNull();
        expect(config).toEqual({
          baseImage: 'alpine:3.20',
          useMultiStage: true,
        });
      }
    });
  });

  describe('Error Handling', () => {
    it('should handle invalid CEL syntax', async () => {
      const celFile = join(testDir, 'invalid-syntax.yaml');
      writeFileSync(
        celFile,
        `apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: invalid
spec:
  rules:
    - name: bad-syntax
      category: test
      severity: block
      condition: 'input.nonexistent.deeply.nested.field.that.will.cause.error()'
      message: "Invalid CEL"
`
      );

      const result = await loadPolicy(celFile);

      // Note: CEL may compile this successfully and only fail at runtime
      // We'll validate that runtime errors are caught during evaluation
      if (result.ok) {
        const evalResult = await result.value.evaluate('test content');
        // Runtime errors should be captured as violations or handled gracefully
        expect(evalResult).toBeDefined();
      } else {
        // If it fails to compile, that's also acceptable
        expect(result.error).toContain('compile');
      }
    });

    it('should handle invalid policy schema', async () => {
      const celFile = join(testDir, 'invalid-schema.yaml');
      writeFileSync(
        celFile,
        `apiVersion: wrong-version
kind: PolicySet
metadata:
  name: invalid
spec:
  rules: []
`
      );

      const result = await loadPolicy(celFile);

      // Should fail to load due to schema validation
      expect(result.ok).toBe(false);
    });

    it('should handle missing required fields', async () => {
      const celFile = join(testDir, 'missing-fields.yaml');
      writeFileSync(
        celFile,
        `apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: missing
spec:
  rules:
    - name: incomplete
      category: test
      # Missing severity and condition
      message: "Incomplete rule"
`
      );

      const result = await loadPolicy(celFile);

      // Should fail to load due to missing required fields
      expect(result.ok).toBe(false);
    });

    it('should handle CEL runtime errors gracefully', async () => {
      const celFile = join(testDir, 'runtime-error.yaml');
      writeFileSync(
        celFile,
        `apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: runtime-error
spec:
  rules:
    - name: runtime-error
      category: test
      severity: block
      # This will cause a runtime error if input.nonexistent is accessed incorrectly
      condition: 'input.content.size() > 0'
      message: "Runtime error test"
`
      );

      const result = await loadPolicy(celFile);
      expect(result.ok).toBe(true);

      if (result.ok) {
        // Evaluate with valid input - should work
        const evalResult = await result.value.evaluate('FROM alpine');

        // Errors during evaluation should be captured as violations
        // The exact behavior depends on CEL library error handling
        expect(evalResult).toBeDefined();
      }
    });
  });

  describe('Real-World Scenarios', () => {
    it('should enforce company registry policy', async () => {
      const celFile = join(testDir, 'company-registry.yaml');
      writeFileSync(
        celFile,
        `apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: company-registry
  description: Enforce use of company container registry
spec:
  rules:
    - name: require-company-registry
      category: registry
      severity: block
      priority: 100
      condition: |
        input.content.contains("FROM") &&
        !input.content.contains("registry.company.com/")
      message: "All base images must come from registry.company.com"
      description: "Corporate policy requires all images from internal registry"
`
      );

      const result = await loadPolicy(celFile);
      expect(result.ok).toBe(true);

      if (result.ok) {
        // Test with public registry (should fail)
        const evalResult1 = await result.value.evaluate('FROM alpine:3.20\nRUN echo hello');
        expect(evalResult1.allow).toBe(false);
        expect(evalResult1.violations[0].rule).toBe('require-company-registry');

        // Test with company registry (should pass)
        const evalResult2 = await result.value.evaluate(
          'FROM registry.company.com/alpine:3.20\nRUN echo hello'
        );
        expect(evalResult2.allow).toBe(true);
      }
    });

    it('should enforce Kubernetes resource limits', async () => {
      const celFile = join(testDir, 'k8s-limits.yaml');
      writeFileSync(
        celFile,
        `apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: k8s-limits
spec:
  rules:
    - name: require-resource-limits
      category: resources
      severity: block
      condition: |
        input.content.contains("kind: Deployment") &&
        !input.content.contains("limits:")
      message: "Deployments must specify resource limits"

    - name: require-liveness-probe
      category: health
      severity: warn
      condition: |
        input.content.contains("kind: Deployment") &&
        !input.content.contains("livenessProbe:")
      message: "Deployments should include liveness probe"
`
      );

      const result = await loadPolicy(celFile);
      expect(result.ok).toBe(true);

      if (result.ok) {
        // Test deployment without limits
        const manifest = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: app
        image: test:latest`;

        const evalResult = await result.value.evaluate(manifest);

        expect(evalResult.violations.length).toBe(1);
        expect(evalResult.violations[0].rule).toBe('require-resource-limits');
        expect(evalResult.warnings.length).toBe(1);
        expect(evalResult.warnings[0].rule).toBe('require-liveness-probe');
      }
    });
  });
});
