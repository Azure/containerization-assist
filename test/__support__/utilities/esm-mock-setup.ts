/**
 * ESM Mock Setup Utilities
 * Centralized ESM mocking patterns for Jest with ES modules
 */

import { jest } from '@jest/globals';
import { 
  createComprehensiveDockerMock,
  createComprehensiveK8sMock,
  createComprehensiveAIMock,
  createComprehensiveSessionMock,
  createMockDockerode,
  createMockLogger,
  createMockConfig,
  createMockTrivyScanner,
} from './mock-factories';

/**
 * Setup all infrastructure mocks for ESM modules
 * Call this at the top of test files before any imports
 */
export function setupESMMocks() {
  // Docker mocks
  jest.unstable_mockModule('dockerode', () => ({
    default: jest.fn().mockImplementation(() => createMockDockerode()),
  }));
  
  // Kubernetes mocks
  jest.unstable_mockModule('@kubernetes/client-node', () => createComprehensiveK8sMock());
  
  // File system mocks (commonly needed)
  jest.unstable_mockModule('node:fs', () => ({
    existsSync: jest.fn().mockReturnValue(true),
    readFileSync: jest.fn().mockReturnValue('mock file content'),
    writeFileSync: jest.fn(),
    promises: {
      readFile: jest.fn().mockResolvedValue('mock file content'),
      writeFile: jest.fn().mockResolvedValue(undefined),
      mkdir: jest.fn().mockResolvedValue(undefined),
      access: jest.fn().mockResolvedValue(undefined),
      stat: jest.fn().mockResolvedValue({ isDirectory: () => true, size: 1000 }),
    },
  }));
  
  // Child process mocks (for command execution)
  jest.unstable_mockModule('node:child_process', () => ({
    exec: jest.fn((cmd, callback) => callback(null, { stdout: 'mock output', stderr: '' })),
    execSync: jest.fn().mockReturnValue('mock output'),
    spawn: jest.fn().mockReturnValue({
      stdout: { on: jest.fn() },
      stderr: { on: jest.fn() },
      on: jest.fn((event, handler) => {
        if (event === 'close') handler(0);
      }),
    }),
  }));
}

/**
 * Setup Docker-specific mocks
 */
export function setupDockerMocks() {
  const dockerodeMock = createMockDockerode();
  
  jest.unstable_mockModule('dockerode', () => ({
    default: jest.fn().mockImplementation(() => dockerodeMock),
  }));
  
  jest.unstable_mockModule('tar-fs', () => ({
    pack: jest.fn().mockReturnValue({
      pipe: jest.fn(),
      on: jest.fn((event, handler) => {
        if (event === 'end') setTimeout(handler, 0);
      }),
    }),
  }));
  
  return { dockerodeMock };
}

/**
 * Setup Kubernetes-specific mocks
 */
export function setupKubernetesMocks() {
  const k8sMock = createComprehensiveK8sMock();
  
  jest.unstable_mockModule('@kubernetes/client-node', () => k8sMock);
  
  return { k8sMock };
}

/**
 * Setup AI Service mocks
 */
export function setupAIMocks() {
  const aiMock = createComprehensiveAIMock();
  
  jest.unstable_mockModule('../../src/infrastructure/ai-service', () => ({
    AIService: jest.fn().mockImplementation(() => aiMock),
    default: aiMock,
  }));
  
  return { aiMock };
}

/**
 * Setup Session mocks
 */
export function setupSessionMocks() {
  const sessionMock = createComprehensiveSessionMock();
  
  jest.unstable_mockModule('../../src/infrastructure/session-store', () => ({
    SessionStore: jest.fn().mockImplementation(() => sessionMock),
    default: sessionMock,
  }));
  
  return { sessionMock };
}

/**
 * Setup infrastructure layer mocks
 */
export function setupInfrastructureMocks() {
  const dockerMock = createMockDockerode();
  const k8sMock = createComprehensiveK8sMock();
  const trivyMock = createMockTrivyScanner();
  
  jest.unstable_mockModule('dockerode', () => ({
    default: jest.fn().mockImplementation(() => dockerMock),
  }));
  
  jest.unstable_mockModule('@kubernetes/client-node', () => k8sMock);
  
  jest.unstable_mockModule('../../../src/infrastructure/scanners/trivy-scanner', () => ({
    TrivyScanner: jest.fn().mockImplementation(() => trivyMock),
    default: trivyMock,
  }));
  
  return { dockerMock, k8sMock, trivyMock };
}

/**
 * Helper for dynamic imports after mocking
 * Use this to import modules after mocks have been set up
 */
export async function importWithMocks<T>(modulePath: string): Promise<T> {
  // Ensure mocks are set up before import
  if (!jest.isMockFunction(jest.fn())) {
    throw new Error('Jest mocks not initialized. Call setup functions before importing.');
  }
  return await import(modulePath);
}

/**
 * Create a mock context for tool handlers
 */
export function createMockToolContext() {
  const logger = createMockLogger();
  const config = createMockConfig();
  
  return {
    logger,
    config,
    services: {
      docker: createComprehensiveDockerMock(),
      kubernetes: createComprehensiveK8sMock(),
      ai: createComprehensiveAIMock(),
      session: createComprehensiveSessionMock(),
    },
    progress: {
      emit: jest.fn().mockResolvedValue(undefined),
    },
  };
}

/**
 * Reset all mocks between tests
 * Call this in beforeEach or afterEach
 */
