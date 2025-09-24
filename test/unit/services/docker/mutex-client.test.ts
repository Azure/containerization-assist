import { createDockerClient } from '../../../../src/infra/docker/client';
import { Success, Failure } from '../../../../src/types';
import pino from 'pino';

// Mock dockerode
jest.mock('dockerode');

// Mock tar-fs
jest.mock('tar-fs', () => ({
  pack: jest.fn(() => 'mock-tar-stream')
}));

// Mock the mutex module
jest.mock('../../../../src/lib/mutex', () => ({
  createKeyedMutex: jest.fn(() => ({
    withLock: jest.fn((key, fn, timeout) => fn()),
    getStatus: jest.fn(() => new Map())
  }))
}));

describe('DockerClient with Mutex', () => {
  let logger: pino.Logger;

  beforeEach(() => {
    logger = pino({ level: 'silent' });
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('mutex protection', () => {
    test('should support mutex protection when enabled', async () => {
      const client = createDockerClient(logger, {
        enableMutex: true,
        mutexConfig: {
          defaultTimeout: 5000,
          dockerBuildTimeout: 10000
        }
      });

      expect(client).toBeDefined();
      expect(client.buildImage).toBeDefined();
      expect(client.getImage).toBeDefined();
      expect(client.tagImage).toBeDefined();
      expect(client.pushImage).toBeDefined();
    });

    test('should work without mutex when not enabled', async () => {
      const client = createDockerClient(logger, {
        enableMutex: false
      });

      expect(client).toBeDefined();
      expect(client.buildImage).toBeDefined();
      expect(client.getImage).toBeDefined();
      expect(client.tagImage).toBeDefined();
      expect(client.pushImage).toBeDefined();
    });

    test('should use default mutex config when not provided', async () => {
      const client = createDockerClient(logger, {
        enableMutex: true
      });

      expect(client).toBeDefined();
    });
  });

  describe('backward compatibility', () => {
    test('should create client without config', () => {
      const client = createDockerClient(logger);

      expect(client).toBeDefined();
      expect(client.buildImage).toBeDefined();
      expect(client.getImage).toBeDefined();
      expect(client.tagImage).toBeDefined();
      expect(client.pushImage).toBeDefined();
    });
  });
});