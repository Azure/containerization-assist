/**
 * Tests for ToolContext implementation
 */

import { describe, test, expect, jest, beforeEach } from '@jest/globals';
import type { Logger } from 'pino';
import { createToolContext } from '@/mcp/context';
import { extractProgressToken, createProgressReporter } from '@/mcp/context';
import type { Server } from '@modelcontextprotocol/sdk/server/index.js';

// Mock server and logger
const createMockServer = (): Server =>
  ({
    sendNotification: jest.fn(),
  }) as any;

const createMockLogger = (): Logger =>
  ({
    debug: jest.fn(),
    info: jest.fn(),
    error: jest.fn(),
    warn: jest.fn(),
    child: jest.fn(() => createMockLogger()),
  }) as any;

describe('ToolContext', () => {
  let mockServer: Server;
  let mockLogger: Logger;

  beforeEach(() => {
    mockServer = createMockServer();
    mockLogger = createMockLogger();
    jest.clearAllMocks();
  });

  describe('createToolContext', () => {
    test('creates valid ToolContext with required properties', () => {
      const context = createToolContext(mockLogger);

      expect(context).toHaveProperty('logger');
      expect(context.logger).toBe(mockLogger);
      // Optional properties are omitted when not provided
      expect(context.signal).toBeUndefined();
      expect(context.progress).toBeUndefined();
    });

    test('forwards abort signal', () => {
      const abortController = new AbortController();
      const context = createToolContext(mockLogger, { signal: abortController.signal });

      expect(context.signal).toBe(abortController.signal);
    });

    test('includes progress reporter if provided', () => {
      const mockProgressReporter = jest.fn();
      const context = createToolContext(mockLogger, { progress: mockProgressReporter });

      expect(context.progress).toBe(mockProgressReporter);
    });
  });

  describe('extractProgressToken', () => {
    test('extracts progress token from request metadata', () => {
      const request = {
        params: {
          _meta: {
            progressToken: 'test-token-123',
          },
        },
      };

      const token = extractProgressToken(request);
      expect(token).toBe('test-token-123');
    });

    test('returns undefined for missing metadata', () => {
      expect(extractProgressToken({})).toBeUndefined();
      expect(extractProgressToken({ params: {} })).toBeUndefined();
      expect(extractProgressToken({ params: { _meta: {} } })).toBeUndefined();
    });

    test('handles non-string progress tokens', () => {
      const request = {
        params: {
          _meta: {
            progressToken: 12345, // Not a string
          },
        },
      };

      const token = extractProgressToken(request);
      expect(token).toBeUndefined();
    });

    test('handles null/undefined request safely', () => {
      expect(extractProgressToken(null)).toBeUndefined();
      expect(extractProgressToken(undefined)).toBeUndefined();
    });
  });

  describe('createProgressReporter', () => {
    test('returns undefined when no progress token provided', () => {
      const reporter = createProgressReporter(undefined, mockLogger);
      expect(reporter).toBeUndefined();
    });

    test('creates progress reporter when token provided', () => {
      const reporter = createProgressReporter('test-token', mockLogger);
      expect(reporter).toBeInstanceOf(Function);
    });

    test('progress reporter logs progress', async () => {
      const reporter = createProgressReporter('test-token', mockLogger);

      if (reporter) {
        await reporter('Processing...', 50, 100);
        expect(mockLogger.debug).toHaveBeenCalledWith(
          expect.objectContaining({
            progressToken: 'test-token',
            message: 'Processing...',
            progress: 50,
            total: 100,
            type: 'progress_notification',
          }),
          'Progress notification logged - no sendNotification callback available',
        );
      }
    });
  });

  describe('createToolContextWithProgress', () => {
    test('creates context with progress token extraction', () => {
      const request = {
        params: {
          _meta: {
            progressToken: 'test-token-123',
          },
        },
      };

      const context = createToolContext(mockLogger, {
        progress: request,
      });

      expect(context).toHaveProperty('progress');
      expect(context.progress).toBeInstanceOf(Function);
    });

    test('creates context without progress when no token', () => {
      const request = { params: {} };

      const context = createToolContext(mockLogger, {
        progress: request,
      });

      // Optional property is omitted when not extractable from request
      expect('progress' in context).toBe(false);
    });
  });

  describe('error handling and logging', () => {
    test('handles progress reporting errors gracefully', async () => {
      const reporter = createProgressReporter('test-token', mockLogger);

      // Mock logger methods to throw
      (mockLogger.debug as jest.Mock).mockImplementation(() => {
        throw new Error('Logger error');
      });

      if (reporter) {
        // Should not throw despite logger error
        await expect(reporter('test', 50, 100)).resolves.not.toThrow();
      }
    });
  });
});
