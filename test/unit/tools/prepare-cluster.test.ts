/**
 * Unit Tests: Prepare Cluster Tool
 * Tests the prepare cluster tool functionality with mock Kubernetes client
 */

import { jest } from '@jest/globals';
import { createToolSessionHelpersMock } from '../../__support__/mocks/tool-session-helpers.mock';

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
  create: jest.fn().mockResolvedValue({
    sessionId: 'test-session-123',
    workflow_state: {},
    metadata: {},
    completed_steps: [],
    errors: {},
    current_step: null,
    createdAt: '2025-09-08T11:12:40.362Z',
    updatedAt: '2025-09-08T11:12:40.362Z'
  }),
  get: jest.fn(),
  update: jest.fn(),
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

jest.mock('@lib/session', () => ({
  createSessionManager: jest.fn(() => mockSessionManager),
}));

jest.mock('@lib/kubernetes', () => ({
  createKubernetesClient: jest.fn(() => mockK8sClient),
}));

// Mock MCP helper modules
jest.mock('../../../src/mcp/tool-session-helpers', () => createToolSessionHelpersMock());

// Import these after mocks are set up
import { prepareCluster } from '../../../src/tools/prepare-cluster/tool';
import type { PrepareClusterParams } from '../../../src/tools/prepare-cluster/schema';

// wrapTool mock removed - tool now uses direct implementation

jest.mock('@lib/logger', () => ({
  createTimer: jest.fn(() => mockTimer),
  createLogger: jest.fn(() => createMockLogger()),
}));

