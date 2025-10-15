/**
 * Unit Tests: Build Image Tool
 * Tests the build-image tool functionality with mock Docker client and filesystem
 */

import { jest } from '@jest/globals';
import { promises as fs } from 'node:fs';

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
const mockSessionManager = {
  create: jest.fn(),
  get: jest.fn(),
  update: jest.fn(),
};

const mockDockerClient = {
  buildImage: jest.fn(),
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

// Mock the session helpers
const mockSessionFacade = {
  id: 'test-session-123',
  get: jest.fn(),
  set: jest.fn(),
  pushStep: jest.fn(),
};

function createMockToolContext() {
  return {
    logger: createMockLogger(),
    sessionManager: mockSessionManager,
    session: mockSessionFacade,
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

    mockSessionManager.get.mockResolvedValue({
      ok: true,
      value: {
        completed_steps: [],
        createdAt: new Date('2025-09-08T11:12:40.362Z'),
        updatedAt: new Date('2025-09-08T11:12:40.362Z'),
      },
    });

    // Default mock implementations
    mockFs.access.mockResolvedValue(undefined);
    mockFs.stat.mockResolvedValue({ isFile: () => true } as any);
    mockFs.readFile.mockResolvedValue(mockDockerfile);
    mockFs.writeFile.mockResolvedValue(undefined);
    mockSessionManager.update.mockResolvedValue({
      ok: true,
      value: {
        completed_steps: ['build-image'],
        createdAt: new Date('2025-09-08T11:12:40.362Z'),
        updatedAt: new Date(),
      },
    });

    // Default successful Docker build
    mockDockerClient.buildImage.mockResolvedValue(
      createSuccessResult({
        imageId: 'sha256:mock-image-id',
        tags: ['myapp:latest', 'myapp:v1.0'],
        size: 123456789,
        layers: 8,
        logs: ['Step 1/8 : FROM node:18-alpine', 'Successfully built mock-image-id'],
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

    it('should update session with build result', async () => {
      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      // The orchestrator automatically stores results via sessionFacade.storeResult()
      // Tools no longer manually manipulate session.set('results')
      // This test verifies the tool returns successfully
      if (result.ok) {
        expect(result.value).toHaveProperty('imageId');
        expect(result.value).toHaveProperty('tags');
      }
    });
  });

  describe('Dockerfile Resolution', () => {
    it('should fail when Dockerfile does not exist', async () => {
      mockFs.stat.mockRejectedValue(new Error('Dockerfile not found'));

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Dockerfile not found');
      }
    });

    it('should use dockerfilePath when provided', async () => {
      const customConfig = {
        ...config,
        dockerfilePath: '/test/repo/custom/Dockerfile',
      };

      mockFs.stat.mockResolvedValue({ isFile: () => true } as any);
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
        expect(result.error).toBe('Permission denied');
      }
    });

    it('should handle Docker client errors', async () => {
      mockDockerClient.buildImage.mockRejectedValue(new Error('Docker daemon not running'));

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Docker daemon not running');
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
          logs: ['Step 1/8 : FROM node:18-alpine', 'Successfully built mock-image-id'],
          layers: 8,
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
