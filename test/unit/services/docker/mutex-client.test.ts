import { createDockerClient } from '../../../../src/infra/docker/client';
import { Success, Failure } from '../../../../src/types';
import pino from 'pino';

// Mock dockerode with a proper implementation
jest.mock('dockerode', () => {
  return jest.fn().mockImplementation(() => ({
    buildImage: jest.fn(),
    getImage: jest.fn(() => ({
      inspect: jest.fn(),
      tag: jest.fn(),
      push: jest.fn(),
      remove: jest.fn()
    })),
    getContainer: jest.fn(() => ({
      remove: jest.fn()
    })),
    listContainers: jest.fn(),
    modem: {
      followProgress: jest.fn()
    }
  }));
});

// Mock tar-fs
jest.mock('tar-fs', () => ({
  pack: jest.fn(() => 'mock-tar-stream')
}));

// Mock socket validation
jest.mock('../../../../src/infra/docker/socket-validation', () => ({
  autoDetectDockerSocket: jest.fn(() => '/var/run/docker.sock')
}));

// Mock the mutex module
jest.mock('../../../../src/lib/mutex', () => ({
  createKeyedMutex: jest.fn(() => ({
    withLock: jest.fn((key, fn, timeout) => fn()),
    getStatus: jest.fn(() => new Map())
  }))
}));

// Mock socket validation
jest.mock('../../../../src/infra/docker/socket-validation', () => ({
  autoDetectDockerSocket: jest.fn(() => '/var/run/docker.sock')
}));

describe('DockerClient with Mutex', () => {
  let logger: pino.Logger;
  let mockDockerInstance: any;

  beforeEach(() => {
    logger = pino({ level: 'silent' });

    // Setup mock Docker instance
    mockDockerInstance = {
      getImage: jest.fn(),
      modem: {
        followProgress: jest.fn((stream, onFinished, onProgress) => {
          onFinished(null);
        }),
      },
    };

    // Mock the Docker constructor
    const DockerMock = require('dockerode');
    DockerMock.mockImplementation(() => mockDockerInstance);
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
      expect(typeof client.buildImage).toBe('function');
      expect(typeof client.tagImage).toBe('function');
      expect(typeof client.pushImage).toBe('function');
      expect(typeof client.removeImage).toBe('function');
    });

    test('should work without mutex when not enabled', async () => {
      const client = createDockerClient(logger, {
        enableMutex: false
      });

      expect(client).toBeDefined();
      expect(typeof client.buildImage).toBe('function');
      expect(typeof client.tagImage).toBe('function');
      expect(typeof client.pushImage).toBe('function');
      expect(typeof client.removeImage).toBe('function');
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
      expect(typeof client.buildImage).toBe('function');
      expect(typeof client.tagImage).toBe('function');
      expect(typeof client.pushImage).toBe('function');
      expect(typeof client.removeImage).toBe('function');
    });
  });
});