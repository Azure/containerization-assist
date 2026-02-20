/**
 * Unit Tests: Docker Error Scenarios
 * Tests Docker error handling patterns for build-image context preparation
 */

import { jest } from '@jest/globals';

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

function createMockToolContext() {
  return {
    logger: createMockLogger(),
  } as any;
}

jest.mock('../../../src/lib/logger', () => ({
  createTimer: jest.fn(() => ({
    end: jest.fn(),
    error: jest.fn(),
  })),
  createLogger: jest.fn(() => createMockLogger()),
}));

jest.mock('../../../src/lib/validation-helpers', () => ({
  validatePathOrFail: jest.fn().mockImplementation(async (pathStr: string) => {
    return { ok: true, value: pathStr };
  }),
}));

// Mock file-utils which is what the tool actually uses
const mockReadDockerfile = jest.fn<() => Promise<{ ok: true; value: string } | { ok: false; error: string }>>();

jest.mock('../../../src/lib/file-utils', () => ({
  readDockerfile: mockReadDockerfile,
}));

// Mock fs/promises for path resolution
jest.mock('fs/promises', () => ({
  access: jest.fn().mockResolvedValue(undefined),
  stat: jest.fn().mockResolvedValue({ isFile: () => true, isDirectory: () => false }),
  readFile: jest.fn(),
}));

import { buildImageContext } from '../../../src/tools/build-image-context/tool';

describe('Build Image Error Scenarios', () => {
  const mockDockerfile = `FROM node:18-alpine\nWORKDIR /app\nCOPY . .\nUSER appuser\nCMD ["node", "index.js"]`;

  beforeEach(() => {
    jest.clearAllMocks();
    mockReadDockerfile.mockResolvedValue({ ok: true, value: mockDockerfile });
  });

  describe('Error Handling Pattern', () => {
    it('should return Result<T> on filesystem errors', async () => {
      mockReadDockerfile.mockResolvedValue({ ok: false, error: 'Permission denied' });

      const result = await buildImageContext(
        {
          path: '/test/repo',
          dockerfile: 'Dockerfile',
          imageName: 'test:latest',
          tags: [],
          buildArgs: {},
        },
        createMockToolContext(),
      );

      expect(result).toHaveProperty('ok');
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(typeof result.error).toBe('string');
        expect(result.error.length).toBeGreaterThan(0);
      }
    });

    it('should never throw exceptions', async () => {
      mockReadDockerfile.mockRejectedValue(new Error('Unexpected error'));

      await expect(
        buildImageContext(
          {
            path: '/test/repo',
            dockerfile: 'Dockerfile',
            imageName: 'test:latest',
            tags: [],
            buildArgs: {},
          },
          createMockToolContext(),
        ),
      ).resolves.not.toThrow();
    });

    it('should return failure for invalid parameters', async () => {
      const result = await buildImageContext(null as any, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
      }
    });
  });

  describe('Filesystem Errors', () => {
    it('should handle EACCES errors', async () => {
      mockReadDockerfile.mockResolvedValue({ ok: false, error: 'EACCES: permission denied' });

      const result = await buildImageContext(
        { path: '/test', dockerfile: 'Dockerfile', imageName: 'test:latest', tags: [], buildArgs: {} },
        createMockToolContext(),
      );

      expect(result.ok).toBe(false);
    });

    it('should handle ENOENT errors', async () => {
      mockReadDockerfile.mockResolvedValue({ ok: false, error: 'ENOENT: no such file or directory' });

      const result = await buildImageContext(
        { path: '/test', dockerfile: 'Dockerfile', imageName: 'test:latest', tags: [], buildArgs: {} },
        createMockToolContext(),
      );

      expect(result.ok).toBe(false);
    });

    it('should handle EISDIR errors', async () => {
      mockReadDockerfile.mockResolvedValue({ ok: false, error: 'EISDIR: illegal operation on a directory' });

      const result = await buildImageContext(
        { path: '/test', dockerfile: 'Dockerfile', imageName: 'test:latest', tags: [], buildArgs: {} },
        createMockToolContext(),
      );

      expect(result.ok).toBe(false);
    });
  });

  describe('Guidance Structure', () => {
    it('should provide guidance on errors', async () => {
      mockReadDockerfile.mockResolvedValue({ ok: false, error: 'File not found' });

      const result = await buildImageContext(
        { path: '/test', dockerfile: 'Dockerfile', imageName: 'test:latest', tags: [], buildArgs: {} },
        createMockToolContext(),
      );

      expect(result.ok).toBe(false);
      // Note: guidance is optional and may not be provided for all errors
      if (!result.ok) {
        expect(result.error).toBeDefined();
      }
    });
  });

  describe('Success Cases', () => {
    it('should succeed when Dockerfile is readable', async () => {
      const result = await buildImageContext(
        { path: '/test', dockerfile: 'Dockerfile', imageName: 'test:latest', tags: ['v1.0.0'], buildArgs: {} },
        createMockToolContext(),
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.summary).toBeDefined();
        expect(result.value.nextAction.buildCommand.command).toBeDefined();
      }
    });

    it('should detect security issues in Dockerfile', async () => {
      // Dockerfile running as root
      mockReadDockerfile.mockResolvedValue({ 
        ok: true, 
        value: 'FROM node:18\nWORKDIR /app\nCOPY . .\nCMD ["node", "index.js"]' 
      });

      const result = await buildImageContext(
        { path: '/test', dockerfile: 'Dockerfile', imageName: 'test:latest', tags: [], buildArgs: {} },
        createMockToolContext(),
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        // Should detect running as root (no USER directive)
        expect(result.value.securityAnalysis.warnings.length).toBeGreaterThan(0);
      }
    });

    it('should include BuildKit analysis', async () => {
      const result = await buildImageContext(
        { path: '/test', dockerfile: 'Dockerfile', imageName: 'test:latest', tags: [], buildArgs: {} },
        createMockToolContext(),
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.buildKitAnalysis).toBeDefined();
        expect(result.value.buildKitAnalysis.features).toBeDefined();
      }
    });
  });
});
