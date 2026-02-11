/**
 * Tests for MCP context helper functions, especially progress notifications
 */

import { describe, it, expect, jest, beforeEach } from '@jest/globals';
import type { Server } from '@modelcontextprotocol/sdk/server/index.js';
import type { Logger } from 'pino';
import {
  extractProgressToken,
  createProgressReporter,
  extractProgressReporter,
} from '@/mcp/context-helpers';

describe('MCP Context Helpers', () => {
  let mockServer: Server;
  let mockLogger: Logger;

  beforeEach(() => {
    mockServer = {} as Server;
    mockLogger = {
      debug: jest.fn(),
      warn: jest.fn(),
      info: jest.fn(),
      error: jest.fn(),
    } as unknown as Logger;
  });

  describe('extractProgressToken', () => {
    it('should extract progressToken from params._meta (canonical MCP spec location)', () => {
      const request = {
        params: {
          _meta: {
            progressToken: 'nested-token-456',
          },
        },
      };
      const token = extractProgressToken(request);
      expect(token).toBe('nested-token-456');
    });

    it('should return undefined for invalid input', () => {
      expect(extractProgressToken(null)).toBeUndefined();
      expect(extractProgressToken(undefined)).toBeUndefined();
      expect(extractProgressToken('string')).toBeUndefined();
      expect(extractProgressToken(123)).toBeUndefined();
      expect(extractProgressToken({})).toBeUndefined();
    });

    it('should return undefined when params is missing', () => {
      const request = { _meta: { progressToken: 'should-not-find' } };
      const token = extractProgressToken(request);
      expect(token).toBeUndefined();
    });

    it('should return undefined when _meta is missing', () => {
      const request = { params: { progressToken: 'should-not-find' } };
      const token = extractProgressToken(request);
      expect(token).toBeUndefined();
    });

    it('should return undefined when progressToken is not a string', () => {
      const request = { params: { _meta: { progressToken: 123 } } };
      const token = extractProgressToken(request);
      expect(token).toBeUndefined();
    });
  });

  describe('createProgressReporter', () => {
    it('should return undefined when no token provided', () => {
      const reporter = createProgressReporter(undefined, mockLogger);
      expect(reporter).toBeUndefined();
    });

    it('should create a reporter with string token', async () => {
      const mockSendNotification = jest.fn<(notification: unknown) => Promise<void>>().mockResolvedValue(undefined);
      const reporter = createProgressReporter('test-token', mockLogger, mockSendNotification);

      expect(reporter).toBeDefined();
      await reporter!('Processing step 1', 1, 5);

      expect(mockSendNotification).toHaveBeenCalledTimes(1);
      expect(mockSendNotification).toHaveBeenCalledWith({
        method: 'notifications/progress',
        params: {
          progressToken: 'test-token',
          progress: 1,
          total: 5,
          message: 'Processing step 1',
        },
      });
    });

    it('should create a reporter with numeric token', async () => {
      const mockSendNotification = jest.fn<(notification: unknown) => Promise<void>>().mockResolvedValue(undefined);
      const reporter = createProgressReporter(12345, mockLogger, mockSendNotification);

      expect(reporter).toBeDefined();
      await reporter!('Processing', 50, 100);

      expect(mockSendNotification).toHaveBeenCalledTimes(1);
      expect(mockSendNotification).toHaveBeenCalledWith({
        method: 'notifications/progress',
        params: {
          progressToken: '12345',
          progress: 50,
          total: 100,
          message: 'Processing',
        },
      });
    });

    it('should handle notification without total', async () => {
      const mockSendNotification = jest.fn<(notification: unknown) => Promise<void>>().mockResolvedValue(undefined);
      const reporter = createProgressReporter('token', mockLogger, mockSendNotification);

      await reporter!('Processing', 25);

      expect(mockSendNotification).toHaveBeenCalledWith({
        method: 'notifications/progress',
        params: {
          progressToken: 'token',
          progress: 25,
          message: 'Processing',
        },
      });
    });

    it('should include metadata when provided', async () => {
      const mockSendNotification = jest.fn<(notification: unknown) => Promise<void>>().mockResolvedValue(undefined);
      const reporter = createProgressReporter('token', mockLogger, mockSendNotification);

      await reporter!('Processing', 10, 20, { stepName: 'validation', details: 'checking files' });

      expect(mockSendNotification).toHaveBeenCalledWith({
        method: 'notifications/progress',
        params: {
          progressToken: 'token',
          progress: 10,
          total: 20,
          message: 'Processing',
          stepName: 'validation',
          details: 'checking files',
        },
      });
    });

    it('should log warning on sendNotification failure', async () => {
      const mockSendNotification = jest.fn<(notification: unknown) => Promise<void>>().mockRejectedValue(new Error('Network error'));
      const reporter = createProgressReporter('token', mockLogger, mockSendNotification);

      await reporter!('Processing', 1, 5);

      expect(mockLogger.warn).toHaveBeenCalledWith(
        expect.objectContaining({
          progressToken: 'token',
          error: 'Network error',
        }),
        'Failed to send MCP progress notification',
      );
    });

    it('should fallback to logging when no sendNotification provided', async () => {
      const reporter = createProgressReporter('token', mockLogger);

      await reporter!('Processing', 1, 5);

      expect(mockLogger.debug).toHaveBeenCalledWith(
        expect.objectContaining({
          progressToken: 'token',
          message: 'Processing',
          progress: 1,
          total: 5,
        }),
        'Progress notification logged - no sendNotification callback available',
      );
    });
  });

  describe('extractProgressReporter', () => {
    it('should return undefined when no progress provided', () => {
      const reporter = extractProgressReporter(undefined, mockLogger);
      expect(reporter).toBeUndefined();
    });

    it('should return undefined when null provided', () => {
      const reporter = extractProgressReporter(null, mockLogger);
      expect(reporter).toBeUndefined();
    });

    it('should create reporter from string token', async () => {
      const mockSendNotification = jest.fn<(notification: unknown) => Promise<void>>().mockResolvedValue(undefined);
      const reporter = extractProgressReporter('extracted-token', mockLogger, mockSendNotification);

      expect(reporter).toBeDefined();
      await reporter!('Test message', 1, 1);

      expect(mockSendNotification).toHaveBeenCalledWith(
        expect.objectContaining({
          method: 'notifications/progress',
          params: expect.objectContaining({
            progressToken: 'extracted-token',
          }),
        }),
      );
    });

    it('should create reporter from number token', async () => {
      const mockSendNotification = jest.fn<(notification: unknown) => Promise<void>>().mockResolvedValue(undefined);
      const reporter = extractProgressReporter(12345, mockLogger, mockSendNotification);

      expect(reporter).toBeDefined();
      await reporter!('Nested test', 5, 10);

      expect(mockSendNotification).toHaveBeenCalledWith(
        expect.objectContaining({
          method: 'notifications/progress',
          params: expect.objectContaining({
            progressToken: '12345',
            progress: 5,
            total: 10,
          }),
        }),
      );
    });
  });

  describe('Integration: Full progress notification flow', () => {
    it('should handle complete progress reporting lifecycle', async () => {
      const mockSendNotification = jest.fn<(notification: unknown) => Promise<void>>().mockResolvedValue(undefined);

      const reporter = extractProgressReporter('lifecycle-token', mockLogger, mockSendNotification);
      expect(reporter).toBeDefined();

      // Simulate multi-step progress reporting
      await reporter!('Starting task', 0, 100);
      await reporter!('Processing files', 25, 100);
      await reporter!('Validating results', 50, 100);
      await reporter!('Finalizing', 75, 100);
      await reporter!('Complete', 100, 100);

      expect(mockSendNotification).toHaveBeenCalledTimes(5);

      // Verify first and last calls
      expect(mockSendNotification).toHaveBeenNthCalledWith(1, {
        method: 'notifications/progress',
        params: {
          progressToken: 'lifecycle-token',
          progress: 0,
          total: 100,
          message: 'Starting task',
        },
      });

      expect(mockSendNotification).toHaveBeenNthCalledWith(5, {
        method: 'notifications/progress',
        params: {
          progressToken: 'lifecycle-token',
          progress: 100,
          total: 100,
          message: 'Complete',
        },
      });
    });
  });
});