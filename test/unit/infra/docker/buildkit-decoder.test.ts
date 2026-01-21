/**
 * Unit tests for BuildKit trace decoder
 */

import { describe, it, expect, jest } from '@jest/globals';
import { decodeBuildKitTrace, formatBuildKitStatus } from '@/infra/docker/buildkit-decoder';
import type { Logger } from 'pino';
import protobuf from 'protobufjs';

describe('BuildKit decoder', () => {
  let mockLogger: Logger;

  beforeEach(() => {
    mockLogger = {
      debug: jest.fn(),
      info: jest.fn(),
      warn: jest.fn(),
      error: jest.fn(),
    } as unknown as Logger;
  });

  describe('formatBuildKitStatus', () => {
    it('should return null for empty status', () => {
      const result = formatBuildKitStatus({
        steps: [],
        logs: [],
        warnings: [],
        errors: [],
      });
      expect(result).toBeNull();
    });

    it('should prioritize errors over other messages', () => {
      const result = formatBuildKitStatus({
        steps: ['Step 1'],
        logs: ['Log message'],
        warnings: ['Warning message'],
        errors: ['Error message'],
      });
      expect(result).toBe('Error message');
    });

    it('should return completed steps when no errors', () => {
      const result = formatBuildKitStatus({
        steps: ['[1/3] FROM node:18', '[2/3] COPY package.json .'],
        logs: [],
        warnings: [],
        errors: [],
      });
      expect(result).toBe('[2/3] COPY package.json .');
    });

    it('should return logs when no errors or steps', () => {
      const result = formatBuildKitStatus({
        steps: [],
        logs: ['npm install started', 'npm install complete'],
        warnings: [],
        errors: [],
      });
      expect(result).toBe('npm install complete');
    });

    it('should return warnings with emoji prefix', () => {
      const result = formatBuildKitStatus({
        steps: [],
        logs: [],
        warnings: ['Deprecated package detected'],
        errors: [],
      });
      expect(result).toBe('⚠️  Deprecated package detected');
    });
  });

  describe('decodeBuildKitTrace', () => {
    it('should return null for invalid input', () => {
      const result = decodeBuildKitTrace(null, mockLogger);
      expect(result).toBeNull();
    });

    it('should return null for non-string input', () => {
      const result = decodeBuildKitTrace({ foo: 'bar' }, mockLogger);
      expect(result).toBeNull();
    });

    it('should return null for invalid base64', () => {
      const result = decodeBuildKitTrace('not-valid-base64!!!', mockLogger);
      expect(result).toBeNull();
    });

    it('should handle empty protobuf message', () => {
      const result = decodeBuildKitTrace('', mockLogger);
      expect(result).toBeNull();
    });
  });
});
