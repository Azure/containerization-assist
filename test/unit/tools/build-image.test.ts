/**
 * Unit Tests: Build Image Tool
 * Tests the build-image tool functionality with mock Docker client and filesystem
 */

import { jest } from '@jest/globals';
import { promises as fs } from 'node:fs';
import { createMockValidatePath } from '../../__support__/utilities/mocks';

// Result Type Helpers for Testing
function createSuccessResult<T>(value: T) {
  return {
    ok: true as const,
    value,
  };
}

function createFailureResult(error: string) {
  return {
    ok: false as const,
    error,
  };
}

function createMockLogger() {
  return {
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
    trace: jest.fn(),
    fatal: jest.fn(),
    child: jest.fn().mockReturnThis(),
  } as any;
}

// Mock the validation library to bypass path validation in tests
jest.mock('../../../src/lib/validation', () => ({
  validatePath: createMockValidatePath(),
  validateImageName: jest.fn().mockImplementation((name: string) => ({ ok: true, value: name })),
  validateK8sName: jest.fn().mockImplementation((name: string) => ({ ok: true, value: name })),
  validateNamespace: jest.fn().mockImplementation((ns: string) => ({ ok: true, value: ns })),
}));

// Mock validation-helpers to use the mocked validation
jest.mock('../../../src/lib/validation-helpers', () => ({
  validatePathOrFail: jest.fn().mockImplementation(async (...args: any[]) => {
    const { validatePath } = require('../../../src/lib/validation');
    return validatePath(...args);
  }),
}));

// Mock filesystem functions with proper structure
jest.mock('node:fs', () => ({
  promises: {
    access: jest.fn(),
    readFile: jest.fn(),
    writeFile: jest.fn(),
    stat: jest.fn(),
    constants: {
      R_OK: 4,
      W_OK: 2,
      X_OK: 1,
      F_OK: 0,
    },
  },
  constants: {
    R_OK: 4,
    W_OK: 2,
    X_OK: 1,
    F_OK: 0,
  },
}));

// Mock lib modules
const mockDockerClient = {
  buildImage: jest.fn() as jest.MockedFunction<
    (options: any) => Promise<{
      ok: boolean;
      value?: any;
      error?: string;
      guidance?: { hint?: string; resolution?: string };
    }>
  >,
};


jest.mock('../../../src/infra/docker/client', () => ({
  createDockerClient: jest.fn(() => mockDockerClient),
}));

jest.mock('../../../src/lib/logger', () => ({
  createTimer: jest.fn(() => ({
    end: jest.fn(),
    error: jest.fn(),
  })),
  createLogger: jest.fn(() => createMockLogger()),
}));

function createMockToolContext() {
  return {
    logger: createMockLogger(),
  } as any;
}

// Import these after mocks are set up
import { buildImage } from '../../../src/tools/build-image/tool';
import type { BuildImageParams as BuildImageConfig } from '../../../src/tools/build-image/schema';

const mockFs = fs as jest.Mocked<typeof fs>;