export function resetAllMocks() {
  jest.clearAllMocks();
  jest.resetModules();
}

/**
 * Common test patterns for Result<T> types
 */
export const ResultMatchers = {
  toBeOk: (result: any) => {
    return {
      pass: result?.kind === 'ok',
      message: () => `Expected Result to be Ok, but was ${result?.kind}`,
    };
  },
  
  toBeFail: (result: any) => {
    return {
      pass: result?.kind === 'fail',
      message: () => `Expected Result to be Fail, but was ${result?.kind}`,
    };
  },
  
  toHaveError: (result: any, expectedError: string) => {
    const pass = result?.kind === 'fail' && result?.error?.includes(expectedError);
    return {
      pass,
      message: () => 
        pass 
          ? `Result has expected error: ${expectedError}`
          : `Expected error "${expectedError}", got: ${result?.error}`,
    };
  },
};

/**
 * Common beforeEach/afterEach hooks
 */
export const TestHooks = {
  beforeEach: () => {
    jest.clearAllMocks();
  },
  
  afterEach: () => {
    jest.resetModules();
  },
  
  afterAll: () => {
    jest.restoreAllMocks();
  },
};

/**
 * Mock response generators for common scenarios
 */
export const MockResponses = {
  dockerBuild: {
    success: () => ({
      aux: { ID: 'sha256:mock-build-id' },
      stream: 'Successfully built sha256:mock-build-id\n',
    }),
    
    failure: () => ({
      error: 'Build failed: Invalid Dockerfile',
      errorDetail: { message: 'Invalid Dockerfile' },
    }),
    
    progress: () => [
      { stream: 'Step 1/5 : FROM node:18-alpine\n' },
      { stream: ' ---> Using cache\n' },
      { stream: 'Step 2/5 : WORKDIR /app\n' },
      { stream: ' ---> Running in abc123\n' },
      { stream: 'Successfully built sha256:mock-build-id\n' },
      { aux: { ID: 'sha256:mock-build-id' } },
    ],
  },
  
  dockerPush: {
    success: () => ({
      status: 'Pushed',
      id: 'latest',
      progressDetail: {},
    }),
    
    failure: () => ({
      error: 'unauthorized: authentication required',
    }),
  },
  
  k8sDeploy: {
    success: () => ({
      kind: 'Status',
      apiVersion: 'v1',
      status: 'Success',
      message: 'deployment.apps/test-app created',
    }),
    
    failure: () => ({
      kind: 'Status',
      apiVersion: 'v1',
      status: 'Failure',
      message: 'deployments.apps "test-app" already exists',
      reason: 'AlreadyExists',
      code: 409,
    }),
  },
};

/**
 * Test data factories for consistent test data
 */
export const TestDataFactories = {
  dockerImage: (tag = 'test:latest') => ({
    Id: `sha256:${Math.random().toString(36).substring(2, 66)}`,
    RepoTags: [tag],
    Size: Math.floor(Math.random() * 100000000),
    Created: Date.now() / 1000,
  }),
  
  k8sPod: (name = 'test-pod') => ({
    metadata: { name, namespace: 'default', uid: `uid-${Date.now()}` },
    spec: { containers: [{ name: 'main', image: 'test:latest' }] },
    status: { phase: 'Running', containerStatuses: [{ ready: true }] },
  }),
  
  deployment: (name = 'test-deployment') => ({
    metadata: { name, namespace: 'default' },
    spec: {
      replicas: 2,
      selector: { matchLabels: { app: name } },
      template: {
        metadata: { labels: { app: name } },
        spec: { containers: [{ name: 'main', image: 'test:latest' }] },
      },
    },
    status: { readyReplicas: 2, replicas: 2 },
  }),
};

/**
 * Export helper types for TypeScript
 */
export type MockedDocker = ReturnType<typeof createMockDockerode>;
export type MockedK8s = ReturnType<typeof createComprehensiveK8sMock>;
export type MockedAI = ReturnType<typeof createComprehensiveAIMock>;
export type MockedSession = ReturnType<typeof createComprehensiveSessionMock>;

// Helper functions that need to be imported from mock-factories
function createComprehensiveDockerMock() {
  return createMockDockerode();
}

function createComprehensiveAIMock() {
  return {
    generateDockerfile: jest.fn().mockResolvedValue({
      content: 'FROM node:18-alpine\nWORKDIR /app\nCMD ["node", "index"]',
      reasoning: 'Generated based on Node.js application',
    }),
    analyzeRepository: jest.fn().mockResolvedValue({
      language: 'javascript',
      framework: 'express',
      dependencies: ['express', 'pino'],
    }),
    enhanceManifests: jest.fn().mockImplementation((manifests) => 
      Promise.resolve(manifests.map((m: any) => ({ ...m, enhanced: true })))
    ),
    generateStructured: jest.fn().mockResolvedValue({}),
    isAvailable: jest.fn().mockReturnValue(true),
    initialize: jest.fn().mockResolvedValue(undefined),
  };
}

function createComprehensiveSessionMock() {
  return {
    create: jest.fn().mockResolvedValue('session-123'),
    get: jest.fn().mockResolvedValue({
      id: 'session-123',
      state: 'active',
      data: {},
    }),
    update: jest.fn().mockResolvedValue(true),
    updateAtomic: jest.fn().mockResolvedValue(true),
    delete: jest.fn().mockResolvedValue(true),
    list: jest.fn().mockResolvedValue([]),
    initialize: jest.fn().mockResolvedValue(undefined),
  };
}