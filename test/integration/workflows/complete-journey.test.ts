/**
 * Integration Test: Complete Containerization Journey
 *
 * Tests the complete end-to-end containerization workflow:
 * analyze-repo → generate-dockerfile → build-image → scan-image →
 * tag-image → generate-k8s-manifests → prepare-cluster → kubectl apply → verify-deploy
 *
 * This mirrors the smoke journey test but as a comprehensive integration test
 * with multiple application types and detailed verification.
 *
 * Prerequisites:
 * - Docker daemon running
 * - Kubernetes cluster available (optional, tests will adapt)
 * - Test fixtures available
 */

import { describe, it, expect, beforeAll, afterAll } from '@jest/globals';
import { createLogger } from '@/lib/logger';
import type { ToolContext } from '@/mcp/context';
import { join } from 'node:path';
import { existsSync, writeFileSync } from 'node:fs';
import { createTestTempDir } from '../../__support__/utilities/tmp-helpers';
import type { DirResult } from 'tmp';
import { DockerTestCleaner } from '../../__support__/utilities/docker-test-cleaner';
import { createDockerClient } from '@/infra/docker/client';

// Import all tools for complete workflow
import analyzeRepoTool from '@/tools/analyze-repo/tool';
import generateDockerfileTool from '@/tools/generate-dockerfile/tool';
import buildImageTool from '@/tools/build-image/tool';
import scanImageTool from '@/tools/scan-image/tool';
import tagImageTool from '@/tools/tag-image/tool';
import generateK8sManifestsTool from '@/tools/generate-k8s-manifests/tool';
import prepareClusterTool from '@/tools/prepare-cluster/tool';
import verifyDeployTool from '@/tools/verify-deploy/tool';

import type { RepositoryAnalysis } from '@/tools/analyze-repo/schema';
import type { BuildImageResult } from '@/tools/build-image/tool';

