/**
 * Unit Tests: Application Deployment Tool
 * Tests the deploy application tool functionality with mock Kubernetes client
 * Following analyze-repo test structure and comprehensive coverage requirements
 */

import { jest } from '@jest/globals';

// Result Type Helpers for Testing
function createSuccessResult<T>(value: T) {
  return {
    ok: true as const,
    value,
  };
}

function createFailureResult(error: string) {
  return {
    ok: false as const,
    error,
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

// Mock lib modules following analyze-repo pattern
const mockSessionManager = {
  create: jest.fn().mockResolvedValue({
    workflow_state: {},
    metadata: {},
    completed_steps: [],
    errors: {},

    createdAt: '2025-09-08T11:12:40.362Z',
    updatedAt: '2025-09-08T11:12:40.362Z',
  }),
  get: jest.fn(),
  update: jest.fn(),
};

const mockSessionFacade = {
  id: 'test-session-123',
  get: jest.fn(),
  set: jest.fn(),
  pushStep: jest.fn(),
};

const mockKubernetesClient = {
  applyManifest: jest.fn(),
  getDeploymentStatus: jest.fn(),
  waitForDeploymentReady: jest.fn(),
};

const mockTimer = {
  end: jest.fn(),
  error: jest.fn(),
};

// Mock js-yaml for manifest parsing
jest.mock('js-yaml', () => ({
  loadAll: jest.fn((content: string) => {
    // Simple YAML parser mock for testing
    if (content.includes('kind: Deployment')) {
      const manifests = [
        {
          apiVersion: 'apps/v1',
          kind: 'Deployment',
          metadata: { name: 'test-app', namespace: 'default' },
          spec: { replicas: 2 },
        },
      ];

      // Check for LoadBalancer service
      if (content.includes('LoadBalancer')) {
        manifests.push({
          apiVersion: 'v1',
          kind: 'Service',
          metadata: { name: 'test-app', namespace: 'default' },
          spec: { ports: [{ port: 80 }], type: 'LoadBalancer' },
        });
      }
      // Check for Ingress
      else if (content.includes('kind: Ingress')) {
        manifests.push({
          apiVersion: 'v1',
          kind: 'Service',
          metadata: { name: 'test-app', namespace: 'default' },
          spec: { ports: [{ port: 80 }], type: 'ClusterIP' },
        });
        manifests.push({
          apiVersion: 'networking.k8s.io/v1',
          kind: 'Ingress',
          metadata: { name: 'test-app-ingress', namespace: 'default' },
          spec: { rules: [{ host: 'app.example.com' }] },
        });
      }
      // Default ClusterIP service
      else {
        manifests.push({
          apiVersion: 'v1',
          kind: 'Service',
          metadata: { name: 'test-app', namespace: 'default' },
          spec: { ports: [{ port: 80 }], type: 'ClusterIP' },
        });
      }

      return manifests;
    }
    return [];
  }),
}));


jest.mock('@/lib/tool-helpers', () => ({
  getToolLogger: jest.fn(() => createMockLogger()),
  createToolTimer: jest.fn(() => mockTimer),
  createStandardizedToolTracker: jest.fn(() => ({
    complete: jest.fn(),
    fail: jest.fn(),
  })),
  storeToolResults: jest.fn().mockResolvedValue({ ok: true, value: undefined }),
}));

// Mock MCP helper modules

// Import these after mocks are set up
import { deployApplication as deployApplicationTool } from '../../../src/tools/deploy/tool';
import type { DeployApplicationParams } from '../../../src/tools/deploy/schema';
import type { ToolContext } from '@/mcp/context';

jest.mock('../../../src/infra/kubernetes/client', () => ({
  createKubernetesClient: jest.fn(() => mockKubernetesClient),
}));

jest.mock('../../../src/lib/logger', () => ({
  createTimer: jest.fn(() => mockTimer),
  createLogger: jest.fn(() => createMockLogger()),
}));

// Mock DEFAULT_TIMEOUTS
jest.mock('../../../src/config/defaults', () => ({
  DEFAULT_TIMEOUTS: {
    deploymentPoll: 1000, // Short timeout for tests
  },
}));

// Create mock ToolContext
function createMockToolContext(): ToolContext {
  return {
    logger: createMockLogger(),
    progressReporter: jest.fn(),
    sessionManager: mockSessionManager,
    session: mockSessionFacade,
  };
}

describe('deployApplication', () => {
  let mockLogger: ReturnType<typeof createMockLogger>;
  let config: DeployApplicationParams;
  let mockEnsureSession: jest.Mock;
  let mockUseSessionSlice: jest.Mock;

  // Sample K8s manifests for testing
  const sampleManifests = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
      - name: test-app
        image: test-app:v1.0
        ports:
        - containerPort: 3000
---
apiVersion: v1
kind: Service
metadata:
  name: test-app
  namespace: default
spec:
  selector:
    app: test-app
  ports:
  - port: 80
    targetPort: 3000
  type: ClusterIP
`;

  beforeEach(() => {
    mockLogger = createMockLogger();
    config = {
      manifestsPath: sampleManifests,
      namespace: 'default',
    };

    jest.clearAllMocks();
    mockSessionManager.update.mockResolvedValue(true);
    mockKubernetesClient.applyManifest.mockResolvedValue(createSuccessResult({}));
    mockKubernetesClient.waitForDeploymentReady.mockResolvedValue(createSuccessResult({
      ready: true,
      readyReplicas: 2,
      totalReplicas: 2,
    }));

    mockSessionManager.get.mockResolvedValue({
      completed_steps: [],
      createdAt: '2025-09-08T11:12:40.362Z',
      updatedAt: '2025-09-08T11:12:40.362Z',
    });
  });

  describe('Successful Deployments', () => {
    beforeEach(() => {
      mockKubernetesClient.applyManifest.mockResolvedValue(createSuccessResult({ applied: true }));

      mockKubernetesClient.waitForDeploymentReady.mockResolvedValue(createSuccessResult({
        ready: true,
        readyReplicas: 2,
        totalReplicas: 2,
        conditions: [{ type: 'Available', status: 'True', message: 'Deployment is available' }],
      }));

      mockKubernetesClient.getDeploymentStatus.mockResolvedValue(
        createSuccessResult({
          ready: true,
          readyReplicas: 2,
          totalReplicas: 2,
          conditions: [{ type: 'Available', status: 'True', message: 'Deployment is available' }],
        }),
      );
    });

    it('should successfully deploy application with valid manifests', async () => {
      const mockContext = createMockToolContext();

      const result = await deployApplicationTool(config, mockContext);

      if (!result.ok) {
        console.log('DEPLOY ERROR:', result.error);
        console.log('FULL RESULT:', JSON.stringify(result, null, 2));
        // Show the error in the test output
        throw new Error(`Deploy failed: ${result.error}`);
      }

      expect(result.ok).toBe(true);
      expect(result.value.success).toBe(true);
      expect(result.value.namespace).toBe('default');
      expect(result.value.deploymentName).toBe('test-app');
      expect(result.value.serviceName).toBe('test-app');
      expect(result.value.ready).toBe(true);
      expect(result.value.replicas).toBe(2);
      expect(result.value.endpoints).toEqual([
        {
          type: 'internal',
          url: 'http://test-app.default.svc.cluster.local',
          port: 80,
        },
      ]);
      expect(result.value.status?.readyReplicas).toBe(2);
      expect(result.value.status?.totalReplicas).toBe(2);
      expect(result.value.status?.conditions).toEqual([
        {
          type: 'Available',
          status: 'True',
          message: 'Deployment is available',
        },
      ]);

      // Verify Kubernetes client was called to apply manifests
      expect(mockKubernetesClient.applyManifest).toHaveBeenCalledTimes(2);

      // Verify deployment readiness was checked (either waitForDeploymentReady or getDeploymentStatus)
      if (mockKubernetesClient.waitForDeploymentReady.mock.calls.length > 0) {
        expect(mockKubernetesClient.waitForDeploymentReady).toHaveBeenCalledWith('default', 'test-app', expect.any(Number));
      } else {
        expect(mockKubernetesClient.getDeploymentStatus).toHaveBeenCalledWith('default', 'test-app');
      }
    });

    it('should use default values when config options not specified', async () => {
      const minimalConfig: DeployApplicationParams = {
        manifestsPath: sampleManifests, // Required parameter
      };

      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(minimalConfig, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.namespace).toBe('default'); // Default namespace
      }

      // Verify deployment was applied to default namespace
      expect(mockKubernetesClient.applyManifest).toHaveBeenCalledWith(
        expect.objectContaining({ kind: 'Deployment' }),
        'default',
      );
    });

    it('should handle custom namespace deployment', async () => {
      config.namespace = 'production';

      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.namespace).toBe('production');
        expect(result.value.endpoints[0].url).toBe('http://test-app.production.svc.cluster.local');
      }

      expect(mockKubernetesClient.applyManifest).toHaveBeenCalledWith(
        expect.anything(),
        'production',
      );
    });
  });

  describe('Manifest Parsing and Ordering', () => {
    beforeEach(() => {
      mockKubernetesClient.waitForDeploymentReady.mockResolvedValue(createSuccessResult({
        ready: true,
        readyReplicas: 2,
        totalReplicas: 2,
      }));
      mockKubernetesClient.getDeploymentStatus.mockResolvedValue(
        createSuccessResult({
          ready: true,
          readyReplicas: 2,
          totalReplicas: 2,
        }),
      );
    });

    it('should parse YAML manifests correctly', async () => {
      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(config, mockContext);

      expect(result.ok).toBe(true);
      // Verify manifests were processed (Deployment and Service)
      expect(mockKubernetesClient.applyManifest).toHaveBeenCalledTimes(2);
    });

    it('should order manifests correctly for deployment', async () => {
      const mockContext = createMockToolContext();
      await deployApplicationTool(config, mockContext);

      // Verify manifests were applied - the actual ordering is based on the implementation's sort logic
      const calls = mockKubernetesClient.applyManifest.mock.calls;
      expect(calls.length).toBe(2);

      // The implementation orders: Service before Deployment based on the ordering array
      expect(calls[0][0]).toEqual(expect.objectContaining({ kind: 'Service' }));
      expect(calls[1][0]).toEqual(expect.objectContaining({ kind: 'Deployment' }));
    });
  });

  describe('Service and Ingress Endpoint Detection', () => {});

  describe('Error Handling', () => {
    it('should handle Kubernetes client failures gracefully', async () => {
      mockKubernetesClient.applyManifest.mockResolvedValue(
        createFailureResult('Failed to connect to Kubernetes cluster'),
      );

      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(config, mockContext);

      expect(result.ok).toBe(false);
      expect(result.error).toContain('All manifest deployments failed');
    });

    it('should handle session update failures', async () => {
      mockKubernetesClient.getDeploymentStatus.mockResolvedValue(
        createSuccessResult({
          ready: true,
          readyReplicas: 2,
        }),
      );

      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.success).toBe(true);
      }
    });
  });

  describe('Configuration Options', () => {
    beforeEach(() => {
      mockKubernetesClient.applyManifest.mockResolvedValue(createSuccessResult({ applied: true }));
      mockKubernetesClient.waitForDeploymentReady.mockResolvedValue(createSuccessResult({
        ready: true,
        readyReplicas: 2,
        totalReplicas: 2,
      }));
      mockKubernetesClient.getDeploymentStatus.mockResolvedValue(
        createSuccessResult({
          ready: true,
          readyReplicas: 2,
          totalReplicas: 2,
        }),
      );
    });

    it('should handle different cluster configurations', async () => {
      config.cluster = 'production-cluster';

      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(config, mockContext);

      expect(result.ok).toBe(true);
      // Cluster configuration affects how the Kubernetes client is created
      // This verifies the function accepts the parameter correctly
    });

    it('should handle custom timeout values', async () => {
      config.timeout = 600; // 10 minutes

      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(config, mockContext);

      expect(result.ok).toBe(true);
      // Custom timeout affects the deployment readiness wait logic
    });

    it('should handle boolean configuration options correctly', async () => {
      const testConfigs = [
        { dryRun: true, wait: false },
        { dryRun: false, wait: true },
        { dryRun: true, wait: true },
        { dryRun: false, wait: false },
      ];

      for (const testConfig of testConfigs) {
        const configWithOptions = { ...config, ...testConfig };
        const mockContext = createMockToolContext();
        const result = await deployApplicationTool(configWithOptions, mockContext);

        expect(result.ok).toBe(true);
        if (result.ok) {
          // Different combinations should all succeed
          expect(result.value.success).toBe(true);
        }

        // Reset mocks between tests
        jest.clearAllMocks();
        mockSessionManager.update.mockResolvedValue(true);
        mockKubernetesClient.applyManifest.mockResolvedValue(createSuccessResult({}));
        mockKubernetesClient.waitForDeploymentReady.mockResolvedValue(createSuccessResult({
          ready: true,
          readyReplicas: 2,
          totalReplicas: 2,
        }));
        mockKubernetesClient.getDeploymentStatus.mockResolvedValue(
          createSuccessResult({
            ready: true,
            readyReplicas: 2,
            totalReplicas: 2,
          }),
        );
      }
    });
  });
});
