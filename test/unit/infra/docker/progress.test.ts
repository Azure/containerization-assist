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

  describe('processStreamLog', () => {
    it('should call onProgress callback with stream log line', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
        onProgress: mockProgressCallback,
      });

      const logLine = 'Step 1/5 : FROM node:18';
      tracker.processStreamLog(logLine);

      expect(mockProgressCallback).toHaveBeenCalledWith(logLine);
      expect(mockProgressCallback).toHaveBeenCalledTimes(1);
    });

    it('should not call onProgress if callback is not provided', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
      });

      const logLine = 'Step 1/5 : FROM node:18';
      tracker.processStreamLog(logLine);

      // Should not throw and callback should not be called
      expect(mockProgressCallback).not.toHaveBeenCalled();
    });

    it('should not call onProgress for empty log lines', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
        onProgress: mockProgressCallback,
      });

      tracker.processStreamLog('');

      expect(mockProgressCallback).not.toHaveBeenCalled();
    });

    it('should handle multiple stream log lines', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
        onProgress: mockProgressCallback,
      });

      const logLines = [
        'Step 1/5 : FROM node:18',
        'Step 2/5 : WORKDIR /app',
        'Step 3/5 : COPY package.json .',
      ];

      logLines.forEach((line) => tracker.processStreamLog(line));

      expect(mockProgressCallback).toHaveBeenCalledTimes(3);
      expect(mockProgressCallback).toHaveBeenNthCalledWith(1, logLines[0]);
      expect(mockProgressCallback).toHaveBeenNthCalledWith(2, logLines[1]);
      expect(mockProgressCallback).toHaveBeenNthCalledWith(3, logLines[2]);
    });
  });

  describe('processBuildKitTrace', () => {
    it('should decode and extract BuildKit trace data', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
        onProgress: mockProgressCallback,
      });

      // Create a simple BuildKit trace with readable content
      // BuildKit traces are base64-encoded strings
      const readableContent = '[1/6] FROM docker.io/library/node:18';
      const encodedTrace = Buffer.from(readableContent).toString('base64');

      const extractedMessage = tracker.processBuildKitTrace(encodedTrace);

      expect(extractedMessage).toBe('[1/6] FROM docker.io/library/node:18');
      expect(mockProgressCallback).toHaveBeenCalledWith('[1/6] FROM docker.io/library/node:18');
      expect(mockLogger.debug).toHaveBeenCalled();
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

    it('should strip control characters from decoded trace', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
        onProgress: mockProgressCallback,
      });

      // Create content with control characters (using only ASCII range)
      const contentWithControlChars = '\x00\x01[2/6] WORKDIR /app\x7F';
      const encodedTrace = Buffer.from(contentWithControlChars, 'binary').toString('base64');

      const extractedMessage = tracker.processBuildKitTrace(encodedTrace);

      // Control characters should be stripped
      expect(extractedMessage).toBe('[2/6] WORKDIR /app');
      expect(mockProgressCallback).toHaveBeenCalledWith('[2/6] WORKDIR /app');
    });

    it('should clean up multiple spaces', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
        onProgress: mockProgressCallback,
      });

      const contentWithSpaces = '[3/6]   COPY    package.json    .';
      const encodedTrace = Buffer.from(contentWithSpaces).toString('base64');

      const extractedMessage = tracker.processBuildKitTrace(encodedTrace);

      expect(extractedMessage).toBe('[3/6] COPY package.json .');
    });

    it('should return empty string for trace with only control characters', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
        onProgress: mockProgressCallback,
      });

      // Use only ASCII control characters
      const onlyControlChars = '\x00\x01\x02\x7F';
      const encodedTrace = Buffer.from(onlyControlChars, 'binary').toString('base64');

      const result = tracker.processBuildKitTrace(encodedTrace);

      expect(result).toBe('');
      expect(mockProgressCallback).not.toHaveBeenCalled();
    });

    it('should return empty string for trace with only whitespace', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
        onProgress: mockProgressCallback,
      });

      const onlyWhitespace = '   \t\n  ';
      const encodedTrace = Buffer.from(onlyWhitespace).toString('base64');

      const result = tracker.processBuildKitTrace(encodedTrace);

      expect(result).toBe('');
      expect(mockProgressCallback).not.toHaveBeenCalled();
    });

    it('should handle BuildKit trace with RUN command', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
        onProgress: mockProgressCallback,
      });

      const runCommand = '[4/6] RUN npm install';
      const encodedTrace = Buffer.from(runCommand).toString('base64');

      const extractedMessage = tracker.processBuildKitTrace(encodedTrace);

      expect(extractedMessage).toBe('[4/6] RUN npm install');
      expect(mockProgressCallback).toHaveBeenCalledWith('[4/6] RUN npm install');
    });

    it('should handle BuildKit trace with COPY command', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
        onProgress: mockProgressCallback,
      });

      const copyCommand = '[5/6] COPY . .';
      const encodedTrace = Buffer.from(copyCommand).toString('base64');

      const extractedMessage = tracker.processBuildKitTrace(encodedTrace);

      expect(extractedMessage).toBe('[5/6] COPY . .');
      expect(mockProgressCallback).toHaveBeenCalledWith('[5/6] COPY . .');
    });

    it('should handle BuildKit trace with CMD command', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
        onProgress: mockProgressCallback,
      });

      const cmdCommand = '[6/6] CMD npm start';
      const encodedTrace = Buffer.from(cmdCommand).toString('base64');

      const extractedMessage = tracker.processBuildKitTrace(encodedTrace);

      expect(extractedMessage).toBe('[6/6] CMD npm start');
      expect(mockProgressCallback).toHaveBeenCalledWith('[6/6] CMD npm start');
    });

    it('should handle invalid base64 gracefully', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
        onProgress: mockProgressCallback,
      });

      // This will cause Buffer.from to throw or decode incorrectly
      const invalidBase64 = 'not-valid-base64!!!';

      const result = tracker.processBuildKitTrace(invalidBase64);

      // Should not throw and should log debug error
      expect(mockLogger.debug).toHaveBeenCalled();
      expect(mockProgressCallback).toHaveBeenCalled(); // May extract some garbled text
    });

    it('should not call onProgress if no callback provided', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
      });

      const readableContent = '[1/6] FROM node:18';
      const encodedTrace = Buffer.from(readableContent).toString('base64');

      const extractedMessage = tracker.processBuildKitTrace(encodedTrace);

      expect(extractedMessage).toBe('[1/6] FROM node:18');
      expect(mockProgressCallback).not.toHaveBeenCalled();
    });

    it('should handle complex BuildKit trace with metadata', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
        onProgress: mockProgressCallback,
      });

      // Simulate more realistic BuildKit output with some binary data
      const complexContent = 'some binary \x00\x01 [internal] load build context';
      const encodedTrace = Buffer.from(complexContent, 'binary').toString('base64');

      const extractedMessage = tracker.processBuildKitTrace(encodedTrace);

      // Should extract the readable part and strip binary
      expect(extractedMessage).toContain('[internal] load build context');
    });

    it('should preserve important build information', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
        onProgress: mockProgressCallback,
      });

      const messages = [
        '[1/6] FROM docker.io/library/node:18-alpine',
        '[2/6] WORKDIR /usr/src/app',
        '[3/6] COPY package*.json ./',
        '[4/6] RUN npm ci --only=production',
        '[5/6] COPY . .',
        '[6/6] CMD ["node", "server.js"]',
      ];

      messages.forEach((msg) => {
        const encoded = Buffer.from(msg).toString('base64');
        const result = tracker.processBuildKitTrace(encoded);
        expect(result).toBe(msg);
      });

      expect(mockProgressCallback).toHaveBeenCalledTimes(6);
    });
  });

  describe('integration with both stream and BuildKit', () => {
    it('should handle mixed stream logs and BuildKit traces', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
        onProgress: mockProgressCallback,
      });

      // Simulate a build with both types
      tracker.processStreamLog('Sending build context to Docker daemon');

      const buildKitTrace1 = Buffer.from('[1/3] FROM node:18').toString('base64');
      tracker.processBuildKitTrace(buildKitTrace1);

      tracker.processStreamLog('Step 2/3 : WORKDIR /app');

      const buildKitTrace2 = Buffer.from('[2/3] WORKDIR /app').toString('base64');
      tracker.processBuildKitTrace(buildKitTrace2);

      expect(mockProgressCallback).toHaveBeenCalledTimes(4);
    });
  });
});
