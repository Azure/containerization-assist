/**
 * Unit tests for Docker build progress tracking
 */

import { describe, it, expect, jest, beforeEach } from '@jest/globals';
import { createProgressTracker, type ProgressCallback } from '@/infra/docker/progress';
import type { Logger } from 'pino';
import protobuf from 'protobufjs';

// Helper function to create valid BuildKit protobuf
function createValidBuildKitTrace(stepName: string): string {
  const Timestamp = new protobuf.Type('Timestamp')
    .add(new protobuf.Field('seconds', 1, 'int64'))
    .add(new protobuf.Field('nanos', 2, 'int32'));

  const Vertex = new protobuf.Type('Vertex')
    .add(new protobuf.Field('digest', 1, 'string'))
    .add(new protobuf.Field('name', 3, 'string'))
    .add(new protobuf.Field('started', 5, 'google.protobuf.Timestamp'))
    .add(new protobuf.Field('completed', 6, 'google.protobuf.Timestamp'));

  const VertexStatus = new protobuf.Type('VertexStatus')
    .add(new protobuf.Field('ID', 1, 'string'));

  const VertexLog = new protobuf.Type('VertexLog')
    .add(new protobuf.Field('vertex', 1, 'string'));

  const VertexWarning = new protobuf.Type('VertexWarning')
    .add(new protobuf.Field('vertex', 1, 'string'));

  const StatusResponse = new protobuf.Type('StatusResponse')
    .add(new protobuf.Field('vertexes', 1, 'Vertex', 'repeated'))
    .add(new protobuf.Field('statuses', 2, 'VertexStatus', 'repeated'))
    .add(new protobuf.Field('logs', 3, 'VertexLog', 'repeated'))
    .add(new protobuf.Field('warnings', 4, 'VertexWarning', 'repeated'));

  const googleProtobuf = new protobuf.Namespace('google.protobuf');
  googleProtobuf.add(Timestamp);

  const mobyBuildkit = new protobuf.Namespace('moby.buildkit.v1');
  mobyBuildkit.add(Vertex);
  mobyBuildkit.add(VertexStatus);
  mobyBuildkit.add(VertexLog);
  mobyBuildkit.add(VertexWarning);
  mobyBuildkit.add(StatusResponse);

  const root = new protobuf.Root();
  root.add(googleProtobuf);
  root.add(mobyBuildkit);

  const StatusResponseType = root.lookupType('moby.buildkit.v1.StatusResponse');
  const message = StatusResponseType.create({
    vertexes: [
      {
        digest: 'sha256:abc123',
        name: stepName,
        started: { seconds: 1000, nanos: 0 },
        completed: { seconds: 1010, nanos: 0 },
      },
    ],
  });

  const encoded = StatusResponseType.encode(message).finish();
  return Buffer.from(encoded).toString('base64');
}

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

    it('should successfully decode valid protobuf and call onProgress', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
        onProgress: mockProgressCallback,
      });

      const validTrace = createValidBuildKitTrace('[1/3] FROM node:18');
      const result = tracker.processBuildKitTrace(validTrace);

      expect(result).toBe('[1/3] FROM node:18');
      expect(mockProgressCallback).toHaveBeenCalledWith('[1/3] FROM node:18');
      expect(mockProgressCallback).toHaveBeenCalledTimes(1);
    });

    it('should successfully decode valid protobuf without callback', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
      });

      const validTrace = createValidBuildKitTrace('[1/3] FROM node:18');
      const result = tracker.processBuildKitTrace(validTrace);

      expect(result).toBe('[1/3] FROM node:18');
    });

    it('should filter out duplicate messages', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
        onProgress: mockProgressCallback,
      });

      const validTrace = createValidBuildKitTrace('[1/3] FROM node:18');

      // Process the same message twice
      const result1 = tracker.processBuildKitTrace(validTrace);
      const result2 = tracker.processBuildKitTrace(validTrace);

      expect(result1).toBe('[1/3] FROM node:18');
      expect(result2).toBe(''); // Second call returns empty because it's a duplicate
      expect(mockProgressCallback).toHaveBeenCalledTimes(1); // Callback only called once
    });

    it('should handle different messages sequentially', () => {
      const tracker = createProgressTracker({
        logger: mockLogger,
        onProgress: mockProgressCallback,
      });

      const trace1 = createValidBuildKitTrace('[1/3] FROM node:18');
      const trace2 = createValidBuildKitTrace('[2/3] COPY package.json .');

      const result1 = tracker.processBuildKitTrace(trace1);
      const result2 = tracker.processBuildKitTrace(trace2);

      expect(result1).toBe('[1/3] FROM node:18');
      expect(result2).toBe('[2/3] COPY package.json .');
      expect(mockProgressCallback).toHaveBeenCalledTimes(2);
      expect(mockProgressCallback).toHaveBeenNthCalledWith(1, '[1/3] FROM node:18');
      expect(mockProgressCallback).toHaveBeenNthCalledWith(2, '[2/3] COPY package.json .');
    });
  });
});
