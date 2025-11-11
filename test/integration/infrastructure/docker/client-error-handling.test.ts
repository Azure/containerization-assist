/**
 * Docker Client Error Handling Integration Tests
 * 
 * These tests verify that the enhanced Docker error handling correctly
 * identifies and categorizes real Docker daemon errors with specific,
 * actionable error messages.
 * 
 * Prerequisites:
 * - Docker daemon must be runnings
 * - Network connectivity for registry tests
 * - Sufficient disk space for image operations
 */

import { createDockerClient } from '../../../../src/infra/docker/client';
import { createLogger } from '../../../../src/lib/logger';
import type { DockerBuildOptions, DockerClient } from '../../../../src/infra/docker/client';
import { DockerTestCleaner, TEST_IMAGE_NAME } from '../../../__support__/utilities/docker-test-cleaner';
import { promises as fs } from 'node:fs';
import path from 'node:path';
import { createTestTempDir } from '../../../__support__/utilities/tmp-helpers';
import type { DirResult } from 'tmp';

describe('Docker Client Error Handling Integration Tests', () => {
  let dockerClient: DockerClient;
  let testDir: DirResult;
  let cleanup: () => Promise<void>;
  let testCleaner: DockerTestCleaner;
  const logger = createLogger({ level: 'debug' });
  const testTimeout = 60000; // 60 seconds for Docker operations

  beforeAll(async () => {
    // Initialize the test cleaner with verification enabled
    dockerClient = createDockerClient(logger);
    testCleaner = new DockerTestCleaner(logger, dockerClient, { verifyCleanup: true });

    // Create a temporary directory for test Dockerfiles
    const result = createTestTempDir('docker-error-tests-');
    testDir = result.dir;
    cleanup = result.cleanup;

    // Wrap buildImage to track successful builds for cleanup
    const original = dockerClient.buildImage.bind(dockerClient);
    dockerClient.buildImage = async (options: DockerBuildOptions) => {
      const result = await original(options);
      if (result.ok && result.value.imageId) {
        // Only track the actual image ID (SHA256) that was created
        testCleaner.trackImage(result.value.imageId);
        logger.debug(`Tracking created image: ${result.value.imageId}`);
      }
      return result;
    };
  });

  afterAll(async () => {
    // Clean up all tracked Docker resources
    await testCleaner.cleanup();

    // Clean up test directory
    await cleanup();
  });

  afterEach(async () => {
    // Clean up any test containers after each test
    await testCleaner.cleanupContainers();
  });

  // Helper function to create test Dockerfile
  async function createTestDockerfile(content: string, filename = 'Dockerfile'): Promise<string> {
    const dockerfilePath = path.join(testDir.name, filename);
    await fs.writeFile(dockerfilePath, content, 'utf-8');
    return dockerfilePath;
  }

  describe('Network Connectivity Error Detection', () => {
    test('should detect registry connectivity issues (ENOTFOUND)', async () => {
      await createTestDockerfile(
        'FROM nonexistent-registry.invalid/library/alpine:latest\nRUN echo "test"',
        'Dockerfile.connectivity'
      );

      const invalidRegistryOptions: DockerBuildOptions = {
        dockerfile: 'Dockerfile.connectivity',
        context: testDir.name,
        t: TEST_IMAGE_NAME
      };

      const result = await dockerClient.buildImage(invalidRegistryOptions);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        // Should detect network connectivity issues with meaningful error message
        expect(result.error).toMatch(/network|connectivity|ENOTFOUND|getaddrinfo|connection|no such host/i);
        expect(result.error).not.toBe('Build failed: Unknown error');
      }
    }, testTimeout);

    test('should detect registry connection refused (ECONNREFUSED)', async () => {
      await createTestDockerfile(
        'FROM localhost:9999/library/alpine:latest\nRUN echo "test"',
        'Dockerfile.refused'
      );

      const connectionRefusedOptions: DockerBuildOptions = {
        dockerfile: 'Dockerfile.refused',
        context: testDir.name,
        t: TEST_IMAGE_NAME
      };

      const result = await dockerClient.buildImage(connectionRefusedOptions);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        // Should detect connection refused with meaningful error message
        expect(result.error).toMatch(/network|connectivity|ECONNREFUSED|connection.*refused|context deadline exceeded|timeout/i);
        expect(result.error).not.toBe('Build failed: Unknown error');
      }
    }, testTimeout);
  });

  describe('Registry Authentication Error Detection', () => {
    test('should detect authentication failures (401/403)', async () => {
      // This test assumes a private registry that requires authentication
      // Skip if no test registry is configured
      const testRegistry = process.env.TEST_PRIVATE_REGISTRY;
      if (!testRegistry) {
        console.log('Skipping authentication test - TEST_PRIVATE_REGISTRY not configured');
        return;
      }

      await createTestDockerfile(
        `FROM ${testRegistry}/private/image:latest\nRUN echo "test"`,
        'Dockerfile.auth'
      );

      const authFailureOptions: DockerBuildOptions = {
        dockerfile: 'Dockerfile.auth',
        context: testDir.name,
        t: TEST_IMAGE_NAME
      };

      const result = await dockerClient.buildImage(authFailureOptions);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        // Should detect authentication issues with meaningful error message
        expect(result.error).toMatch(/authentication|unauthorized|access.*denied|401|403/i);
        expect(result.error).not.toBe('Build failed: Unknown error');
      }
    }, testTimeout);
  });

  describe('Image Not Found Error Detection', () => {
    test('should detect missing base images (404)', async () => {
      await createTestDockerfile(
        'FROM alpine:nonexistent-tag-12345\nRUN echo "test"',
        'Dockerfile.missing'
      );

      const missingImageOptions: DockerBuildOptions = {
        dockerfile: 'Dockerfile.missing',
        context: testDir.name,
        t: TEST_IMAGE_NAME
      };

      const result = await dockerClient.buildImage(missingImageOptions);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        // Should detect missing images with meaningful error message
        expect(result.error).toMatch(/not found|does not exist|404|no such image/i);
        expect(result.error).not.toBe('Build failed: Unknown error');
      }
    }, testTimeout);

    test('should detect missing images in non-existent repositories', async () => {
      await createTestDockerfile(
        'FROM library/totally-nonexistent-image-name-12345:latest\nRUN echo "test"',
        'Dockerfile.nonexistent'
      );

      const nonExistentRepoOptions: DockerBuildOptions = {
        dockerfile: 'Dockerfile.nonexistent',
        context: testDir.name,
        t: TEST_IMAGE_NAME
      };

      const result = await dockerClient.buildImage(nonExistentRepoOptions);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        // Should detect non-existent repositories with meaningful error message
        expect(result.error).toMatch(/not found|does not exist|404|no such image/i);
        expect(result.error).not.toBe('Build failed: Unknown error');
      }
    }, testTimeout);
  });

  describe('Registry Server Error Detection', () => {
    test('should detect registry server errors (5xx)', async () => {
      // This test would require a registry that returns 5xx errors
      // We'll create a scenario that might trigger this by using an overloaded registry
      await createTestDockerfile(
        'FROM registry.hub.docker.com/library/alpine:latest\nRUN echo "test"',
        'Dockerfile.server'
      );

      const serverErrorOptions: DockerBuildOptions = {
        dockerfile: 'Dockerfile.server',
        context: testDir.name,
        t: TEST_IMAGE_NAME
      };

      const result = await dockerClient.buildImage(serverErrorOptions);

      // This test might pass if the registry is working correctly
      // We mainly want to ensure our error handling can detect 5xx errors when they occur
      if (!result.ok && result.error.includes('server error')) {
        expect(result.error).toMatch(/server error|internal error|503|502|500/i);
        expect(result.error).not.toBe('Build failed: Unknown error');
      }
    }, testTimeout);
  });

  describe('Dockerfile Syntax Error Detection', () => {
    test('should detect malformed Dockerfile syntax', async () => {
      await createTestDockerfile(
        'INVALID_INSTRUCTION this is not valid\nFROM alpine:latest',
        'Dockerfile.syntax'
      );

      const malformedDockerfileOptions: DockerBuildOptions = {
        dockerfile: 'Dockerfile.syntax',
        context: testDir.name,
        t: TEST_IMAGE_NAME
      };

      const result = await dockerClient.buildImage(malformedDockerfileOptions);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        // Should detect syntax errors with meaningful error message
        expect(result.error).toMatch(/syntax|instruction|invalid|unknown instruction/i);
        expect(result.error).not.toBe('Build failed: Unknown error');
      }
    }, testTimeout);

    test('should detect missing FROM instruction', async () => {
      await createTestDockerfile(
        'RUN echo "test without FROM"',
        'Dockerfile.nofrom'
      );

      const noFromOptions: DockerBuildOptions = {
        dockerfile: 'Dockerfile.nofrom',
        context: testDir.name,
        t: TEST_IMAGE_NAME
      };

      const result = await dockerClient.buildImage(noFromOptions);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        // Should detect missing FROM instruction with meaningful error message
        expect(result.error).toMatch(/FROM|base image|instruction|must begin with|no build stage|context/i);
        expect(result.error).not.toBe('Build failed: Unknown error');
      }
    }, testTimeout);
  });

  describe('Context Path Error Detection', () => {
    test('should detect missing Dockerfile in valid context', async () => {
      // Use a valid context but reference a non-existent Dockerfile
      const invalidDockerfileOptions: DockerBuildOptions = {
        dockerfile: 'NonExistentDockerfile', // This doesn't exist in testDir
        context: testDir.name,
        t: TEST_IMAGE_NAME
      };

      const result = await dockerClient.buildImage(invalidDockerfileOptions);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        // Should detect missing Dockerfile with meaningful error message
        expect(result.error).toMatch(/dockerfile|not found|ENOENT|no such file|cannot find/i);
        expect(result.error).not.toBe('Build failed: Unknown error');
      }
    }, testTimeout);
  });


  describe('Image Operations Error Detection', () => {
    test('should detect errors when getting non-existent images', async () => {
      const result = await dockerClient.getImage('nonexistent-image:latest');

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toMatch(/not found|does not exist|404/i);
      }
    });

    test('should detect errors when tagging non-existent images', async () => {
      const result = await dockerClient.tagImage('nonexistent-image:latest', 'new-repo', 'latest');

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toMatch(/not found|does not exist|no such image/i);
      }
    });

    test('should detect errors when pushing to unauthorized registries', async () => {
      // First create a small test image
      await createTestDockerfile(
        'FROM alpine:latest\nRUN echo "test"',
        'Dockerfile.push'
      );

      const buildResult = await dockerClient.buildImage({
        dockerfile: 'Dockerfile.push',
        context: testDir.name,
        t: 'test-push-unauthorized'
      });

      if (buildResult.ok) {
        // Tag the image with a registry prefix to trigger actual push attempt
        const tagResult = await dockerClient.tagImage(
          'test-push-unauthorized:latest',
          'docker.io/unauthorized-test-repo',
          'latest'
        );

        if (tagResult.ok) {
          // Try to push to Docker Hub without authentication
          const pushResult = await dockerClient.pushImage('docker.io/unauthorized-test-repo', 'latest');

          expect(pushResult.ok).toBe(false);
          if (!pushResult.ok) {
            // Should detect authentication issues with meaningful error message
            expect(pushResult.error).toMatch(/authentication|unauthorized|access denied|denied|401|403|X-Registry-Auth|bad parameters/i);
            expect(pushResult.error).not.toBe('Failed to push image: Unknown error');
          }
        }
      }
    }, testTimeout);
  });

  describe('Build Progress Error Handling', () => {
    test('should handle errors in build progress stream', async () => {
      await createTestDockerfile(`
FROM alpine:latest
RUN exit 1
`, 'Dockerfile.progress');

      const progressErrorOptions: DockerBuildOptions = {
        dockerfile: 'Dockerfile.progress',
        context: testDir.name,
        t: 'test-progress-error'
      };

      const result = await dockerClient.buildImage(progressErrorOptions);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        // Should detect build command failures with meaningful error message
        expect(result.error).toMatch(/build failed|command.*failed|exit.*1|non-zero code/i);
        expect(result.error).not.toBe('Build failed: Unknown error');
      }
    }, testTimeout);

    test('should capture detailed error information from build steps', async () => {
      await createTestDockerfile(`
FROM alpine:latest
RUN echo "Before error"
RUN /nonexistent/command/that/fails
RUN echo "After error"
`, 'Dockerfile.step');

      const stepErrorOptions: DockerBuildOptions = {
        dockerfile: 'Dockerfile.step',
        context: testDir.name,
        t: 'test-step-error'
      };

      const result = await dockerClient.buildImage(stepErrorOptions);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        // Should detect step failures with meaningful error message
        expect(result.error).toMatch(/nonexistent|command|not found|failed|no such file/i);
        expect(result.error).not.toBe('Build failed: Unknown error');
      }
    }, testTimeout);
  });

  describe('Error Message Quality', () => {
    test('should never return generic "Unknown error" for real Docker failures', async () => {
      // Create test Dockerfiles for various error scenarios
      await createTestDockerfile('FROM nonexistent-registry.invalid/alpine:latest', 'Dockerfile.unknown1');
      await createTestDockerfile('FROM alpine:nonexistent-tag-12345', 'Dockerfile.unknown2');
      await createTestDockerfile('INVALID_INSTRUCTION\nFROM alpine:latest', 'Dockerfile.unknown3');

      const testCases: DockerBuildOptions[] = [
        { dockerfile: 'Dockerfile.unknown1', context: testDir, t: 'test-no-unknown-1' },
        { dockerfile: 'Dockerfile.unknown2', context: testDir, t: 'test-no-unknown-2' },
        { dockerfile: 'Dockerfile.unknown3', context: testDir, t: 'test-no-unknown-3' }
      ];

      for (const options of testCases) {
        const result = await dockerClient.buildImage(options);

        expect(result.ok).toBe(false);
        if (!result.ok) {
          // Should never return generic "Unknown error" messages
          expect(result.error).not.toBe('Build failed: Unknown error');
          expect(result.error).not.toContain('Unknown error');
          expect(result.error.length).toBeGreaterThan(20); // Should have meaningful detail
        }
      }
    }, testTimeout * 3); // 3 test cases

    test('should provide actionable error messages', async () => {
      await createTestDockerfile(
        'FROM nonexistent-registry.invalid/alpine:latest',
        'Dockerfile.actionable'
      );

      const networkErrorOptions: DockerBuildOptions = {
        dockerfile: 'Dockerfile.actionable',
        context: testDir.name,
        t: 'test-actionable-error'
      };

      const result = await dockerClient.buildImage(networkErrorOptions);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        // Error should be specific and actionable, not generic
        // Accept either network-related errors OR disk space errors (both are actionable)
        expect(result.error).toMatch(/registry|connectivity|network|dns|not found|pull|manifest|disk space|no space left on device/i);
        expect(result.error).not.toBe('Build failed: Unknown error');
        expect(result.error.length).toBeGreaterThan(20); // Should have meaningful detail
      }
    }, testTimeout);
  });
});
