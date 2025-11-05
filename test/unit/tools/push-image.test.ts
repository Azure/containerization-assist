/**
 * Unit tests for push-image tool with fake DockerClient
 */

import { describe, it, expect, beforeEach } from '@jest/globals';
import type { DockerClient } from '../../../src/infra/docker/client';
import type { Result } from '../../../src/types';
import pushImageTool from '../../../src/tools/push-image/tool';
import type { ToolContext } from '../../../src/types';

describe('push-image tool', () => {
  let fakeDocker: DockerClient;
  let tagImageCalled: boolean;
  let pushImageCalled: boolean;
  let createMockContext: () => ToolContext;

  beforeEach(() => {
    tagImageCalled = false;
    pushImageCalled = false;

    // Create fake DockerClient implementation
    fakeDocker = {
      async pushImage(repository: string, tag: string): Promise<Result<{ digest: string }>> {
        pushImageCalled = true;
        if (repository.includes('fail/repo')) {
          return { ok: false, error: 'Push failed: connection error' };
        }
        return {
          ok: true,
          value: { digest: `sha256:${Date.now().toString(16)}` }
        };
      },

      async tagImage(source: string, repo: string, tag: string): Promise<Result<void>> {
        tagImageCalled = true;
        if (source === 'bad-image') {
          return { ok: false, error: 'Image not found' };
        }
        return { ok: true, value: undefined };
      },

      async buildImage(): Promise<Result<{ imageId: string; logs?: string[] }>> {
        // Not used in push-image
        return { ok: true, value: { imageId: 'test-id' } };
      },

      async getImage(): Promise<Result<any>> {
        // Not used in push-image
        return { ok: true, value: {} };
      },

      async removeImage(): Promise<Result<void>> {
        // Not used in push-image
        return { ok: true, value: undefined };
      },

      async removeContainer(): Promise<Result<void>> {
        // Not used in push-image
        return { ok: true, value: undefined };
      },

      async pullImage(): Promise<Result<void>> {
        // Not used in push-image
        return { ok: true, value: undefined };
      },

      async runContainer(): Promise<Result<any>> {
        // Not used in push-image
        return { ok: true, value: {} };
      },

      async listImages(): Promise<Result<any[]>> {
        // Not used in push-image
        return { ok: true, value: [] };
      },

      async listContainers(): Promise<Result<any[]>> {
        // Not used in push-image
        return { ok: true, value: [] };
      },

      async inspectContainer(): Promise<Result<any>> {
        // Not used in push-image
        return { ok: true, value: {} };
      },

      async execContainer(): Promise<Result<any>> {
        // Not used in push-image
        return { ok: true, value: {} };
      },

      async getContainerLogs(): Promise<Result<string>> {
        // Not used in push-image
        return { ok: true, value: '' };
      },

      async pruneImages(): Promise<Result<any>> {
        // Not used in push-image
        return { ok: true, value: {} };
      }
    };

    // Create mock context factory after fakeDocker is defined
    createMockContext = () => ({
      logger: {
        info: jest.fn(),
        warn: jest.fn(),
        error: jest.fn(),
        debug: jest.fn(),
        trace: jest.fn(),
      } as any,
      docker: fakeDocker,
    } as ToolContext);
  });

  describe('success scenarios', () => {
    it('should push Docker Hub image without registry parameter (defaults to docker.io)', async () => {
      const result = await pushImageTool.handler({
        imageId: 'myapp:v1.0.0'
      }, createMockContext());

      expect(result.ok).toBe(true);
      expect(pushImageCalled).toBe(true);

      if (result.ok) {
        expect(result.value).toMatchObject({
          success: true,
          registry: 'docker.io',
          pushedTag: 'myapp:v1.0.0'
        });
        expect(result.value.digest).toMatch(/^sha256:/);
      }
    });

    it('should push to custom registry using registry parameter', async () => {
      const result = await pushImageTool.handler({
        imageId: 'myapp:v1.0.0',
        registry: 'myregistry.azurecr.io'
      }, createMockContext());

      expect(result.ok).toBe(true);
      expect(tagImageCalled).toBe(true);
      expect(pushImageCalled).toBe(true);

      if (result.ok) {
        expect(result.value).toMatchObject({
          success: true,
          registry: 'myregistry.azurecr.io',
          pushedTag: 'myregistry.azurecr.io/myapp:v1.0.0'
        });
      }
    });

    it('should push to GCR using registry parameter', async () => {
      const result = await pushImageTool.handler({
        imageId: 'my-project/myapp:v1.0.0',
        registry: 'gcr.io'
      }, createMockContext());

      expect(result.ok).toBe(true);
      expect(tagImageCalled).toBe(true);
      expect(pushImageCalled).toBe(true);

      if (result.ok) {
        expect(result.value).toMatchObject({
          success: true,
          registry: 'gcr.io',
          pushedTag: 'gcr.io/my-project/myapp:v1.0.0'
        });
      }
    });

    it('should push to ACR using registry parameter with namespace', async () => {
      const result = await pushImageTool.handler({
        imageId: 'production/myapp:v1.0.0',
        registry: 'myregistry.azurecr.io'
      }, createMockContext());

      expect(result.ok).toBe(true);
      expect(tagImageCalled).toBe(true);
      expect(pushImageCalled).toBe(true);

      if (result.ok) {
        expect(result.value).toMatchObject({
          success: true,
          registry: 'myregistry.azurecr.io',
          pushedTag: 'myregistry.azurecr.io/production/myapp:v1.0.0'
        });
      }
    });

    it('should push to localhost registry using registry parameter', async () => {
      const result = await pushImageTool.handler({
        imageId: 'myapp:v1.0.0',
        registry: 'localhost:5000'
      }, createMockContext());

      expect(result.ok).toBe(true);
      expect(tagImageCalled).toBe(true);
      expect(pushImageCalled).toBe(true);

      if (result.ok) {
        expect(result.value).toMatchObject({
          success: true,
          registry: 'localhost:5000',
          pushedTag: 'localhost:5000/myapp:v1.0.0'
        });
      }
    });

    it('should handle image without tag (default to latest)', async () => {
      const result = await pushImageTool.handler({
        imageId: 'myapp'
      }, createMockContext());

      expect(result.ok).toBe(true);
      expect(pushImageCalled).toBe(true);

      if (result.ok) {
        expect(result.value.pushedTag).toBe('myapp:latest');
        expect(result.value.registry).toBe('docker.io');
      }
    });
  });

  describe('failure scenarios', () => {
    it('should return error when imageId is missing', async () => {
      const result = await pushImageTool.handler({} as any, createMockContext());

      expect(result.ok).toBe(false);
      expect(pushImageCalled).toBe(false);

      if (!result.ok) {
        expect(result.error).toBe('Missing required parameter: imageId');
      }
    });

    it('should return error when tag fails', async () => {
      const result = await pushImageTool.handler({
        imageId: 'bad-image'
      }, createMockContext());

      expect(result.ok).toBe(false);
      expect(tagImageCalled).toBe(true);
      expect(pushImageCalled).toBe(false);

      if (!result.ok) {
        expect(result.error).toBe('Failed to tag image: Image not found');
      }
    });

    it('should return error when push fails', async () => {
      const result = await pushImageTool.handler({
        imageId: 'fail/repo:v1'
      }, createMockContext());

      expect(result.ok).toBe(false);
      expect(tagImageCalled).toBe(true);
      expect(pushImageCalled).toBe(true);

      if (!result.ok) {
        expect(result.error).toBe('Failed to push image: Push failed: connection error');
      }
    });
  });

  describe('summary formatting', () => {
    it('should generate summary with truncated sha256 digest for Docker Hub', async () => {
      // Mock pushImage to return a specific sha256 digest
      fakeDocker.pushImage = async () => ({
        ok: true,
        value: { digest: 'sha256:abcdef1234567890abcdef1234567890' }
      });

      const result = await pushImageTool.handler({
        imageId: 'myapp:v1.0.0'
      }, createMockContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.summary).toBeDefined();
        expect(result.value.summary).toContain('sha256:abcdef...');
        expect(result.value.summary).toContain('✅ Pushed image to registry');
        expect(result.value.summary).toContain('docker.io/myapp:v1.0.0');
      }
    });

    it('should generate summary for custom registry with registry parameter', async () => {
      fakeDocker.pushImage = async () => ({
        ok: true,
        value: { digest: 'sha256:abcdef1234567890abcdef1234567890' }
      });

      const result = await pushImageTool.handler({
        imageId: 'my-project/myapp:v1.0.0',
        registry: 'gcr.io'
      }, createMockContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.summary).toBeDefined();
        expect(result.value.summary).toContain('sha256:abcdef...');
        expect(result.value.summary).toContain('gcr.io/my-project/myapp:v1.0.0');
      }
    });

    it('should handle sha512 digest correctly', async () => {
      // Mock pushImage to return a sha512 digest
      fakeDocker.pushImage = async () => ({
        ok: true,
        value: { digest: 'sha512:fedcba9876543210fedcba9876543210' }
      });

      const result = await pushImageTool.handler({
        imageId: 'myapp:v1.0.0'
      }, createMockContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.summary).toBeDefined();
        expect(result.value.summary).toContain('sha512:fedcba...');
        expect(result.value.summary).not.toContain('sha512:fedcba9876543210'); // Should be truncated
      }
    });

    it('should handle digest with different algorithm correctly', async () => {
      // Mock pushImage to return a blake2b digest (hypothetical future algorithm)
      fakeDocker.pushImage = async () => ({
        ok: true,
        value: { digest: 'blake2b:xyz123abc456def789ghi012' }
      });

      const result = await pushImageTool.handler({
        imageId: 'myapp:v1.0.0'
      }, createMockContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.summary).toBeDefined();
        expect(result.value.summary).toContain('blake2b:xyz123...');
      }
    });
  });
});
