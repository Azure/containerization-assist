/**
 * Unit tests for push-image tool with fake DockerClient
 */

import { describe, it, expect, beforeEach } from '@jest/globals';
import type { DockerClient } from '../../../src/services/docker-client';
import type { Result } from '../../../src/types';
import { makePushImage } from '../../../src/tools/push-image/tool';

describe('push-image tool', () => {
  let fakeDocker: DockerClient;
  let tagImageCalled: boolean;
  let pushImageCalled: boolean;
  
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
  });
  
  describe('success scenarios', () => {
    it('should push image successfully with digest', async () => {
      const tool = makePushImage(fakeDocker);
      const result = await tool.handler({
        imageId: 'myapp:v1.0.0'
      });
      
      expect(pushImageCalled).toBe(true);
      expect(result).toHaveProperty('value');
      expect(result).not.toHaveProperty('error');
      
      if ('value' in result) {
        expect(result.value).toMatchObject({
          success: true,
          registry: 'docker.io',
          pushedTag: 'myapp:v1.0.0'
        });
        expect(result.value?.digest).toMatch(/^sha256:/);
      }
      
      expect(result.content).toHaveLength(1);
      expect(result.content[0]).toMatchObject({
        type: 'text',
        text: expect.stringContaining('Successfully pushed image')
      });
    });
    
    it('should tag and push to custom registry', async () => {
      const tool = makePushImage(fakeDocker);
      const result = await tool.handler({
        imageId: 'myapp:v1.0.0',
        registry: 'gcr.io/my-project'
      });
      
      expect(tagImageCalled).toBe(true);
      expect(pushImageCalled).toBe(true);
      expect(result).toHaveProperty('value');
      
      if ('value' in result) {
        expect(result.value).toMatchObject({
          success: true,
          registry: 'gcr.io/my-project',
          pushedTag: 'gcr.io/my-project/myapp:v1.0.0'
        });
      }
    });
    
    it('should handle image without tag (default to latest)', async () => {
      const tool = makePushImage(fakeDocker);
      const result = await tool.handler({
        imageId: 'myapp'
      });
      
      expect(pushImageCalled).toBe(true);
      expect(result).toHaveProperty('value');
      
      if ('value' in result) {
        expect(result.value?.pushedTag).toBe('myapp:latest');
      }
    });
  });
  
  describe('failure scenarios', () => {
    it('should return error when imageId is missing', async () => {
      const tool = makePushImage(fakeDocker);
      const result = await tool.handler({});
      
      expect(pushImageCalled).toBe(false);
      expect(result).toHaveProperty('error');
      expect(result.error).toBe('Missing required parameter: imageId');
      
      expect(result.content[0]).toMatchObject({
        type: 'text',
        text: 'Error: imageId is required to push an image'
      });
    });
    
    it('should return error when tag fails', async () => {
      const tool = makePushImage(fakeDocker);
      const result = await tool.handler({
        imageId: 'bad-image',
        registry: 'gcr.io/my-project'
      });
      
      expect(tagImageCalled).toBe(true);
      expect(pushImageCalled).toBe(false);
      expect(result).toHaveProperty('error');
      expect(result.error).toBe('Image not found');
      
      expect(result.content[0]).toMatchObject({
        type: 'text',
        text: 'Failed to tag image: Image not found'
      });
    });
    
    it('should return error when push fails after successful tag', async () => {
      const tool = makePushImage(fakeDocker);
      const result = await tool.handler({
        imageId: 'fail/repo:v1',
        registry: 'my-registry.io'
      });
      
      expect(tagImageCalled).toBe(true);
      expect(pushImageCalled).toBe(true);
      expect(result).toHaveProperty('error');
      expect(result.error).toBe('Push failed: connection error');
      
      expect(result.content[0]).toMatchObject({
        type: 'text',
        text: 'Failed to push image: Push failed: connection error'
      });
    });
  });
  
  describe('edge cases', () => {
    it('should handle registry URL with protocol', async () => {
      const tool = makePushImage(fakeDocker);
      const result = await tool.handler({
        imageId: 'myapp:v1',
        registry: 'https://gcr.io/my-project/'
      });
      
      expect(result).toHaveProperty('value');
      if ('value' in result) {
        expect(result.value?.pushedTag).toBe('gcr.io/my-project/myapp:v1');
      }
    });
    
    it('should not double-prefix registry if already in imageId', async () => {
      const tool = makePushImage(fakeDocker);
      const result = await tool.handler({
        imageId: 'gcr.io/my-project/myapp:v1',
        registry: 'gcr.io/my-project'
      });
      
      expect(result).toHaveProperty('value');
      if ('value' in result) {
        expect(result.value?.pushedTag).toBe('gcr.io/my-project/myapp:v1');
      }
    });
  });
});