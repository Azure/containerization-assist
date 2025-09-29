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
  create: jest.fn().mockResolvedValue({
    "sessionId": "test-session-123",
    "workflow_state": {},
    "metadata": {},
    "completed_steps": [],
    "errors": {},
    "current_step": null,
    "createdAt": "2025-09-08T11:12:40.362Z",
    "updatedAt": "2025-09-08T11:12:40.362Z"
  }),
  get: jest.fn(),
  update: jest.fn(),
};

const mockDockerClient = {
  buildImage: jest.fn(),
};

jest.mock('@/session/core', () => ({
  createSessionManager: jest.fn(() => mockSessionManager),
}));

jest.mock('../../../src/lib/docker', () => ({
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
      sessionId: 'test-session-123',
      path: '.',
      dockerfile: 'Dockerfile',
      imageName: 'test-app:latest',
      tags: ['myapp:latest', 'myapp:v1.0'],
      buildArgs: {},
      noCache: false,
    };

    // Reset all mocks
    jest.clearAllMocks();
    
    // Setup session mock
    mockSessionManager.get.mockResolvedValue({
      sessionId: 'test-session-123',
      results: {
        'analyze_repo': {
          language: 'javascript',
          framework: 'express',
        },
        'generate-dockerfile': {
          path: '/test/repo/Dockerfile',
          content: mockDockerfile,
        },
      },
      completed_steps: [],
      errors: {},
      current_step: null,
      metadata: {},
      createdAt: '2025-09-08T11:12:40.362Z',
      updatedAt: '2025-09-08T11:12:40.362Z',
    });

    // Default mock implementations
    mockFs.access.mockResolvedValue(undefined);
    mockFs.readFile.mockResolvedValue(mockDockerfile);
    mockFs.writeFile.mockResolvedValue(undefined);
    mockSessionManager.update.mockResolvedValue(true);

    // Reset session facade mock
    jest.clearAllMocks();

    // Default successful Docker build
    mockDockerClient.buildImage.mockResolvedValue(createSuccessResult({
      imageId: 'sha256:mock-image-id',
      tags: ['myapp:latest', 'myapp:v1.0'],
      size: 123456789,
      layers: 8,
      logs: ['Step 1/8 : FROM node:18-alpine', 'Successfully built mock-image-id'],
    }));
  });

  describe('Successful Build', () => {
    beforeEach(() => {
      // Setup session facade with build context
      mockSessionFacade.get.mockImplementation((key: string) => {
        if (key === 'results') {
          return {
            'analyze-repo': {
              language: 'javascript',
              framework: 'express',
            },
            'generate-dockerfile': {
              path: '/test/repo/Dockerfile',
              content: mockDockerfile,
            },
          };
        }
        return undefined;
      });
    });

    it('should successfully build Docker image with default settings', async () => {
      const mockContext = createMockToolContext();
      const result = await buildImage(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.success).toBe(true);
        expect(result.value.sessionId).toBe('test-session-123');
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
            LANGUAGE: 'javascript',
            FRAMEWORK: 'express',
          }),
        })
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
            LANGUAGE: 'javascript',
            FRAMEWORK: 'express',
          }),
        })
      );
    });


    it('should update session with build result', async () => {
      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      // The build-image tool updates session using session facade
      // It calls set multiple times: once for results, once for current_step
      expect(mockSessionFacade.set).toHaveBeenCalledWith('results', expect.objectContaining({
        'build-image': expect.any(Object),
      }));
      expect(mockSessionFacade.set).toHaveBeenCalledWith('current_step', 'build-image');
    });
  });

  describe('Dockerfile Resolution', () => {
    it('should use generated Dockerfile when original not found', async () => {
      mockSessionManager.get.mockResolvedValue({
        workflow_state: {},
        results: {
          'analyze_repo': { language: 'javascript' },
          'generate-dockerfile': {
            path: '/test/repo/Dockerfile',
            content: mockDockerfile,
          },
        },
        repo_path: '/test/repo',
      });

      // Mock original Dockerfile not found, but generated one exists
      mockFs.access
        .mockRejectedValueOnce(new Error('Original Dockerfile not found'))
        .mockResolvedValueOnce(undefined); // Generated Dockerfile exists

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      expect(mockDockerClient.buildImage).toHaveBeenCalledWith(
        expect.objectContaining({
          context: '.',
          dockerfile: 'Dockerfile',
        })
      );
    });

    it('should create Dockerfile from session content when none exists', async () => {
      mockSessionManager.get.mockResolvedValue({
        workflow_state: {},
        results: {
          'analyze_repo': { language: 'javascript' },
          'generate-dockerfile': {
            content: mockDockerfile,
          },
        },
        repo_path: '/test/repo',
      });

      // Mock both original and generated Dockerfiles not found
      mockFs.access.mockRejectedValue(new Error('Dockerfile not found'));

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      expect(mockFs.writeFile).toHaveBeenCalledWith(
        'Dockerfile',
        mockDockerfile,
        'utf-8'
      );
      expect(mockDockerClient.buildImage).toHaveBeenCalledWith(
        expect.objectContaining({
          context: '.',
          dockerfile: 'Dockerfile',
        })
      );
    });
  });

  describe('Security Analysis', () => {
    it('should detect security warnings in build args', async () => {
      mockSessionManager.get.mockResolvedValue({
        workflow_state: {},
        results: {
          'analyze_repo': { language: 'javascript' },
          'generate-dockerfile': {
            path: '/test/repo/Dockerfile',
            content: mockDockerfile,
          },
        },
        repo_path: '/test/repo',
      });

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
          ])
        );
      }
    });

    it('should detect sudo usage in Dockerfile', async () => {
      const dockerfileWithSudo = `FROM ubuntu:20.04
RUN sudo apt-get update
USER appuser`;

      mockSessionManager.get.mockResolvedValue({
        workflow_state: {},
        results: {
          'analyze_repo': { language: 'javascript' },
          'generate-dockerfile': {
            path: '/test/repo/Dockerfile',
            content: dockerfileWithSudo,
          },
        },
        repo_path: '/test/repo',
      });

      mockFs.readFile.mockResolvedValue(dockerfileWithSudo);

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.securityWarnings).toContain(
          'Using sudo in Dockerfile - consider running as non-root'
        );
      }
    });

    it('should detect :latest tags in Dockerfile', async () => {
      const dockerfileWithLatest = `FROM node:latest
WORKDIR /app
USER appuser`;

      mockSessionManager.get.mockResolvedValue({
        workflow_state: {},
        results: {
          'analyze_repo': { language: 'javascript' },
          'generate-dockerfile': {
            path: '/test/repo/Dockerfile',
            content: dockerfileWithLatest,
          },
        },
        repo_path: '/test/repo',
      });

      mockFs.readFile.mockResolvedValue(dockerfileWithLatest);

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.securityWarnings).toContain(
          'Using :latest tag - consider pinning versions for reproducibility'
        );
      }
    });

    it('should detect missing USER instruction', async () => {
      const dockerfileWithoutUser = `FROM node:18-alpine
WORKDIR /app
COPY . .
CMD ["node", "index.js"]`;

      mockSessionManager.get.mockResolvedValue({
        workflow_state: {},
        results: {
          'analyze_repo': { language: 'javascript' },
          'generate-dockerfile': {
            path: '/test/repo/Dockerfile',
            content: dockerfileWithoutUser,
          },
        },
        repo_path: '/test/repo',
      });

      mockFs.readFile.mockResolvedValue(dockerfileWithoutUser);

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.securityWarnings).toContain(
          'Container may run as root - consider adding a non-root USER'
        );
      }
    });

    it('should detect root user', async () => {
      const dockerfileWithRootUser = `FROM node:18-alpine
WORKDIR /app
COPY . .
USER root
CMD ["node", "index.js"]`;

      mockSessionManager.get.mockResolvedValue({
        workflow_state: {},
        results: {
          'analyze_repo': { language: 'javascript' },
          'generate-dockerfile': {
            path: '/test/repo/Dockerfile',
            content: dockerfileWithRootUser,
          },
        },
        repo_path: '/test/repo',
      });

      mockFs.readFile.mockResolvedValue(dockerfileWithRootUser);

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.securityWarnings).toContain(
          'Container may run as root - consider adding a non-root USER'
        );
      }
    });
  });

  describe('Error Handling', () => {
    it('should auto-create session when not found', async () => {
      // Setup session facade with dockerfile content
      mockSessionFacade.get.mockImplementation((key: string) => {
        if (key === 'results') {
          return {
            'generate-dockerfile': {
              path: '/test/repo/Dockerfile',
              content: mockDockerfile,
            },
          };
        }
        return undefined;
      });
      
      // Setup filesystem mocks for this test
      mockFs.access.mockResolvedValue(undefined);
      mockFs.readFile.mockResolvedValue(mockDockerfile);
      
      // Setup docker build mock
      mockDockerClient.buildImage.mockResolvedValue(createSuccessResult({
        imageId: 'sha256:mock-image-id',
        logs: [
          'Step 1/8 : FROM node:18-alpine',
          'Successfully built mock-image-id',
        ],
        layers: 8,
      }));

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      // Session facade should have been called to get results
      expect(mockSessionFacade.get).toHaveBeenCalledWith('results');
    });

    it('should return error when Dockerfile not found and no session content', async () => {
      // Setup session facade with empty dockerfile content
      mockSessionFacade.get.mockImplementation((key: string) => {
        if (key === 'results') {
          return {
            'generate-dockerfile': {}, // Empty dockerfile content
          };
        }
        return undefined;
      });

      mockFs.access.mockRejectedValue(new Error('Dockerfile not found'));

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Dockerfile not found');
      }
    });

    it('should return error when Docker build fails', async () => {
      // Setup session facade with dockerfile content
      mockSessionFacade.get.mockImplementation((key: string) => {
        if (key === 'results') {
          return {
            'analyze-repo': { language: 'javascript' },
            'generate-dockerfile': {
              path: '/test/repo/Dockerfile',
              content: mockDockerfile,
            },
          };
        }
        return undefined;
      });

      mockFs.access.mockResolvedValue(undefined);
      mockFs.readFile.mockResolvedValue(mockDockerfile);

      mockDockerClient.buildImage.mockResolvedValue(
        createFailureResult('Docker build failed: syntax error')
      );

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Docker build failed: syntax error');
      }
    });

    it('should handle filesystem errors', async () => {
      // Setup session facade with dockerfile content
      mockSessionFacade.get.mockImplementation((key: string) => {
        if (key === 'results') {
          return {
            'generate-dockerfile': {
              path: '/test/repo/Dockerfile',
              content: mockDockerfile,
            },
          };
        }
        return undefined;
      });

      mockFs.access.mockResolvedValue(undefined);
      mockFs.readFile.mockRejectedValue(new Error('Permission denied'));

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Permission denied');
      }
    });

    it('should handle Docker client errors', async () => {
      // Setup session facade with dockerfile content
      mockSessionFacade.get.mockImplementation((key: string) => {
        if (key === 'results') {
          return {
            'generate-dockerfile': {
              path: '/test/repo/Dockerfile',
              content: mockDockerfile,
            },
          };
        }
        return undefined;
      });
      
      mockFs.access.mockResolvedValue(undefined);
      mockFs.readFile.mockResolvedValue(mockDockerfile);

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
      // Setup session facade with python/flask analysis
      mockSessionFacade.get.mockImplementation((key: string) => {
        if (key === 'results') {
          return {
            'analyze-repo': {
              language: 'python',
              framework: 'flask',
            },
            'generate-dockerfile': {
              path: '/test/repo/Dockerfile',
              content: mockDockerfile,
            },
          };
        }
        return undefined;
      });

      // Setup filesystem mocks
      mockFs.access.mockResolvedValue(undefined);
      mockFs.readFile.mockResolvedValue(mockDockerfile);

      // Setup docker build mock
      mockDockerClient.buildImage.mockResolvedValue(createSuccessResult({
        imageId: 'sha256:mock-image-id',
        logs: [
          'Step 1/8 : FROM node:18-alpine',
          'Successfully built mock-image-id',
        ],
        layers: 8,
      }));
    });

    it('should include language and framework from analysis', async () => {
      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      expect(mockDockerClient.buildImage).toHaveBeenCalledWith(
        expect.objectContaining({
          buildargs: expect.objectContaining({
            LANGUAGE: 'python',
            FRAMEWORK: 'flask',
          }),
        })
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
        })
      );
    });

    it('should handle missing analysis data gracefully', async () => {
      // Setup session facade with only dockerfile content (no analysis)
      mockSessionFacade.get.mockImplementation((key: string) => {
        if (key === 'results') {
          return {
            'generate-dockerfile': {
              path: '/test/repo/Dockerfile',
              content: mockDockerfile,
            },
          };
        }
        return undefined;
      });

      mockSessionManager.get.mockResolvedValue({
        workflow_state: {},
        results: {
          'generate-dockerfile': {
            path: '/test/repo/Dockerfile',
            content: mockDockerfile,
          },
        },
        repo_path: '/test/repo',
      });

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      expect(mockDockerClient.buildImage).toHaveBeenCalledWith(
        expect.objectContaining({
          buildargs: expect.objectContaining({
            NODE_ENV: expect.any(String),
            BUILD_DATE: expect.any(String),
            VCS_REF: expect.any(String),
            // Should not include LANGUAGE or FRAMEWORK
          }),
        })
      );
      expect(mockDockerClient.buildImage).toHaveBeenCalledWith(
        expect.objectContaining({
          buildargs: expect.not.objectContaining({
            LANGUAGE: expect.any(String),
            FRAMEWORK: expect.any(String),
          }),
        })
      );
    });
  });

  describe('Environment Variables', () => {
    it('should use NODE_ENV from environment', async () => {
      const originalNodeEnv = process.env.NODE_ENV;
      process.env.NODE_ENV = 'staging';

      mockSessionManager.get.mockResolvedValue({
        workflow_state: {},
        results: {
          'analyze_repo': { language: 'javascript' },
          'generate-dockerfile': {
            path: '/test/repo/Dockerfile',
            content: mockDockerfile,
          },
        },
        repo_path: '/test/repo',
      });

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      expect(mockDockerClient.buildImage).toHaveBeenCalledWith(
        expect.objectContaining({
          buildargs: expect.objectContaining({
            NODE_ENV: 'staging',
          }),
        })
      );

      // Restore original NODE_ENV
      process.env.NODE_ENV = originalNodeEnv;
    });

    it('should use GIT_COMMIT from environment', async () => {
      const originalGitCommit = process.env.GIT_COMMIT;
      process.env.GIT_COMMIT = 'abc123def456';

      mockSessionManager.get.mockResolvedValue({
        workflow_state: {},
        results: {
          'analyze_repo': { language: 'javascript' },
          'generate-dockerfile': {
            path: '/test/repo/Dockerfile',
            content: mockDockerfile,
          },
        },
        repo_path: '/test/repo',
      });

      const result = await buildImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      expect(mockDockerClient.buildImage).toHaveBeenCalledWith(
        expect.objectContaining({
          buildargs: expect.objectContaining({
            VCS_REF: 'abc123def456',
          }),
        })
      );

      // Restore original GIT_COMMIT
      process.env.GIT_COMMIT = originalGitCommit;
    });
  });
});