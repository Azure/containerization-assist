/**
 * Unit tests for tool-context-helpers
 */

import { describe, it, expect, beforeEach, jest } from '@jest/globals';
import { setupToolContext, createToolExecutionContext } from '@/lib/tool-context-helpers';
import type { ToolContext } from '@/mcp/context';
import * as toolHelpers from '@/lib/tool-helpers';

// Mock the tool-helpers module
jest.mock('@/lib/tool-helpers', () => ({
  getToolLogger: jest.fn(),
  createToolTimer: jest.fn(),
}));

describe('tool-context-helpers', () => {
  let mockContext: ToolContext;
  let mockLogger: any;
  let mockTimer: any;

  beforeEach(() => {
    // Create mock logger
    mockLogger = {
      info: jest.fn(),
      error: jest.fn(),
      warn: jest.fn(),
      debug: jest.fn(),
      child: jest.fn(),
    };

    // Create mock timer
    mockTimer = {
      start: jest.fn(),
      end: jest.fn(),
      error: jest.fn(),
    };

    // Create mock context
    mockContext = {
      logger: mockLogger,
      signal: undefined,
      progress: undefined,
    };

    // Reset mocks
    jest.clearAllMocks();

    // Setup default mock implementations
    (toolHelpers.getToolLogger as jest.Mock).mockReturnValue(mockLogger);
    (toolHelpers.createToolTimer as jest.Mock).mockReturnValue(mockTimer);
  });

  describe('setupToolContext', () => {
    it('should return logger and timer', () => {
      const result = setupToolContext(mockContext, 'test-tool');

      expect(result).toHaveProperty('logger');
      expect(result).toHaveProperty('timer');
      expect(result.logger).toBe(mockLogger);
      expect(result.timer).toBe(mockTimer);
    });

    it('should call getToolLogger with correct arguments', () => {
      setupToolContext(mockContext, 'test-tool');

      expect(toolHelpers.getToolLogger).toHaveBeenCalledWith(mockContext, 'test-tool');
      expect(toolHelpers.getToolLogger).toHaveBeenCalledTimes(1);
    });

    it('should call createToolTimer with correct arguments', () => {
      setupToolContext(mockContext, 'test-tool');

      expect(toolHelpers.createToolTimer).toHaveBeenCalledWith(mockLogger, 'test-tool');
      expect(toolHelpers.createToolTimer).toHaveBeenCalledTimes(1);
    });

    it('should work with different tool names', () => {
      const tools = ['build-image', 'deploy', 'scan-image', 'analyze-repo'];

      tools.forEach((toolName) => {
        jest.clearAllMocks();
        const result = setupToolContext(mockContext, toolName);

        expect(toolHelpers.getToolLogger).toHaveBeenCalledWith(mockContext, toolName);
        expect(toolHelpers.createToolTimer).toHaveBeenCalledWith(mockLogger, toolName);
        expect(result.logger).toBe(mockLogger);
        expect(result.timer).toBe(mockTimer);
      });
    });

    it('should handle context without logger', () => {
      const contextWithoutLogger = {
        ...mockContext,
        logger: undefined as any,
      };

      const result = setupToolContext(contextWithoutLogger, 'test-tool');

      expect(toolHelpers.getToolLogger).toHaveBeenCalledWith(contextWithoutLogger, 'test-tool');
      expect(result.logger).toBe(mockLogger);
      expect(result.timer).toBe(mockTimer);
    });

    it('should return destructurable object', () => {
      const { logger, timer } = setupToolContext(mockContext, 'test-tool');

      expect(logger).toBe(mockLogger);
      expect(timer).toBe(mockTimer);
      expect(logger).toHaveProperty('info');
      expect(timer).toHaveProperty('start');
      expect(timer).toHaveProperty('end');
    });

    it('should create timer that can be used immediately', () => {
      const { logger, timer } = setupToolContext(mockContext, 'test-tool');

      timer.start();
      expect(mockTimer.start).toHaveBeenCalled();

      logger.info({ test: 'data' }, 'test message');
      expect(mockLogger.info).toHaveBeenCalledWith({ test: 'data' }, 'test message');

      timer.end();
      expect(mockTimer.end).toHaveBeenCalled();
    });
  });

  describe('createToolExecutionContext', () => {
    it('should return logger, timer, and context', () => {
      const result = createToolExecutionContext(mockContext, 'test-tool');

      expect(result).toHaveProperty('logger');
      expect(result).toHaveProperty('timer');
      expect(result).toHaveProperty('context');
      expect(result.logger).toBe(mockLogger);
      expect(result.timer).toBe(mockTimer);
      expect(result.context).toBe(mockContext);
    });

    it('should preserve original context', () => {
      const contextWithSignal = {
        ...mockContext,
        signal: new AbortController().signal,
        progress: jest.fn(),
      };

      const result = createToolExecutionContext(contextWithSignal, 'test-tool');

      expect(result.context).toBe(contextWithSignal);
      expect(result.context.signal).toBe(contextWithSignal.signal);
      expect(result.context.progress).toBe(contextWithSignal.progress);
    });

    it('should call setupToolContext internally', () => {
      const result = createToolExecutionContext(mockContext, 'test-tool');

      expect(toolHelpers.getToolLogger).toHaveBeenCalledWith(mockContext, 'test-tool');
      expect(toolHelpers.createToolTimer).toHaveBeenCalledWith(mockLogger, 'test-tool');
      expect(result.logger).toBe(mockLogger);
      expect(result.timer).toBe(mockTimer);
    });

    it('should work with different tool names', () => {
      const tools = ['prepare-cluster', 'verify-deploy', 'tag-image'];

      tools.forEach((toolName) => {
        jest.clearAllMocks();
        const result = createToolExecutionContext(mockContext, toolName);

        expect(toolHelpers.getToolLogger).toHaveBeenCalledWith(mockContext, toolName);
        expect(toolHelpers.createToolTimer).toHaveBeenCalledWith(mockLogger, toolName);
        expect(result.context).toBe(mockContext);
      });
    });

    it('should return destructurable object', () => {
      const { logger, timer, context } = createToolExecutionContext(mockContext, 'test-tool');

      expect(logger).toBe(mockLogger);
      expect(timer).toBe(mockTimer);
      expect(context).toBe(mockContext);
    });

    it('should allow access to context properties', () => {
      const mockProgress = jest.fn();
      const contextWithProgress = {
        ...mockContext,
        progress: mockProgress,
      };

      const { context } = createToolExecutionContext(contextWithProgress, 'test-tool');

      expect(context.progress).toBe(mockProgress);
      if (context.progress) {
        context.progress('test progress');
        expect(mockProgress).toHaveBeenCalledWith('test progress');
      }
    });
  });

  describe('Integration scenarios', () => {
    it('should support typical tool initialization pattern', () => {
      // Simulating a typical tool handler
      const { logger, timer } = setupToolContext(mockContext, 'example-tool');

      timer.start();
      logger.info('Starting tool execution');

      // Simulate some work
      logger.debug({ data: 'test' }, 'Processing data');

      timer.end();
      logger.info('Tool execution complete');

      expect(mockTimer.start).toHaveBeenCalled();
      expect(mockTimer.end).toHaveBeenCalled();
      expect(mockLogger.info).toHaveBeenCalledTimes(2);
      expect(mockLogger.debug).toHaveBeenCalledTimes(1);
    });

    it('should support error handling pattern', () => {
      const { logger, timer } = setupToolContext(mockContext, 'example-tool');

      timer.start();

      try {
        throw new Error('Test error');
      } catch (error) {
        timer.error(error);
        logger.error({ error }, 'Tool execution failed');
      }

      expect(mockTimer.error).toHaveBeenCalled();
      expect(mockLogger.error).toHaveBeenCalled();
    });

    it('should work with context that includes progress reporting', () => {
      const mockProgress = jest.fn();
      const contextWithProgress = {
        ...mockContext,
        progress: mockProgress,
      };

      const { logger, timer, context } = createToolExecutionContext(
        contextWithProgress,
        'example-tool',
      );

      timer.start();
      logger.info('Starting');

      if (context.progress) {
        context.progress('Step 1');
        context.progress('Step 2');
      }

      timer.end();

      expect(mockProgress).toHaveBeenCalledWith('Step 1');
      expect(mockProgress).toHaveBeenCalledWith('Step 2');
      expect(mockTimer.start).toHaveBeenCalled();
      expect(mockTimer.end).toHaveBeenCalled();
    });
  });

  describe('Type safety', () => {
    it('should return correctly typed logger', () => {
      const { logger } = setupToolContext(mockContext, 'test-tool');

      // These should all be valid method calls
      logger.info('test');
      logger.error('error');
      logger.warn('warning');
      logger.debug('debug');

      expect(mockLogger.info).toHaveBeenCalled();
      expect(mockLogger.error).toHaveBeenCalled();
      expect(mockLogger.warn).toHaveBeenCalled();
      expect(mockLogger.debug).toHaveBeenCalled();
    });

    it('should return correctly typed timer', () => {
      const { timer } = setupToolContext(mockContext, 'test-tool');

      // These should all be valid method calls
      timer.start();
      timer.end();
      timer.error(new Error('test'));

      expect(mockTimer.start).toHaveBeenCalled();
      expect(mockTimer.end).toHaveBeenCalled();
      expect(mockTimer.error).toHaveBeenCalled();
    });
  });
});
