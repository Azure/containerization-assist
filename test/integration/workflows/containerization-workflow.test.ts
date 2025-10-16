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
import { existsSync } from 'node:fs';
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

  beforeAll(async () => {
    // Create temporary directory for outputs
    const result = createTestTempDir('workflow-test-');
    testDir = result.dir;
    cleanup = result.cleanup;

    // Initialize Docker test cleaner
    const dockerClient = createDockerClient(logger);
    testCleaner = new DockerTestCleaner(logger, dockerClient, { verifyCleanup: true });
  });

  afterAll(async () => {
    // Clean up Docker resources
    await testCleaner.cleanup();

    // Clean up temporary directory
    await cleanup();
  });

  describe('Single Module Application Workflow', () => {
    it('should complete full workflow for Node.js application', async () => {
      const fixturePath = join(fixtureBasePath, 'node-express');

      // Skip if fixture doesn't exist
      if (!existsSync(fixturePath)) {
        console.warn(`Skipping test: fixture not found at ${fixturePath}`);
        return;
      }

      // Step 1: Analyze repository
      const analysisResult = await analyzeRepoTool.handler({
        repositoryPath: fixturePath,
      }, toolContext);

      expect(analysisResult.ok).toBe(true);
      if (!analysisResult.ok) {
        console.error('Analysis failed:', analysisResult.error);
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
        console.error('Dockerfile generation failed:', dockerfileResult.error);
        return;
      }

      const dockerfilePlan = dockerfileResult.value as DockerfilePlan;
      expect(dockerfilePlan.recommendations.dockerfile).toBeDefined();
      expect(dockerfilePlan.recommendations.dockerfile).toContain('FROM');

      // Step 3: Build Docker image (if Docker is available)
      const imageName = `workflow-test-node:${Date.now()}`;
      const buildResult = await buildImageTool.handler({
        dockerfilePath: join(testDir.name, 'Dockerfile.node'),
        context: fixturePath,
        imageName,
      }, toolContext);

      if (buildResult.ok) {
        const build = buildResult.value as BuildResult;
        expect(build.imageId).toBeDefined();
        expect(build.imageTags).toContain(imageName);

        // Track for cleanup
        testCleaner.trackImage(build.imageId);

        // Step 4: Tag the image
        const tagResult = await tagImageTool.handler({
          source: imageName,
          tag: `workflow-test-node:latest`,
        }, toolContext);

        if (tagResult.ok) {
          testCleaner.trackImage('workflow-test-node:latest');
        }

        // Step 5: Scan image for vulnerabilities (if Docker is available)
        const scanResult = await scanImageTool.handler({
          imageId: build.imageId,
        }, toolContext);

        // Scan might fail if Trivy not installed, but should still return a result
        if (scanResult.ok) {
          const scanData = scanResult.value as any;
          expect(scanData).toBeDefined();
        }
      } else {
        console.warn('Skipping Docker operations - Docker not available:', buildResult.error);
      }

      // Step 6: Generate Kubernetes manifests
      const k8sResult = await generateK8sTool.handler({
        repositoryPath: fixturePath,
        modules: analysis.modules,
        outputPath: join(testDir.name, 'k8s-node.yaml'),
        imageName,
      }, toolContext);

      expect(k8sResult.ok).toBe(true);
      if (!k8sResult.ok) {
        console.error('K8s manifest generation failed:', k8sResult.error);
        return;
      }

      const k8sPlan = k8sResult.value as K8sManifestPlan;
      expect(k8sPlan.manifests).toBeDefined();
      expect(k8sPlan.manifests.length).toBeGreaterThan(0);
      expect(k8sPlan.manifests.some(m => m.kind === 'Deployment')).toBe(true);
      expect(k8sPlan.manifests.some(m => m.kind === 'Service')).toBe(true);
    }, testTimeout);

    it('should complete workflow for Python application', async () => {
      const fixturePath = join(fixtureBasePath, 'python-flask');

      // Skip if fixture doesn't exist
      if (!existsSync(fixturePath)) {
        console.warn(`Skipping test: fixture not found at ${fixturePath}`);
        return;
      }

      // Step 1: Analyze repository
      const analysisResult = await analyzeRepoTool.handler({
        repositoryPath: fixturePath,
      }, toolContext);

      expect(analysisResult.ok).toBe(true);
      if (!analysisResult.ok) return;

      const analysis = analysisResult.value as RepositoryAnalysis;
      expect(analysis.modules[0].language).toBe('python');

      // Step 2: Generate Dockerfile
      const dockerfileResult = await generateDockerfileTool.handler({
        repositoryPath: fixturePath,
        modules: analysis.modules,
        outputPath: join(testDir.name, 'Dockerfile.python'),
      }, toolContext);

      expect(dockerfileResult.ok).toBe(true);
      if (!dockerfileResult.ok) return;

      const dockerfilePlan = dockerfileResult.value as DockerfilePlan;
      expect(dockerfilePlan.recommendations.dockerfile).toContain('FROM python');

      // Step 3: Generate K8s manifests
      const k8sResult = await generateK8sTool.handler({
        repositoryPath: fixturePath,
        modules: analysis.modules,
        outputPath: join(testDir.name, 'k8s-python.yaml'),
      }, toolContext);

      expect(k8sResult.ok).toBe(true);
      if (!k8sResult.ok) return;

      const k8sPlan = k8sResult.value as K8sManifestPlan;
      expect(k8sPlan.manifests).toBeDefined();
      expect(k8sPlan.manifests.length).toBeGreaterThan(0);
    }, testTimeout);
  });

  describe('Workflow Error Handling', () => {
    it('should handle invalid repository path gracefully', async () => {
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
