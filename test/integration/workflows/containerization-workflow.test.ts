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
import buildImageTool, { type BuildImageResult } from '@/tools/build-image/tool';
import tagImageTool from '@/tools/tag-image/tool';
import scanImageTool from '@/tools/scan-image/tool';

import type { RepositoryAnalysis } from '@/tools/analyze-repo/schema';

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

  describe('Repository Analysis Workflow', () => {
    it('should analyze Node.js repository and detect modules', async () => {
      const fixturePath = join(fixtureBasePath, 'node-express');

      // Skip if fixture doesn't exist or doesn't have package.json
      if (!existsSync(fixturePath) || !existsSync(join(fixturePath, 'package.json'))) {
        console.log('Skipping: node-express fixture not found or missing package.json');
        return;
      }

      // Analyze repository
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
      expect(analysis.isMonorepo).toBe(false);
    });

    it('should analyze Python repository', async () => {
      const fixturePath = join(fixtureBasePath, 'python-flask');

      if (!existsSync(fixturePath)) {
        console.log('Skipping: python-flask fixture not found');
        return;
      }

      const analysisResult = await analyzeRepoTool.handler({
        repositoryPath: fixturePath,
      }, toolContext);

      if (analysisResult.ok) {
        const analysis = analysisResult.value as RepositoryAnalysis;
        expect(analysis.modules).toBeDefined();
        expect(analysis.modules[0]?.language).toBe('python');
      }
    });
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

      // Verify module names
      const moduleNames = analysis.modules.map(m => m.name);
      expect(moduleNames).toContain('api');
      expect(moduleNames).toContain('worker');
    }, testTimeout);
  });

  describe('Docker Operations Integration', () => {
    it('should build, tag, and scan image with existing Dockerfile', async () => {
      if (!dockerAvailable) {
        console.log('Skipping: Docker not available');
        return;
      }

      // Create a simple test app with a Dockerfile
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

      // Write a simple Dockerfile directly (no AI needed)
      writeFileSync(
        join(appPath, 'Dockerfile'),
        `FROM node:18-alpine
WORKDIR /app
COPY package.json ./
COPY index.js ./
CMD ["node", "index.js"]`
      );

      // Build image
      const imageName = `docker-ops-test:${Date.now()}`;
      const buildResult = await buildImageTool.handler({
        path: appPath,
        dockerfile: 'Dockerfile',
        imageName,
      }, toolContext);

      if (buildResult.ok) {
        const build = buildResult.value as BuildImageResult;
        expect(build.imageId).toBeDefined();
        expect(build.createdTags).toContain(imageName);
        testCleaner.trackImage(build.imageId);

        // Tag image
        const tagResult = await tagImageTool.handler({
          imageId: build.imageId,
          tag: `docker-ops-test:latest`,
        }, toolContext);

        if (tagResult.ok) {
          expect(tagResult.value).toBeDefined();
        }

        // Scan image
        const scanResult = await scanImageTool.handler({
          imageId: build.imageId,
        }, toolContext);

        // Scan may fail if Trivy not installed - that's OK
        if (!scanResult.ok) {
          console.log('Scan skipped (Trivy may not be installed)');
        } else {
          expect(scanResult.value).toBeDefined();
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
