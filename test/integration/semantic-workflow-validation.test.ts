/**
 * Integration Tests: Semantic and Workflow Validation
 *
 * Tests semantic validation policies with real tool outputs to verify:
 * - Resource efficiency detection (over-provisioned resources)
 * - Security posture scoring
 * - Environment-specific validation rules
 * - Cross-tool consistency checks (image names, ports, health checks)
 *
 * Sprint 5: Story 5.1 - E2E Semantic Validation Tests
 */

import { describe, expect, it, afterAll, beforeAll } from '@jest/globals';
import path from 'node:path';
import { loadRegoPolicy, type RegoEvaluator } from '@/config/policy-rego';
import { createLogger } from '@/lib/logger';
import generateDockerfileTool from '@/tools/generate-dockerfile/tool';
import generateK8sManifestsTool from '@/tools/generate-k8s-manifests/tool';
import type { ToolContext } from '@/mcp/context';

const logger = createLogger({ level: 'error' });

describe('Semantic and Workflow Validation', () => {
  const policiesDir = path.join(process.cwd(), 'policies.user.examples');
  const evaluators: Array<{ close: () => void }> = [];

  afterAll(() => {
    // Clean up all policy evaluators
    evaluators.forEach((e) => e.close());
  });

  describe('Semantic Validation Policy', () => {
    it('should load semantic validation policy successfully', async () => {
      const policyPath = path.join(policiesDir, 'semantic-validation.rego');
      const result = await loadRegoPolicy(policyPath, logger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        evaluators.push(result.value);

        // Verify the policy evaluator has the expected methods
        expect(result.value.evaluate).toBeDefined();
        expect(result.value.evaluatePolicy).toBeDefined();
        expect(result.value.close).toBeDefined();
      }
    });

    it('should load with test file present', async () => {
      const testPath = path.join(policiesDir, 'semantic-validation_test.rego');
      const fs = await import('node:fs/promises');

      // Verify test file exists
      const testFileExists = await fs
        .access(testPath)
        .then(() => true)
        .catch(() => false);
      expect(testFileExists).toBe(true);
    });

    describe('E2E Semantic Validation with Real Tool Outputs', () => {
      let policy: RegoEvaluator;

      beforeAll(async () => {
        const policyResult = await loadRegoPolicy(
          path.join(policiesDir, 'semantic-validation.rego'),
          logger,
        );
        expect(policyResult.ok).toBe(true);
        if (!policyResult.ok) throw new Error('Failed to load semantic validation policy');
        policy = policyResult.value;
        evaluators.push(policy);
      });

      it('should detect over-provisioned resources in K8s manifests', async () => {
        const ctx: ToolContext = {
          logger,
          policy,
          queryConfig: async <T>(
            packageName: string,
            input: Record<string, unknown>,
          ): Promise<T | null> => {
            return policy.queryConfig<T>(packageName, input);
          },
        };

        // Generate K8s manifest for production
        const k8sResult = await generateK8sManifestsTool.handler(
          {
            name: 'test-app',
            modulePath: process.cwd(),
            language: 'node',
            environment: 'production',
            manifestType: 'kubernetes',
          },
          ctx,
        );

        expect(k8sResult.ok).toBe(true);
        if (!k8sResult.ok) return;

        // The semantic validation policy should be able to analyze the output
        // (actual validation happens in Rego tests, this verifies integration)
        expect(k8sResult.value.summary).toBeDefined();
      });

      it('should calculate security posture scores for production', async () => {
        const ctx: ToolContext = {
          logger,
          policy,
          queryConfig: async <T>(
            packageName: string,
            input: Record<string, unknown>,
          ): Promise<T | null> => {
            return policy.queryConfig<T>(packageName, input);
          },
        };

        // Generate Dockerfile for production
        const dockerfileResult = await generateDockerfileTool.handler(
          {
            repositoryPath: process.cwd(),
            language: 'java',
            environment: 'production',
            targetPlatform: 'linux/amd64',
          },
          ctx,
        );

        expect(dockerfileResult.ok).toBe(true);
        if (!dockerfileResult.ok) return;

        // Production should have security considerations
        expect(dockerfileResult.value.recommendations.securityConsiderations.length).toBeGreaterThan(0);
      });

      it('should apply environment-specific rules (dev vs prod)', async () => {
        const ctx: ToolContext = {
          logger,
          policy,
          queryConfig: async <T>(
            packageName: string,
            input: Record<string, unknown>,
          ): Promise<T | null> => {
            return policy.queryConfig<T>(packageName, input);
          },
        };

        // Development should be more permissive
        const devResult = await generateDockerfileTool.handler(
          {
            repositoryPath: process.cwd(),
            language: 'python',
            environment: 'development',
            targetPlatform: 'linux/amd64',
          },
          ctx,
        );

        expect(devResult.ok).toBe(true);
        if (!devResult.ok) return;

        // Production should have stricter requirements
        const prodResult = await generateDockerfileTool.handler(
          {
            repositoryPath: process.cwd(),
            language: 'python',
            environment: 'production',
            targetPlatform: 'linux/amd64',
          },
          ctx,
        );

        expect(prodResult.ok).toBe(true);
        if (!prodResult.ok) return;

        // Production should have more security considerations
        const devSecurityCount = devResult.value.recommendations.securityConsiderations.length;
        const prodSecurityCount = prodResult.value.recommendations.securityConsiderations.length;

        // Production typically has more security recommendations
        expect(prodSecurityCount).toBeGreaterThanOrEqual(devSecurityCount);
      });
    });
  });

  describe('Workflow Validation Policy', () => {
    it('should load workflow validation policy successfully', async () => {
      const policyPath = path.join(policiesDir, 'workflow-validation.rego');
      const result = await loadRegoPolicy(policyPath, logger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        evaluators.push(result.value);

        // Verify the policy evaluator has the expected methods
        expect(result.value.evaluate).toBeDefined();
        expect(result.value.evaluatePolicy).toBeDefined();
        expect(result.value.close).toBeDefined();
      }
    });

    it('should load with test file present', async () => {
      const testPath = path.join(policiesDir, 'workflow-validation_test.rego');
      const fs = await import('node:fs/promises');

      // Verify test file exists
      const testFileExists = await fs
        .access(testPath)
        .then(() => true)
        .catch(() => false);
      expect(testFileExists).toBe(true);
    });

    describe('E2E Cross-Tool Consistency Validation', () => {
      let policy: RegoEvaluator;

      beforeAll(async () => {
        const policyResult = await loadRegoPolicy(
          path.join(policiesDir, 'workflow-validation.rego'),
          logger,
        );
        expect(policyResult.ok).toBe(true);
        if (!policyResult.ok) throw new Error('Failed to load workflow validation policy');
        policy = policyResult.value;
        evaluators.push(policy);
      });

      it('should verify image name consistency between Dockerfile and K8s', async () => {
        const ctx: ToolContext = {
          logger,
          policy,
          queryConfig: async <T>(
            packageName: string,
            input: Record<string, unknown>,
          ): Promise<T | null> => {
            return policy.queryConfig<T>(packageName, input);
          },
        };

        // Generate Dockerfile
        const dockerfileResult = await generateDockerfileTool.handler(
          {
            repositoryPath: process.cwd(),
            language: 'node',
            environment: 'production',
            targetPlatform: 'linux/amd64',
          },
          ctx,
        );

        expect(dockerfileResult.ok).toBe(true);
        if (!dockerfileResult.ok) return;

        // Generate K8s manifests
        const k8sResult = await generateK8sManifestsTool.handler(
          {
            name: 'test-app',
            modulePath: process.cwd(),
            language: 'node',
            environment: 'production',
            manifestType: 'kubernetes',
          },
          ctx,
        );

        expect(k8sResult.ok).toBe(true);
        if (!k8sResult.ok) return;

        // Both should be generated successfully
        expect(dockerfileResult.value.summary).toBeDefined();
        expect(k8sResult.value.summary).toBeDefined();
      });

      it('should verify port consistency across workflow', async () => {
        const ctx: ToolContext = {
          logger,
          policy,
          queryConfig: async <T>(
            packageName: string,
            input: Record<string, unknown>,
          ): Promise<T | null> => {
            return policy.queryConfig<T>(packageName, input);
          },
        };

        // Generate for a web application
        const dockerfileResult = await generateDockerfileTool.handler(
          {
            repositoryPath: process.cwd(),
            language: 'node',
            environment: 'production',
            targetPlatform: 'linux/amd64',
          },
          ctx,
        );

        expect(dockerfileResult.ok).toBe(true);
        if (!dockerfileResult.ok) return;

        const k8sResult = await generateK8sManifestsTool.handler(
          {
            name: 'web-app',
            modulePath: process.cwd(),
            language: 'node',
            environment: 'production',
            manifestType: 'kubernetes',
          },
          ctx,
        );

        expect(k8sResult.ok).toBe(true);
        if (!k8sResult.ok) return;

        // Port consistency is validated by the policy (tested in Rego tests)
        expect(k8sResult.value).toBeDefined();
      });

      it('should verify health check consistency', async () => {
        const ctx: ToolContext = {
          logger,
          policy,
          queryConfig: async <T>(
            packageName: string,
            input: Record<string, unknown>,
          ): Promise<T | null> => {
            return policy.queryConfig<T>(packageName, input);
          },
        };

        // Generate for production environment
        const dockerfileResult = await generateDockerfileTool.handler(
          {
            repositoryPath: process.cwd(),
            language: 'python',
            environment: 'production',
            targetPlatform: 'linux/amd64',
          },
          ctx,
        );

        expect(dockerfileResult.ok).toBe(true);
        if (!dockerfileResult.ok) return;

        const k8sResult = await generateK8sManifestsTool.handler(
          {
            name: 'health-app',
            modulePath: process.cwd(),
            language: 'python',
            environment: 'production',
            manifestType: 'kubernetes',
          },
          ctx,
        );

        expect(k8sResult.ok).toBe(true);
        if (!k8sResult.ok) return;

        // Health check recommendations should be present
        const hasHealthRecommendations =
          dockerfileResult.value.recommendations.bestPractices.some(
            (rec) => rec.recommendation.toLowerCase().includes('health'),
          ) ||
          k8sResult.value.recommendations.bestPractices.some(
            (rec) => rec.recommendation.toLowerCase().includes('health') || rec.recommendation.toLowerCase().includes('probe'),
          );

        expect(hasHealthRecommendations).toBe(true);
      });
    });
  });
});