describe('prepareCluster', () => {
  let mockLogger: ReturnType<typeof createMockLogger>;
  let config: PrepareClusterParams;
  let mockGetSession: jest.Mock;
  let mockUpdateSession: jest.Mock;
  let mockEnsureSession: jest.Mock;
  let mockUseSessionSlice: jest.Mock;

  beforeEach(() => {
    mockLogger = createMockLogger();
    config = {
      sessionId: 'test-session-123',
      namespace: 'test-namespace',
      environment: 'production',
    };

    // Get mocked functions
    const sessionHelpers = require('@mcp/tool-session-helpers');
    mockGetSession = sessionHelpers.getSession = jest.fn();
    mockUpdateSession = sessionHelpers.updateSession = jest.fn();
    mockEnsureSession = sessionHelpers.ensureSession = jest.fn();
    mockUseSessionSlice = sessionHelpers.useSessionSlice = jest.fn();

    // Reset all mocks
    jest.clearAllMocks();
    mockSessionManager.update.mockResolvedValue(true);
    
    // Setup default session helper mocks
    mockGetSession.mockResolvedValue({
      ok: true,
      value: {
        id: 'test-session-123',
        state: {
          sessionId: 'test-session-123',
          workflow_state: {},
          metadata: {},
          completed_steps: [],
        },
        isNew: false,
      },
    });
    mockUpdateSession.mockResolvedValue({ ok: true });
    
    // Setup ensureSession mock
    mockEnsureSession.mockResolvedValue({
      ok: true,
      value: {
        id: 'test-session-123',
        state: {
          sessionId: 'test-session-123',
          metadata: {},
          completed_steps: [],
          errors: {},
          current_step: null,
          createdAt: new Date(),
          updatedAt: new Date(),
        },
      },
    });
    
    // Setup useSessionSlice mock
    mockUseSessionSlice.mockReturnValue({
      get: jest.fn().mockResolvedValue({ ok: true, value: null }),
      set: jest.fn().mockResolvedValue({ ok: true }),
      patch: jest.fn().mockResolvedValue({ ok: true }),
    });
  });

  describe('Successful cluster preparation', () => {
    beforeEach(() => {
      // Mock successful connectivity
      mockK8sClient.ping.mockResolvedValue(true);
      mockK8sClient.namespaceExists.mockResolvedValue(false);
      mockK8sClient.applyManifest.mockResolvedValue({ success: true });
      mockK8sClient.checkPermissions.mockResolvedValue(true);
      mockK8sClient.checkIngressController.mockResolvedValue(true);
      
      // Mock session
      mockSessionManager.get.mockResolvedValue({
        sessionId: 'test-session-123',
        workflow_state: {},
        metadata: {},
      });
    });


    it('should handle existing namespace', async () => {
      mockK8sClient.namespaceExists.mockResolvedValue(true);
      
      const mockContext = { 
        logger: mockLogger,
        sessionManager: mockSessionManager 
      } as any;
      const result = await prepareCluster(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.checks.namespaceExists).toBe(true);
      }
      // Should not attempt to create namespace
      expect(mockK8sClient.applyManifest).not.toHaveBeenCalledWith(
        expect.objectContaining({ kind: 'Namespace' }),
        undefined
      );
    });

  });

  describe('Error handling', () => {
    beforeEach(() => {
      mockSessionManager.get.mockResolvedValue({
        sessionId: 'test-session-123',
        workflow_state: {},
        metadata: {},
      });
    });

    it('should return error when cluster is not reachable', async () => {
      mockK8sClient.ping.mockResolvedValue(false);

      const mockContext = { 
        logger: mockLogger,
        sessionManager: mockSessionManager 
      } as any;
      const result = await prepareCluster(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Cannot connect to Kubernetes cluster');
      }
    });

    it('should return error when namespace creation fails', async () => {
      mockK8sClient.ping.mockResolvedValue(true);
      mockK8sClient.namespaceExists.mockResolvedValue(false);
      mockK8sClient.applyManifest.mockResolvedValue({ success: false, error: 'Failed to create namespace' });
      
      const mockContext = { 
        logger: mockLogger,
        sessionManager: mockSessionManager 
      } as any;
      const result = await prepareCluster(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Failed to create namespace');
      }
    });

    it('should handle Kubernetes client errors', async () => {
      mockK8sClient.ping.mockRejectedValue(new Error('Connection timeout'));

      const mockContext = { 
        logger: mockLogger,
        sessionManager: mockSessionManager 
      } as any;
      const result = await prepareCluster(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Cannot connect to Kubernetes cluster');
      }
    });
  });

  describe('Optional features', () => {
    beforeEach(() => {
      mockSessionManager.get.mockResolvedValue({
        sessionId: 'test-session-123',
        workflow_state: {},
        metadata: {},
      });
      mockK8sClient.ping.mockResolvedValue(true);
      mockK8sClient.namespaceExists.mockResolvedValue(true);
      mockK8sClient.checkPermissions.mockResolvedValue(true);
    });

    it('should setup RBAC when requested', async () => {
      mockK8sClient.applyManifest.mockResolvedValue({ success: true });
      
      // In production environment, RBAC is automatically setup
      const mockContext = { 
        logger: mockLogger,
        sessionManager: mockSessionManager 
      } as any;
      const result = await prepareCluster(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.checks.rbacConfigured).toBe(true);
      }
    });

    it('should check ingress controller when requested', async () => {
      mockK8sClient.checkIngressController.mockResolvedValue(true);
      
      // In production, checkRequirements is true, so ingress is checked
      const mockContext = { 
        logger: mockLogger,
        sessionManager: mockSessionManager 
      } as any;
      const result = await prepareCluster(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.checks.ingressController).toBe(true);
      }
    });
  });

  describe('Session management', () => {
    it('should create new session if not exists', async () => {
      // Mock getSession to indicate a new session was created
      mockGetSession.mockResolvedValue({
        ok: true,
        value: {
          id: 'test-session-123',
          state: {
            sessionId: 'test-session-123',
            workflow_state: {},
            metadata: {},
            completed_steps: [],
          },
          isNew: true,
        },
      });
      mockK8sClient.ping.mockResolvedValue(true);
      mockK8sClient.namespaceExists.mockResolvedValue(true);
      mockK8sClient.checkPermissions.mockResolvedValue(true);

      const mockContext = { 
        logger: mockLogger,
        sessionManager: mockSessionManager 
      } as any;
      await prepareCluster(config, mockContext);

      // Verify session was retrieved/created (using ensureSession now)
      expect(mockEnsureSession).toHaveBeenCalledWith(mockContext, 'test-session-123');
    });
  });
});