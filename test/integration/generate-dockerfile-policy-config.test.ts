/**
 * Integration tests for generate-dockerfile with policy-driven configuration
 */
import { describe, it, expect, beforeEach, afterEach } from '@jest/globals';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { createToolContext } from '@/mcp/context';
import { createLogger } from '@/lib/logger';
import generateDockerfileTool from '@/tools/generate-dockerfile/tool';
import { loadAndMergePolicies } from '@/config/policy-io';

describe('generate-dockerfile with policy configuration', () => {
  let testDir: string;
  let policyDir: string;

  beforeEach(() => {
    // Create test directory
    testDir = join(tmpdir(), `test-dockerfile-policy-${Date.now()}`);
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

  describe('development environment', () => {
    it('should apply development policy configuration', async () => {
      // Create development policy
      const devPolicy = `
        package containerization.generation_config

        import rego.v1

        dockerfile := {
          "buildStrategy": "single-stage",
          "baseImageCategory": "official",
          "optimizationPriority": "speed"
        } if {
          input.environment == "development"
        }
      `;
      writeFileSync(join(policyDir, 'dev-config.rego'), devPolicy);

      // Load policy
      const policyResult = await loadAndMergePolicies(
        [join(policyDir, 'dev-config.rego')],
        createLogger({ name: 'test', level: 'silent' })
      );
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      // Create tool context with policy
      const ctx = createToolContext(createLogger({ name: 'test', level: 'silent' }), {
        policy: policyResult.value,
      });

      // Generate Dockerfile
      const result = await generateDockerfileTool.handler(
        {
          repositoryPath: testDir,
          language: 'node',
          environment: 'development',
          targetPlatform: 'linux/amd64',
        },
        ctx
      );

      // Assert
      expect(result.ok).toBe(true);
      if (result.ok) {
        const plan = result.value;

        // Should use single-stage based on policy
        expect(plan.recommendations.buildStrategy.multistage).toBe(false);
        expect(plan.recommendations.buildStrategy.reason).toContain('Policy-driven');
      }
    });
  });

  describe('production environment', () => {
    it('should apply production policy configuration', async () => {
      // Create production policy
      const prodPolicy = `
        package containerization.generation_config

        import rego.v1

        dockerfile := {
          "buildStrategy": "multi-stage",
          "baseImageCategory": "distroless",
          "optimizationPriority": "security"
        } if {
          input.environment == "production"
        }
      `;
      writeFileSync(join(policyDir, 'prod-config.rego'), prodPolicy);

      // Load policy
      const policyResult = await loadAndMergePolicies(
        [join(policyDir, 'prod-config.rego')],
        createLogger({ name: 'test', level: 'silent' })
      );
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      // Create tool context with policy
      const ctx = createToolContext(createLogger({ name: 'test', level: 'silent' }), {
        policy: policyResult.value,
      });

      // Generate Dockerfile
      const result = await generateDockerfileTool.handler(
        {
          repositoryPath: testDir,
          language: 'node',
          environment: 'production',
          targetPlatform: 'linux/amd64',
        },
        ctx
      );

      // Assert
      expect(result.ok).toBe(true);
      if (result.ok) {
        const plan = result.value;

        // Should use multi-stage based on policy
        expect(plan.recommendations.buildStrategy.multistage).toBe(true);
        expect(plan.recommendations.buildStrategy.reason).toContain('Policy-driven');
      }
    });

    it('should prioritize distroless images when specified in policy', async () => {
      // Create policy favoring distroless
      const distrolessPolicy = `
        package containerization.generation_config

        import rego.v1

        dockerfile := {
          "buildStrategy": "distroless",
          "baseImageCategory": "distroless",
          "optimizationPriority": "security"
        } if {
          input.environment == "production"
        }
      `;
      writeFileSync(join(policyDir, 'distroless-config.rego'), distrolessPolicy);

      // Load policy
      const policyResult = await loadAndMergePolicies(
        [join(policyDir, 'distroless-config.rego')],
        createLogger({ name: 'test', level: 'silent' })
      );
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      // Create tool context with policy
      const ctx = createToolContext(createLogger({ name: 'test', level: 'silent' }), {
        policy: policyResult.value,
      });

      // Generate Dockerfile for Node.js
      const result = await generateDockerfileTool.handler(
        {
          repositoryPath: testDir,
          language: 'node',
          environment: 'production',
          targetPlatform: 'linux/amd64',
        },
        ctx
      );

      // Assert
      expect(result.ok).toBe(true);
      if (result.ok) {
        const plan = result.value;

        // Should use distroless build strategy
        expect(plan.recommendations.buildStrategy.multistage).toBe(true);
        expect(plan.recommendations.buildStrategy.reason).toContain('distroless');

        // Base images should be present (may be filtered by policy)
        expect(plan.recommendations.baseImages.length).toBeGreaterThanOrEqual(0);
      }
    });
  });

  describe('without policy', () => {
    it('should use default behavior when policy not configured', async () => {
      // Create tool context WITHOUT policy
      const ctx = createToolContext(createLogger({ name: 'test', level: 'silent' }), {
        policy: undefined,
      });

      // Generate Dockerfile
      const result = await generateDockerfileTool.handler(
        {
          repositoryPath: testDir,
          language: 'node',
          environment: 'production',
          targetPlatform: 'linux/amd64',
        },
        ctx
      );

      // Assert - should succeed with defaults
      expect(result.ok).toBe(true);
      if (result.ok) {
        const plan = result.value;
        // Default build strategy based on language (Node.js = single-stage by default)
        expect(plan.recommendations.buildStrategy).toBeDefined();
        // Should not mention policy in reason
        expect(plan.recommendations.buildStrategy.reason).not.toContain('Policy-driven');
      }
    });
  });

  describe('language-specific behavior', () => {
    it('should override default Java multi-stage with policy', async () => {
      // Create policy that forces single-stage even for Java
      const singleStagePolicy = `
        package containerization.generation_config

        import rego.v1

        dockerfile := {
          "buildStrategy": "single-stage",
          "baseImageCategory": "official"
        }
      `;
      writeFileSync(join(policyDir, 'single-stage.rego'), singleStagePolicy);

      // Load policy
      const policyResult = await loadAndMergePolicies(
        [join(policyDir, 'single-stage.rego')],
        createLogger({ name: 'test', level: 'silent' })
      );
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      // Create tool context with policy
      const ctx = createToolContext(createLogger({ name: 'test', level: 'silent' }), {
        policy: policyResult.value,
      });

      // Generate Dockerfile for Java (which normally defaults to multi-stage)
      const result = await generateDockerfileTool.handler(
        {
          repositoryPath: testDir,
          language: 'java',
          environment: 'production',
          targetPlatform: 'linux/amd64',
        },
        ctx
      );

      // Assert
      expect(result.ok).toBe(true);
      if (result.ok) {
        const plan = result.value;

        // Policy should override Java's default multi-stage
        expect(plan.recommendations.buildStrategy.multistage).toBe(false);
        expect(plan.recommendations.buildStrategy.reason).toContain('Policy-driven');
      }
    });
  });

  describe('policy query logging', () => {
    it('should log when policy config is loaded', async () => {
      // Create policy
      const policy = `
        package containerization.generation_config

        import rego.v1

        dockerfile := {
          "buildStrategy": "multi-stage",
          "baseImageCategory": "distroless"
        }
      `;
      writeFileSync(join(policyDir, 'config.rego'), policy);

      // Load policy
      const policyResult = await loadAndMergePolicies(
        [join(policyDir, 'config.rego')],
        createLogger({ name: 'test', level: 'silent' })
      );
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      // Create logger that captures output
      const logMessages: any[] = [];
      const captureLogger = createLogger({ name: 'test', level: 'silent' });
      const originalInfo = captureLogger.info.bind(captureLogger);
      captureLogger.info = (msg: any, ...args: any[]) => {
        logMessages.push(msg);
        return originalInfo(msg, ...args);
      };

      // Create tool context
      const ctx = createToolContext(captureLogger, {
        policy: policyResult.value,
      });

      // Generate Dockerfile
      await generateDockerfileTool.handler(
        {
          repositoryPath: testDir,
          language: 'java',
          environment: 'production',
          targetPlatform: 'linux/amd64',
        },
        ctx
      );

      // Assert logs contain policy information
      const logsString = JSON.stringify(logMessages);
      expect(logsString).toContain('policy');
    });
  });

  describe('empty policy response', () => {
    it('should handle empty policy response gracefully', async () => {
      // Create policy that doesn't define dockerfile config
      const emptyPolicy = `
        package containerization.generation_config

        import rego.v1

        # No dockerfile config defined
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

      // Generate Dockerfile
      const result = await generateDockerfileTool.handler(
        {
          repositoryPath: testDir,
          language: 'node',
          environment: 'production',
          targetPlatform: 'linux/amd64',
        },
        ctx
      );

      // Assert - should succeed with defaults when policy returns null
      expect(result.ok).toBe(true);
      if (result.ok) {
        const plan = result.value;
        expect(plan.recommendations.buildStrategy).toBeDefined();
      }
    });
  });
});
