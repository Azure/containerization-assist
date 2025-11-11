/**
 * Integration tests for template injection and dynamic defaults
 * Sprint 3: Tests template injection in generate-dockerfile and generate-k8s-manifests
 */

import { describe, it, expect, beforeAll } from '@jest/globals';
import path from 'node:path';
import { loadRegoPolicy } from '@/config/policy-rego';
import generateDockerfileTool from '@/tools/generate-dockerfile/tool';
import generateK8sManifestsTool from '@/tools/generate-k8s-manifests/tool';
import { createLogger } from '@/lib/logger';
import type { ToolContext } from '@/mcp/context';

const logger = createLogger({ level: 'error' });

describe('Template Injection Integration Tests', () => {
  const policyDir = path.resolve(process.cwd(), 'policies.user.examples');
  const templatePolicyPath = path.join(policyDir, 'templates.rego');
  const dynamicDefaultsPolicyPath = path.join(policyDir, 'dynamic-defaults.rego');

  // Verify policy files exist
  beforeAll(() => {
    const fs = require('node:fs');
    expect(fs.existsSync(templatePolicyPath)).toBe(true);
    expect(fs.existsSync(dynamicDefaultsPolicyPath)).toBe(true);
  });

  describe('Dockerfile Template Injection', () => {
    it('injects CA certificates template for all environments', async () => {
      const policyResult = await loadRegoPolicy(templatePolicyPath, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      const policy = policyResult.value;

      const ctx: ToolContext = {
        logger,
        policy,
        queryConfig: async <T>(packageName: string, input: Record<string, unknown>): Promise<T | null> => {
          return policy.queryConfig<T>(packageName, input);
        },
      };

      const result = await generateDockerfileTool.handler(
        {
          repositoryPath: process.cwd(),
          language: 'python',
          environment: 'development',
          targetPlatform: 'linux/amd64',
        },
        ctx,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      const plan = result.value;

      // Check for CA cert template in recommendations
      const caCertRec = [
        ...plan.recommendations.securityConsiderations,
        ...plan.recommendations.bestPractices,
      ].find((rec) => rec.id === 'org-ca-certificates');

      expect(caCertRec).toBeDefined();
      expect(caCertRec?.policyDriven).toBe(true);
      expect(caCertRec?.recommendation).toContain('ca-certificates');

      policy.close();
    });

    it('injects Java observability template only in production', async () => {
      const policyResult = await loadRegoPolicy(templatePolicyPath, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      const policy = policyResult.value;

      const ctx: ToolContext = {
        logger,
        policy,
        queryConfig: async <T>(packageName: string, input: Record<string, unknown>): Promise<T | null> => {
          return policy.queryConfig<T>(packageName, input);
        },
      };

      // Production - should have observability template
      const prodResult = await generateDockerfileTool.handler(
        {
          repositoryPath: process.cwd(),
          language: 'java',
          environment: 'production',
          targetPlatform: 'linux/amd64',
        },
        ctx,
      );

      expect(prodResult.ok).toBe(true);
      if (!prodResult.ok) return;

      const prodPlan = prodResult.value;
      const prodObsRec = [
        ...prodPlan.recommendations.securityConsiderations,
        ...prodPlan.recommendations.bestPractices,
      ].find((rec) => rec.id === 'org-java-observability');

      expect(prodObsRec).toBeDefined();
      expect(prodObsRec?.policyDriven).toBe(true);

      // Development - should NOT have observability template
      const devResult = await generateDockerfileTool.handler(
        {
          repositoryPath: process.cwd(),
          language: 'java',
          environment: 'development',
          targetPlatform: 'linux/amd64',
        },
        ctx,
      );

      expect(devResult.ok).toBe(true);
      if (!devResult.ok) return;

      const devPlan = devResult.value;
      const devObsRec = [
        ...devPlan.recommendations.securityConsiderations,
        ...devPlan.recommendations.bestPractices,
      ].find((rec) => rec.id === 'org-java-observability');

      expect(devObsRec).toBeUndefined();

      policy.close();
    });

    it('injects security hardening template in production', async () => {
      const policyResult = await loadRegoPolicy(templatePolicyPath, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      const policy = policyResult.value;

      const ctx: ToolContext = {
        logger,
        policy,
        queryConfig: async <T>(packageName: string, input: Record<string, unknown>): Promise<T | null> => {
          return policy.queryConfig<T>(packageName, input);
        },
      };

      const result = await generateDockerfileTool.handler(
        {
          repositoryPath: process.cwd(),
          language: 'python',
          environment: 'production',
          targetPlatform: 'linux/amd64',
        },
        ctx,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      const plan = result.value;
      const hardeningRec = plan.recommendations.securityConsiderations.find(
        (rec) => rec.id === 'org-security-hardening',
      );

      expect(hardeningRec).toBeDefined();
      expect(hardeningRec?.policyDriven).toBe(true);
      expect(hardeningRec?.recommendation).toContain('non-root user');

      policy.close();
    });
  });

  describe('Kubernetes Template Injection', () => {
    it('injects log forwarder sidecar in production', async () => {
      const policyResult = await loadRegoPolicy(templatePolicyPath, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      const policy = policyResult.value;

      const ctx: ToolContext = {
        logger,
        policy,
        queryConfig: async <T>(packageName: string, input: Record<string, unknown>): Promise<T | null> => {
          return policy.queryConfig<T>(packageName, input);
        },
      };

      const result = await generateK8sManifestsTool.handler(
        {
          name: 'test-app',
          modulePath: process.cwd(),
          language: 'java',
          environment: 'production',
          manifestType: 'kubernetes',
        },
        ctx,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      const plan = result.value;
      const sidecarRec = plan.recommendations.bestPractices.find(
        (rec) => rec.id === 'org-log-forwarder',
      );

      expect(sidecarRec).toBeDefined();
      expect(sidecarRec?.policyDriven).toBe(true);
      expect(sidecarRec?.recommendation).toContain('Fluentd');

      policy.close();
    });

    it('injects secrets volume for all environments', async () => {
      const policyResult = await loadRegoPolicy(templatePolicyPath, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      const policy = policyResult.value;

      const ctx: ToolContext = {
        logger,
        policy,
        queryConfig: async <T>(packageName: string, input: Record<string, unknown>): Promise<T | null> => {
          return policy.queryConfig<T>(packageName, input);
        },
      };

      const result = await generateK8sManifestsTool.handler(
        {
          name: 'test-app',
          modulePath: process.cwd(),
          language: 'python',
          environment: 'development',
          manifestType: 'kubernetes',
        },
        ctx,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      const plan = result.value;
      const secretsVolumeRec = plan.recommendations.bestPractices.find(
        (rec) => rec.id === 'org-secrets-volume',
      );
      const secretsMountRec = plan.recommendations.bestPractices.find(
        (rec) => rec.id === 'org-secrets-volume-mount',
      );

      expect(secretsVolumeRec).toBeDefined();
      expect(secretsVolumeRec?.policyDriven).toBe(true);
      expect(secretsMountRec).toBeDefined();
      expect(secretsMountRec?.policyDriven).toBe(true);

      policy.close();
    });

    it('injects DB migration init container for Java apps', async () => {
      const policyResult = await loadRegoPolicy(templatePolicyPath, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      const policy = policyResult.value;

      const ctx: ToolContext = {
        logger,
        policy,
        queryConfig: async <T>(packageName: string, input: Record<string, unknown>): Promise<T | null> => {
          return policy.queryConfig<T>(packageName, input);
        },
      };

      const result = await generateK8sManifestsTool.handler(
        {
          name: 'test-app',
          modulePath: process.cwd(),
          language: 'java',
          environment: 'production',
          manifestType: 'kubernetes',
        },
        ctx,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      const plan = result.value;
      const initContainerRec = plan.recommendations.bestPractices.find(
        (rec) => rec.id === 'org-db-migration',
      );

      expect(initContainerRec).toBeDefined();
      expect(initContainerRec?.policyDriven).toBe(true);
      expect(initContainerRec?.recommendation).toContain('Flyway');

      policy.close();
    });
  });

  describe('Dynamic Defaults', () => {
    it('calculates replica count based on environment', async () => {
      const policyResult = await loadRegoPolicy(dynamicDefaultsPolicyPath, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      const policy = policyResult.value;

      const ctx: ToolContext = {
        logger,
        policy,
        queryConfig: async <T>(packageName: string, input: Record<string, unknown>): Promise<T | null> => {
          return policy.queryConfig<T>(packageName, input);
        },
      };

      // Production - should have 3 replicas
      const prodResult = await generateK8sManifestsTool.handler(
        {
          name: 'test-app',
          modulePath: process.cwd(),
          language: 'java',
          environment: 'production',
          manifestType: 'kubernetes',
        },
        ctx,
      );

      expect(prodResult.ok).toBe(true);
      if (!prodResult.ok) return;

      const prodPlan = prodResult.value;
      const prodReplicaRec = prodPlan.recommendations.resourceManagement?.find(
        (rec) => rec.id === 'policy-replica-count',
      );

      expect(prodReplicaRec).toBeDefined();
      expect(prodReplicaRec?.recommendation).toContain('3');

      policy.close();
    });

    it('calculates health check timing based on language', async () => {
      const policyResult = await loadRegoPolicy(dynamicDefaultsPolicyPath, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      const policy = policyResult.value;

      const ctx: ToolContext = {
        logger,
        policy,
        queryConfig: async <T>(packageName: string, input: Record<string, unknown>): Promise<T | null> => {
          return policy.queryConfig<T>(packageName, input);
        },
      };

      // Java - long startup (120s -> 96s initial delay)
      const javaResult = await generateK8sManifestsTool.handler(
        {
          name: 'test-app',
          modulePath: process.cwd(),
          language: 'java',
          environment: 'production',
          manifestType: 'kubernetes',
        },
        ctx,
      );

      expect(javaResult.ok).toBe(true);
      if (!javaResult.ok) return;

      const javaPlan = javaResult.value;
      const javaHealthRec = javaPlan.recommendations.bestPractices.find(
        (rec) => rec.id === 'policy-health-check-config',
      );

      expect(javaHealthRec).toBeDefined();
      expect(javaHealthRec?.recommendation).toContain('initialDelay=96s');

      policy.close();
    });

    it('calculates HPA config with traffic and criticality multipliers', async () => {
      const policyResult = await loadRegoPolicy(dynamicDefaultsPolicyPath, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      const policy = policyResult.value;

      const ctx: ToolContext = {
        logger,
        policy,
        queryConfig: async <T>(packageName: string, input: Record<string, unknown>): Promise<T | null> => {
          return policy.queryConfig<T>(packageName, input);
        },
      };

      // Production + high traffic + tier-1 = 3 * 2 * 2 = 12 replicas
      const result = await generateK8sManifestsTool.handler(
        {
          name: 'test-app',
          modulePath: process.cwd(),
          language: 'java',
          environment: 'production',
          manifestType: 'kubernetes',
          trafficLevel: 'high',
          criticalityTier: 'tier-1',
        },
        ctx,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      const plan = result.value;
      const hpaRec = plan.recommendations.resourceManagement?.find(
        (rec) => rec.id === 'policy-hpa-config',
      );

      expect(hpaRec).toBeDefined();
      expect(hpaRec?.policyDriven).toBe(true);
      expect(hpaRec?.recommendation).toContain('min=12');
      expect(hpaRec?.recommendation).toContain('max=36'); // 12 * 3

      policy.close();
    });
  });
});
