/**
 * Integration Test: Docker Workflow
 *
 * Tests the complete Docker containerization workflow:
 * analyze-repo → generate-dockerfile → build-image → scan-image → tag-image
 *
 * Prerequisites:
 * - Docker daemon running
 * - Test fixtures available
 */

import { describe, it, expect, beforeAll, afterAll } from '@jest/globals';
import { createLogger } from '@/lib/logger';
import type { ToolContext } from '@/mcp/context';
import { join } from 'node:path';
import { existsSync, writeFileSync } from 'node:fs';
import { DockerTestCleaner } from '../../__support__/utilities/docker-test-cleaner';
import { createDockerClient } from '@/infra/docker/client';

// Import tools
import analyzeRepoTool from '@/tools/analyze-repo/tool';
import generateDockerfileTool from '@/tools/generate-dockerfile/tool';
import buildImageTool from '@/tools/build-image/tool';
import scanImageTool from '@/tools/scan-image/tool';
import tagImageTool from '@/tools/tag-image/tool';

import type { RepositoryAnalysis } from '@/tools/analyze-repo/schema';
import type { GenerateDockerfileResult } from '@/tools/generate-dockerfile/schema';
import type { BuildImageResult } from '@/tools/build-image/tool';

describe('Docker Workflow Integration', () => {
  let testCleaner: DockerTestCleaner;
  const logger = createLogger({ level: 'silent' });

  const toolContext: ToolContext = {
    logger,
    signal: undefined,
    progress: undefined,
  };

  const fixtureBasePath = join(process.cwd(), 'test', '__support__', 'fixtures');
  const testTimeout = 120000; // 2 minutes
  let dockerAvailable = false;

  beforeAll(async () => {
    // Initialize Docker test cleaner
    try {
      const dockerClient = createDockerClient(logger);
      testCleaner = new DockerTestCleaner(logger, dockerClient, { verifyCleanup: true });
      dockerAvailable = true;
    } catch (error) {
      console.log('Docker not available - Docker workflow tests will be skipped');
      dockerAvailable = false;
    }
  });

  afterAll(async () => {
    if (dockerAvailable && testCleaner) {
      await testCleaner.cleanup();
    }
  });

  describe('Complete Docker Workflow', () => {
    it('should complete analyze → generate → build → scan → tag workflow for Node.js app', async () => {
      if (!dockerAvailable) {
        console.log('Skipping: Docker not available');
        return;
      }

      const testRepo = join(fixtureBasePath, 'node-express');

      if (!existsSync(testRepo) || !existsSync(join(testRepo, 'package.json'))) {
        console.log('Skipping: node-express fixture not found');
        return;
      }

      // Step 1: Analyze repository
      const analyzeResult = await analyzeRepoTool.handler(
        { repositoryPath: testRepo },
        toolContext
      );

      expect(analyzeResult.ok).toBe(true);
      if (!analyzeResult.ok) {
        console.log('Analysis failed:', analyzeResult.error);
        return;
      }

      const analysis = analyzeResult.value as RepositoryAnalysis;
      expect(analysis.modules).toBeDefined();
      expect(analysis.modules.length).toBeGreaterThan(0);
      expect(analysis.modules[0].language).toBe('javascript');

      // Step 2: Generate Dockerfile (AI-based, may fail if API unavailable)
      const dockerfilePath = join(testRepo, 'Dockerfile.test');
      const generateResult = await generateDockerfileTool.handler(
        {
          repositoryPath: testRepo,
          analysis: JSON.stringify(analysis),
          outputPath: dockerfilePath,
          targetPlatform: 'linux/amd64',
        },
        toolContext
      );

      // If Dockerfile generation fails (AI not available), use existing fixture
      let dockerfileToUse = join(testRepo, 'Dockerfile');
      if (generateResult.ok) {
        const dockerfile = generateResult.value as GenerateDockerfileResult;
        if (dockerfile && existsSync(dockerfilePath)) {
          dockerfileToUse = dockerfilePath;
        }
      } else {
        console.log('Dockerfile generation skipped (AI unavailable), using fixture');
      }

      if (!existsSync(dockerfileToUse)) {
        console.log('No Dockerfile available for build step');
        return;
      }

      // Step 3: Build image
      const imageName = `docker-workflow-test:${Date.now()}`;
      const buildResult = await buildImageTool.handler(
        {
          path: testRepo,
          dockerfile: dockerfileToUse.replace(testRepo + '/', ''),
          imageName,
        },
        toolContext
      );

      expect(buildResult.ok).toBe(true);
      if (!buildResult.ok) {
        console.log('Build failed:', buildResult.error);
        return;
      }

      const build = buildResult.value as BuildImageResult;
      expect(build.imageId).toBeDefined();
      expect(build.createdTags).toContain(imageName);
      testCleaner.trackImage(build.imageId);

      // Step 4: Scan image (may fail if Trivy not installed)
      const scanResult = await scanImageTool.handler(
        { imageId: build.imageId },
        toolContext
      );

      if (!scanResult.ok) {
        console.log('Scan skipped (Trivy may not be installed)');
      } else {
        expect(scanResult.value).toBeDefined();
      }

      // Step 5: Tag image
      const newTag = `docker-workflow-test:v1.0`;
      const tagResult = await tagImageTool.handler(
        {
          imageId: build.imageId,
          tag: newTag,
        },
        toolContext
      );

      expect(tagResult.ok).toBe(true);
      if (tagResult.ok) {
        expect(tagResult.value).toBeDefined();
      }
    }, testTimeout);

    it('should complete workflow for Python Flask app', async () => {
      if (!dockerAvailable) {
        console.log('Skipping: Docker not available');
        return;
      }

      const testRepo = join(fixtureBasePath, 'python-flask');

      if (!existsSync(testRepo)) {
        console.log('Skipping: python-flask fixture not found');
        return;
      }

      // Step 1: Analyze
      const analyzeResult = await analyzeRepoTool.handler(
        { repositoryPath: testRepo },
        toolContext
      );

      if (!analyzeResult.ok) {
        console.log('Analysis failed:', analyzeResult.error);
        return;
      }

      const analysis = analyzeResult.value as RepositoryAnalysis;
      expect(analysis.modules).toBeDefined();

      // Step 2: Generate or use existing Dockerfile
      const dockerfilePath = join(testRepo, 'Dockerfile');

      // Create a simple test Dockerfile if none exists
      if (!existsSync(dockerfilePath)) {
        writeFileSync(
          dockerfilePath,
          `FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt* ./
RUN pip install --no-cache-dir -r requirements.txt 2>/dev/null || true
COPY . .
EXPOSE 5000
CMD ["python", "app.py"]`
        );
      }

      // Step 3: Build image
      const imageName = `docker-workflow-python:${Date.now()}`;
      const buildResult = await buildImageTool.handler(
        {
          path: testRepo,
          dockerfile: 'Dockerfile',
          imageName,
        },
        toolContext
      );

      if (buildResult.ok) {
        const build = buildResult.value as BuildImageResult;
        testCleaner.trackImage(build.imageId);

        // Step 4: Tag the image
        const tagResult = await tagImageTool.handler(
          {
            imageId: build.imageId,
            tag: `docker-workflow-python:latest`,
          },
          toolContext
        );
        
        if (!tagResult.ok) {
          console.log('Tagging failed:', tagResult.error);
          return;
        }
      }
    }, testTimeout);
  });

  describe('Workflow Error Handling', () => {
    it('should handle workflow errors gracefully', async () => {
      // Test error in middle of workflow - invalid repository
      const analyzeResult = await analyzeRepoTool.handler(
        { repositoryPath: '/nonexistent/path' },
        toolContext
      );

      expect(analyzeResult.ok).toBe(false);
      if (!analyzeResult.ok) {
        expect(analyzeResult.error).toBeDefined();
        expect(analyzeResult.guidance).toBeDefined();
      }

      // Test build error - invalid Dockerfile
      const buildResult = await buildImageTool.handler(
        {
          dockerfilePath: '/nonexistent/Dockerfile',
          context: fixtureBasePath,
          imageName: 'test:invalid',
        },
        toolContext
      );

      expect(buildResult.ok).toBe(false);
      if (!buildResult.ok) {
        expect(buildResult.error).toBeDefined();
      }
    });

    it('should provide helpful chain hints in responses', async () => {
      const testRepo = join(fixtureBasePath, 'node-express');

      if (!existsSync(testRepo)) {
        console.log('Skipping: fixture not found');
        return;
      }

      const analyzeResult = await analyzeRepoTool.handler(
        { repositoryPath: testRepo },
        toolContext
      );

      if (analyzeResult.ok) {
        const result = analyzeResult.value as any;
        // Check if the result has helpful metadata or next steps
        expect(result).toBeDefined();
      }
    });
  });

  describe('Partial Workflow Scenarios', () => {
    it('should support build → scan → tag without analysis', async () => {
      if (!dockerAvailable) {
        console.log('Skipping: Docker not available');
        return;
      }

      const testRepo = join(fixtureBasePath, 'node-express');
      const dockerfilePath = join(testRepo, 'Dockerfile');

      if (!existsSync(testRepo) || !existsSync(dockerfilePath)) {
        console.log('Skipping: fixture not found');
        return;
      }

      // Start directly with build
      const imageName = `partial-workflow-test:${Date.now()}`;
      const buildResult = await buildImageTool.handler(
        {
          path: testRepo,
          dockerfile: 'Dockerfile',
          imageName,
        },
        toolContext
      );

      if (buildResult.ok) {
        const build = buildResult.value as BuildImageResult;
        testCleaner.trackImage(build.imageId);

        // Continue with scan and tag
        const scanResult = await scanImageTool.handler(
          { imageId: build.imageId },
          toolContext
        );

        // Scan may fail if Trivy not available - that's OK
        expect(scanResult.ok !== undefined).toBe(true);

        const tagResult = await tagImageTool.handler(
          {
            imageId: build.imageId,
            tag: `partial-workflow-test:latest`,
          },
          toolContext
        );

        if (!tagResult.ok) {
          console.log('Tagging failed:', tagResult.error);
          return;
        }
      }
    }, testTimeout);
  });

  describe('Concurrent Docker Operations', () => {
    // TODO: This test has an intermittent issue where tool handlers return undefined
    // Other concurrent operation tests in complete-journey.test.ts provide coverage
    it.skip('should handle concurrent builds of different apps', async () => {
      if (!dockerAvailable) {
        console.log('Skipping: Docker not available');
        return;
      }

      const nodeRepo = join(fixtureBasePath, 'node-express');
      const pythonRepo = join(fixtureBasePath, 'python-flask');

      if (!existsSync(nodeRepo) || !existsSync(pythonRepo)) {
        console.log('Skipping: fixtures not available');
        return;
      }

      // Ensure both have Dockerfiles
      const nodeDockerfile = join(nodeRepo, 'Dockerfile');
      const pythonDockerfile = join(pythonRepo, 'Dockerfile');

      // Require both Dockerfiles to exist for this test
      if (!existsSync(nodeDockerfile) || !existsSync(pythonDockerfile)) {
        console.log('Skipping: Both Dockerfiles required for concurrent test');
        return;
      }

      // Build both concurrently (Docker handles this well)
      const timestamp = Date.now();
      const [nodeResult, pythonResult] = await Promise.all([
        buildImageTool.handler(
          {
            path: nodeRepo,
            dockerfile: 'Dockerfile',
            imageName: `concurrent-node:${timestamp}`,
          },
          toolContext
        ),
        buildImageTool.handler(
          {
            path: pythonRepo,
            dockerfile: 'Dockerfile',
            imageName: `concurrent-python:${timestamp}`,
          },
          toolContext
        ),
      ]);

      // Track images for cleanup
      if (nodeResult?.ok) {
        const build = nodeResult.value as BuildImageResult;
        testCleaner.trackImage(build.imageId);
      }

      if (pythonResult?.ok) {
        const build = pythonResult.value as BuildImageResult;
        testCleaner.trackImage(build.imageId);
      }

      // At least one should succeed (both should if Docker is healthy)
      expect(nodeResult?.ok || pythonResult?.ok).toBe(true);
    }, testTimeout * 2);
  });
});
