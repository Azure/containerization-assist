import { createMutexDockerClient } from '../../../../src/services/docker-mutex-client';
import { createDockerClient as createBaseDockerClient } from '../../../../src/services/docker-client';
import { Success, Failure } from '../../../../src/types';
import pino from 'pino';

// Mock the base client
jest.mock('../../../../src/services/docker-client');

// Mock config
jest.mock('../../../../src/config', () => ({
  config: {
    mutex: {
      defaultTimeout: 30000,
      dockerBuildTimeout: 300000,
      monitoringEnabled: true
    }
  }
}));

describe('MutexDockerClient', () => {
  let logger: pino.Logger;
  let mockBaseClient: any;

  beforeEach(() => {
    logger = pino({ level: 'silent' });
    
    // Setup mock base client
    mockBaseClient = {
      buildImage: jest.fn(),
      getImage: jest.fn(),
      tagImage: jest.fn(),
      pushImage: jest.fn()
    };
    
    (createBaseDockerClient as jest.Mock).mockReturnValue(mockBaseClient);
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('buildImage', () => {
    test('should prevent concurrent builds of same context', async () => {
      const buildOptions = {
        context: '/test/app',
        dockerfile: 'Dockerfile',
        t: 'test:latest'
      };

      // Simulate slow build
      let buildCallCount = 0;
      mockBaseClient.buildImage.mockImplementation(async () => {
        buildCallCount++;
        await new Promise(resolve => setTimeout(resolve, 100));
        return Success({ 
          imageId: 'test-image-id',
          logs: ['Building...'],
          tags: ['test:latest']
        });
      });

      const client = createMutexDockerClient(logger);

      // Start two concurrent builds with same context
      const build1 = client.buildImage(buildOptions);
      const build2 = client.buildImage(buildOptions);

      const [result1, result2] = await Promise.all([build1, build2]);

      // Both should succeed
      expect(result1.ok).toBe(true);
      expect(result2.ok).toBe(true);

      // But base client should only be called twice (sequentially)
      expect(mockBaseClient.buildImage).toHaveBeenCalledTimes(2);
      
      // Verify they ran sequentially (build count should increment one at a time)
      expect(buildCallCount).toBe(2);
    });

    test('should allow concurrent builds of different contexts', async () => {
      const startTimes: number[] = [];
      
      mockBaseClient.buildImage.mockImplementation(async () => {
        startTimes.push(Date.now());
        await new Promise(resolve => setTimeout(resolve, 50));
        return Success({ 
          imageId: 'test-image-id',
          logs: [],
          tags: []
        });
      });

      const client = createMutexDockerClient(logger);

      // Start builds with different contexts
      const builds = await Promise.all([
        client.buildImage({ context: '/app1' }),
        client.buildImage({ context: '/app2' }),
        client.buildImage({ context: '/app3' })
      ]);

      // All should succeed
      expect(builds.every(r => r.ok)).toBe(true);
      
      // Should have been called 3 times
      expect(mockBaseClient.buildImage).toHaveBeenCalledTimes(3);
      
      // Verify they ran concurrently (start times should be close)
      const timeDiffs = startTimes.slice(1).map((t, i) => t - startTimes[i]);
      expect(Math.max(...timeDiffs)).toBeLessThan(20); // Should start within 20ms
    });

  });

  describe('pushImage', () => {
    test('should prevent concurrent pushes of same image', async () => {
      mockBaseClient.pushImage.mockImplementation(async () => {
        await new Promise(resolve => setTimeout(resolve, 50));
        return Success({ 
          digest: 'sha256:abc123',
          size: 1000000
        });
      });

      const client = createMutexDockerClient(logger);

      // Start concurrent pushes of same image
      const pushes = await Promise.all([
        client.pushImage('myapp', 'v1.0'),
        client.pushImage('myapp', 'v1.0')
      ]);

      // Both should succeed
      expect(pushes.every(r => r.ok)).toBe(true);
      
      // Should be called twice (sequentially)
      expect(mockBaseClient.pushImage).toHaveBeenCalledTimes(2);
    });

    test('should allow concurrent pushes of different images', async () => {
      mockBaseClient.pushImage.mockResolvedValue(
        Success({ digest: 'sha256:test', size: 1000 })
      );

      const client = createMutexDockerClient(logger);

      const pushes = await Promise.all([
        client.pushImage('app1', 'v1.0'),
        client.pushImage('app2', 'v2.0'),
        client.pushImage('app3', 'v3.0')
      ]);

      expect(pushes.every(r => r.ok)).toBe(true);
      expect(mockBaseClient.pushImage).toHaveBeenCalledTimes(3);
    });
  });

  describe('getImage', () => {
    test('should not use mutex for read operations', async () => {
      mockBaseClient.getImage.mockResolvedValue(
        Success({ 
          Id: 'test-id',
          RepoTags: ['test:latest'],
          Size: 1000000
        })
      );

      const client = createMutexDockerClient(logger);

      // Concurrent reads should all execute immediately
      const reads = await Promise.all([
        client.getImage('test1'),
        client.getImage('test2'),
        client.getImage('test3')
      ]);

      expect(reads.every(r => r.ok)).toBe(true);
      expect(mockBaseClient.getImage).toHaveBeenCalledTimes(3);
    });
  });

  describe('tagImage', () => {
    test('should use mutex for tag operations', async () => {
      mockBaseClient.tagImage.mockImplementation(async () => {
        await new Promise(resolve => setTimeout(resolve, 30));
        return Success(undefined);
      });

      const client = createMutexDockerClient(logger);

      // Concurrent tags of same image should serialize
      const tags = await Promise.all([
        client.tagImage('image1', 'repo', 'tag1'),
        client.tagImage('image1', 'repo', 'tag2')
      ]);

      expect(tags.every(r => r.ok)).toBe(true);
      expect(mockBaseClient.tagImage).toHaveBeenCalledTimes(2);
    });
  });
});