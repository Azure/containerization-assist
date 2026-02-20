/**
 * Docker Client Error Handling Integration Tests
 * 
 * These tests verify that the Docker client correctly handles errors
 * for image operations (get, tag, push, remove).
 * 
 * Note: Build operations are now handled by the agent executing docker build
 * commands directly - see build-image tool for context preparation.
 * 
 * Prerequisites:
 * - Docker daemon must be running
 */

import { createDockerClient } from '../../../../src/infra/docker/client';
import { createLogger } from '../../../../src/lib/logger';
import type { DockerClient } from '../../../../src/infra/docker/client';

describe('Docker Client Error Handling Integration Tests', () => {
  let dockerClient: DockerClient;
  const logger = createLogger({ level: 'debug' });
  const testTimeout = 30000;

  beforeAll(async () => {
    dockerClient = createDockerClient(logger);
  });

  describe('Image Operations Error Detection', () => {
    test('should detect errors when getting non-existent images', async () => {
      const result = await dockerClient.getImage('nonexistent-image-12345:latest');

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toMatch(/not found|does not exist|404|No such image/i);
      }
    }, testTimeout);

    test('should detect errors when inspecting non-existent images', async () => {
      const result = await dockerClient.inspectImage('nonexistent-image-12345:latest');

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toMatch(/not found|does not exist|404|No such image/i);
      }
    }, testTimeout);

    test('should detect errors when tagging non-existent images', async () => {
      const result = await dockerClient.tagImage('nonexistent-image-12345:latest', 'new-repo', 'latest');

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toMatch(/not found|does not exist|no such image/i);
      }
    }, testTimeout);

    test('should detect errors when removing non-existent images', async () => {
      const result = await dockerClient.removeImage('nonexistent-image-12345:latest');

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toMatch(/not found|does not exist|no such image/i);
      }
    }, testTimeout);
  });

  describe('Container Operations Error Detection', () => {
    test('should detect errors when removing non-existent containers', async () => {
      const result = await dockerClient.removeContainer('nonexistent-container-12345');

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toMatch(/not found|does not exist|no such container/i);
      }
    }, testTimeout);

    test('should successfully list containers', async () => {
      const result = await dockerClient.listContainers({ all: false });

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(Array.isArray(result.value)).toBe(true);
      }
    }, testTimeout);
  });

  describe('Docker Daemon Connectivity', () => {
    test('should successfully ping Docker daemon', async () => {
      const result = await dockerClient.ping();

      expect(result.ok).toBe(true);
    }, testTimeout);
  });

  describe('Push Operations Error Detection', () => {
    test('should detect errors when pushing non-existent local images', async () => {
      // Try to push a non-existent local image
      const pushResult = await dockerClient.pushImage(
        'nonexistent-image-for-push-test-12345',
        'latest'
      );

      expect(pushResult.ok).toBe(false);
      if (!pushResult.ok) {
        // Should detect image not found error
        expect(pushResult.error).toMatch(
          /not found|does not exist|no such image|reference does not exist/i
        );
      }
    }, testTimeout);
  });

  describe('Error Message Quality', () => {
    test('should never return generic "Unknown error" for real Docker failures', async () => {
      const testCases = [
        () => dockerClient.getImage('nonexistent-image-12345:latest'),
        () => dockerClient.tagImage('nonexistent-image-12345:latest', 'repo', 'tag'),
        () => dockerClient.removeImage('nonexistent-image-12345:latest'),
        () => dockerClient.removeContainer('nonexistent-container-12345'),
      ];

      for (const testFn of testCases) {
        const result = await testFn();

        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).not.toBe('Unknown error');
          expect(result.error).not.toContain('Unknown error');
          expect(result.error.length).toBeGreaterThan(10);
        }
      }
    }, testTimeout);
  });
});