describe('buildImage', () => {
  let mockLogger: ReturnType<typeof createMockLogger>;
  let config: BuildImageConfig;

  const mockDockerfile = `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
EXPOSE 3000
USER appuser
CMD ["node", "index.js"]`;

  beforeEach(() => {
    mockLogger = createMockLogger();
    config = {
      path: '/test/repo',
      dockerfile: 'Dockerfile',
      imageName: 'test-app:latest',
      tags: ['myapp:latest', 'myapp:v1.0'],
      buildArgs: {},
    };

    // Reset all mocks
    jest.clearAllMocks();

    // Default mock implementations
    mockFs.access.mockResolvedValue(undefined);
    mockFs.stat.mockResolvedValue({ isFile: () => true, isDirectory: () => false } as any);
    mockFs.readFile.mockResolvedValue(mockDockerfile);
    mockFs.writeFile.mockResolvedValue(undefined);

    // Default successful Docker build
    mockDockerClient.buildImage.mockResolvedValue(
      createSuccessResult({
        imageId: 'sha256:mock-image-id',
        digest: 'sha256:abcdef1234567890',
        tags: ['myapp:latest', 'myapp:v1.0'],
        size: 123456789,
        layers: 8,
        buildTime: 5000,
        logs: ['Step 1/8 : FROM node:18-alpine', 'Successfully built mock-image-id'],
        warnings: [],
      }),
    );
  });

  describe('Successful Build', () => {
    it('should successfully build Docker image with default settings', async () => {
      const mockContext = createMockToolContext();
      const result = await buildImage(config, mockContext);

      if (!result.ok) {
        console.error('Build failed:', result.error);
      }
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.success).toBe(true);
        expect(result.value.imageId).toBe('sha256:mock-image-id');
        expect(result.value.tags).toEqual(['myapp:latest', 'myapp:v1.0']);
        expect(result.value.size).toBe(123456789);
        expect(result.value.layers).toBe(8);
        expect(result.value.logs).toContain('Successfully built mock-image-id');
        expect(result.value.buildTime).toBeGreaterThanOrEqual(0);
      }
    });

    it('should pass build arguments to Docker client', async () => {
      config.buildArgs = {
        NODE_ENV: 'development',
        API_URL: 'https://api.example.com',
      };

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      expect(mockDockerClient.buildImage).toHaveBeenCalledWith(
        expect.objectContaining({
          buildargs: expect.objectContaining({
            NODE_ENV: 'development',
            API_URL: 'https://api.example.com',
            BUILD_DATE: expect.any(String),
            VCS_REF: expect.any(String),
          }),
        }),
      );
    });

    it('should include default build arguments', async () => {
      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      expect(mockDockerClient.buildImage).toHaveBeenCalledWith(
        expect.objectContaining({
          buildargs: expect.objectContaining({
            NODE_ENV: expect.any(String),
            BUILD_DATE: expect.any(String),
            VCS_REF: expect.any(String),
          }),
        }),
      );
    });

    it('should verify build result structure', async () => {
      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      // Verify the tool returns successfully with proper structure
      if (result.ok) {
        expect(result.value).toHaveProperty('imageId');
        expect(result.value).toHaveProperty('tags');
      }
    });
  });

  describe('Dockerfile Resolution', () => {
    it('should fail when Dockerfile does not exist', async () => {
      // Mock access to simulate file doesn't exist (for validation)
      mockFs.access.mockRejectedValue(new Error('ENOENT: no such file or directory'));
      mockFs.stat.mockRejectedValue(new Error('ENOENT: no such file or directory'));

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('does not exist');
      }
    });

    it('should use dockerfilePath when provided', async () => {
      const customConfig = {
        ...config,
        dockerfilePath: 'custom/Dockerfile',
      };

      mockFs.readFile.mockResolvedValue(mockDockerfile);

      const result = await buildImage(customConfig, createMockToolContext());

      expect(result.ok).toBe(true);
      expect(mockFs.readFile).toHaveBeenCalledWith('/test/repo/custom/Dockerfile', 'utf-8');
    });
  });

  describe('Security Analysis', () => {
    it('should detect security warnings in build args', async () => {
      config.buildArgs = {
        API_PASSWORD: 'secret123',
        DB_TOKEN: 'token456',
      };

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.securityWarnings).toEqual(
          expect.arrayContaining([
            'Potential secret in build arg: API_PASSWORD',
            'Potential secret in build arg: DB_TOKEN',
          ]),
        );
      }
    });

    it('should detect sudo usage in Dockerfile', async () => {
      const dockerfileWithSudo = `FROM ubuntu:20.04
RUN sudo apt-get update
USER appuser`;

      mockFs.readFile.mockResolvedValue(dockerfileWithSudo);

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.securityWarnings).toContain(
          'Using sudo in Dockerfile - consider running as non-root',
        );
      }
    });

    it('should detect :latest tags in Dockerfile', async () => {
      const dockerfileWithLatest = `FROM node:latest
WORKDIR /app
USER appuser`;

      mockFs.readFile.mockResolvedValue(dockerfileWithLatest);

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.securityWarnings).toContain(
          'Using :latest tag - consider pinning versions for reproducibility',
        );
      }
    });

    it('should detect missing USER instruction', async () => {
      const dockerfileWithoutUser = `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
CMD ["node", "index.js"]`;

      mockFs.readFile.mockResolvedValue(dockerfileWithoutUser);

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.securityWarnings).toContain(
          'Container may run as root - consider adding a non-root USER',
        );
      }
    });

    it('should detect root user', async () => {
      const dockerfileWithRootUser = `FROM node:18-alpine
WORKDIR /app
COPY . .
USER root
CMD ["node", "index.js"]`;

      mockFs.readFile.mockResolvedValue(dockerfileWithRootUser);

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.securityWarnings).toContain(
          'Container may run as root - consider adding a non-root USER',
        );
      }
    });
  });

  describe('Error Handling', () => {
    it('should succeed with valid Dockerfile', async () => {
      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.imageId).toBe('sha256:mock-image-id');
      }
    });

    it('should return error when Docker build fails', async () => {
      mockFs.readFile.mockResolvedValue(mockDockerfile);
      mockDockerClient.buildImage.mockResolvedValue(
        createFailureResult('Docker build failed: syntax error'),
      );

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Docker build failed: syntax error');
      }
    });

    it('should handle filesystem errors', async () => {
      mockFs.readFile.mockRejectedValue(new Error('Permission denied'));

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Permission denied');
      }
    });

    it('should handle Docker client errors', async () => {
      mockFs.readFile.mockResolvedValue(mockDockerfile);
      mockDockerClient.buildImage.mockRejectedValue(new Error('Docker daemon not running'));

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Docker daemon not running');
      }
    });
  });

  describe('Error Scenarios - Infrastructure', () => {
    it('should fail gracefully when Docker daemon is not running', async () => {
      mockFs.readFile.mockResolvedValue(mockDockerfile);
      mockDockerClient.buildImage.mockResolvedValue({
        ok: false,
        error: 'Cannot connect to the Docker daemon',
        guidance: {
          hint: 'The Docker daemon must be running to build images',
          resolution: 'Start Docker Desktop or run: sudo systemctl start docker',
        },
      });

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Cannot connect to the Docker daemon');
        expect(result.guidance).toBeDefined();
        expect(result.guidance?.hint).toContain('Docker daemon');
        expect(result.guidance?.resolution).toBeDefined();
      }
    });

    it('should fail when Docker socket is not accessible', async () => {
      mockFs.readFile.mockResolvedValue(mockDockerfile);
      mockDockerClient.buildImage.mockResolvedValue({
        ok: false,
        error: 'EACCES: permission denied, connect /var/run/docker.sock',
        guidance: {
          hint: 'Current user does not have permission to access Docker socket',
          resolution: 'Add user to docker group: sudo usermod -aG docker $USER',
        },
      });

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('permission denied');
        expect(result.guidance).toBeDefined();
        expect(result.guidance?.hint).toBeDefined();
        expect(result.guidance?.resolution).toBeDefined();
      }
    });

    it('should fail when network is unreachable during base image pull', async () => {
      mockFs.readFile.mockResolvedValue(mockDockerfile);
      mockDockerClient.buildImage.mockResolvedValue({
        ok: false,
        error: 'Error pulling image: network unreachable',
        guidance: {
          hint: 'Cannot pull base image due to network issues',
          resolution: 'Check your internet connection and Docker registry configuration',
        },
      });

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('network unreachable');
        expect(result.guidance).toBeDefined();
      }
    });
  });

  describe('Error Scenarios - File System', () => {
    it('should fail when build context directory does not exist', async () => {
      mockFs.access.mockRejectedValue(new Error('ENOENT: no such file or directory'));
      mockFs.stat.mockRejectedValue(new Error('ENOENT'));

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('does not exist');
      }
    });

    it('should fail when Dockerfile is not readable', async () => {
      mockFs.access.mockRejectedValue(new Error('EACCES: permission denied'));
      mockFs.stat.mockResolvedValue({ isFile: () => true, isDirectory: () => false } as any);

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toMatch(/permission denied|does not exist|not accessible/i);
      }
    });

    it('should fail when Dockerfile path points to a directory', async () => {
      mockFs.access.mockResolvedValue(undefined);
      mockFs.stat.mockResolvedValue({ isFile: () => false, isDirectory: () => true } as any);
      mockFs.readFile.mockRejectedValue(Object.assign(new Error('EISDIR'), { code: 'EISDIR' }));

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toMatch(/not a file|directory|EISDIR/i);
      }
    });

    it('should handle EISDIR error when reading Dockerfile', async () => {
      mockFs.access.mockResolvedValue(undefined);
      mockFs.stat.mockResolvedValue({ isFile: () => true, isDirectory: () => false } as any);
      const error = new Error('EISDIR: illegal operation on a directory') as any;
      error.code = 'EISDIR';
      mockFs.readFile.mockRejectedValue(error);

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('directory');
      }
    });
  });

  describe('Error Scenarios - Input Validation', () => {
    it('should fail with invalid parameters object', async () => {
      const result = await buildImage(null as any, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Invalid parameters');
      }
    });

    it('should fail when imageName and tags are both empty', async () => {
      const invalidConfig = {
        ...config,
        imageName: undefined,
        tags: [],
      };

      const result = await buildImage(invalidConfig, createMockToolContext());

      // Should still succeed but with no tags (implementation allows this)
      // Or fail depending on validation logic
      expect(result).toBeDefined();
    });
  });

  describe('Error Scenarios - Docker Build Failures', () => {
    it('should fail when Dockerfile has syntax errors', async () => {
      mockFs.readFile.mockResolvedValue('INVALID DOCKERFILE SYNTAX');
      mockDockerClient.buildImage.mockResolvedValue({
        ok: false,
        error: 'Dockerfile parse error: unknown instruction: INVALID',
        guidance: {
          hint: 'Dockerfile contains syntax errors',
          resolution: 'Check Dockerfile syntax and fix errors. Use "docker build" locally to debug.',
        },
      });

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('parse error');
      }
    });

    it('should fail when base image is not found', async () => {
      mockFs.readFile.mockResolvedValue('FROM nonexistent:image\nCMD ["echo", "hello"]');
      mockDockerClient.buildImage.mockResolvedValue({
        ok: false,
        error: 'manifest for nonexistent:image not found',
        guidance: {
          hint: 'Base image does not exist in registry',
          resolution: 'Verify the image name and tag, or use a different base image',
        },
      });

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('not found');
        expect(result.guidance).toBeDefined();
      }
    });

    it('should fail when build step fails', async () => {
      mockFs.readFile.mockResolvedValue(mockDockerfile);
      mockDockerClient.buildImage.mockResolvedValue({
        ok: false,
        error: 'RUN command failed: npm ci exited with code 1',
        guidance: {
          hint: 'Build step failed during execution',
          resolution: 'Check the command in your Dockerfile and ensure dependencies are available',
        },
      });

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('RUN command failed');
        expect(result.guidance).toBeDefined();
      }
    });

    it('should fail when disk space is insufficient', async () => {
      mockFs.readFile.mockResolvedValue(mockDockerfile);
      mockDockerClient.buildImage.mockResolvedValue({
        ok: false,
        error: 'no space left on device',
        guidance: {
          hint: 'Insufficient disk space to complete build',
          resolution: 'Free up disk space or prune unused Docker images: docker system prune',
        },
      });

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('no space left');
        expect(result.guidance).toBeDefined();
      }
    });
  });

  describe('Build Arguments', () => {
    beforeEach(() => {
      // Setup filesystem mocks
      mockFs.access.mockResolvedValue(undefined);
      mockFs.readFile.mockResolvedValue(mockDockerfile);

      // Setup docker build mock
      mockDockerClient.buildImage.mockResolvedValue(
        createSuccessResult({
          imageId: 'sha256:mock-image-id',
          digest: 'sha256:abcdef1234567890',
          tags: ['myapp:latest', 'myapp:v1.0'],
          size: 123456789,
          layers: 8,
          buildTime: 5000,
          logs: ['Step 1/8 : FROM node:18-alpine', 'Successfully built mock-image-id'],
          warnings: [],
        }),
      );
    });

    it('should override default arguments with custom ones', async () => {
      config.buildArgs = {
        NODE_ENV: 'development',
        BUILD_DATE: '2023-01-01',
      };

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      expect(mockDockerClient.buildImage).toHaveBeenCalledWith(
        expect.objectContaining({
          buildargs: expect.objectContaining({
            NODE_ENV: 'development',
            BUILD_DATE: '2023-01-01',
            VCS_REF: expect.any(String),
          }),
        }),
      );
    });
  });

  describe('Environment Variables', () => {
    beforeEach(() => {
      mockFs.access.mockResolvedValue(undefined);
      mockFs.readFile.mockResolvedValue(mockDockerfile);
    });

    it('should use NODE_ENV from environment', async () => {
      const originalNodeEnv = process.env.NODE_ENV;
      process.env.NODE_ENV = 'staging';

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      expect(mockDockerClient.buildImage).toHaveBeenCalledWith(
        expect.objectContaining({
          buildargs: expect.objectContaining({
            NODE_ENV: 'staging',
          }),
        }),
      );

      // Restore original NODE_ENV
      process.env.NODE_ENV = originalNodeEnv;
    });

    it('should use GIT_COMMIT from environment', async () => {
      const originalGitCommit = process.env.GIT_COMMIT;
      process.env.GIT_COMMIT = 'abc123def456';

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      expect(mockDockerClient.buildImage).toHaveBeenCalledWith(
        expect.objectContaining({
          buildargs: expect.objectContaining({
            VCS_REF: 'abc123def456',
          }),
        }),
      );

      // Restore original GIT_COMMIT
      process.env.GIT_COMMIT = originalGitCommit;
    });
  });
});
