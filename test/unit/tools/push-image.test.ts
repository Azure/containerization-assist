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
      sessionManager: {} as any,
      sampling: {} as any,
      docker: fakeDocker,
    } as ToolContext);
  });
  
  describe('success scenarios', () => {
    it('should push image successfully with digest', async () => {
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
    
    it('should tag and push to custom registry', async () => {
      const result = await pushImageTool.handler({
        imageId: 'myapp:v1.0.0',
        registry: 'gcr.io/my-project'
      }, createMockContext());
      
      expect(result.ok).toBe(true);
      expect(tagImageCalled).toBe(true);
      expect(pushImageCalled).toBe(true);

      if (result.ok) {
        expect(result.value).toMatchObject({
          success: true,
          registry: 'gcr.io/my-project',
          pushedTag: 'gcr.io/my-project/myapp:v1.0.0'
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
        imageId: 'bad-image',
        registry: 'gcr.io/my-project'
      }, createMockContext());
      
      expect(result.ok).toBe(false);
      expect(tagImageCalled).toBe(true);
      expect(pushImageCalled).toBe(false);

      if (!result.ok) {
        expect(result.error).toBe('Failed to tag image: Image not found');
      }
    });
    
    it('should return error when push fails after successful tag', async () => {
      const result = await pushImageTool.handler({
        imageId: 'fail/repo:v1',
        registry: 'my-registry.io'
      }, createMockContext());
      
      expect(result.ok).toBe(false);
      expect(tagImageCalled).toBe(true);
      expect(pushImageCalled).toBe(true);

      if (!result.ok) {
        expect(result.error).toBe('Failed to push image: Push failed: connection error');
      }
    });
  });
  
  describe('edge cases', () => {
    it('should handle registry URL with protocol', async () => {
      const result = await pushImageTool.handler({
        imageId: 'myapp:v1',
        registry: 'https://gcr.io/my-project/'
      }, createMockContext());
      
      expect(result.ok).toBe(true);

      if (result.ok) {
        expect(result.value.pushedTag).toBe('gcr.io/my-project/myapp:v1');
      }
    });
    
    it('should not double-prefix registry if already in imageId', async () => {
      const result = await pushImageTool.handler({
        imageId: 'gcr.io/my-project/myapp:v1',
        registry: 'gcr.io/my-project'
      }, createMockContext());
      
      expect(result.ok).toBe(true);

      if (result.ok) {
        expect(result.value.pushedTag).toBe('gcr.io/my-project/myapp:v1');
      }
    });
  });
});