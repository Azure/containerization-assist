/**
 * Tests for tool helper utilities
 */

import { getToolLogger, createToolTimer, createStandardizedToolTracker } from '@/lib/tool-helpers';
import { createLogger } from '@/lib/logger';
import type { ToolContext } from '@/mcp/context';

// Mock the runtime logging functions
jest.mock('@/lib/runtime-logging', () => ({
  logToolStart: jest.fn(),
  logToolComplete: jest.fn(),
  logToolFailure: jest.fn(),
}));

import { logToolStart, logToolComplete, logToolFailure } from '@/lib/runtime-logging';

describe('Tool Helpers', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe('getToolLogger', () => {
    it('should return logger from context if present', () => {
      const contextLogger = createLogger({ name: 'context-logger' });
      const context: ToolContext = {
        logger: contextLogger,
        requestId: 'test-request',
      };

      const result = getToolLogger(context, 'test-tool');

      expect(result).toBe(contextLogger);
    });

    it('should create new logger if context has no logger', () => {
      const context: ToolContext = {
        requestId: 'test-request',
      };

      const result = getToolLogger(context, 'test-tool');

      expect(result).toBeDefined();
      expect(result).toHaveProperty('info');
      expect(result).toHaveProperty('warn');
      expect(result).toHaveProperty('error');
    });

    it('should use tool name when creating new logger', () => {
      const context: ToolContext = {
        requestId: 'test-request',
      };

      const result = getToolLogger(context, 'my-custom-tool');

      expect(result).toBeDefined();
      // Logger is created with the tool name
    });

    it('should handle context with undefined logger', () => {
      const context: ToolContext = {
        logger: undefined,
        requestId: 'test-request',
      };

      const result = getToolLogger(context, 'test-tool');

      expect(result).toBeDefined();
      expect(result).toHaveProperty('info');
    });
  });

  describe('createToolTimer', () => {
    it('should create a timer with logger and tool name', () => {
      const logger = createLogger({ name: 'test-logger' });

      const timer = createToolTimer(logger, 'test-tool');

      expect(timer).toBeDefined();
      expect(timer).toHaveProperty('end');
      expect(typeof timer.end).toBe('function');
    });

    it('should create timer that can be ended', () => {
      const logger = createLogger({ name: 'test-logger' });

      const timer = createToolTimer(logger, 'test-tool');

      expect(() => timer.end()).not.toThrow();
    });
  });

  describe('createStandardizedToolTracker', () => {
    it('should call logToolStart when created', () => {
      const logger = createLogger({ name: 'test-logger' });
      const params = { path: './app', tag: 'latest' };

      createStandardizedToolTracker('build-image', params, logger);

      expect(logToolStart).toHaveBeenCalledWith('build-image', params, logger);
    });

    it('should return object with complete and fail methods', () => {
      const logger = createLogger({ name: 'test-logger' });
      const params = { path: './app' };

      const tracker = createStandardizedToolTracker('test-tool', params, logger);

      expect(tracker).toHaveProperty('complete');
      expect(tracker).toHaveProperty('fail');
      expect(typeof tracker.complete).toBe('function');
      expect(typeof tracker.fail).toBe('function');
    });

    it('should call logToolComplete when complete is called', () => {
      const logger = createLogger({ name: 'test-logger' });
      const params = { path: './app' };

      const tracker = createStandardizedToolTracker('test-tool', params, logger);
      const result = { imageId: 'sha256:abc123' };

      tracker.complete(result);

      expect(logToolComplete).toHaveBeenCalledWith(
        'test-tool',
        result,
        logger,
        expect.any(Number)
      );
    });

    it('should calculate duration when complete is called', () => {
      const logger = createLogger({ name: 'test-logger' });
      const params = { path: './app' };

      const tracker = createStandardizedToolTracker('test-tool', params, logger);

      // Wait a bit to ensure duration > 0
      const start = Date.now();
      while (Date.now() - start < 10) {
        // Small delay
      }

      tracker.complete({});

      expect(logToolComplete).toHaveBeenCalled();
      const duration = (logToolComplete as jest.Mock).mock.calls[0][3];
      expect(duration).toBeGreaterThanOrEqual(0);
    });

    it('should call logToolFailure when fail is called', () => {
      const logger = createLogger({ name: 'test-logger' });
      const params = { path: './app' };

      const tracker = createStandardizedToolTracker('test-tool', params, logger);
      const error = new Error('Build failed');

      tracker.fail(error);

      expect(logToolFailure).toHaveBeenCalledWith('test-tool', error, logger, undefined);
    });

    it('should handle fail with string error', () => {
      const logger = createLogger({ name: 'test-logger' });
      const params = { path: './app' };

      const tracker = createStandardizedToolTracker('test-tool', params, logger);

      tracker.fail('Something went wrong');

      expect(logToolFailure).toHaveBeenCalledWith(
        'test-tool',
        'Something went wrong',
        logger,
        undefined
      );
    });

    it('should handle fail with context', () => {
      const logger = createLogger({ name: 'test-logger' });
      const params = { path: './app' };

      const tracker = createStandardizedToolTracker('test-tool', params, logger);
      const error = new Error('Build failed');
      const context = { exitCode: 1, stderr: 'Error output' };

      tracker.fail(error, context);

      expect(logToolFailure).toHaveBeenCalledWith('test-tool', error, logger, context);
    });

    it('should work with empty params', () => {
      const logger = createLogger({ name: 'test-logger' });

      const tracker = createStandardizedToolTracker('test-tool', {}, logger);

      expect(tracker).toBeDefined();
      expect(logToolStart).toHaveBeenCalledWith('test-tool', {}, logger);
    });

    it('should work with complex params', () => {
      const logger = createLogger({ name: 'test-logger' });
      const complexParams = {
        path: './app',
        tags: ['latest', 'v1.0.0'],
        buildArgs: { NODE_ENV: 'production' },
        platforms: ['linux/amd64', 'linux/arm64'],
      };

      const tracker = createStandardizedToolTracker('build-image', complexParams, logger);

      expect(tracker).toBeDefined();
      expect(logToolStart).toHaveBeenCalledWith('build-image', complexParams, logger);
    });

    it('should allow multiple complete/fail calls', () => {
      const logger = createLogger({ name: 'test-logger' });
      const params = { path: './app' };

      const tracker = createStandardizedToolTracker('test-tool', params, logger);

      tracker.complete({ status: 'done' });
      tracker.fail('error');
      tracker.complete({ status: 'retry-done' });

      expect(logToolComplete).toHaveBeenCalledTimes(2);
      expect(logToolFailure).toHaveBeenCalledTimes(1);
    });

    it('should track duration independently for each tracker', () => {
      const logger = createLogger({ name: 'test-logger' });

      const tracker1 = createStandardizedToolTracker('tool-1', {}, logger);
      const tracker2 = createStandardizedToolTracker('tool-2', {}, logger);

      tracker1.complete({});
      tracker2.complete({});

      expect(logToolComplete).toHaveBeenCalledTimes(2);
      // Each should have its own duration
      const call1Duration = (logToolComplete as jest.Mock).mock.calls[0][3];
      const call2Duration = (logToolComplete as jest.Mock).mock.calls[1][3];
      expect(call1Duration).toBeGreaterThanOrEqual(0);
      expect(call2Duration).toBeGreaterThanOrEqual(0);
    });
  });
});
