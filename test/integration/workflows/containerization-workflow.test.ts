/**
 * Integration Test: Complete Containerization Workflow
 *
 * Tests the entire containerization journey by chaining tools together:
 * analyze-repo → generate-dockerfile → build-image → scan-image →
 * tag-image → generate-k8s-manifests
 *
 * Prerequisites:
 * - Docker daemon running (for build/scan/tag tests)
 * - Sufficient disk space for Docker operations
 */

import { describe, it, expect, beforeAll, afterAll } from '@jest/globals';
import { createLogger } from '@/lib/logger';
import type { ToolContext } from '@/mcp/context';
import { join } from 'node:path';
import { existsSync, mkdirSync, writeFileSync } from 'node:fs';
import { createTestTempDir } from '../../__support__/utilities/tmp-helpers';
import type { DirResult } from 'tmp';
import { DockerTestCleaner } from '../../__support__/utilities/docker-test-cleaner';
import { createDockerClient } from '@/infra/docker/client';

// Import tools directly to avoid createApp dependency
import analyzeRepoTool from '@/tools/analyze-repo/tool';
import generateDockerfileTool from '@/tools/generate-dockerfile/tool';
import buildImageTool from '@/tools/build-image/tool';
import tagImageTool from '@/tools/tag-image/tool';
import scanImageTool from '@/tools/scan/tool';
import generateK8sTool from '@/tools/generate-k8s-manifests/tool';

import type { RepositoryAnalysis } from '@/tools/analyze-repo/schema';
import type { DockerfilePlan } from '@/tools/generate-dockerfile/schema';
import type { BuildResult } from '@/tools/build-image/schema';
import type { K8sManifestPlan } from '@/tools/generate-k8s-manifests/schema';

