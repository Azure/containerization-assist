import { describe, it, expect, jest, beforeEach } from '@jest/globals';
import {
  getImageMetadata,
  DockerRegistry,
  createDockerRegistry,
  type ImageMetadata,
  type RegistryConfig,
} from '../../../../src/infra/docker/registry';
import Docker from 'dockerode';

// Mock fetch globally
global.fetch = jest.fn() as jest.MockedFunction<typeof fetch>;

describe('Docker Registry Client', () => {
  let mockLogger: any;

  beforeEach(() => {
    jest.clearAllMocks();

    mockLogger = {
      debug: jest.fn(),
      info: jest.fn(),
      warn: jest.fn(),
      error: jest.fn(),
    };
  });

  describe('getImageMetadata', () => {
    it('should fetch real metadata from Docker Hub for official images', async () => {
      const mockResponse = {
        ok: true,
        json: jest.fn().mockResolvedValue({
          digest: 'sha256:abc123',
          full_size: 50000000,
          last_updated: '2023-01-01T00:00:00Z',
          images: [{ architecture: 'amd64', os: 'linux' }],
        }),
      };

      (global.fetch as jest.Mock).mockResolvedValue(mockResponse);

      const result = await getImageMetadata('node', '18-alpine', mockLogger);

      expect(result).toEqual({
        name: 'node',
        tag: '18-alpine',
        digest: 'sha256:abc123',
        size: 50000000,
        lastUpdated: '2023-01-01T00:00:00Z',
        architecture: 'amd64',
        os: 'linux',
      });

      expect(global.fetch).toHaveBeenCalledWith(
        'https://hub.docker.com/v2/repositories/library/node/tags/18-alpine',
        expect.objectContaining({
          headers: { Accept: 'application/json' },
          signal: expect.any(AbortSignal),
        })
      );

      expect(mockLogger.debug).toHaveBeenCalledWith(
        { imageName: 'node', tag: '18-alpine', size: 50000000 },
        'Fetched real image metadata'
      );
    });

    it('should fetch real metadata from Docker Hub for user/org images', async () => {
      const mockResponse = {
        ok: true,
        json: jest.fn().mockResolvedValue({
          digest: 'sha256:def456',
          size: 100000000,
          tag_last_pushed: '2023-02-01T00:00:00Z',
          images: [{ architecture: 'arm64', os: 'linux' }],
        }),
      };

      (global.fetch as jest.Mock).mockResolvedValue(mockResponse);

      const result = await getImageMetadata('myorg/myapp', 'latest', mockLogger);

      expect(result.digest).toBe('sha256:def456');
      expect(result.size).toBe(100000000);
      expect(result.architecture).toBe('arm64');

      expect(global.fetch).toHaveBeenCalledWith(
        'https://hub.docker.com/v2/repositories/myorg/myapp/tags/latest',
        expect.objectContaining({
          headers: { Accept: 'application/json' },
          signal: expect.any(AbortSignal),
        })
      );
    });

    it('should fallback to estimates when Docker Hub fetch fails', async () => {
      const mockResponse = {
        ok: false,
        status: 404,
      };

      (global.fetch as jest.Mock).mockResolvedValue(mockResponse);

      const result = await getImageMetadata('node', '18-alpine', mockLogger);

      expect(result.name).toBe('node');
      expect(result.tag).toBe('18-alpine');
      expect(result.size).toBe(5 * 1024 * 1024); // 5MB estimate for alpine tag (tag pattern takes priority)
      expect(result.lastUpdated).toBeDefined();
      expect(result.digest).toBeUndefined();

      expect(mockLogger.debug).toHaveBeenCalledWith(
        { imageName: 'node', tag: '18-alpine', status: 404 },
        'Failed to fetch from Docker Hub'
      );

      expect(mockLogger.debug).toHaveBeenCalledWith(
        { imageName: 'node', tag: '18-alpine', estimatedSize: 5 * 1024 * 1024 },
        'Using estimated image metadata'
      );
    });

    it('should fallback to estimates when fetch throws an error', async () => {
      (global.fetch as jest.Mock).mockRejectedValue(new Error('Network error'));

      const result = await getImageMetadata('python', '3.11-slim', mockLogger);

      expect(result.name).toBe('python');
      expect(result.tag).toBe('3.11-slim');
      expect(result.size).toBe(150 * 1024 * 1024); // 150MB estimate for python slim
      expect(result.lastUpdated).toBeDefined();

      expect(mockLogger.debug).toHaveBeenCalledWith(
        { error: expect.any(Error), imageName: 'python', tag: '3.11-slim' },
        'Error fetching Docker Hub metadata'
      );
    });

    describe('size estimation', () => {
      beforeEach(() => {
        // Mock fetch to always fail so we test estimation
        (global.fetch as jest.Mock).mockResolvedValue({ ok: false, status: 404 });
      });

      it('should estimate sizes for alpine images', async () => {
        const result = await getImageMetadata('custom/app', 'alpine', mockLogger);
        expect(result.size).toBe(5 * 1024 * 1024); // 5MB
      });

      it('should estimate sizes for scratch images', async () => {
        const result = await getImageMetadata('custom/app', 'scratch', mockLogger);
        expect(result.size).toBe(0); // 0MB
      });

      it('should estimate sizes for slim images', async () => {
        const result = await getImageMetadata('custom/app', 'slim', mockLogger);
        expect(result.size).toBe(150 * 1024 * 1024); // 150MB
      });

      it('should estimate sizes for debian bullseye images', async () => {
        const result = await getImageMetadata('custom/app', 'bullseye', mockLogger);
        expect(result.size).toBe(250 * 1024 * 1024); // 250MB
      });

      it('should estimate sizes for Node.js images', async () => {
        let result = await getImageMetadata('node', '18-alpine', mockLogger);
        expect(result.size).toBe(5 * 1024 * 1024); // 5MB (alpine tag pattern takes priority)

        result = await getImageMetadata('node', '18-slim', mockLogger);
        expect(result.size).toBe(150 * 1024 * 1024); // 150MB (slim tag pattern takes priority)

        result = await getImageMetadata('node', '18', mockLogger);
        expect(result.size).toBe(350 * 1024 * 1024); // 350MB (node image pattern)
      });

      it('should estimate sizes for Python images', async () => {
        let result = await getImageMetadata('python', '3.11-alpine', mockLogger);
        expect(result.size).toBe(5 * 1024 * 1024); // 5MB (alpine tag pattern takes priority)

        result = await getImageMetadata('python', '3.11-slim', mockLogger);
        expect(result.size).toBe(150 * 1024 * 1024); // 150MB (slim tag pattern takes priority)

        result = await getImageMetadata('python', '3.11', mockLogger);
        expect(result.size).toBe(400 * 1024 * 1024); // 400MB (python image pattern)
      });

      it('should estimate sizes for Go images', async () => {
        let result = await getImageMetadata('golang', '1.20-alpine', mockLogger);
        expect(result.size).toBe(5 * 1024 * 1024); // 5MB (alpine tag pattern takes priority)

        result = await getImageMetadata('golang', '1.20', mockLogger);
        expect(result.size).toBe(800 * 1024 * 1024); // 800MB (golang image pattern)
      });

      it('should estimate sizes for Java images', async () => {
        let result = await getImageMetadata('openjdk', '17-alpine', mockLogger);
        expect(result.size).toBe(5 * 1024 * 1024); // 5MB (alpine tag pattern takes priority)

        result = await getImageMetadata('eclipse-temurin', '17-jdk-slim', mockLogger);
        expect(result.size).toBe(150 * 1024 * 1024); // 150MB (slim tag pattern takes priority)

        result = await getImageMetadata('openjdk', '17', mockLogger);
        expect(result.size).toBe(600 * 1024 * 1024); // 600MB (openjdk image pattern)
      });

      it('should use default estimate for unknown images', async () => {
        const result = await getImageMetadata('unknown/image', 'latest', mockLogger);
        expect(result.size).toBe(500 * 1024 * 1024); // 500MB for latest tag pattern
      });

      it('should prioritize tag patterns over image name patterns', async () => {
        // alpine tag should override node image estimation, but node + alpine gives 50MB
        const result = await getImageMetadata('node', 'alpine', mockLogger);
        expect(result.size).toBe(5 * 1024 * 1024); // 5MB for alpine tag pattern
      });
    });
  });


  describe('Docker Hub API integration', () => {
    it('should handle official images with library namespace', async () => {
      const mockResponse = { ok: true, json: jest.fn().mockResolvedValue({}) };
      (global.fetch as jest.Mock).mockResolvedValue(mockResponse);

      await getImageMetadata('redis', 'latest', mockLogger);

      expect(global.fetch).toHaveBeenCalledWith(
        'https://hub.docker.com/v2/repositories/library/redis/tags/latest',
        expect.objectContaining({
          headers: { Accept: 'application/json' },
          signal: expect.any(AbortSignal),
        })
      );
    });

    it('should handle user/org images without library namespace', async () => {
      const mockResponse = { ok: true, json: jest.fn().mockResolvedValue({}) };
      (global.fetch as jest.Mock).mockResolvedValue(mockResponse);

      await getImageMetadata('bitnami/redis', '7.0', mockLogger);

      expect(global.fetch).toHaveBeenCalledWith(
        'https://hub.docker.com/v2/repositories/bitnami/redis/tags/7.0',
        expect.objectContaining({
          headers: { Accept: 'application/json' },
          signal: expect.any(AbortSignal),
        })
      );
    });

    it('should handle nested repository paths', async () => {
      const mockResponse = { ok: true, json: jest.fn().mockResolvedValue({}) };
      (global.fetch as jest.Mock).mockResolvedValue(mockResponse);

      await getImageMetadata('registry.io/org/repo', 'v1.0', mockLogger);

      expect(global.fetch).toHaveBeenCalledWith(
        'https://hub.docker.com/v2/repositories/registry.io/org/tags/v1.0',
        expect.objectContaining({
          headers: { Accept: 'application/json' },
          signal: expect.any(AbortSignal),
        })
      );
    });

    it('should handle response with alternative size field names', async () => {
      const mockResponse = {
        ok: true,
        json: jest.fn().mockResolvedValue({
          size: 25000000, // Alternative to full_size
          tag_last_pushed: '2023-04-01T00:00:00Z', // Alternative to last_updated
        }),
      };

      (global.fetch as jest.Mock).mockResolvedValue(mockResponse);

      const result = await getImageMetadata('alpine', 'latest', mockLogger);

      expect(result.size).toBe(25000000);
      expect(result.lastUpdated).toBe('2023-04-01T00:00:00Z');
    });

    it('should handle response with missing optional fields', async () => {
      const mockResponse = {
        ok: true,
        json: jest.fn().mockResolvedValue({
          // Only minimal data
        }),
      };

      (global.fetch as jest.Mock).mockResolvedValue(mockResponse);

      const result = await getImageMetadata('minimal/image', 'latest', mockLogger);

      expect(result.name).toBe('minimal/image');
      expect(result.tag).toBe('latest');
      expect(result.digest).toBeUndefined();
      expect(result.size).toBeUndefined();
      expect(result.lastUpdated).toBeUndefined();
      expect(result.architecture).toBeUndefined();
      expect(result.os).toBeUndefined();
    });
  });

  describe('DockerRegistry', () => {
    let mockDocker: any;
    let registryClient: DockerRegistry;

    beforeEach(() => {
      jest.clearAllMocks();

      mockDocker = {
        checkAuth: jest.fn(),
        getImage: jest.fn(),
      };

      registryClient = createDockerRegistry(mockDocker as unknown as Docker, mockLogger);
    });

    describe('authenticate', () => {
      it('should authenticate with username and password', async () => {
        const config: RegistryConfig = {
          url: 'https://registry.example.com',
          username: 'testuser',
          password: 'testpass',
        };

        mockDocker.checkAuth.mockResolvedValue(undefined);

        const result = await registryClient.authenticate(config);

        expect(result.ok).toBe(true);
        expect(mockDocker.checkAuth).toHaveBeenCalledWith({
          username: 'testuser',
          password: 'testpass',
          serveraddress: 'registry.example.com',
        });
        expect(mockLogger.info).toHaveBeenCalledWith(
          { registry: 'registry.example.com' },
          'Registry authentication successful',
        );
      });

      it('should authenticate with token', async () => {
        const config: RegistryConfig = {
          url: 'docker.io',
          token: 'test-token-123',
        };

        mockDocker.checkAuth.mockResolvedValue(undefined);

        const result = await registryClient.authenticate(config);

        expect(result.ok).toBe(true);
        expect(mockDocker.checkAuth).toHaveBeenCalledWith({
          username: '',
          password: 'test-token-123',
          serveraddress: 'https://index.docker.io/v1/',
        });
      });

      it('should fail when no credentials provided', async () => {
        const config: RegistryConfig = {
          url: 'registry.example.com',
        };

        const result = await registryClient.authenticate(config);

        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('username/password or token');
          expect(result.guidance?.hint).toBeDefined();
        }
      });

      it('should handle authentication failure', async () => {
        const config: RegistryConfig = {
          url: 'registry.example.com',
          username: 'wronguser',
          password: 'wrongpass',
        };

        mockDocker.checkAuth.mockRejectedValue(new Error('Invalid credentials'));

        const result = await registryClient.authenticate(config);

        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('Authentication failed');
          expect(result.guidance?.message).toBe('Registry authentication failed');
          expect(result.guidance?.hint).toContain('Invalid credentials');
        }
      });
    });

    describe('healthCheck', () => {
      it('should return true for accessible registry (200)', async () => {
        const mockResponse = { ok: true, status: 200 };
        (global.fetch as jest.Mock).mockResolvedValue(mockResponse);

        const result = await registryClient.healthCheck('https://registry.example.com');

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value).toBe(true);
        }
      });

      it('should return true for accessible registry requiring auth (401)', async () => {
        const mockResponse = { ok: false, status: 401 };
        (global.fetch as jest.Mock).mockResolvedValue(mockResponse);

        const result = await registryClient.healthCheck('https://registry.example.com');

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value).toBe(true);
        }
      });

      it('should return false for inaccessible registry', async () => {
        const mockResponse = { ok: false, status: 500 };
        (global.fetch as jest.Mock).mockResolvedValue(mockResponse);

        const result = await registryClient.healthCheck('https://registry.example.com');

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value).toBe(false);
        }
      });

      it('should return false on network error', async () => {
        (global.fetch as jest.Mock).mockRejectedValue(new Error('Network error'));

        const result = await registryClient.healthCheck('https://registry.example.com');

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value).toBe(false);
        }
      });

      it('should handle Docker Hub health check', async () => {
        const mockResponse = { ok: true, status: 200 };
        (global.fetch as jest.Mock).mockResolvedValue(mockResponse);

        const result = await registryClient.healthCheck('docker.io');

        expect(result.ok).toBe(true);
        expect(global.fetch).toHaveBeenCalledWith(
          'https://registry-1.docker.io/v2/',
          expect.any(Object),
        );
      });
    });

    describe('imageExists', () => {
      it('should return true when image exists', async () => {
        const mockImage = {
          inspect: jest.fn().mockResolvedValue({ Id: 'sha256:abc123' }),
        };

        mockDocker.getImage.mockReturnValue(mockImage);

        const result = await registryClient.imageExists('node:18-alpine');

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value).toBe(true);
        }
        expect(mockDocker.getImage).toHaveBeenCalledWith('node:18-alpine');
      });

      it('should return false when image does not exist (404)', async () => {
        const mockImage = {
          inspect: jest.fn().mockRejectedValue(new Error('(HTTP code 404) no such image')),
        };

        mockDocker.getImage.mockReturnValue(mockImage);

        const result = await registryClient.imageExists('nonexistent:latest');

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value).toBe(false);
        }
      });

      it('should handle image name with digest', async () => {
        const mockImage = {
          inspect: jest.fn().mockResolvedValue({ Id: 'sha256:abc123' }),
        };

        mockDocker.getImage.mockReturnValue(mockImage);

        const result = await registryClient.imageExists('node@sha256:abc123');

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value).toBe(true);
        }
      });
    });

    describe('listTags', () => {
      it('should list tags for Docker Hub official repository', async () => {
        const mockResponse = {
          ok: true,
          json: jest.fn().mockResolvedValue({
            results: [{ name: 'latest' }, { name: '18-alpine' }, { name: '18-slim' }],
          }),
        };

        (global.fetch as jest.Mock).mockResolvedValue(mockResponse);

        const result = await registryClient.listTags('node');

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value).toEqual(['latest', '18-alpine', '18-slim']);
        }
        expect(global.fetch).toHaveBeenCalledWith(
          'https://hub.docker.com/v2/repositories/library/node/tags?page_size=100',
          expect.objectContaining({
            headers: { Accept: 'application/json' },
          }),
        );
      });

      it('should list tags for Docker Hub user/org repository', async () => {
        const mockResponse = {
          ok: true,
          json: jest.fn().mockResolvedValue({
            results: [{ name: 'v1.0.0' }, { name: 'v1.1.0' }],
          }),
        };

        (global.fetch as jest.Mock).mockResolvedValue(mockResponse);

        const result = await registryClient.listTags('bitnami/redis');

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value).toEqual(['v1.0.0', 'v1.1.0']);
        }
        expect(global.fetch).toHaveBeenCalledWith(
          'https://hub.docker.com/v2/repositories/bitnami/redis/tags?page_size=100',
          expect.any(Object),
        );
      });

      it('should list tags for private registry', async () => {
        // First authenticate
        const config: RegistryConfig = {
          url: 'registry.example.com',
          username: 'testuser',
          password: 'testpass',
        };

        mockDocker.checkAuth.mockResolvedValue(undefined);
        await registryClient.authenticate(config);

        // Then list tags
        const mockResponse = {
          ok: true,
          json: jest.fn().mockResolvedValue({
            tags: ['v1.0', 'v2.0', 'latest'],
          }),
        };

        (global.fetch as jest.Mock).mockResolvedValue(mockResponse);

        const result = await registryClient.listTags('registry.example.com/myapp');

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value).toEqual(['v1.0', 'v2.0', 'latest']);
        }
        expect(global.fetch).toHaveBeenCalledWith(
          'https://registry.example.com/v2/myapp/tags/list',
          expect.objectContaining({
            headers: expect.objectContaining({
              Accept: 'application/json',
              Authorization: expect.stringContaining('Basic'),
            }),
          }),
        );
      });

      it('should handle unauthorized access to private registry', async () => {
        const mockResponse = { ok: false, status: 401 };

        (global.fetch as jest.Mock).mockResolvedValue(mockResponse);

        const result = await registryClient.listTags('registry.example.com/private/repo');

        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('Authentication required');
          expect(result.guidance?.hint).toContain('Unauthorized');
        }
      });

      it('should handle API errors', async () => {
        const mockResponse = { ok: false, status: 404 };

        (global.fetch as jest.Mock).mockResolvedValue(mockResponse);

        const result = await registryClient.listTags('node');

        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('404');
        }
      });
    });

    describe('createDockerRegistry', () => {
      it('should create a registry client instance', () => {
        const client = createDockerRegistry(mockDocker as unknown as Docker, mockLogger);

        expect(client).toBeInstanceOf(DockerRegistry);
      });
    });
  });
});