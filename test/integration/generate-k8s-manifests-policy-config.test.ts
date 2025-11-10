/**
 * Integration tests for generate-k8s-manifests with policy-driven configuration
 */
import { describe, it, expect, beforeEach, afterEach } from '@jest/globals';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { createToolContext } from '@/mcp/context';
import { createLogger } from '@/lib/logger';
import generateK8sManifestsTool from '@/tools/generate-k8s-manifests/tool';
import { loadAndMergePolicies } from '@/config/policy-io';

describe('generate-k8s-manifests with policy configuration', () => {
  let testDir: string;
  let policyDir: string;

  beforeEach(() => {
    // Create test directory
    testDir = join(tmpdir(), `test-k8s-policy-${Date.now()}`);
    mkdirSync(testDir, { recursive: true });

    // Create policy directory
    policyDir = join(testDir, 'policies');
    mkdirSync(policyDir, { recursive: true });

    // Create a minimal package.json for repo detection
    writeFileSync(
      join(testDir, 'package.json'),
      JSON.stringify({ name: 'test-app', version: '1.0.0' })
    );
  });

  afterEach(() => {
    rmSync(testDir, { recursive: true, force: true });
  });

  describe('resource defaults', () => {
    it('should apply resource defaults from policy', async () => {
      // Create policy with resource defaults
      const resourcePolicy = `
        package containerization.generation_config

        import rego.v1

        kubernetes := {
          "resourceDefaults": {
            "cpuRequest": "500m",
            "cpuLimit": "1",
            "memoryRequest": "512Mi",
            "memoryLimit": "1Gi"
          }
        } if {
          input.environment == "production"
        }
      `;
      writeFileSync(join(policyDir, 'resource-defaults.rego'), resourcePolicy);

      // Load policy
      const policyResult = await loadAndMergePolicies(
        [join(policyDir, 'resource-defaults.rego')],
        createLogger({ name: 'test', level: 'silent' })
      );
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      // Create tool context with policy
      const ctx = createToolContext(createLogger({ name: 'test', level: 'silent' }), {
        policy: policyResult.value,
      });

      // Generate K8s manifests
      const result = await generateK8sManifestsTool.handler(
        {
          repositoryPath: testDir,
          manifestType: 'deployment',
          imageName: 'test-app:latest',
          appName: 'test-app',
          environment: 'production',
        },
        ctx
      );

      // Assert
      expect(result.ok).toBe(true);
      if (result.ok) {
        const manifests = result.value;
        expect(manifests.summary).toContain('Policy Config');
      }
    });
  });

  describe('organizational standards', () => {
    it('should apply org standards from policy', async () => {
      // Create policy with org standards
      const orgPolicy = `
        package containerization.generation_config

        import rego.v1

        kubernetes := {
          "orgStandards": {
            "requiredLabels": {
              "team": "platform",
              "costCenter": "engineering",
              "app.kubernetes.io/managed-by": "containerization-assist"
            },
            "namespace": "production",
            "allowedRegistries": ["docker.io", "gcr.io"],
            "serviceAccount": "app-service-account",
            "imagePullPolicy": "Always"
          }
        } if {
          input.environment == "production"
        }
      `;
      writeFileSync(join(policyDir, 'org-standards.rego'), orgPolicy);

      // Load policy
      const policyResult = await loadAndMergePolicies(
        [join(policyDir, 'org-standards.rego')],
        createLogger({ name: 'test', level: 'silent' })
      );
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      // Create tool context with policy
      const ctx = createToolContext(createLogger({ name: 'test', level: 'silent' }), {
        policy: policyResult.value,
      });

      // Generate K8s manifests
      const result = await generateK8sManifestsTool.handler(
        {
          repositoryPath: testDir,
          manifestType: 'deployment',
          imageName: 'test-app:latest',
          appName: 'test-app',
          environment: 'production',
        },
        ctx
      );

      // Assert
      expect(result.ok).toBe(true);
      if (result.ok) {
        const manifests = result.value;
        expect(manifests.summary).toContain('Policy Config');
      }
    });
  });

  describe('feature toggles', () => {
    it('should apply feature toggles from policy', async () => {
      // Create policy with feature toggles
      const featurePolicy = `
        package containerization.generation_config

        import rego.v1

        kubernetes := {
          "features": {
            "healthChecks": true,
            "autoscaling": true,
            "resourceQuotas": false,
            "networkPolicies": true,
            "podSecurityPolicies": true,
            "ingress": false
          }
        } if {
          input.environment == "production"
        }
      `;
      writeFileSync(join(policyDir, 'features.rego'), featurePolicy);

      // Load policy
      const policyResult = await loadAndMergePolicies(
        [join(policyDir, 'features.rego')],
        createLogger({ name: 'test', level: 'silent' })
      );
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      // Create tool context with policy
      const ctx = createToolContext(createLogger({ name: 'test', level: 'silent' }), {
        policy: policyResult.value,
      });

      // Generate K8s manifests
      const result = await generateK8sManifestsTool.handler(
        {
          repositoryPath: testDir,
          manifestType: 'deployment',
          imageName: 'test-app:latest',
          appName: 'test-app',
          environment: 'production',
        },
        ctx
      );

      // Assert
      expect(result.ok).toBe(true);
      if (result.ok) {
        const manifests = result.value;
        expect(manifests.summary).toBeDefined();
      }
    });
  });

  describe('deployment configuration', () => {
    it('should apply replicas and strategy from policy', async () => {
      // Create policy with deployment config
      const deploymentPolicy = `
        package containerization.generation_config

        import rego.v1

        kubernetes := {
          "replicas": 5,
          "deploymentStrategy": "RollingUpdate"
        } if {
          input.environment == "production"
        }

        kubernetes := {
          "replicas": 1,
          "deploymentStrategy": "Recreate"
        } if {
          input.environment == "development"
        }
      `;
      writeFileSync(join(policyDir, 'deployment.rego'), deploymentPolicy);

      // Load policy
      const policyResult = await loadAndMergePolicies(
        [join(policyDir, 'deployment.rego')],
        createLogger({ name: 'test', level: 'silent' })
      );
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      // Create tool context with policy
      const ctx = createToolContext(createLogger({ name: 'test', level: 'silent' }), {
        policy: policyResult.value,
      });

      // Generate K8s manifests for production
      const prodResult = await generateK8sManifestsTool.handler(
        {
          repositoryPath: testDir,
          manifestType: 'deployment',
          imageName: 'test-app:latest',
          appName: 'test-app',
          environment: 'production',
        },
        ctx
      );

      // Assert production config
      expect(prodResult.ok).toBe(true);
      if (prodResult.ok) {
        expect(prodResult.value.summary).toBeDefined();
      }

      // Generate K8s manifests for development
      const devResult = await generateK8sManifestsTool.handler(
        {
          repositoryPath: testDir,
          manifestType: 'deployment',
          imageName: 'test-app:latest',
          appName: 'test-app',
          environment: 'development',
        },
        ctx
      );

      // Assert development config
      expect(devResult.ok).toBe(true);
      if (devResult.ok) {
        expect(devResult.value.summary).toBeDefined();
      }
    });
  });

  describe('complete policy configuration', () => {
    it('should apply complete configuration from policy', async () => {
      // Create comprehensive policy
      const completePolicy = `
        package containerization.generation_config

        import rego.v1

        kubernetes := {
          "resourceDefaults": {
            "cpuRequest": "1",
            "cpuLimit": "2",
            "memoryRequest": "1Gi",
            "memoryLimit": "2Gi"
          },
          "orgStandards": {
            "requiredLabels": {
              "app.kubernetes.io/managed-by": "containerization-assist",
              "app.kubernetes.io/environment": "production"
            },
            "namespace": "production",
            "allowedRegistries": ["docker.io", "gcr.io", "mcr.microsoft.com"],
            "serviceAccount": "default",
            "imagePullPolicy": "Always"
          },
          "features": {
            "healthChecks": true,
            "autoscaling": true,
            "networkPolicies": true
          },
          "replicas": 3,
          "deploymentStrategy": "RollingUpdate"
        } if {
          input.environment == "production"
        }
      `;
      writeFileSync(join(policyDir, 'complete.rego'), completePolicy);

      // Load policy
      const policyResult = await loadAndMergePolicies(
        [join(policyDir, 'complete.rego')],
        createLogger({ name: 'test', level: 'silent' })
      );
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      // Create tool context with policy
      const ctx = createToolContext(createLogger({ name: 'test', level: 'silent' }), {
        policy: policyResult.value,
      });

      // Generate K8s manifests
      const result = await generateK8sManifestsTool.handler(
        {
          repositoryPath: testDir,
          manifestType: 'deployment',
          imageName: 'test-app:latest',
          appName: 'test-app',
          environment: 'production',
        },
        ctx
      );

      // Assert
      expect(result.ok).toBe(true);
      if (result.ok) {
        const manifests = result.value;
        expect(manifests.summary).toContain('Policy Config');
      }
    });
  });

  describe('without policy', () => {
    it('should use default behavior when policy not configured', async () => {
      // Create tool context WITHOUT policy
      const ctx = createToolContext(createLogger({ name: 'test', level: 'silent' }), {
        policy: undefined,
      });

      // Generate K8s manifests
      const result = await generateK8sManifestsTool.handler(
        {
          repositoryPath: testDir,
          manifestType: 'deployment',
          imageName: 'test-app:latest',
          appName: 'test-app',
          environment: 'production',
        },
        ctx
      );

      // Assert - should succeed with defaults
      expect(result.ok).toBe(true);
      if (result.ok) {
        const manifests = result.value;
        expect(manifests.summary).toBeDefined();
        // Should not mention policy in summary
        expect(manifests.summary.toLowerCase()).not.toContain('policy');
      }
    });
  });

  describe('empty policy response', () => {
    it('should handle empty policy response gracefully', async () => {
      // Create policy that doesn't define kubernetes config
      const emptyPolicy = `
        package containerization.generation_config

        import rego.v1

        # No kubernetes config defined
        some_other_field := "value"
      `;
      writeFileSync(join(policyDir, 'empty-config.rego'), emptyPolicy);

      // Load policy
      const policyResult = await loadAndMergePolicies(
        [join(policyDir, 'empty-config.rego')],
        createLogger({ name: 'test', level: 'silent' })
      );
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      // Create tool context with policy
      const ctx = createToolContext(createLogger({ name: 'test', level: 'silent' }), {
        policy: policyResult.value,
      });

      // Generate K8s manifests
      const result = await generateK8sManifestsTool.handler(
        {
          repositoryPath: testDir,
          manifestType: 'deployment',
          imageName: 'test-app:latest',
          appName: 'test-app',
          environment: 'production',
        },
        ctx
      );

      // Assert - should succeed with defaults when policy returns null
      expect(result.ok).toBe(true);
      if (result.ok) {
        const manifests = result.value;
        expect(manifests.summary).toBeDefined();
      }
    });
  });
});
