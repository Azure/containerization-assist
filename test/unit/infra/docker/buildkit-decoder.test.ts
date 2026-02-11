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

  // Helper function to create protobuf root with full schema
  function createProtobufRoot(): protobuf.Root {
    const Timestamp = new protobuf.Type('Timestamp')
      .add(new protobuf.Field('seconds', 1, 'int64'))
      .add(new protobuf.Field('nanos', 2, 'int32'));

    const Vertex = new protobuf.Type('Vertex')
      .add(new protobuf.Field('digest', 1, 'string'))
      .add(new protobuf.Field('inputs', 2, 'string', 'repeated'))
      .add(new protobuf.Field('name', 3, 'string'))
      .add(new protobuf.Field('cached', 4, 'bool'))
      .add(new protobuf.Field('started', 5, 'google.protobuf.Timestamp'))
      .add(new protobuf.Field('completed', 6, 'google.protobuf.Timestamp'))
      .add(new protobuf.Field('error', 7, 'string'));

    const VertexStatus = new protobuf.Type('VertexStatus')
      .add(new protobuf.Field('ID', 1, 'string'))
      .add(new protobuf.Field('vertex', 2, 'string'));

    const VertexLog = new protobuf.Type('VertexLog')
      .add(new protobuf.Field('vertex', 1, 'string'))
      .add(new protobuf.Field('timestamp', 2, 'google.protobuf.Timestamp'))
      .add(new protobuf.Field('stream', 3, 'int64'))
      .add(new protobuf.Field('msg', 4, 'bytes'));

    const VertexWarning = new protobuf.Type('VertexWarning')
      .add(new protobuf.Field('vertex', 1, 'string'))
      .add(new protobuf.Field('level', 2, 'int64'))
      .add(new protobuf.Field('short', 3, 'bytes'))
      .add(new protobuf.Field('detail', 4, 'bytes', 'repeated'))
      .add(new protobuf.Field('url', 5, 'string'));

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

    return root;
  }

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

    it('should decode valid protobuf with all data types', () => {
      const root = createProtobufRoot();
      const StatusResponseType = root.lookupType('moby.buildkit.v1.StatusResponse');

      const message = StatusResponseType.create({
        vertexes: [
          {
            digest: 'sha256:step1',
            name: '[1/3] FROM node:18',
            started: { seconds: 1000, nanos: 0 },
            completed: { seconds: 1010, nanos: 0 },
          },
          {
            digest: 'sha256:step2',
            name: '[2/3] COPY package.json .',
            started: { seconds: 1020, nanos: 0 },
            completed: { seconds: 1025, nanos: 0 },
          },
          {
            digest: 'sha256:error-step',
            name: '[3/3] RUN npm install',
            error: 'ENOENT: no such file or directory',
          },
        ],
        logs: [
          {
            vertex: 'sha256:step2',
            timestamp: { seconds: 1021, nanos: 0 },
            stream: 1,
            msg: Buffer.from('Copying files...'),
          },
        ],
        warnings: [
          {
            vertex: 'sha256:step1',
            level: 1,
            short: Buffer.from('Using latest tag'),
          },
        ],
      });

      const encoded = StatusResponseType.encode(message).finish();
      const base64 = Buffer.from(encoded).toString('base64');

      const result = decodeBuildKitTrace(base64, mockLogger);

      expect(result).not.toBeNull();
      // Verify steps extraction
      expect(result?.steps).toHaveLength(2);
      expect(result?.steps).toContain('[1/3] FROM node:18');
      expect(result?.steps).toContain('[2/3] COPY package.json .');
      // Verify logs extraction
      expect(result?.logs).toHaveLength(1);
      expect(result?.logs[0]).toBe('Copying files...');
      // Verify warnings extraction
      expect(result?.warnings).toHaveLength(1);
      expect(result?.warnings[0]).toBe('Using latest tag');
      // Verify errors extraction
      expect(result?.errors).toHaveLength(1);
      expect(result?.errors[0]).toBe('[3/3] RUN npm install: ENOENT: no such file or directory');
    });

    it('should filter out empty and whitespace-only log messages', () => {
      const root = createProtobufRoot();
      const StatusResponseType = root.lookupType('moby.buildkit.v1.StatusResponse');

      const message = StatusResponseType.create({
        logs: [
          {
            vertex: 'sha256:abc',
            timestamp: { seconds: 1000, nanos: 0 },
            stream: 1,
            msg: Buffer.from('Valid log message'),
          },
          {
            vertex: 'sha256:def',
            timestamp: { seconds: 1001, nanos: 0 },
            stream: 1,
            msg: Buffer.from('   '), // Whitespace only - should be filtered
          },
          {
            vertex: 'sha256:ghi',
            timestamp: { seconds: 1002, nanos: 0 },
            stream: 1,
            msg: Buffer.from(''), // Empty - should be filtered
          },
        ],
      });

      const encoded = StatusResponseType.encode(message).finish();
      const base64 = Buffer.from(encoded).toString('base64');

      const result = decodeBuildKitTrace(base64, mockLogger);

      expect(result).not.toBeNull();
      expect(result?.logs).toHaveLength(1);
      expect(result?.logs[0]).toBe('Valid log message');
    });

    it('should only include vertices with both started and completed timestamps', () => {
      const root = createProtobufRoot();
      const StatusResponseType = root.lookupType('moby.buildkit.v1.StatusResponse');

      const message = StatusResponseType.create({
        vertexes: [
          {
            digest: 'sha256:completed',
            name: '[1/3] FROM node:18',
            started: { seconds: 1000, nanos: 0 },
            completed: { seconds: 1010, nanos: 0 },
          },
          {
            digest: 'sha256:started-only',
            name: '[2/3] COPY package.json .',
            started: { seconds: 1020, nanos: 0 },
          },
          {
            digest: 'sha256:not-started',
            name: '[3/3] RUN npm install',
          },
        ],
      });

      const encoded = StatusResponseType.encode(message).finish();
      const base64 = Buffer.from(encoded).toString('base64');

      const result = decodeBuildKitTrace(base64, mockLogger);

      expect(result).not.toBeNull();
      expect(result?.steps).toHaveLength(1);
      expect(result?.steps[0]).toBe('[1/3] FROM node:18');
    });

    it('should handle warnings with non-Buffer short field', () => {
      const root = createProtobufRoot();
      const StatusResponseType = root.lookupType('moby.buildkit.v1.StatusResponse');

      const message = StatusResponseType.create({
        warnings: [
          {
            vertex: 'sha256:warn1',
            level: 1,
            short: Buffer.from('Warning as string'),
          },
        ],
      });

      const encoded = StatusResponseType.encode(message).finish();
      const base64 = Buffer.from(encoded).toString('base64');

      const result = decodeBuildKitTrace(base64, mockLogger);

      expect(result).not.toBeNull();
      expect(result?.warnings).toHaveLength(1);
      expect(typeof result?.warnings[0]).toBe('string');
      expect(result?.warnings[0]).toContain('Warning');
    });

    it('should handle logs with various msg encodings', () => {
      const root = createProtobufRoot();
      const StatusResponseType = root.lookupType('moby.buildkit.v1.StatusResponse');

      const message = StatusResponseType.create({
        logs: [
          {
            vertex: 'sha256:log1',
            timestamp: { seconds: 1000, nanos: 0 },
            stream: 1,
            msg: Buffer.from('String log message'),
          },
        ],
      });

      const encoded = StatusResponseType.encode(message).finish();
      const base64 = Buffer.from(encoded).toString('base64');

      const result = decodeBuildKitTrace(base64, mockLogger);

      expect(result).not.toBeNull();
      expect(result?.logs).toHaveLength(1);
      expect(typeof result?.logs[0]).toBe('string');
      expect(result?.logs[0]).toContain('String log');
    });
  });
});
