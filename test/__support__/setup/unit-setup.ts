import { jest } from '@jest/globals';
import { createMockInfrastructure } from '../utilities/mock-infrastructure';

// Global test timeout for unit tests
jest.setTimeout(10000);

// Mock Docker client for unit tests
function createMockDockerClient() {
  return {
    buildImage: jest.fn(),
    pushImage: jest.fn(),
    tagImage: jest.fn(),
    listImages: jest.fn(),
    removeImage: jest.fn(),
  };
}

// Mock Kubernetes client for unit tests
function createMockKubernetesClient() {
  return {
    applyManifest: jest.fn(),
    deleteManifest: jest.fn(),
    getNamespace: jest.fn(),
    createNamespace: jest.fn(),
    listPods: jest.fn(),
  };
}

// Mock external dependencies by default for unit tests
jest.mock('../../../src/lib/docker', () => ({
  DockerClient: jest.fn(),
  createDockerClient: jest.fn(() => createMockDockerClient()),
}));

jest.mock('../../../src/lib/kubernetes', () => ({
  KubernetesClient: jest.fn(),
  createKubernetesClient: jest.fn(() => createMockKubernetesClient()),
}));

jest.mock('../../../src/lib/session', () => ({
  SessionManager: jest.fn(),
  createSessionManager: jest.fn(() => ({
    get: jest.fn(async (id) => ({
      ok: true,
      value: {
        sessionId: id,
        metadata: {},
        completed_steps: [],
        errors: {},
        current_step: null,
        createdAt: new Date(),
        updatedAt: new Date(),
      },
    })),
    update: jest.fn(async (id, state) => ({
      ok: true,
      value: {
        sessionId: id,
        ...state,
        updatedAt: new Date(),
      },
    })),
    create: jest.fn(async (id) => ({
      ok: true,
      value: {
        sessionId: id || 'test-session-' + Math.random().toString(36).substr(2, 9),
        metadata: {},
        completed_steps: [],
        errors: {},
        current_step: null,
        createdAt: new Date(),
        updatedAt: new Date(),
      },
    })),
    delete: jest.fn(async () => ({ ok: true, value: undefined })),
    list: jest.fn(async () => ({ ok: true, value: [] })),
    cleanup: jest.fn(async () => ({ ok: true, value: undefined })),
    close: jest.fn(),
  })),
}));

jest.mock('../../../src/lib/logger', () => ({
  createLogger: jest.fn(() => ({
    info: jest.fn(),
    error: jest.fn(),
    warn: jest.fn(),
    debug: jest.fn(),
    child: jest.fn(() => ({
      info: jest.fn(),
      error: jest.fn(),
      warn: jest.fn(),
      debug: jest.fn(),
    })),
  })),
  createTimer: jest.fn(() => ({
    end: jest.fn(),
    error: jest.fn(),
    checkpoint: jest.fn(),
  })),
}));

// Global test utilities
(global as any).createTestInfrastructure = createMockInfrastructure;
(global as any).TEST_TIMEOUT = 10000;

// Console cleanup
const originalConsole = console;
beforeEach(() => {
  // Suppress console output in unit tests unless DEBUG is set
  if (!process.env.DEBUG && !process.env.JEST_DEBUG) {
    console.log = jest.fn();
    console.warn = jest.fn();
    console.error = jest.fn();
  }
});

afterEach(() => {
  if (!process.env.DEBUG) {
    console.log = originalConsole.log;
    console.warn = originalConsole.warn;
    console.error = originalConsole.error;
  }
  jest.clearAllMocks();
});

export {};
