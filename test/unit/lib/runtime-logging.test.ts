/**
 * Unit tests for runtime logging helpers
 */

import { describe, it, expect, beforeEach, jest } from '@jest/globals';
import {
  logToolStart,
  logToolComplete,
  logToolFailure,
  LOG_FORMAT,
} from '@/lib/runtime-logging';
import { createStandardizedToolTracker } from '@/lib/tool-helpers';
import type { Logger } from 'pino';

describe('Runtime Logging - Tool Execution Helpers', () => {
  let mockLogger: Logger;
  let infoSpy: jest.Mock;
  let errorSpy: jest.Mock;

  beforeEach(() => {
    infoSpy = jest.fn();
    errorSpy = jest.fn();
    mockLogger = {
      info: infoSpy,
      error: errorSpy,
    } as unknown as Logger;
  });

  describe('LOG_FORMAT constants', () => {
    it('should define standard format constants', () => {
      expect(LOG_FORMAT.STARTING).toBe('starting');
      expect(LOG_FORMAT.COMPLETED).toBe('completed');
      expect(LOG_FORMAT.FAILED).toBe('failed');
    });
  });

  describe('logToolStart', () => {
    it('should log tool start with consistent format', () => {
      const params = { path: './app', tags: ['latest'] };

      logToolStart('build-image', params, mockLogger);

      expect(infoSpy).toHaveBeenCalledTimes(1);
      expect(infoSpy).toHaveBeenCalledWith(params, 'Starting build-image');
    });

    it('should work with different tool names', () => {
      logToolStart('generate-k8s-manifests', { namespace: 'default' }, mockLogger);

      expect(infoSpy).toHaveBeenCalledWith(
        { namespace: 'default' },
        'Starting generate-k8s-manifests'
      );
    });

    it('should handle empty parameters', () => {
      logToolStart('analyze-repo', {}, mockLogger);

      expect(infoSpy).toHaveBeenCalledWith({}, 'Starting analyze-repo');
    });
  });

  describe('logToolComplete', () => {
    it('should log tool completion with consistent format', () => {
      const result = { imageId: 'sha256:abc123' };

      logToolComplete('build-image', result, mockLogger);

      expect(infoSpy).toHaveBeenCalledTimes(1);
      expect(infoSpy).toHaveBeenCalledWith(result, 'Completed build-image');
    });

    it('should include duration when provided', () => {
      const result = { imageId: 'sha256:abc123' };
      const duration = 5000;

      logToolComplete('build-image', result, mockLogger, duration);

      expect(infoSpy).toHaveBeenCalledWith(
        { ...result, durationMs: duration },
        'Completed build-image'
      );
    });

    it('should not include duration when not provided', () => {
      const result = { imageId: 'sha256:abc123' };

      logToolComplete('build-image', result, mockLogger);

      const callArgs = infoSpy.mock.calls[0];
      expect(callArgs[0]).not.toHaveProperty('durationMs');
    });
  });

  describe('logToolFailure', () => {
    it('should log tool failure with error message', () => {
      const error = new Error('Docker daemon not running');

      logToolFailure('build-image', error, mockLogger);

      expect(errorSpy).toHaveBeenCalledTimes(1);
      expect(errorSpy).toHaveBeenCalledWith(
        { error: 'Docker daemon not running' },
        'Failed build-image'
      );
    });

    it('should handle string errors', () => {
      logToolFailure('build-image', 'Connection timeout', mockLogger);

      expect(errorSpy).toHaveBeenCalledWith(
        { error: 'Connection timeout' },
        'Failed build-image'
      );
    });

    it('should include context when provided', () => {
      const error = new Error('Build failed');
      const context = { path: './app', dockerfile: 'Dockerfile' };

      logToolFailure('build-image', error, mockLogger, context);

      expect(errorSpy).toHaveBeenCalledWith(
        { ...context, error: 'Build failed' },
        'Failed build-image'
      );
    });
  });

  describe('createStandardizedToolTracker', () => {
    it('should log start immediately when created', () => {
      createStandardizedToolTracker('build-image', { path: './app' }, mockLogger);

      expect(infoSpy).toHaveBeenCalledTimes(1);
      expect(infoSpy).toHaveBeenCalledWith({ path: './app' }, 'Starting build-image');
    });

    it('should provide complete method that logs with duration', () => {
      const tracker = createStandardizedToolTracker(
        'build-image',
        { path: './app' },
        mockLogger
      );

      // Clear the start log
      infoSpy.mockClear();

      tracker.complete({ imageId: 'sha256:abc123' });

      expect(infoSpy).toHaveBeenCalledTimes(1);
      const [logData, message] = infoSpy.mock.calls[0];
      expect(message).toBe('Completed build-image');
      expect(logData).toHaveProperty('imageId', 'sha256:abc123');
      expect(logData).toHaveProperty('durationMs');
      expect(typeof logData.durationMs).toBe('number');
      expect(logData.durationMs).toBeGreaterThanOrEqual(0);
    });

    it('should provide fail method that logs errors', () => {
      const tracker = createStandardizedToolTracker(
        'build-image',
        { path: './app' },
        mockLogger
      );

      const error = new Error('Build failed');
      tracker.fail(error);

      expect(errorSpy).toHaveBeenCalledTimes(1);
      expect(errorSpy).toHaveBeenCalledWith(
        { error: 'Build failed' },
        'Failed build-image'
      );
    });

    it('should support fail with additional context', () => {
      const tracker = createStandardizedToolTracker(
        'build-image',
        { path: './app' },
        mockLogger
      );

      tracker.fail('Docker daemon not running', { dockerfile: 'Dockerfile' });

      expect(errorSpy).toHaveBeenCalledWith(
        { dockerfile: 'Dockerfile', error: 'Docker daemon not running' },
        'Failed build-image'
      );
    });

    it('should calculate duration correctly', async () => {
      const tracker = createStandardizedToolTracker(
        'build-image',
        { path: './app' },
        mockLogger
      );

      // Clear the start log
      infoSpy.mockClear();

      // Wait a bit to ensure measurable duration
      await new Promise(resolve => setTimeout(resolve, 10));

      tracker.complete({ imageId: 'sha256:abc123' });

      const [logData] = infoSpy.mock.calls[0];
      expect(logData.durationMs).toBeGreaterThan(0);
    });
  });

  describe('Tool logging integration pattern', () => {
    it('should follow the standard tool execution pattern', () => {
      // Simulate a tool execution
      const toolName = 'build-image';
      const params = { path: './app', tags: ['latest'] };

      // Start tracking
      const tracker = createStandardizedToolTracker(toolName, params, mockLogger);

      // Verify start was logged
      expect(infoSpy).toHaveBeenCalledWith(params, `Starting ${toolName}`);

      // Clear logs
      infoSpy.mockClear();
      errorSpy.mockClear();

      // Simulate success
      const result = { imageId: 'sha256:abc123', buildTime: 5000 };
      tracker.complete(result);

      // Verify completion was logged
      expect(infoSpy).toHaveBeenCalledTimes(1);
      const [completionData, completionMessage] = infoSpy.mock.calls[0];
      expect(completionMessage).toBe(`Completed ${toolName}`);
      expect(completionData).toMatchObject(result);
      expect(completionData).toHaveProperty('durationMs');
    });

    it('should handle error pattern correctly', () => {
      const toolName = 'build-image';
      const params = { path: './app' };

      const tracker = createStandardizedToolTracker(toolName, params, mockLogger);

      // Clear start log
      infoSpy.mockClear();

      // Simulate error
      const error = new Error('Docker daemon not running');
      tracker.fail(error, { dockerfile: 'Dockerfile' });

      // Verify error was logged
      expect(errorSpy).toHaveBeenCalledTimes(1);
      expect(errorSpy).toHaveBeenCalledWith(
        { dockerfile: 'Dockerfile', error: 'Docker daemon not running' },
        `Failed ${toolName}`
      );
    });
  });

  describe('Message format consistency', () => {
    const tools = [
      'build-image',
      'push-image',
      'tag-image',
      'generate-k8s-manifests',
      'fix-dockerfile',
      'deploy',
      'verify-deploy',
      'prepare-cluster',
    ];

    tools.forEach(toolName => {
      it(`should use consistent format for ${toolName}`, () => {
        const tracker = createStandardizedToolTracker(toolName, {}, mockLogger);

        infoSpy.mockClear();
        tracker.complete({});

        const [, startMessage] = infoSpy.mock.calls[0];
        expect(startMessage).toBe(`Completed ${toolName}`);
        expect(startMessage).toMatch(/^Completed /);
      });
    });

    it('should ensure all start messages follow "Starting X" format', () => {
      tools.forEach(toolName => {
        infoSpy.mockClear();
        logToolStart(toolName, {}, mockLogger);

        const [, message] = infoSpy.mock.calls[0];
        expect(message).toBe(`Starting ${toolName}`);
        expect(message).toMatch(/^Starting /);
      });
    });

    it('should ensure all completion messages follow "Completed X" format', () => {
      tools.forEach(toolName => {
        infoSpy.mockClear();
        logToolComplete(toolName, {}, mockLogger);

        const [, message] = infoSpy.mock.calls[0];
        expect(message).toBe(`Completed ${toolName}`);
        expect(message).toMatch(/^Completed /);
      });
    });

    it('should ensure all failure messages follow "Failed X" format', () => {
      tools.forEach(toolName => {
        errorSpy.mockClear();
        logToolFailure(toolName, 'Test error', mockLogger);

        const [, message] = errorSpy.mock.calls[0];
        expect(message).toBe(`Failed ${toolName}`);
        expect(message).toMatch(/^Failed /);
      });
    });
  });
});