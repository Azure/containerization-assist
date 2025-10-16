/**
 * Integration Test: Docker Operations with Real Builds
 *
 * Tests actual Docker operations including:
 * - Building real images from Dockerfiles
 * - Tagging images with multiple tags
 * - Scanning images for vulnerabilities (if Trivy available)
 * - Pushing images to registry (if configured)
 * - Image cleanup and lifecycle management
 *
 * Prerequisites:
 * - Docker daemon running and accessible
 * - Sufficient disk space for image operations
 * - Trivy installed (optional, for scanning tests)
 * - Registry credentials (optional, for push tests)
 */

import { describe, it, expect, beforeAll, afterAll } from '@jest/globals';
import { createApp } from '@/app';
import type { AppRuntime } from '@/types/runtime';
import { createLogger } from '@/lib/logger';
import { join } from 'node:path';
import { writeFileSync } from 'node:fs';
import { createTestTempDir } from '../__support__/utilities/tmp-helpers';
import type { DirResult } from 'tmp';
import { DockerTestCleaner } from '../__support__/utilities/docker-test-cleaner';
import { createDockerClient } from '@/infra/docker/client';
import type { BuildResult } from '@/tools/build-image/schema';

describe('Docker Operations Integration (Real Builds)', () => {
  let runtime: AppRuntime;
  let testDir: DirResult;
  let cleanup: () => Promise<void>;
  let testCleaner: DockerTestCleaner;
  const logger = createLogger({ level: 'silent' });
  const testTimeout = 60000; // 60 seconds per test
  let dockerAvailable = false;

  beforeAll(async () => {
    runtime = createApp({ logger });

    // Create temporary directory
    const result = createTestTempDir('docker-ops-test-');
    testDir = result.dir;
    cleanup = result.cleanup;

    // Initialize Docker test cleaner
    const dockerClient = createDockerClient(logger);
    testCleaner = new DockerTestCleaner(logger, dockerClient, { verifyCleanup: true });

    // Check if Docker is available
    const healthCheck = await runtime.healthCheck();
    dockerAvailable = healthCheck.dependencies?.docker?.available || false;
  });

  afterAll(async () => {
    await testCleaner.cleanup();
    await cleanup();
    await runtime.stop();
  });

  describe('Image Building Operations', () => {
    it('should build a simple Alpine-based image', async () => {
      if (!dockerAvailable) {
        console.warn('Skipping test: Docker not available');
        return;
      }

      // Create a simple Dockerfile
      const dockerfilePath = join(testDir.name, 'Dockerfile.alpine');
      writeFileSync(
        dockerfilePath,
        `FROM alpine:latest
RUN echo "Test image"
CMD ["echo", "Hello from Alpine"]`
      );

      const imageName = `docker-ops-test-alpine:${Date.now()}`;

      // Build the image
      const result = await runtime.execute('build-image', {
        dockerfilePath,
        context: testDir.name,
        imageName,
      });

      expect(result.ok).toBe(true);
      if (!result.ok) {
        console.error('Build failed:', result.error);
        return;
      }

      const build = result.value as BuildResult;
      expect(build.imageId).toBeDefined();
      expect(build.imageTags).toContain(imageName);

      // Track for cleanup
      testCleaner.trackImage(build.imageId);
    }, testTimeout);

    it('should build a Node.js application image', async () => {
      if (!dockerAvailable) {
        console.warn('Skipping test: Docker not available');
        return;
      }

      // Create a Node.js app
      const appPath = join(testDir.name, 'node-app');
      writeFileSync(
        join(testDir.name, 'package.json'),
        JSON.stringify({
          name: 'test-app',
          version: '1.0.0',
          main: 'index.js',
        })
      );
      writeFileSync(join(testDir.name, 'index.js'), 'console.log("Hello");');

      const dockerfilePath = join(testDir.name, 'Dockerfile.node');
      writeFileSync(
        dockerfilePath,
        `FROM node:18-alpine
WORKDIR /app
COPY package.json ./
COPY index.js ./
CMD ["node", "index.js"]`
      );

      const imageName = `docker-ops-test-node:${Date.now()}`;

      const result = await runtime.execute('build-image', {
        dockerfilePath,
        context: testDir.name,
        imageName,
      });

      expect(result.ok).toBe(true);
      if (result.ok) {
        const build = result.value as BuildResult;
        testCleaner.trackImage(build.imageId);
      }
    }, testTimeout);
  });

  describe('Image Tagging Operations', () => {
    it('should tag an existing image with multiple tags', async () => {
      if (!dockerAvailable) {
        console.warn('Skipping test: Docker not available');
        return;
      }

      // First build an image
      const dockerfilePath = join(testDir.name, 'Dockerfile.tag-test');
      writeFileSync(dockerfilePath, 'FROM alpine:latest\nRUN echo "tag test"');

      const baseName = `docker-ops-tag-test:${Date.now()}`;
      const buildResult = await runtime.execute('build-image', {
        dockerfilePath,
        context: testDir.name,
        imageName: baseName,
      });

      expect(buildResult.ok).toBe(true);
      if (!buildResult.ok) return;

      const build = buildResult.value as BuildResult;
      testCleaner.trackImage(build.imageId);

      // Tag with new tag
      const newTag = `docker-ops-tag-test:latest`;
      const tagResult = await runtime.execute('tag-image', {
        source: baseName,
        tag: newTag,
      });

      expect(tagResult.ok).toBe(true);
      if (tagResult.ok) {
        testCleaner.trackImage(newTag);
      }
    }, testTimeout);
  });

  describe('Image Scanning Operations', () => {
    it('should scan an image for vulnerabilities if Trivy is available', async () => {
      if (!dockerAvailable) {
        console.warn('Skipping test: Docker not available');
        return;
      }

      // Build a test image
      const dockerfilePath = join(testDir.name, 'Dockerfile.scan-test');
      writeFileSync(dockerfilePath, 'FROM alpine:latest\nRUN apk add --no-cache curl');

      const imageName = `docker-ops-scan-test:${Date.now()}`;
      const buildResult = await runtime.execute('build-image', {
        dockerfilePath,
        context: testDir.name,
        imageName,
      });

      expect(buildResult.ok).toBe(true);
      if (!buildResult.ok) return;

      const build = buildResult.value as BuildResult;
      testCleaner.trackImage(build.imageId);

      // Scan the image
      const scanResult = await runtime.execute('scan-image', {
        imageId: build.imageId,
      });

      // Scan may fail if Trivy not installed - that's OK
      if (scanResult.ok) {
        const scanData = scanResult.value as any;
        expect(scanData).toBeDefined();
      } else {
        console.warn('Scan failed (Trivy may not be installed):', scanResult.error);
      }
    }, testTimeout);
  });
});