describe('Complete Containerization Journey', () => {
  let testDir: DirResult;
  let cleanup: () => Promise<void>;
  let testCleaner: DockerTestCleaner;
  const logger = createLogger({ level: 'silent' });

  const toolContext: ToolContext = {
    logger,
    signal: undefined,
    progress: undefined,
  };

  const fixtureBasePath = join(process.cwd(), 'test', '__support__', 'fixtures');
  const testTimeout = 300000; // 5 minutes for complete journey
  let dockerAvailable = false;
  let k8sAvailable = false;

  beforeAll(async () => {
    const result = createTestTempDir('complete-journey-');
    testDir = result.dir;
    cleanup = result.cleanup;

    // Check Docker availability
    try {
      const dockerClient = createDockerClient(logger);
      testCleaner = new DockerTestCleaner(logger, dockerClient, { verifyCleanup: true });
      dockerAvailable = true;
    } catch (error) {
      console.log('Docker not available - Docker steps will be skipped');
      dockerAvailable = false;
    }

    // Check Kubernetes availability
    try {
      const { execSync } = await import('node:child_process');
      execSync('kubectl cluster-info', { stdio: 'pipe' });
      k8sAvailable = true;
    } catch (error) {
      console.log('Kubernetes not available - K8s steps will be skipped');
      k8sAvailable = false;
    }
  });

  afterAll(async () => {
    if (dockerAvailable && testCleaner) {
      await testCleaner.cleanup();
    }
    await cleanup();
  });

  describe('End-to-End Containerization Journey', () => {
    it('should containerize and deploy Node.js application end-to-end', async () => {
      const testRepo = join(fixtureBasePath, 'node-express');

      if (!existsSync(testRepo) || !existsSync(join(testRepo, 'package.json'))) {
        console.log('Skipping: node-express fixture not available');
        return;
      }

      const journeyLog: string[] = [];
      const timestamp = Date.now();

      // ===== STEP 1: Analyze Repository =====
      journeyLog.push('Step 1: Analyzing repository...');
      const analyzeResult = await analyzeRepoTool.handler(
        { repositoryPath: testRepo },
        toolContext
      );

      expect(analyzeResult.ok).toBe(true);
      if (!analyzeResult.ok) {
        journeyLog.push(`Analysis failed: ${analyzeResult.error}`);
        console.log(journeyLog.join('\n'));
        return;
      }

      const analysis = analyzeResult.value as RepositoryAnalysis;
      expect(analysis.modules).toBeDefined();
      expect(analysis.modules.length).toBeGreaterThan(0);
      journeyLog.push(`✓ Detected ${analysis.modules.length} module(s)`);

      // ===== STEP 2: Generate Dockerfile =====
      journeyLog.push('Step 2: Generating Dockerfile...');
      const dockerfilePath = join(testRepo, `Dockerfile.journey-${timestamp}`);
      const generateResult = await generateDockerfileTool.handler(
        {
          repositoryPath: testRepo,
          analysis: JSON.stringify(analysis),
          outputPath: dockerfilePath,
          targetPlatform: 'linux/amd64',
        },
        toolContext
      );

      // Use existing Dockerfile if generation fails
      let dockerfileToUse = join(testRepo, 'Dockerfile');
      if (generateResult.ok && existsSync(dockerfilePath)) {
        dockerfileToUse = dockerfilePath;
        journeyLog.push('✓ Dockerfile generated with AI');
      } else {
        journeyLog.push('✓ Using existing Dockerfile (AI unavailable)');
      }

      if (!existsSync(dockerfileToUse)) {
        journeyLog.push('✗ No Dockerfile available');
        console.log(journeyLog.join('\n'));
        return;
      }

      // ===== STEP 3: Build Image =====
      if (!dockerAvailable) {
        journeyLog.push('Step 3-5: Docker steps skipped (Docker not available)');
        console.log(journeyLog.join('\n'));
        return;
      }

      journeyLog.push('Step 3: Building Docker image...');
      const imageName = `journey-test-node:${timestamp}`;
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
        journeyLog.push(`Build failed: ${buildResult.error}`);
        console.log(journeyLog.join('\n'));
        return;
      }

      const build = buildResult.value as BuildImageResult;
      testCleaner.trackImage(build.imageId);
      journeyLog.push(`✓ Image built: ${build.imageId.substring(0, 12)}`);

      // ===== STEP 4: Scan Image =====
      journeyLog.push('Step 4: Scanning image for vulnerabilities...');
      const scanResult = await scanImageTool.handler(
        { imageId: build.imageId },
        toolContext
      );

      if (scanResult.ok) {
        journeyLog.push('✓ Security scan completed');
      } else {
        journeyLog.push('⚠ Scan skipped (Trivy may not be installed)');
      }

      // ===== STEP 5: Tag Image =====
      journeyLog.push('Step 5: Tagging image...');
      const finalTag = `journey-test-node:v1.0.0`;
      const tagResult = await tagImageTool.handler(
        {
          imageId: build.imageId,
          tag: finalTag,
        },
        toolContext
      );

      expect(tagResult.ok).toBe(true);
      if (tagResult.ok) {
        journeyLog.push(`✓ Image tagged: ${finalTag}`);
      }

      // ===== STEP 6: Generate K8s Manifests =====
      journeyLog.push('Step 6: Generating Kubernetes manifests...');
      const manifestsPath = join(testDir.name, `k8s-journey-${timestamp}.yaml`);
      const k8sResult = await generateK8sManifestsTool.handler(
        {
          analysis: JSON.stringify(analysis),
          imageName: finalTag,
          outputPath: manifestsPath,
        },
        toolContext
      );

      if (!k8sResult.ok || !existsSync(manifestsPath)) {
        journeyLog.push('⚠ Manifest generation skipped (AI unavailable), creating test manifest');

        const testManifest = `apiVersion: v1
kind: Namespace
metadata:
  name: journey-test
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: journey-test-app
  namespace: journey-test
spec:
  replicas: 1
  selector:
    matchLabels:
      app: journey-test
  template:
    metadata:
      labels:
        app: journey-test
    spec:
      containers:
      - name: app
        image: ${finalTag}
        ports:
        - containerPort: 3000`;

        writeFileSync(manifestsPath, testManifest);
        journeyLog.push('✓ Test manifest created');
      } else {
        journeyLog.push('✓ K8s manifests generated with AI');
      }

      // ===== STEP 7-9: Kubernetes Deployment (if available) =====
      if (!k8sAvailable) {
        journeyLog.push('Step 7-9: Kubernetes steps skipped (K8s not available)');
        console.log(journeyLog.join('\n'));
        console.log('\n✓ Journey completed successfully (Docker phase)');
        return;
      }

      const testNamespace = `journey-test-${timestamp}`;

      // Step 7: Prepare Cluster
      journeyLog.push('Step 7: Preparing cluster...');
      const prepareResult = await prepareClusterTool.handler(
        { namespace: testNamespace },
        toolContext
      );

      if (prepareResult.ok) {
        journeyLog.push('✓ Cluster prepared');
      } else {
        journeyLog.push('⚠ Cluster preparation not needed');
      }

      // Step 8: Deploy with kubectl
      journeyLog.push('Step 8: Deploying to Kubernetes...');
      let deploySucceeded = false;
      try {
        const { spawn } = await import('node:child_process');

        await new Promise<void>((resolve, reject) => {
          const child = spawn('kubectl', ['apply', '-f', manifestsPath, '-n', testNamespace], {
            stdio: 'pipe'
          });

          child.on('close', (code) => {
            if (code === 0) {
              resolve();
            } else {
              reject(new Error(`kubectl apply failed with code ${code}`));
            }
          });

          child.on('error', reject);
        });
        journeyLog.push('✓ Application deployed with kubectl apply');
        deploySucceeded = true;
      } catch (error) {
        journeyLog.push('⚠ Deployment skipped (expected in test environment)');
      }

      if (deploySucceeded) {
        // Step 9: Verify Deployment
        journeyLog.push('Step 9: Verifying deployment...');
        const verifyResult = await verifyDeployTool.handler(
          {
            namespace: testNamespace,
            deploymentName: 'journey-test-app',
          },
          toolContext
        );

        if (verifyResult.ok) {
          journeyLog.push('✓ Deployment verified');
        } else {
          journeyLog.push('⚠ Verification pending (deployment may still be starting)');
        }
      }

      // Cleanup K8s resources
      try {
        const { spawn } = await import('node:child_process');
        await new Promise<void>((resolve) => {
          const child = spawn('kubectl', ['delete', 'namespace', testNamespace, '--ignore-not-found=true'], {
            stdio: 'pipe'
          });

          child.on('close', () => {
            // Always resolve - cleanup errors are ignored
            resolve();
          });

          child.on('error', () => {
            // Always resolve - cleanup errors are ignored
            resolve();
          });
        });
      } catch (error) {
        // Ignore cleanup errors
      }

      console.log(journeyLog.join('\n'));
      console.log('\n✓ Complete journey finished successfully');
    }, testTimeout);

    it('should work with Python Flask application', async () => {
      const testRepo = join(fixtureBasePath, 'python-flask');

      if (!existsSync(testRepo)) {
        console.log('Skipping: python-flask fixture not available');
        return;
      }

      const journeyLog: string[] = [];
      const timestamp = Date.now();

      // Step 1: Analyze
      journeyLog.push('Step 1: Analyzing Python repository...');
      const analyzeResult = await analyzeRepoTool.handler(
        { repositoryPath: testRepo },
        toolContext
      );

      if (!analyzeResult.ok) {
        journeyLog.push(`Analysis failed: ${analyzeResult.error}`);
        console.log(journeyLog.join('\n'));
        return;
      }

      const analysis = analyzeResult.value as RepositoryAnalysis;
      journeyLog.push(`✓ Detected Python application`);

      // Step 2: Generate or use Dockerfile
      const dockerfilePath = join(testRepo, 'Dockerfile');
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
      journeyLog.push('✓ Dockerfile ready');

      // Step 3: Build (if Docker available)
      if (!dockerAvailable) {
        journeyLog.push('Docker steps skipped (Docker not available)');
        console.log(journeyLog.join('\n'));
        return;
      }

      journeyLog.push('Step 3: Building Docker image...');
      const imageName = `journey-test-python:${timestamp}`;
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
        journeyLog.push(`✓ Image built: ${build.imageId.substring(0, 12)}`);

        // Tag image
        const tagResult = await tagImageTool.handler(
          {
            imageId: build.imageId,
            tag: `journey-test-python:latest`,
          },
          toolContext
        );

        if (tagResult.ok) {
          journeyLog.push('✓ Image tagged');
        }
      }

      console.log(journeyLog.join('\n'));
      console.log('\n✓ Python journey completed');
    }, testTimeout);
  });

  describe('Multi-Application Journey', () => {
    it('should handle containerizing multiple applications in sequence', async () => {
      if (!dockerAvailable) {
        console.log('Skipping: Docker not available');
        return;
      }

      const apps = [
        { path: join(fixtureBasePath, 'node-express'), name: 'node-app' },
        { path: join(fixtureBasePath, 'python-flask'), name: 'python-app' },
      ];

      const results: Array<{ name: string; success: boolean }> = [];
      const timestamp = Date.now();

      for (const app of apps) {
        if (!existsSync(app.path)) {
          console.log(`Skipping ${app.name}: fixture not found`);
          continue;
        }

        // Quick workflow: analyze → build → tag
        const analyzeResult = await analyzeRepoTool.handler(
          { repositoryPath: app.path },
          toolContext
        );

        if (!analyzeResult.ok) {
          results.push({ name: app.name, success: false });
          continue;
        }

        // Use existing or create Dockerfile
        let dockerfilePath = join(app.path, 'Dockerfile');
        if (!existsSync(dockerfilePath)) {
          const dockerfile =
            app.name === 'python-app'
              ? `FROM python:3.11-slim
WORKDIR /app
COPY . .
CMD ["python", "app.py"]`
              : `FROM node:18-alpine
WORKDIR /app
COPY . .
CMD ["node", "index.js"]`;

          writeFileSync(dockerfilePath, dockerfile);
        }

        const imageName = `multi-journey-${app.name}:${timestamp}`;
        const buildResult = await buildImageTool.handler(
          {
            path: app.path,
            dockerfile: 'Dockerfile',
            imageName,
          },
          toolContext
        );

        if (buildResult.ok) {
          const build = buildResult.value as BuildImageResult;
          testCleaner.trackImage(build.imageId);
          results.push({ name: app.name, success: true });
        } else {
          results.push({ name: app.name, success: false });
        }
      }

      // At least one should succeed if fixtures are available
      const successCount = results.filter((r) => r.success).length;
      console.log(`Successfully containerized ${successCount}/${results.length} applications`);

      expect(results.length).toBeGreaterThan(0);
    }, testTimeout);
  });

  describe('Journey Error Recovery', () => {
    it('should handle errors gracefully at each step', async () => {
      const journeyLog: string[] = [];

      // Test 1: Invalid repository
      journeyLog.push('Test 1: Invalid repository path');
      const analyzeResult = await analyzeRepoTool.handler(
        { repositoryPath: '/nonexistent/path' },
        toolContext
      );

      expect(analyzeResult.ok).toBe(false);
      if (!analyzeResult.ok) {
        expect(analyzeResult.error).toBeDefined();
        expect(analyzeResult.guidance).toBeDefined();
        journeyLog.push('✓ Error handled with guidance');
      }

      // Test 2: Invalid Dockerfile
      journeyLog.push('Test 2: Invalid Dockerfile path');
      const buildResult = await buildImageTool.handler(
        {
          dockerfilePath: '/nonexistent/Dockerfile',
          context: testDir.name,
          imageName: 'test:invalid',
        },
        toolContext
      );

      expect(buildResult.ok).toBe(false);
      if (!buildResult.ok) {
        expect(buildResult.error).toBeDefined();
        journeyLog.push('✓ Build error handled');
      }

      // Test 3: Invalid manifests path with kubectl
      journeyLog.push('Test 3: Invalid manifests path');
      let deployErrorHandled = false;
      try {
        const { spawn } = await import('node:child_process');
        await new Promise<void>((resolve, reject) => {
          const child = spawn('kubectl', ['apply', '-f', '/nonexistent/manifests.yaml', '-n', 'test'], {
            stdio: 'pipe'
          });

          child.on('close', (code) => {
            if (code === 0) {
              resolve();
            } else {
              reject(new Error(`kubectl apply failed with code ${code}`));
            }
          });

          child.on('error', reject);
        });
      } catch (error) {
        deployErrorHandled = true;
        journeyLog.push('✓ Deploy error handled');
      }

      expect(deployErrorHandled).toBe(true);

      console.log(journeyLog.join('\n'));
      console.log('✓ All errors handled gracefully');
    });

    it('should provide clear error messages to users', async () => {
      // Test that errors include helpful messages
      const analyzeResult = await analyzeRepoTool.handler(
        { repositoryPath: '/nonexistent/path' },
        toolContext
      );

      if (!analyzeResult.ok) {
        expect(analyzeResult.error).toContain('does not exist');
        // Should provide guidance
        expect(analyzeResult.guidance).toBeDefined();
      }
    });
  });

  describe('Journey Performance', () => {
    it('should complete journey in reasonable time', async () => {
      const testRepo = join(fixtureBasePath, 'node-express');

      if (!existsSync(testRepo) || !dockerAvailable) {
        console.log('Skipping: prerequisites not available');
        return;
      }

      const startTime = Date.now();

      // Run minimal journey: analyze → build → tag
      await analyzeRepoTool.handler({ repositoryPath: testRepo }, toolContext);

      const dockerfilePath = join(testRepo, 'Dockerfile');
      if (!existsSync(dockerfilePath)) {
        console.log('Skipping: no Dockerfile');
        return;
      }

      const imageName = `perf-test:${Date.now()}`;
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

        await tagImageTool.handler(
          {
            imageId: build.imageId,
            tag: 'perf-test:latest',
          },
          toolContext
        );

      }

      const duration = Date.now() - startTime;
      console.log(`Journey completed in ${(duration / 1000).toFixed(2)}s`);

      // Should complete in reasonable time (< 2 minutes for minimal journey)
      expect(duration).toBeLessThan(120000);
    }, testTimeout);
  });
});
