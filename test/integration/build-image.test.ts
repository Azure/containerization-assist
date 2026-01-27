import { describe, it, expect, beforeAll, afterAll } from '@jest/globals';
import { mkdir, writeFile, rm } from 'node:fs/promises';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { createLogger } from '@/lib/logger';
import { createDockerClient } from '@/infra/docker/client';
import { autoDetectDockerSocket } from '@/infra/docker/socket-validation';
import type { ToolContext } from '@/core/context';
import { DockerTestCleaner } from '../__support__/utilities/docker-test-cleaner';

// Import the tool under test
import buildImageTool from '@/tools/build-image/tool';
import type { BuildImageResult } from '@/tools/build-image/tool';

describe('Build Image - Real Docker Integration', () => {
  let testCleaner: DockerTestCleaner;
  let dockerAvailable = false;
  let testDir: string;
  const logger = createLogger({ level: process.env.CI ? 'silent' : 'info' });

  const toolContext: ToolContext = {
    logger,
    signal: undefined,
    progress: undefined,
  };

  const testTimeout = 120000; // 2 minutes

  beforeAll(async () => {
    // Check Docker availability
    try {
      const dockerClient = createDockerClient(logger);
      const testResult = await dockerClient.listContainers({ all: false });

      if (testResult.ok) {
        dockerAvailable = true;
        testCleaner = new DockerTestCleaner(logger, dockerClient, { verifyCleanup: true });
        console.log('✅ Docker daemon is available');
        console.log('   Socket:', autoDetectDockerSocket());
      } else {
        console.log('⚠️  Docker daemon not available - tests will be skipped');
        console.log('   Socket:', autoDetectDockerSocket());
      }
    } catch (error) {
      console.log('⚠️  Docker check failed:', error);
      console.log('   Socket:', autoDetectDockerSocket());
      dockerAvailable = false;
    }

    // Create test directory
    testDir = join(tmpdir(), `docker-test-${Date.now()}`);
    await mkdir(testDir, { recursive: true });
  });

  afterAll(async () => {
    if (dockerAvailable && testCleaner) {
      await testCleaner.cleanup();
    }
    if (testDir) {
      await rm(testDir, { recursive: true, force: true }).catch(() => {});
    }
  });

  it('should detect correct socket path based on platform', () => {
    const socketPath = autoDetectDockerSocket();

    if (process.platform === 'win32') {
      expect(socketPath).toBe('//./pipe/docker_engine');
      console.log('✓ Windows named pipe:', socketPath);
    } else {
      expect(socketPath).toMatch(/\.sock$/);
      console.log('✓ Unix socket:', socketPath);

      const validSockets = [
        '/var/run/docker.sock',
        ...socketPath.includes('colima') ? [socketPath] : [],
        ...socketPath.includes('lima') ? [socketPath] : [],
      ];
      expect(
        validSockets.some(valid => socketPath === valid || socketPath.includes('colima') || socketPath.includes('lima'))
      ).toBe(true);
    }
  });

  it('should verify Docker daemon is available and accessible', async () => {
    if (!dockerAvailable) {
      throw new Error('Docker daemon is not available. Ensure Docker is running and accessible.');
    }

    const dockerClient = createDockerClient(logger);
    const result = await dockerClient.listContainers({ all: false });

    expect(result.ok).toBe(true);
    console.log('✓ Docker daemon is accessible');
    console.log(`  Platform: ${process.platform}`);
    console.log(`  Socket: ${autoDetectDockerSocket()}`);
  });

  it('should successfully build a Docker image', async () => {
    if (!dockerAvailable) {
      throw new Error('Docker daemon is not available. Ensure Docker is running and accessible.');
    }

    // Create simple Dockerfile
    const dockerfile = `FROM busybox:latest
CMD ["echo", "Test successful"]`;

    await writeFile(join(testDir, 'Dockerfile'), dockerfile);

    const imageName = `test-build:${Date.now()}`;
    const result = await buildImageTool.handler(
      {
        path: testDir,
        imageName,
      },
      toolContext,
    );

    if (!result.ok) {
      console.error('Build failed:', result.error);
      console.error('Guidance:', result.guidance);
      throw new Error(`Docker build failed: ${result.error}`);
    }

    const build = result.value as BuildImageResult;

    // Verify build succeeded
    expect(build.imageId).toBeDefined();
    expect(build.imageId).toMatch(/^sha256:[a-f0-9]+$/);
    expect(build.success).toBe(true);
    expect(build.createdTags).toContain(imageName);

    console.log('✓ Image built successfully');
    console.log(`  Image ID: ${build.imageId.substring(0, 19)}...`);
    console.log(`  Size: ${(build.size / 1024 / 1024).toFixed(2)} MB`);
    console.log(`  Build time: ${(build.buildTime / 1000).toFixed(2)}s`);

    // Track for cleanup
    testCleaner.trackImage(build.imageId);
  }, testTimeout);

  it('should report error if Docker build fails', async () => {
    if (!dockerAvailable) {
      throw new Error('Docker daemon is not available. Ensure Docker is running and accessible.');
    }

    // Create invalid Dockerfile
    const dockerfile = `FROM busybox:latest
RUN exit 1`;

    await writeFile(join(testDir, 'Dockerfile'), dockerfile);

    const result = await buildImageTool.handler(
      {
        path: testDir,
        imageName: `test-fail:${Date.now()}`,
      },
      toolContext,
    );

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error).toBeDefined();
      expect(result.guidance).toBeDefined();
      console.log('✓ Build failure detected correctly');
      console.log(`  Error: ${result.error}`);
      console.log(`  Hint: ${result.guidance?.hint}`);
    }
  }, testTimeout);
});
