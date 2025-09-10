/**
 * Unit Tests: Image Push Tool
 * Tests the push image tool functionality with mock Docker client
 * Following analyze-repo test structure and comprehensive coverage requirements
 */

import { jest } from '@jest/globals';
import { pushImage, type PushImageParams } from '../../../src/tools/push-image/tool';
import { createMockLogger, createSuccessResult, createFailureResult } from '../../__support__/utilities/mock-infrastructure';

// Mock lib modules following analyze-repo pattern
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
  pushImage: jest.fn(),
};

const mockTimer = {
  end: jest.fn(),
  error: jest.fn(),
};

jest.mock('@lib/session', () => ({
  createSessionManager: jest.fn(() => mockSessionManager),
}));

jest.mock('@lib/docker', () => ({
  createDockerClient: jest.fn(() => mockDockerClient),
}));

jest.mock('@lib/logger', () => ({
  createTimer: jest.fn(() => mockTimer),
  createLogger: jest.fn(() => createMockLogger()),
}));

// Mock session helpers
jest.mock('@mcp/tools/session-helpers');

describe('pushImage', () => {
  let mockLogger: ReturnType<typeof createMockLogger>;
  let config: PushImageParams;

  beforeEach(() => {
    mockLogger = createMockLogger();
    config = {
      sessionId: 'test-session-123',
      registry: 'docker.io',
      credentials: {
        username: 'testuser',
        password: 'testpass',
      },
    };

    // Reset all mocks
    jest.clearAllMocks();
    
    // Setup session helper mocks
    const sessionHelpers = require('@mcp/tools/session-helpers');
    sessionHelpers.getSession = jest.fn().mockResolvedValue({
      ok: true,
      value: {
        id: 'test-session-123',
        state: {
          sessionId: 'test-session-123',
          build_result: {
            imageId: 'sha256:mock-image-id',
            tags: ['myapp:v1.0', 'myapp:latest'],
          },
          workflow_state: {
            build_result: {
              imageId: 'sha256:mock-image-id',
              tags: ['myapp:v1.0', 'myapp:latest'],
            },
          },
          metadata: {},
          completed_steps: [],
        },
        isNew: false,
      },
    });
    sessionHelpers.updateSession = jest.fn().mockResolvedValue({ ok: true });
    mockSessionManager.update.mockResolvedValue(true);
  });


  describe('Successful Push Operations', () => {
    beforeEach(() => {
      // Session with tagged images
      mockSessionManager.get.mockResolvedValue({
        
build_result: {
  imageId: 'sha256:mock-image-id',
  tags: ['myapp:v1.0', 'myapp:latest'],
},
        repo_path: '/test/repo',
      });

      // Default successful push result
      mockDockerClient.pushImage.mockResolvedValue(createSuccessResult({
        digest: 'sha256:abc123def456',
        size: 1024000,
      }));
    });

    it('should successfully push image to registry', async () => {
      const result = await pushImage(config, { logger: mockLogger });

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.success).toBe(true);
        expect(result.value.sessionId).toBe('test-session-123');
        expect(result.value.registry).toBe('docker.io');
        expect(result.value.digest).toBe('sha256:abc123def456');
        expect(result.value.pushedTags).toEqual(['myapp:v1.0']);
      }

      // Verify Docker client was called with correct parameters
      expect(mockDockerClient.pushImage).toHaveBeenCalledWith('myapp', 'v1.0');
      
      // Verify session was updated with push results
      const sessionHelpers = require('@mcp/tools/session-helpers');
      expect(sessionHelpers.updateSession).toHaveBeenCalledWith(
        'test-session-123',
        expect.objectContaining({
          completed_steps: expect.arrayContaining(['push']),
          metadata: expect.objectContaining({
            pushResult: expect.objectContaining({
              registry: 'docker.io',
              digest: 'sha256:abc123def456',
              pushedTags: ['myapp:v1.0'],
            }),
          }),
        }),
        expect.any(Object)
      );

      // Verify timer was used correctly
      expect(mockTimer.end).toHaveBeenCalledWith({
        imageTag: 'myapp:v1.0',
        registry: 'docker.io',
        digest: 'sha256:abc123def456',
      });
    });

    it('should use default registry when not specified', async () => {
      const minimalConfig: PushImageParams = {
        sessionId: 'test-session-123',
      };

      const result = await pushImage(minimalConfig, mockLogger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.registry).toBe('docker.io'); // Default registry
      }

      expect(mockDockerClient.pushImage).toHaveBeenCalledWith('myapp', 'v1.0');
    });

    it('should handle image tags without explicit tag (defaults to latest)', async () => {
      const sessionHelpers = require('@mcp/tools/session-helpers');
      sessionHelpers.getSession.mockResolvedValue({
        ok: true,
        value: {
          id: 'test-session-123',
          state: {
            sessionId: 'test-session-123',
            build_result: {
              imageId: 'sha256:mock-image-id',
              tags: ['myapp'], // No explicit tag
            },
            workflow_state: {},
            metadata: {},
            completed_steps: [],
          },
          isNew: false,
        },
      });

      const result = await pushImage(config, { logger: mockLogger });

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.pushedTags).toEqual(['myapp']);
      }

      // Should push with 'latest' tag when no tag specified
      expect(mockDockerClient.pushImage).toHaveBeenCalledWith('myapp', 'latest');
    });

    it('should push the first tag when multiple tags exist', async () => {
      const sessionHelpers = require('@mcp/tools/session-helpers');
      sessionHelpers.getSession.mockResolvedValue({
        ok: true,
        value: {
          id: 'test-session-123',
          state: {
            sessionId: 'test-session-123',
            build_result: {
              imageId: 'sha256:mock-image-id',
              tags: ['myapp:v2.0', 'myapp:latest', 'myapp:stable'],
            },
            workflow_state: {},
            metadata: {},
            completed_steps: [],
          },
          isNew: false,
        },
      });

      const result = await pushImage(config, { logger: mockLogger });

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.pushedTags).toEqual(['myapp:v2.0']);
      }

      // Should use the first tag
      expect(mockDockerClient.pushImage).toHaveBeenCalledWith('myapp', 'v2.0');
    });

  });


  describe('Registry Configuration', () => {
    beforeEach(() => {
      mockSessionManager.get.mockResolvedValue({
        
build_result: {
  imageId: 'sha256:mock-image-id',
  tags: ['myapp:v1.0'],
},
        repo_path: '/test/repo',
      });

      mockDockerClient.pushImage.mockResolvedValue(createSuccessResult({
        digest: 'sha256:abc123def456',
      }));
    });

    it('should handle different registry configurations', async () => {
      const registries = [
        'docker.io',
        'ghcr.io',
        'quay.io',
        'registry.example.com:5000',
      ];

      for (const registry of registries) {
        config.registry = registry;
        const result = await pushImage(config, { logger: mockLogger });

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.registry).toBe(registry);
        }

        // Reset mocks for next iteration
        // Clear only Docker client mock for next iteration
        mockDockerClient.pushImage.mockClear();
        mockDockerClient.pushImage.mockResolvedValue(createSuccessResult({
          digest: 'sha256:abc123def456',
        }));
      }
    });

    it('should handle authentication parameters', async () => {
      config.credentials = {
        username: 'myuser',
        password: 'mypassword',
      };

      const result = await pushImage(config, { logger: mockLogger });

      expect(result.ok).toBe(true);
      // Authentication is typically handled by the Docker client internally
      // This test verifies the function accepts these parameters without error
    });
  });
});

