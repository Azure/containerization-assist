/**
 * End-to-end workflow test for template injection and dynamic defaults
 */
import { describe, it, expect, beforeAll, afterAll } from '@jest/globals';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { createLogger } from '@/lib/logger';
import { loadRegoPolicy } from '@/config/policy-rego';
import generateDockerfileTool from '@/tools/generate-dockerfile/tool';
import generateK8sManifestsTool from '@/tools/generate-k8s-manifests/tool';
import type { ToolContext } from '@/mcp/context';

describe('Template Injection Workflow', () => {
  let testDir: string;
  let policyPath: string;
  const logger = createLogger({ name: 'test', level: 'silent' });

  beforeAll(() => {
    // Create test directory
    testDir = join(tmpdir(), `workflow-templates-${Date.now()}`);
    mkdirSync(testDir, { recursive: true });

    // Create a policy with templates (one package per file - OPA limitation)
    policyPath = join(testDir, 'workflow-policy.rego');
    const policyContent = `
package containerization.templates

import rego.v1

# CA certificates for all
ca_cert_template := {
  "id": "org-ca-certs",
  "section": "security",
  "description": "Install organization CA certificates",
  "content": "COPY certs/org-ca.crt /usr/local/share/ca-certificates/org-ca.crt\\nRUN update-ca-certificates",
  "priority": 100
}

dockerfile_templates contains ca_cert_template

# Production security hardening
security_hardening := {
  "id": "org-security-hardening",
  "section": "security",
  "description": "Apply security hardening",
  "content": "RUN useradd -r -u 1001 -g root appuser\\nUSER appuser",
  "priority": 80,
  "conditions": {
    "environments": ["production"]
  }
}

dockerfile_templates contains security_hardening if {
  input.environment == "production"
}

# Log forwarder sidecar
log_sidecar := {
  "id": "log-forwarder",
  "type": "sidecar",
  "description": "Fluentd sidecar for log aggregation",
  "spec": {
    "name": "log-forwarder",
    "image": "fluent/fluentd:v1.16-1"
  },
  "priority": 90,
  "conditions": {
    "environments": ["production"]
  }
}

kubernetes_templates contains log_sidecar if {
  input.environment == "production"
}

templates := {
  "dockerfile": [template | template := dockerfile_templates[_]],
  "kubernetes": [template | template := kubernetes_templates[_]]
}
    `;
    writeFileSync(policyPath, policyContent);

    // Create minimal package.json
    writeFileSync(
      join(testDir, 'package.json'),
      JSON.stringify({ name: 'test-app', version: '1.0.0' })
    );
  });

  afterAll(() => {
    rmSync(testDir, { recursive: true, force: true });
  });

  it('should inject templates throughout complete workflow', async () => {
    // Load policy
    const policyResult = await loadRegoPolicy(policyPath, logger);
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

    // Step 1: Generate Dockerfile for production Node.js app
    const dockerfileResult = await generateDockerfileTool.handler(
      {
        repositoryPath: testDir,
        language: 'node',
        environment: 'production',
        targetPlatform: 'linux/amd64',
      },
      ctx
    );

    expect(dockerfileResult.ok).toBe(true);
    if (!dockerfileResult.ok) return;

    const dockerfilePlan = dockerfileResult.value;

    // Should have CA cert template (all environments)
    const allDockerfileRecs = [
      ...dockerfilePlan.recommendations.securityConsiderations,
      ...dockerfilePlan.recommendations.bestPractices,
    ];
    const caCertRec = allDockerfileRecs.find((r) => r.id === 'org-ca-certs');
    expect(caCertRec).toBeDefined();
    expect(caCertRec?.policyDriven).toBe(true);

    // Should have security hardening (production only)
    const hardeningRec = allDockerfileRecs.find((r) => r.id === 'org-security-hardening');
    expect(hardeningRec).toBeDefined();
    expect(hardeningRec?.policyDriven).toBe(true);

    // Step 2: Generate K8s manifests for same app
    const k8sResult = await generateK8sManifestsTool.handler(
      {
        name: 'test-app',
        modulePath: testDir,
        language: 'node',
        environment: 'production',
        manifestType: 'kubernetes',
      },
      ctx
    );

    expect(k8sResult.ok).toBe(true);
    if (!k8sResult.ok) return;

    const k8sPlan = k8sResult.value;

    // Should have log forwarder sidecar (production only)
    const allK8sRecs = [
      ...k8sPlan.recommendations.securityConsiderations,
      ...k8sPlan.recommendations.bestPractices,
    ];
    const sidecarRec = allK8sRecs.find((r) => r.id === 'log-forwarder');
    expect(sidecarRec).toBeDefined();
    expect(sidecarRec?.policyDriven).toBe(true);

    policy.close();
  });

  it('should NOT inject production templates in development', async () => {
    // Load policy
    const policyResult = await loadRegoPolicy(policyPath, logger);
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

    // Generate Dockerfile for development
    const dockerfileResult = await generateDockerfileTool.handler(
      {
        repositoryPath: testDir,
        language: 'node',
        environment: 'development',
        targetPlatform: 'linux/amd64',
      },
      ctx
    );

    expect(dockerfileResult.ok).toBe(true);
    if (!dockerfileResult.ok) return;

    const dockerfilePlan = dockerfileResult.value;

    const allRecs = [
      ...dockerfilePlan.recommendations.securityConsiderations,
      ...dockerfilePlan.recommendations.bestPractices,
    ];

    // Should have CA certs (all environments)
    const caCertRec = allRecs.find((r) => r.id === 'org-ca-certs');
    expect(caCertRec).toBeDefined();

    // Should NOT have security hardening (production only)
    const hardeningRec = allRecs.find((r) => r.id === 'org-security-hardening');
    expect(hardeningRec).toBeUndefined();

    policy.close();
  });
});
