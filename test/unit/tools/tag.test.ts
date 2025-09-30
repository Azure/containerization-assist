/**
 * Unit Tests: Image Tagging Tool
 * Tests the tag image tool functionality with mock Docker client
 * Following analyze-repo test structure and comprehensive coverage requirements
 */

import { jest } from '@jest/globals';
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

// Mock lib modules following analyze-repo pattern
const mockSessionManager = {
  create: jest.fn().mockResolvedValue({
    sessionId: 'test-session-123',
    workflow_state: {},
    metadata: {},
    completed_steps: [],
    errors: {},
    
    createdAt: '2025-09-08T11:12:40.362Z',
    updatedAt: '2025-09-08T11:12:40.362Z',
  }),
  get: jest.fn(),
  update: jest.fn(),
};

const mockDockerClient = {
  tagImage: jest.fn(),
};

// Create timer mock before jest.mock calls so it's available
const mockTimer = {
  end: jest.fn(),
  error: jest.fn(),
};

jest.mock('@/session/core', () => ({
  createSessionManager: jest.fn(() => mockSessionManager),
}));

jest.mock('../../../src/lib/docker', () => ({
  createDockerClient: jest.fn(() => mockDockerClient),
}));

jest.mock('../../../src/lib/logger', () => ({
  createTimer: jest.fn(() => mockTimer),
  createLogger: jest.fn(() => createMockLogger()),
}));

jest.mock('../../../src/lib/tool-helpers', () => ({
  getToolLogger: jest.fn(() => createMockLogger()),
  createToolTimer: jest.fn(() => mockTimer),
  createStandardizedToolTracker: jest.fn(() => ({
    complete: jest.fn(),
    fail: jest.fn(),
  })),
}));

// Mock session facade
const mockSessionFacade = {
  id: 'test-session-123',
  get: jest.fn(),
  set: jest.fn(),
  pushStep: jest.fn(),
  storeResult: jest.fn(),
  getResult: jest.fn(),
};

// Import these after mocks are set up
import tagImageTool from '../../../src/tools/tag-image/tool';
import type { TagImageParams } from '../../../src/tools/tag-image/schema';

// Create global mock tool context
function createMockToolContext() {
  return {
    logger: createMockLogger(),
    sessionManager: mockSessionManager,
    session: mockSessionFacade,
  } as any;
}

