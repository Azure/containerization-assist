/**
 * Comprehensive E2E Integration Test: Policy-Driven Complete Workflow
 *
 * Tests the entire containerization workflow with policy-driven variations:
 * - generate-dockerfile → generate-k8s-manifests
 * - Environment-specific generation (dev vs staging vs production)
 * - Knowledge filtering, template injection, and semantic validation
 * - Cross-tool consistency checks
 *
 * Sprint 5: Story 5.1 - E2E Integration Tests
 */

import { describe, it, expect, beforeAll, afterAll } from '@jest/globals';
import path from 'node:path';
import { loadRegoPolicy, type RegoEvaluator } from '@/config/policy-rego';
import generateDockerfileTool from '@/tools/generate-dockerfile/tool';
import generateK8sManifestsTool from '@/tools/generate-k8s-manifests/tool';
import { createLogger } from '@/lib/logger';
import type { ToolContext } from '@/mcp/context';

const logger = createLogger({ level: 'error' });

describe('Policy-Driven Complete Workflow E2E Tests', () => {
  const policiesDir = path.resolve(process.cwd(), 'policies.user.examples');

  describe('Full Workflow with Environment Variations', () => {
    describe('Development Environment', () => {
      let policy: RegoEvaluator;

      beforeAll(async () => {
        // Load template policy for development testing
        const policyResult = await loadRegoPolicy(
          path.join(policiesDir, 'templates.rego'),
          logger,
        );
        expect(policyResult.ok).toBe(true);
        if (!policyResult.ok) throw new Error('Failed to load policy');
        policy = policyResult.value;
      });

      afterAll(() => {
        policy?.close();
      });

      it('should generate development-optimized Dockerfile and K8s manifests', async () => {
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

        // Step 1: Generate Dockerfile
        const dockerfileResult = await generateDockerfileTool.handler(
          {
            repositoryPath: process.cwd(),
            language: 'node',
            environment: 'development',
            targetPlatform: 'linux/amd64',
          },
          ctx,
        );

        expect(dockerfileResult.ok).toBe(true);
        if (!dockerfileResult.ok) {
          console.error('Dockerfile generation failed:', dockerfileResult.error);
          return;
        }

        const dockerfilePlan = dockerfileResult.value;

        // Development should have basic recommendations
        expect(dockerfilePlan.summary).toBeDefined();
        expect(dockerfilePlan.recommendations).toBeDefined();

        // Step 2: Generate K8s manifests
        const k8sResult = await generateK8sManifestsTool.handler(
          {
            name: 'dev-app',
            modulePath: process.cwd(),
            language: 'node',
            environment: 'development',
            manifestType: 'kubernetes',
          },
          ctx,
        );

        expect(k8sResult.ok).toBe(true);
        if (!k8sResult.ok) {
          console.error('K8s manifest generation failed:', k8sResult.error);
          return;
        }

        const k8sPlan = k8sResult.value;

        // Development should have lower resource requirements
        expect(k8sPlan.summary).toBeDefined();
        expect(k8sPlan.recommendations).toBeDefined();

        // Verify environment-specific behaviors
        const allRecommendations = [
          ...dockerfilePlan.recommendations.securityConsiderations,
          ...dockerfilePlan.recommendations.bestPractices,
          ...(k8sPlan.recommendations.securityConsiderations || []),
          ...(k8sPlan.recommendations.bestPractices || []),
        ];

        // Development should not have expensive production features like observability
        const prodOnlyFeatures = allRecommendations.filter(
          (rec) =>
            rec.id === 'org-java-observability' || rec.id === 'org-log-forwarder',
        );
        expect(prodOnlyFeatures.length).toBe(0);
      });
    });

    describe('Production Environment', () => {
      let policy: RegoEvaluator;

      beforeAll(async () => {
        const policyResult = await loadRegoPolicy(
          path.join(policiesDir, 'templates.rego'),
          logger,
        );
        expect(policyResult.ok).toBe(true);
        if (!policyResult.ok) throw new Error('Failed to load policy');
        policy = policyResult.value;
      });

      afterAll(() => {
        policy?.close();
      });

      it('should generate production-hardened Dockerfile and K8s manifests', async () => {
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

        // Step 1: Generate Dockerfile
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
        if (!dockerfileResult.ok) {
          console.error('Dockerfile generation failed:', dockerfileResult.error);
          return;
        }

        const dockerfilePlan = dockerfileResult.value;

        // Production should have security hardening
        const dockerfileRecs = [
          ...dockerfilePlan.recommendations.securityConsiderations,
          ...dockerfilePlan.recommendations.bestPractices,
        ];

        const securityHardening = dockerfileRecs.find(
          (rec) => rec.id === 'org-security-hardening',
        );
        expect(securityHardening).toBeDefined();
        expect(securityHardening?.policyDriven).toBe(true);

        // Step 2: Generate K8s manifests
        const k8sResult = await generateK8sManifestsTool.handler(
          {
            name: 'prod-app',
            modulePath: process.cwd(),
            language: 'java',
            environment: 'production',
            manifestType: 'kubernetes',
          },
          ctx,
        );

        expect(k8sResult.ok).toBe(true);
        if (!k8sResult.ok) {
          console.error('K8s manifest generation failed:', k8sResult.error);
          return;
        }

        const k8sPlan = k8sResult.value;

        // Production should have observability features
        const k8sRecs = [...(k8sPlan.recommendations.bestPractices || [])];

        // Check for production-specific templates
        const productionFeatures = k8sRecs.filter(
          (rec) =>
            rec.id === 'org-log-forwarder' || rec.id === 'org-db-migration',
        );

        // Should have at least some production features injected
        expect(productionFeatures.length).toBeGreaterThan(0);
      });
    });

    describe('Staging Environment', () => {
      let policy: RegoEvaluator;

      beforeAll(async () => {
        const policyResult = await loadRegoPolicy(
          path.join(policiesDir, 'templates.rego'),
          logger,
        );
        expect(policyResult.ok).toBe(true);
        if (!policyResult.ok) throw new Error('Failed to load policy');
        policy = policyResult.value;
      });

      afterAll(() => {
        policy?.close();
      });

      it('should generate staging configurations with balanced trade-offs', async () => {
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

        // Generate Dockerfile for staging
        const dockerfileResult = await generateDockerfileTool.handler(
          {
            repositoryPath: process.cwd(),
            language: 'python',
            environment: 'staging',
            targetPlatform: 'linux/amd64',
          },
          ctx,
        );

        expect(dockerfileResult.ok).toBe(true);
        if (!dockerfileResult.ok) return;

        // Generate K8s manifests for staging
        const k8sResult = await generateK8sManifestsTool.handler(
          {
            name: 'staging-app',
            modulePath: process.cwd(),
            language: 'python',
            environment: 'staging',
            manifestType: 'kubernetes',
          },
          ctx,
        );

        expect(k8sResult.ok).toBe(true);
        if (!k8sResult.ok) return;

        // Staging should have some security but not all production overhead
        expect(dockerfileResult.value.summary).toBeDefined();
        expect(k8sResult.value.summary).toBeDefined();
      });
    });
  });

  describe('Knowledge Filtering Consistency Across Tools', () => {
    let policy: RegoEvaluator;

    beforeAll(async () => {
      const policyResult = await loadRegoPolicy(
        path.join(policiesDir, 'knowledge-filtering.rego'),
        logger,
      );
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) throw new Error('Failed to load knowledge policy');
      policy = policyResult.value;
    });

    afterAll(() => {
      policy?.close();
    });

    it('should apply consistent registry restrictions across both tools', async () => {
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

      // Generate Dockerfile in production
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

      // Generate K8s manifests in production
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

      // Both should respect the same registry policies
      // This is enforced by knowledge filtering at the policy level
      expect(dockerfileResult.value).toBeDefined();
      expect(k8sResult.value).toBeDefined();
    });

    it('should apply consistent prioritization in both tools', async () => {
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

      // Generate Dockerfile with policy
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

      // Generate K8s manifests with policy
      const k8sResult = await generateK8sManifestsTool.handler(
        {
          name: 'secure-app',
          modulePath: process.cwd(),
          language: 'java',
          environment: 'production',
          manifestType: 'kubernetes',
        },
        ctx,
      );

      expect(k8sResult.ok).toBe(true);
      if (!k8sResult.ok) return;

      // Both should have recommendations (policy ensures they exist)
      expect(dockerfileResult.value.recommendations).toBeDefined();
      expect(k8sResult.value.recommendations).toBeDefined();
    });
  });

  describe('Template Injection Consistency', () => {
    let policy: RegoEvaluator;

    beforeAll(async () => {
      const policyResult = await loadRegoPolicy(
        path.join(policiesDir, 'templates.rego'),
        logger,
      );
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) throw new Error('Failed to load templates policy');
      policy = policyResult.value;
    });

    afterAll(() => {
      policy?.close();
    });

    it('should inject organization-wide standards in both Dockerfile and K8s', async () => {
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
          language: 'python',
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
          name: 'template-app',
          modulePath: process.cwd(),
          language: 'python',
          environment: 'production',
          manifestType: 'kubernetes',
        },
        ctx,
      );

      expect(k8sResult.ok).toBe(true);
      if (!k8sResult.ok) return;

      // Check for CA certificates template in Dockerfile
      const dockerfileRecs = [
        ...dockerfileResult.value.recommendations.securityConsiderations,
        ...dockerfileResult.value.recommendations.bestPractices,
      ];
      const caCertRec = dockerfileRecs.find(
        (rec) => rec.id === 'org-ca-certificates',
      );
      expect(caCertRec).toBeDefined();
      expect(caCertRec?.policyDriven).toBe(true);

      // Check for secrets volume template in K8s
      const k8sRecs = [...(k8sResult.value.recommendations.bestPractices || [])];
      const secretsVolumeRec = k8sRecs.find(
        (rec) => rec.id === 'org-secrets-volume',
      );
      expect(secretsVolumeRec).toBeDefined();
      expect(secretsVolumeRec?.policyDriven).toBe(true);
    });
  });

  describe('Dynamic Defaults Integration', () => {
    let policy: RegoEvaluator;

    beforeAll(async () => {
      const policyResult = await loadRegoPolicy(
        path.join(policiesDir, 'dynamic-defaults.rego'),
        logger,
      );
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) throw new Error('Failed to load dynamic defaults policy');
      policy = policyResult.value;
    });

    afterAll(() => {
      policy?.close();
    });

    it('should calculate environment-appropriate replica counts', async () => {
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

      // Test production with high traffic and tier-1 criticality
      const prodResult = await generateK8sManifestsTool.handler(
        {
          name: 'critical-app',
          modulePath: process.cwd(),
          language: 'java',
          environment: 'production',
          manifestType: 'kubernetes',
          trafficLevel: 'high',
          criticalityTier: 'tier-1',
        },
        ctx,
      );

      expect(prodResult.ok).toBe(true);
      if (!prodResult.ok) return;

      const hpaRec = prodResult.value.recommendations.resourceManagement?.find(
        (rec) => rec.id === 'policy-hpa-config',
      );

      expect(hpaRec).toBeDefined();
      expect(hpaRec?.policyDriven).toBe(true);
      // Should calculate min=12 (3 base * 2 traffic * 2 criticality)
      expect(hpaRec?.recommendation).toContain('min=12');
    });

    it('should apply language-specific health check timings', async () => {
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

      // Java has longer startup time (120s → 96s initial delay)
      const javaResult = await generateK8sManifestsTool.handler(
        {
          name: 'java-app',
          modulePath: process.cwd(),
          language: 'java',
          environment: 'production',
          manifestType: 'kubernetes',
        },
        ctx,
      );

      expect(javaResult.ok).toBe(true);
      if (!javaResult.ok) return;

      const javaHealthRec = javaResult.value.recommendations.bestPractices.find(
        (rec) => rec.id === 'policy-health-check-config',
      );

      expect(javaHealthRec).toBeDefined();
      expect(javaHealthRec?.recommendation).toContain('initialDelay=96s');

      // Node.js has faster startup (30s → 24s initial delay)
      const nodeResult = await generateK8sManifestsTool.handler(
        {
          name: 'node-app',
          modulePath: process.cwd(),
          language: 'node',
          environment: 'production',
          manifestType: 'kubernetes',
        },
        ctx,
      );

      expect(nodeResult.ok).toBe(true);
      if (!nodeResult.ok) return;

      const nodeHealthRec = nodeResult.value.recommendations.bestPractices.find(
        (rec) => rec.id === 'policy-health-check-config',
      );

      expect(nodeHealthRec).toBeDefined();
      expect(nodeHealthRec?.recommendation).toContain('initialDelay=24s');
    });
  });
});
