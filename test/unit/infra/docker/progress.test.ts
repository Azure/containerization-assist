/**
 * Unit tests for Docker build progress tracking
 */

import { describe, it, expect, jest, beforeEach } from '@jest/globals';
import { createProgressTracker, type ProgressCallback } from '@/infra/docker/progress';
import type { Logger } from 'pino';

describe('ProgressTracker', () => {
  let mockLogger: Logger;
  let mockProgressCallback: ProgressCallback;

  beforeEach(() => {
    mockLogger = {
      debug: jest.fn(),
      info: jest.fn(),
      warn: jest.fn(),
      error: jest.fn(),
    } as unknown as Logger;

    mockProgressCallback = jest.fn();
  });

  describe('constructor', () => {
    it('should create a progress tracker without onProgress callback', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
      });

      expect(tracker).toBeDefined();
    });

    it('should create a progress tracker with onProgress callback', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
        onProgress: mockProgressCallback,
      });

      expect(tracker).toBeDefined();
    });
  });

  describe('processBuildKitTrace', () => {
    it('should return empty string for invalid protobuf data', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
        onProgress: mockProgressCallback,
      });

      const fakeProtobuf = Buffer.from('fake').toString('base64');
      const result = tracker.processBuildKitTrace(fakeProtobuf);

      expect(result).toBe('');
    });

    it('should return empty string for null auxData', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
        onProgress: mockProgressCallback,
      });

      const result = tracker.processBuildKitTrace(null);

      expect(result).toBe('');
      expect(mockProgressCallback).not.toHaveBeenCalled();
    });

    it('should return empty string for undefined auxData', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
        onProgress: mockProgressCallback,
      });

      const result = tracker.processBuildKitTrace(undefined);

      expect(result).toBe('');
      expect(mockProgressCallback).not.toHaveBeenCalled();
    });

    it('should return empty string for non-string auxData', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
        onProgress: mockProgressCallback,
      });

      const result = tracker.processBuildKitTrace({ some: 'object' });

      expect(result).toBe('');
      expect(mockProgressCallback).not.toHaveBeenCalled();
    });

    it('should not call onProgress if no callback provided', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
      });

      const readableContent = '[1/6] FROM node:18';
      const encodedTrace = Buffer.from(readableContent).toString('base64');

      const extractedMessage = tracker.processBuildKitTrace(encodedTrace);

      expect(extractedMessage).toBe('');
      expect(mockProgressCallback).not.toHaveBeenCalled();
    });
  });
});