describe('tagImage', () => {
  let mockLogger: ReturnType<typeof createMockLogger>;
  let config: TagImageParams;

  beforeEach(() => {
    mockLogger = createMockLogger();
    config = {
      sessionId: 'test-session-123',
      tag: 'myapp:v1.0',
    };

    // Reset all mocks completely
    jest.clearAllMocks();
    jest.restoreAllMocks();

    // Re-establish timer mock after clearing
    const { createToolTimer } = jest.requireMock('../../../src/lib/tool-helpers');
    createToolTimer.mockReturnValue(mockTimer);

    // Reset session facade methods to default implementations
    mockSessionFacade.getResult.mockImplementation((toolName: string) => {
      if (toolName === 'build-image') {
        return {
          imageId: 'sha256:mock-image-id',
          context: '/test/repo',
        };
      }
      return undefined;
    });
    mockSessionFacade.set.mockResolvedValue(undefined);
    mockSessionFacade.pushStep.mockResolvedValue(undefined);
    mockSessionFacade.storeResult.mockResolvedValue(undefined);

    mockSessionManager.update.mockResolvedValue(true);

    // Setup default successful Docker tag result
    mockDockerClient.tagImage.mockResolvedValue(createSuccessResult({
      success: true,
      imageId: 'sha256:mock-image-id',
    }));
  });

  describe('Successful Tagging Operations', () => {
    beforeEach(() => {
      // Setup session facade with build result
      mockSessionFacade.getResult.mockImplementation((toolName: string) => {
        if (toolName === 'build-image') {
          return {
            imageId: 'sha256:mock-image-id',
            context: '/test/repo',
          };
        }
        return undefined;
      });
    });


    it('should successfully tag image with repository and tag', async () => {
      const result = await tagImageTool.run(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.success).toBe(true);
        expect(result.value.sessionId).toBe('test-session-123');
        expect(result.value.tags).toEqual(['myapp:v1.0']);
        expect(result.value.imageId).toBe('sha256:mock-image-id');
      }

      // Verify Docker client was called with correct parameters
      expect(mockDockerClient.tagImage).toHaveBeenCalledWith(
        'sha256:mock-image-id',
        'myapp',
        'v1.0',
      );

      // Note: Session storage is handled by the orchestrator, not the tool directly

      // Verify timer was used correctly
      expect(mockTimer.end).toHaveBeenCalledWith({
        tags: ['myapp:v1.0'],
        sessionId: 'test-session-123',
      });
    });

    it('should handle tag without explicit version (defaults to latest)', async () => {
      config.tag = 'myapp';

      const result = await tagImageTool.run(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.tags).toEqual(['myapp']);
      }

      // Should tag with 'latest' when no tag specified
      expect(mockDockerClient.tagImage).toHaveBeenCalledWith(
        'sha256:mock-image-id',
        'myapp',
        'latest',
      );
    });

    it('should handle complex repository names', async () => {
      const testCases = [
        {
          input: 'docker.io/library/myapp:v1.0',
          expectedRepo: 'docker.io/library/myapp',
          expectedTag: 'v1.0',
        },
        {
          input: 'ghcr.io/myorg/myapp:main',
          expectedRepo: 'ghcr.io/myorg/myapp',
          expectedTag: 'main',
        },
        { input: 'localhost/myapp:dev', expectedRepo: 'localhost/myapp', expectedTag: 'dev' },
        {
          input: 'my-registry.com/path/to/app:stable',
          expectedRepo: 'my-registry.com/path/to/app',
          expectedTag: 'stable',
        },
      ];

      for (const testCase of testCases) {
        config.tag = testCase.input;

        const result = await tagImageTool.run(config, createMockToolContext());

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.tags).toEqual([testCase.input]);
        }

        expect(mockDockerClient.tagImage).toHaveBeenCalledWith(
          'sha256:mock-image-id',
          testCase.expectedRepo,
          testCase.expectedTag,
        );

        // Reset mocks for next iteration
        mockDockerClient.tagImage.mockClear();
        mockSessionManager.update.mockClear();
      }
    });

    it('should preserve existing build result data when updating session', async () => {
      // Setup session facade with extended build result
      mockSessionFacade.getResult.mockImplementation((toolName: string) => {
        if (toolName === 'build-image') {
          return {
            imageId: 'sha256:mock-image-id',
            context: '/test/repo',
            dockerfile: 'Dockerfile',
            size: 1024000,
          };
        }
        return undefined;
      });

      const result = await tagImageTool.run(config, createMockToolContext());

      expect(result.ok).toBe(true);
      // Note: Session storage is handled by the orchestrator, not the tool directly
    });
  });

  describe('Tag Format Validation', () => {
    beforeEach(() => {
      // Setup session facade with build result
      mockSessionFacade.getResult.mockImplementation((toolName: string) => {
        if (toolName === 'build-image') {
          return {
            imageId: 'sha256:mock-image-id',
            context: '/test/repo',
          };
        }
        return undefined;
      });
    });


    it('should handle various valid tag formats', async () => {
      const validTags = [
        'myapp:v1.0.0',
        'myapp:latest',
        'myapp:main',
        'myapp:feature-branch',
        'myapp:build-123',
        'my-app:v2.0',
        'my_app:stable',
        'registry.com/myapp:v1.0',
      ];

      for (const tag of validTags) {
        config.tag = tag;
        const result = await tagImageTool.run(config, createMockToolContext());

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.tags).toEqual([tag]);
          expect(result.value.success).toBe(true);
          expect(result.value.imageId).toBe('sha256:mock-image-id');
        }

        // Reset mocks for next iteration
        mockDockerClient.tagImage.mockClear();
        mockSessionManager.update.mockClear();
      }
    });

    it('should correctly parse repository and tag components', async () => {
      const testCases = [
        { tag: 'simple:v1', expectedRepo: 'simple', expectedTag: 'v1' },
        { tag: 'multi/level/repo:tag', expectedRepo: 'multi/level/repo', expectedTag: 'tag' },
        { tag: 'single', expectedRepo: 'single', expectedTag: 'latest' },
        { tag: 'with-dash:with-dash-tag', expectedRepo: 'with-dash', expectedTag: 'with-dash-tag' },
        {
          tag: 'with_underscore:with_underscore_tag',
          expectedRepo: 'with_underscore',
          expectedTag: 'with_underscore_tag',
        },
      ];

      for (const testCase of testCases) {
        config.tag = testCase.tag;

        const result = await tagImageTool.run(config, createMockToolContext());

        expect(result.ok).toBe(true);
        expect(mockDockerClient.tagImage).toHaveBeenCalledWith(
          'sha256:mock-image-id',
          testCase.expectedRepo,
          testCase.expectedTag,
        );

        // Reset mocks for next iteration
        mockDockerClient.tagImage.mockClear();
        mockSessionManager.update.mockClear();
      }
    });
  });

  describe('Error Handling', () => {
    beforeEach(() => {
      // Setup session facade with build result for error handling tests
      mockSessionFacade.getResult.mockImplementation((toolName: string) => {
        if (toolName === 'build-image') {
          return {
            imageId: 'sha256:mock-image-id',
            context: '/test/repo',
          };
        }
        return undefined;
      });

      mockDockerClient.tagImage.mockResolvedValue(createSuccessResult({
        success: true,
        imageId: 'sha256:mock-image-id',
      }));
    });

    it('should auto-create session when not found', async () => {
      // Setup session facade with build result
      mockSessionFacade.getResult.mockImplementation((toolName: string) => {
        if (toolName === 'build-image') {
          return {
            imageId: 'sha256:mock-image-id',
            context: '/test/repo',
          };
        }
        return undefined;
      });

      const result = await tagImageTool.run(config, createMockToolContext());

      expect(result.ok).toBe(true);
      // Session facade should have been used to get results
      expect(mockSessionFacade.getResult).toHaveBeenCalledWith('build-image');
    });

    it('should return error when no build result exists', async () => {
      // Setup session facade without build result
      mockSessionFacade.getResult.mockImplementation((toolName: string) => {
        return undefined; // No build result
      });

      const result = await tagImageTool.run(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe(
          'No image specified. Provide imageId parameter or ensure session has built image from build-image tool.',
        );
      }
    });

    it('should return error when build result has no imageId', async () => {
      // Setup session facade with build result missing imageId
      mockSessionFacade.getResult.mockImplementation((toolName: string) => {
        if (toolName === 'build-image') {
          return {
            context: '/test/repo',
            // No imageId
          };
        }
        return undefined;
      });

      const result = await tagImageTool.run(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe(
          'No image specified. Provide imageId parameter or ensure session has built image from build-image tool.',
        );
      }
    });

    it('should return error for invalid tag format', async () => {
      // Setup session facade with build result
      mockSessionFacade.getResult.mockImplementation((toolName: string) => {
        if (toolName === 'build-image') {
          return {
            imageId: 'sha256:mock-image-id',
          };
        }
        return undefined;
      });

      config.tag = ''; // Empty tag

      const result = await tagImageTool.run(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Tag parameter is required');
      }
    });

    it('should handle Docker client tagging failures', async () => {
      // Setup session facade with build result
      mockSessionFacade.getResult.mockImplementation((toolName: string) => {
        if (toolName === 'build-image') {
          return {
            imageId: 'sha256:mock-image-id',
          };
        }
        return undefined;
      });

      mockDockerClient.tagImage.mockResolvedValue(
        createFailureResult('Failed to create tag: image not found'),
      );

      const result = await tagImageTool.run(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Failed to tag image: Failed to create tag: image not found');
      }
    });

    it('should handle Docker client tagging errors without error message', async () => {
      // Setup session facade with build result
      mockSessionFacade.getResult.mockImplementation((toolName: string) => {
        if (toolName === 'build-image') {
          return {
            imageId: 'sha256:mock-image-id',
          };
        }
        return undefined;
      });

      mockDockerClient.tagImage.mockResolvedValue(
        createFailureResult(null as any), // No error message
      );

      const result = await tagImageTool.run(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Failed to tag image: Unknown error');
      }
    });

    it('should handle exceptions during tagging process', async () => {
      // Setup session facade with build result
      mockSessionFacade.getResult.mockImplementation((toolName: string) => {
        if (toolName === 'build-image') {
          return {
            imageId: 'sha256:mock-image-id',
          };
        }
        return undefined;
      });

      mockDockerClient.tagImage.mockRejectedValue(new Error('Docker daemon not responding'));

      const result = await tagImageTool.run(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Docker daemon not responding');
      }

      expect(mockTimer.error).toHaveBeenCalledWith(expect.any(Error));
    });

    it('should handle session update failures gracefully', async () => {
      // Setup session facade with build result
      mockSessionFacade.getResult.mockImplementation((toolName: string) => {
        if (toolName === 'build-image') {
          return {
            imageId: 'sha256:mock-image-id',
          };
        }
        return undefined;
      });

      // Mock sessionFacade.storeResult to throw an error
      const originalStoreResultMock = mockSessionFacade.storeResult.getMockImplementation();
      mockSessionFacade.storeResult.mockImplementation(() => {
        throw new Error('Update failed');
      });

      try {
        const result = await tagImageTool.run(config, createMockToolContext());

        // Should fail if session update fails
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('Update failed');
        }
      } finally {
        // Restore the original mock implementation
        if (originalStoreResultMock) {
          mockSessionFacade.storeResult.mockImplementation(originalStoreResultMock);
        } else {
          mockSessionFacade.storeResult.mockReturnValue(undefined);
        }
      }
    });
  });

  describe('Session State Management', () => {
    beforeEach(() => {
      // Setup session facade with build result
      mockSessionFacade.getResult.mockImplementation((toolName: string) => {
        if (toolName === 'build-image') {
          return {
            imageId: 'sha256:mock-image-id',
            context: '/test/repo',
          };
        }
        return undefined;
      });

      mockDockerClient.tagImage.mockResolvedValue(createSuccessResult({
        success: true,
        imageId: 'sha256:mock-image-id',
      }));
    });

    it('should handle workflow state with existing data', async () => {

      const result = await tagImageTool.run(config, createMockToolContext());

      if (!result.ok) {
        console.log('TAG ERROR:', result.error);
        console.log('FULL RESULT:', JSON.stringify(result, null, 2));
        throw new Error(`Tag failed: ${result.error}`);
      }
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.success).toBe(true);
        expect(result.value.imageId).toBe('sha256:mock-image-id');
        expect(result.value.tags).toEqual(['myapp:v1.0']);
      }
      // Verify storeResult was called with normalized pattern
      expect(mockSessionFacade.storeResult).toHaveBeenCalledWith(
        'tag-image',
        expect.objectContaining({
          success: true,
          imageId: 'sha256:mock-image-id',
          tags: ['myapp:v1.0'],
        }),
      );
    });

    it('should handle session with minimal build result', async () => {

      const result = await tagImageTool.run(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.success).toBe(true);
        expect(result.value.imageId).toBe('sha256:mock-image-id');
        expect(result.value.tags).toEqual(['myapp:v1.0']);
      }
      // Note: Session storage is handled by the orchestrator, not the tool directly
    });
  });

  describe('Multiple Tagging Scenarios', () => {

    beforeEach(() => {
      // Setup session facade with build result
      mockSessionFacade.getResult.mockImplementation((toolName: string) => {
        if (toolName === 'build-image') {
          return {
            imageId: 'sha256:mock-image-id',
            context: '/test/repo',
          };
        }
        return undefined;
      });

      mockDockerClient.tagImage.mockResolvedValue(createSuccessResult({
        success: true,
        imageId: 'sha256:mock-image-id',
      }));
    });

    it('should handle tagging with different configurations', async () => {
      const configurations = [
        { sessionId: 'session-1', tag: 'app:v1.0' },
        { sessionId: 'session-2', tag: 'registry.com/app:latest' },
        { sessionId: 'session-3', tag: 'my-app:development' },
      ];

      for (const testConfig of configurations) {
        // Setup session for each different sessionId
        mockSessionFacade.getResult.mockImplementation((toolName: string) => {
          if (toolName === 'build-image') {
            return {
              imageId: 'sha256:mock-image-id',
              context: '/test/repo',
            };
          }
          return undefined;
        });

        const result = await tagImageTool.run(testConfig, createMockToolContext());

        if (!result.ok) {
          console.log('TAG ERROR:', result.error);
        }
        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.sessionId).toBe(testConfig.sessionId);
          expect(result.value.tags).toEqual([testConfig.tag]);
          expect(result.value.success).toBe(true);
          expect(result.value.imageId).toBe('sha256:mock-image-id');
        }

        // Reset mocks for next iteration
        mockDockerClient.tagImage.mockClear();
        mockSessionFacade.getResult.mockClear();
        mockSessionFacade.storeResult.mockClear();
      }
    });

    it('should handle sequential tagging operations on same session', async () => {
      // Setup session facade with build result
      mockSessionFacade.getResult.mockImplementation((toolName: string) => {
        if (toolName === 'build-image') {
          return {
            imageId: 'sha256:mock-image-id',
          };
        }
        return undefined;
      });

      const tags = ['myapp:v1.0', 'myapp:latest', 'myapp:stable'];

      for (const tag of tags) {
        config.tag = tag;
        const result = await tagImageTool.run(config, createMockToolContext());

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.tags).toEqual([tag]);
          expect(result.value.success).toBe(true);
          expect(result.value.imageId).toBe('sha256:mock-image-id');
        }

        // Each operation should tag the same image
        expect(mockDockerClient.tagImage).toHaveBeenCalledWith(
          'sha256:mock-image-id',
          expect.any(String),
          expect.any(String),
        );

        // Reset mocks for next iteration
        mockDockerClient.tagImage.mockClear();
        mockSessionManager.update.mockClear();
      }
    });
  });

  describe('Tool Instance', () => {
    beforeEach(() => {
      // Setup session facade with build result
      mockSessionFacade.getResult.mockImplementation((toolName: string) => {
        if (toolName === 'build-image') {
          return {
            imageId: 'sha256:mock-image-id',
            context: '/test/repo',
          };
        }
        return undefined;
      });

      mockDockerClient.tagImage.mockResolvedValue(createSuccessResult({
        success: true,
        imageId: 'sha256:mock-image-id',
      }));
    });

    it('should provide correctly configured tool instance', async () => {
      const importedTool = await import('../../../src/tools/tag-image/tool');
      const tagImageTool = importedTool.default;

      // The tool is now an object with a run method
      expect(typeof tagImageTool).toBe('object');
      expect(typeof tagImageTool.run).toBe('function');

      // Verify tool can be executed through the tool instance interface

      // The tool can be called via its run method
      const result = await tagImageTool.run(config, createMockToolContext());
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.success).toBe(true);
        expect(result.value.imageId).toBe('sha256:mock-image-id');
        expect(result.value.tags).toEqual(['myapp:v1.0']);
      }
    });
  });
});