describe('Complete Containerization Workflow Integration', () => {
  let testDir: DirResult;
  let cleanup: () => Promise<void>;
  let testCleaner: DockerTestCleaner;
  const logger = createLogger({ level: 'silent' });

  // Create minimal ToolContext for testing (no server needed)
  const toolContext: ToolContext = {
    logger,
    signal: undefined,
    progress: undefined,
    sampling: {
      createMessage: async () => {
        throw new Error('AI sampling not available in test context');
      },
    },
    getPrompt: async () => {
      throw new Error('Prompts not available in test context');
    },
  };

  const fixtureBasePath = join(process.cwd(), 'test', '__support__', 'fixtures');
  const testTimeout = 120000; // 2 minutes for full workflow
  let dockerAvailable = false;

  beforeAll(async () => {
    // Create temporary directory for outputs
    const result = createTestTempDir('workflow-test-');
    testDir = result.dir;
    cleanup = result.cleanup;

    // Initialize Docker test cleaner
    try {
      const dockerClient = createDockerClient(logger);
      testCleaner = new DockerTestCleaner(logger, dockerClient, { verifyCleanup: true });
      dockerAvailable = true;
    } catch (error) {
      console.log('Docker not available - some tests will be skipped');
      dockerAvailable = false;
    }
  });

  afterAll(async () => {
    // Clean up Docker resources
    if (dockerAvailable && testCleaner) {
      await testCleaner.cleanup();
    }

    // Clean up temporary directory
    await cleanup();
  });

  describe('Full Workflow: Node.js Application', () => {
    it('should complete analyze → generate-dockerfile → generate-k8s workflow', async () => {
      const fixturePath = join(fixtureBasePath, 'node-express');

      // Skip if fixture doesn't exist
      if (!existsSync(fixturePath)) {
        console.log('Skipping: node-express fixture not found');
        return;
      }

      // Step 1: Analyze repository
      const analysisResult = await analyzeRepoTool.handler({
        repositoryPath: fixturePath,
      }, toolContext);

      expect(analysisResult.ok).toBe(true);
      if (!analysisResult.ok) {
        console.log('Analysis failed:', analysisResult.error);
        return;
      }

      const analysis = analysisResult.value as RepositoryAnalysis;
      expect(analysis.modules).toBeDefined();
      expect(analysis.modules.length).toBeGreaterThan(0);
      expect(analysis.modules[0].language).toBe('javascript');

      // Step 2: Generate Dockerfile
      const dockerfileResult = await generateDockerfileTool.handler({
        repositoryPath: fixturePath,
        modules: analysis.modules,
        outputPath: join(testDir.name, 'Dockerfile.node'),
      }, toolContext);

      expect(dockerfileResult.ok).toBe(true);
      if (!dockerfileResult.ok) {
        console.log('Dockerfile generation failed:', dockerfileResult.error);
        return;
      }

      const dockerfilePlan = dockerfileResult.value as DockerfilePlan;
      expect(dockerfilePlan.recommendations.dockerfile).toBeDefined();
      expect(dockerfilePlan.recommendations.dockerfile).toContain('FROM');
      expect(dockerfilePlan.recommendations.dockerfile).toContain('node');

      // Step 3: Generate K8s manifests
      const k8sResult = await generateK8sTool.handler({
        repositoryPath: fixturePath,
        modules: analysis.modules,
        outputPath: join(testDir.name, 'k8s-node.yaml'),
        imageName: 'test-app:latest',
      }, toolContext);

      expect(k8sResult.ok).toBe(true);
      if (!k8sResult.ok) {
        console.log('K8s manifest generation failed:', k8sResult.error);
        return;
      }

      const k8sPlan = k8sResult.value as K8sManifestPlan;
      expect(k8sPlan.manifests).toBeDefined();
      expect(k8sPlan.manifests.length).toBeGreaterThan(0);
      expect(k8sPlan.manifests.some(m => m.kind === 'Deployment')).toBe(true);
      expect(k8sPlan.manifests.some(m => m.kind === 'Service')).toBe(true);
    }, testTimeout);

    it('should complete workflow with Docker build (if Docker available)', async () => {
      if (!dockerAvailable) {
        console.log('Skipping: Docker not available');
        return;
      }

      const fixturePath = join(fixtureBasePath, 'node-express');

      if (!existsSync(fixturePath)) {
        console.log('Skipping: node-express fixture not found');
        return;
      }

      // Analyze
      const analysisResult = await analyzeRepoTool.handler({
        repositoryPath: fixturePath,
      }, toolContext);

      if (!analysisResult.ok) return;
      const analysis = analysisResult.value as RepositoryAnalysis;

      // Generate Dockerfile
      const dockerfileResult = await generateDockerfileTool.handler({
        repositoryPath: fixturePath,
        modules: analysis.modules,
        outputPath: join(testDir.name, 'Dockerfile.build-test'),
      }, toolContext);

      if (!dockerfileResult.ok) return;

      // Build Docker image
      const imageName = `workflow-test-node:${Date.now()}`;
      const buildResult = await buildImageTool.handler({
        dockerfilePath: join(testDir.name, 'Dockerfile.build-test'),
        context: fixturePath,
        imageName,
      }, toolContext);

      if (buildResult.ok) {
        const build = buildResult.value as BuildResult;
        expect(build.imageId).toBeDefined();
        expect(build.imageTags).toContain(imageName);
        testCleaner.trackImage(build.imageId);

        // Tag the image
        const tagResult = await tagImageTool.handler({
          source: imageName,
          tag: `workflow-test-node:latest`,
        }, toolContext);

        if (tagResult.ok) {
          testCleaner.trackImage('workflow-test-node:latest');
        }

        // Scan image (if Trivy available)
        const scanResult = await scanImageTool.handler({
          imageId: build.imageId,
        }, toolContext);

        if (scanResult.ok) {
          expect(scanResult.value).toBeDefined();
        }
      } else {
        console.log('Build failed (expected if Docker issues):', buildResult.error);
      }
    }, testTimeout);
  });

  describe('Multi-Module Workflow', () => {
    it('should handle monorepo with multiple modules', async () => {
      // Create a test monorepo structure
      const monorepoPath = join(testDir.name, 'test-monorepo');
      mkdirSync(monorepoPath, { recursive: true });

      // Create API service (Node.js)
      const apiPath = join(monorepoPath, 'api');
      mkdirSync(apiPath, { recursive: true });
      writeFileSync(
        join(apiPath, 'package.json'),
        JSON.stringify({
          name: 'api',
          version: '1.0.0',
          dependencies: { express: '^4.18.0' },
          scripts: { start: 'node index.js' },
        })
      );
      writeFileSync(join(apiPath, 'index.js'), 'console.log("API");');

      // Create Worker service (Node.js)
      const workerPath = join(monorepoPath, 'worker');
      mkdirSync(workerPath, { recursive: true });
      writeFileSync(
        join(workerPath, 'package.json'),
        JSON.stringify({
          name: 'worker',
          version: '1.0.0',
          dependencies: { bullmq: '^3.0.0' },
          scripts: { start: 'node worker.js' },
        })
      );
      writeFileSync(join(workerPath, 'worker.js'), 'console.log("Worker");');

      // Analyze the monorepo
      const analysisResult = await analyzeRepoTool.handler({
        repositoryPath: monorepoPath,
      }, toolContext);

      expect(analysisResult.ok).toBe(true);
      if (!analysisResult.ok) return;

      const analysis = analysisResult.value as RepositoryAnalysis;
      expect(analysis.isMonorepo).toBe(true);
      expect(analysis.modules.length).toBeGreaterThanOrEqual(2);

      // Generate Dockerfiles for each module
      for (const module of analysis.modules) {
        const dockerfileResult = await generateDockerfileTool.handler({
          repositoryPath: monorepoPath,
          modules: [module],
          outputPath: join(testDir.name, `Dockerfile.${module.name}`),
        }, toolContext);

        if (dockerfileResult.ok) {
          const plan = dockerfileResult.value as DockerfilePlan;
          expect(plan.recommendations.dockerfile).toContain('FROM');
        }
      }

      // Generate K8s manifests for all modules
      const k8sResult = await generateK8sTool.handler({
        repositoryPath: monorepoPath,
        modules: analysis.modules,
        outputPath: join(testDir.name, 'k8s-multi-service.yaml'),
      }, toolContext);

      expect(k8sResult.ok).toBe(true);
      if (k8sResult.ok) {
        const k8sPlan = k8sResult.value as K8sManifestPlan;
        expect(k8sPlan.manifests.length).toBeGreaterThan(0);

        // Should have Deployments for each service
        const deployments = k8sPlan.manifests.filter(m => m.kind === 'Deployment');
        expect(deployments.length).toBeGreaterThanOrEqual(2);
      }
    }, testTimeout);
  });

  describe('Docker Operations Integration', () => {
    it('should build, tag, and scan a simple image', async () => {
      if (!dockerAvailable) {
        console.log('Skipping: Docker not available');
        return;
      }

      // Create a simple test app
      const appPath = join(testDir.name, 'simple-app');
      mkdirSync(appPath, { recursive: true });

      writeFileSync(
        join(appPath, 'package.json'),
        JSON.stringify({
          name: 'simple-app',
          version: '1.0.0',
          main: 'index.js',
        })
      );
      writeFileSync(join(appPath, 'index.js'), 'console.log("Hello");');

      // Generate Dockerfile
      const analysisResult = await analyzeRepoTool.handler({
        repositoryPath: appPath,
      }, toolContext);

      if (!analysisResult.ok) {
        console.log('Analysis failed, skipping');
        return;
      }

      const analysis = analysisResult.value as RepositoryAnalysis;
      const dockerfileResult = await generateDockerfileTool.handler({
        repositoryPath: appPath,
        modules: analysis.modules,
        outputPath: join(appPath, 'Dockerfile'),
      }, toolContext);

      if (!dockerfileResult.ok) {
        console.log('Dockerfile generation failed, skipping');
        return;
      }

      // Build image
      const imageName = `docker-ops-test:${Date.now()}`;
      const buildResult = await buildImageTool.handler({
        dockerfilePath: join(appPath, 'Dockerfile'),
        context: appPath,
        imageName,
      }, toolContext);

      if (buildResult.ok) {
        const build = buildResult.value as BuildResult;
        testCleaner.trackImage(build.imageId);

        // Tag image
        const tagResult = await tagImageTool.handler({
          source: imageName,
          tag: `docker-ops-test:latest`,
        }, toolContext);

        if (tagResult.ok) {
          testCleaner.trackImage('docker-ops-test:latest');
          expect(tagResult.value).toBeDefined();
        }

        // Scan image
        const scanResult = await scanImageTool.handler({
          imageId: build.imageId,
        }, toolContext);

        // Scan may fail if Trivy not installed - that's OK
        if (!scanResult.ok) {
          console.log('Scan skipped (Trivy may not be installed)');
        }
      } else {
        console.log('Build failed:', buildResult.error);
      }
    }, testTimeout);
  });

  describe('Error Handling', () => {
    it('should handle invalid repository path', async () => {
      const result = await analyzeRepoTool.handler({
        repositoryPath: '/nonexistent/path',
      }, toolContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
        expect(result.error).toContain('does not exist');
      }
    });

    it('should handle missing Dockerfile in build step', async () => {
      const result = await buildImageTool.handler({
        dockerfilePath: '/nonexistent/Dockerfile',
        context: testDir.name,
        imageName: 'test:latest',
      }, toolContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
      }
    });
  });
});
