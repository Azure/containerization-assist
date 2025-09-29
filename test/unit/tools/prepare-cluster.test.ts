/**
 * Unit Tests: Prepare Cluster Tool
 * Tests the prepare cluster tool functionality with mock Kubernetes client
 */

import { jest } from '@jest/globals';

// Result Type Helpers for Testing
function createSuccessResult<T>(value: T) {
  return {
    ok: true as const,
    value,
  };
}

function createMockLogger() {
  return {
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
    trace: jest.fn(),
    fatal: jest.fn(),
    child: jest.fn().mockReturnThis(),
  } as any;
}

// Mock lib modules
const mockSessionManager = {
  create: jest.fn().mockResolvedValue(createSuccessResult({
    sessionId: 'test-session-123',
    metadata: {},
    completed_steps: [],
    errors: {},
    current_step: null,
    createdAt: new Date('2025-09-08T11:12:40.362Z'),
    updatedAt: new Date('2025-09-08T11:12:40.362Z'),
  })),
  get: jest.fn().mockResolvedValue(createSuccessResult({
    sessionId: 'test-session-123',
    metadata: {},
    completed_steps: [],
    errors: {},
    current_step: null,
    createdAt: new Date('2025-09-08T11:12:40.362Z'),
    updatedAt: new Date('2025-09-08T11:12:40.362Z'),
  })),
  update: jest.fn().mockResolvedValue(createSuccessResult(true)),
};

const mockK8sClient = {
  ping: jest.fn(),
  namespaceExists: jest.fn(),
  applyManifest: jest.fn(),
  checkIngressController: jest.fn(),
  checkPermissions: jest.fn(),
};

const mockTimer = {
  end: jest.fn(),
  error: jest.fn(),
};

jest.mock('@/session/core', () => ({
  SessionManager: jest.fn(() => mockSessionManager),
}));

jest.mock('@/lib/kubernetes', () => ({
  createKubernetesClient: jest.fn(() => mockK8sClient),
}));

// Mock MCP helper modules

// Import these after mocks are set up
import { prepareCluster } from '../../../src/tools/prepare-cluster/tool';
import type { PrepareClusterParams } from '../../../src/tools/prepare-cluster/schema';

jest.mock('@/lib/logger', () => ({
  createTimer: jest.fn(() => mockTimer),
  createLogger: jest.fn(() => createMockLogger()),
}));

jest.mock('@/lib/tool-helpers', () => ({
  getToolLogger: jest.fn(() => createMockLogger()),
  createToolTimer: jest.fn(() => mockTimer),
}));

jest.mock('@/lib/error-utils', () => ({
  extractErrorMessage: jest.fn((error) => error.message || String(error)),
}));

jest.mock('@/lib/platform-utils', () => ({
  getSystemInfo: jest.fn(() => ({ platform: 'linux', arch: 'x64' })),
  getDownloadOS: jest.fn(() => 'linux'),
  getDownloadArch: jest.fn(() => 'amd64'),
}));

jest.mock('@/lib/file-utils', () => ({
  downloadFile: jest.fn(),
  makeExecutable: jest.fn(),
  createTempFile: jest.fn(),
  deleteTempFile: jest.fn(),
}));

jest.mock('@/mcp/ai/sampling-runner', () => ({
  sampleWithRerank: jest.fn(),
}));

jest.mock('@/ai/prompt-engine', () => ({
  buildMessages: jest.fn(),
}));

jest.mock('@/mcp/ai/message-converter', () => ({
  toMCPMessages: jest.fn(),
}));

jest.mock('node:child_process', () => ({
  exec: jest.fn(),
}));

// Create session facade mock
const mockSessionFacade = {
  id: 'test-session-123',
  get: jest.fn(),
  set: jest.fn(),
  pushStep: jest.fn(),
};

function createMockToolContext() {
  return {
    logger: createMockLogger(),
    sessionManager: mockSessionManager,
    session: mockSessionFacade,
  } as any;
}

describe('prepareCluster', () => {
  let config: PrepareClusterParams;

  beforeEach(() => {
    config = {
      sessionId: 'test-session-123',
      namespace: 'test-namespace',
      environment: 'production',
    };

    // Reset all mocks
    jest.clearAllMocks();
    mockSessionManager.update.mockResolvedValue(true);
  });

  describe('Successful cluster preparation', () => {
    beforeEach(() => {
      // Mock successful connectivity
      mockK8sClient.ping.mockResolvedValue(true);
      mockK8sClient.namespaceExists.mockResolvedValue(false);
      mockK8sClient.applyManifest.mockResolvedValue({ success: true });
      mockK8sClient.checkPermissions.mockResolvedValue(true);
      mockK8sClient.checkIngressController.mockResolvedValue(true);
    });

    it('should handle existing namespace', async () => {
      mockK8sClient.namespaceExists.mockResolvedValue(true);

      const mockContext = createMockToolContext();
      const result = await prepareCluster(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.checks.namespaceExists).toBe(true);
      }
      // Should not attempt to create namespace
      expect(mockK8sClient.applyManifest).not.toHaveBeenCalledWith(
        expect.objectContaining({ kind: 'Namespace' }),
        undefined,
      );
    });
  });

  describe('Error handling', () => {

    it('should return error when cluster is not reachable', async () => {
      mockK8sClient.ping.mockResolvedValue(false);

      const mockContext = createMockToolContext();
      const result = await prepareCluster(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Cannot connect to Kubernetes cluster');
      }
    });

    it('should return error when namespace creation fails', async () => {
      mockK8sClient.ping.mockResolvedValue(true);
      mockK8sClient.namespaceExists.mockResolvedValue(false);
      mockK8sClient.applyManifest.mockResolvedValue({
        success: false,
        error: 'Failed to create namespace',
      });

      const mockContext = createMockToolContext();
      const result = await prepareCluster(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Failed to create namespace');
      }
    });

    it('should handle Kubernetes client errors', async () => {
      mockK8sClient.ping.mockRejectedValue(new Error('Connection timeout'));

      const mockContext = createMockToolContext();
      const result = await prepareCluster(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Cannot connect to Kubernetes cluster');
      }
    });
  });

  describe('Optional features', () => {
    beforeEach(() => {
      mockK8sClient.ping.mockResolvedValue(true);
      mockK8sClient.namespaceExists.mockResolvedValue(true);
      mockK8sClient.checkPermissions.mockResolvedValue(true);
    });

    it('should setup RBAC when requested', async () => {
      mockK8sClient.applyManifest.mockResolvedValue({ success: true });

      // In production environment, RBAC is automatically setup
      const mockContext = createMockToolContext();
      const result = await prepareCluster(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.checks.rbacConfigured).toBe(true);
      }
    });

    it('should check ingress controller when requested', async () => {
      mockK8sClient.checkIngressController.mockResolvedValue(true);

      // In production, checkRequirements is true, so ingress is checked
      const mockContext = createMockToolContext();
      const result = await prepareCluster(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.checks.ingressController).toBe(true);
      }
    });
  });

  describe('Session management', () => {
    it('should use session context properly', async () => {
      mockK8sClient.ping.mockResolvedValue(true);
      mockK8sClient.namespaceExists.mockResolvedValue(true);
      mockK8sClient.checkPermissions.mockResolvedValue(true);

      const mockContext = createMockToolContext();
      await prepareCluster(config, mockContext);

      // Verify session facade methods were available
      expect(mockContext.session).toBeDefined();
      expect(mockContext.session.id).toBe('test-session-123');
    });
  });
});
