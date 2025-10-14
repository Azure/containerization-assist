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
const mockDockerClient = {
  tagImage: jest.fn(),
};

// Create timer mock before jest.mock calls so it's available
const mockTimer = {
  end: jest.fn(),
  error: jest.fn(),
};

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


// Import these after mocks are set up
import tagImageTool from '../../../src/tools/tag-image/tool';
import type { TagImageParams } from '../../../src/tools/tag-image/schema';

// Create global mock tool context
function createMockToolContext() {
  return {
    logger: createMockLogger(),
  } as any;
}

describe('tagImage', () => {
  let mockLogger: ReturnType<typeof createMockLogger>;
  let config: TagImageParams;

  beforeEach(() => {
    mockLogger = createMockLogger();
    config = {
      imageId: 'sha256:mock-image-id',
      tag: 'myapp:v1.0',
    };

    // Reset all mocks completely
    jest.clearAllMocks();
    jest.restoreAllMocks();

    // Re-establish timer mock after clearing
    const { createToolTimer } = jest.requireMock('../../../src/lib/tool-helpers');
    createToolTimer.mockReturnValue(mockTimer);

    // Setup default successful Docker tag result
    mockDockerClient.tagImage.mockResolvedValue(
      createSuccessResult({
        success: true,
        imageId: 'sha256:mock-image-id',
      }),
    );
  });

  describe('Successful Tagging Operations', () => {
    it('should successfully tag image with repository and tag', async () => {
      const result = await tagImageTool.run(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.success).toBe(true);
        expect(result.value.tags).toEqual(['myapp:v1.0']);
        expect(result.value.imageId).toBe('sha256:mock-image-id');
      }

      // Verify Docker client was called with correct parameters
      expect(mockDockerClient.tagImage).toHaveBeenCalledWith(
        'sha256:mock-image-id',
        'myapp',
        'v1.0',
      );

      // Verify timer was used correctly
      expect(mockTimer.end).toHaveBeenCalledWith({
        tags: ['myapp:v1.0'],
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
      }
    });

    it('should succeed with valid imageId', async () => {
      const result = await tagImageTool.run(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.imageId).toBe('sha256:mock-image-id');
      }
    });
  });

  describe('Tag Format Validation', () => {
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
      }
    });
  });

  describe('Error Handling', () => {
    it('should succeed with valid imageId', async () => {
      const result = await tagImageTool.run(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.imageId).toBe('sha256:mock-image-id');
      }
    });

    it('should return error when no imageId provided', async () => {
      const configWithoutImage = {
        ...config,
        imageId: undefined,
      };

      const result = await tagImageTool.run(configWithoutImage, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('No image specified');
      }
    });

    it('should return error for invalid tag format', async () => {
      const configWithEmptyTag = {
        ...config,
        tag: '',
      };

      const result = await tagImageTool.run(configWithEmptyTag, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Tag parameter is required');
      }
    });

    it('should handle Docker client tagging failures', async () => {
      mockDockerClient.tagImage.mockResolvedValue(
        createFailureResult('Failed to create tag: image not found'),
      );

      const result = await tagImageTool.run(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Failed to tag image');
      }
    });

    it('should handle Docker client tagging errors without error message', async () => {
      mockDockerClient.tagImage.mockResolvedValue(
        createFailureResult(null as any), // No error message
      );

      const result = await tagImageTool.run(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Failed to tag image');
      }
    });

    it('should handle exceptions during tagging process', async () => {
      mockDockerClient.tagImage.mockRejectedValue(new Error('Docker daemon not responding'));

      const result = await tagImageTool.run(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Docker daemon not responding');
      }

      expect(mockTimer.error).toHaveBeenCalledWith(expect.any(Error));
    });

    it('should succeed with valid imageId and tag', async () => {
      const result = await tagImageTool.run(config, createMockToolContext());

      // Tool should succeed - orchestrator handles result storage
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.success).toBe(true);
        expect(result.value.imageId).toBe('sha256:mock-image-id');
      }
    });
  });

  describe('Successful Operations', () => {
    it('should succeed with valid parameters', async () => {
      const result = await tagImageTool.run(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.success).toBe(true);
        expect(result.value.imageId).toBe('sha256:mock-image-id');
        expect(result.value.tags).toEqual(['myapp:v1.0']);
      }
    });
  });

  describe('Multiple Tagging Scenarios', () => {
    it('should handle tagging with different configurations', async () => {
      const configurations = [
        { imageId: 'sha256:mock-image-id', tag: 'app:v1.0' },
        { imageId: 'sha256:mock-image-id', tag: 'registry.com/app:latest' },
        { imageId: 'sha256:mock-image-id', tag: 'my-app:development' },
      ];

      for (const testConfig of configurations) {
        const result = await tagImageTool.run(testConfig, createMockToolContext());

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.tags).toEqual([testConfig.tag]);
          expect(result.value.success).toBe(true);
          expect(result.value.imageId).toBe('sha256:mock-image-id');
        }

        // Reset mocks for next iteration
        mockDockerClient.tagImage.mockClear();
      }
    });

    it('should handle sequential tagging operations', async () => {
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
      }
    });
  });

  describe('Tool Instance', () => {
    it('should provide correctly configured tool instance', async () => {
      const importedTool = await import('../../../src/tools/tag-image/tool');
      const tagImageTool = importedTool.default;

      // The tool is now an object with a run method
      expect(typeof tagImageTool).toBe('object');
      expect(typeof tagImageTool.run).toBe('function');

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
