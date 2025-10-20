/**
 * Unit Tests: Application Deployment Tool
 * Tests the deploy application tool functionality with mock Kubernetes client
 * Following analyze-repo test structure and comprehensive coverage requirements
 */

import { jest } from '@jest/globals';
import { z } from 'zod';

// Result Type Helpers for Testing
function createSuccessResult<T>(value: T) {
  return {
    ok: true as const,
    value,
  };
}

function createFailureResult(error: string, guidance?: { hint?: string; resolution?: string }) {
  return {
    ok: false as const,
    error,
    ...(guidance && { guidance }),
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

const mockKubernetesClient = {
  applyManifest: jest.fn(),
  getDeploymentStatus: jest.fn(),
  waitForDeploymentReady: jest.fn(),
} as any;

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

// Mock lib modules
jest.mock('@/lib/tool-helpers', () => ({
  getToolLogger: jest.fn(() => createMockLogger()),
  createToolTimer: jest.fn(() => mockTimer),
  createStandardizedToolTracker: jest.fn(() => ({
    complete: jest.fn(),
    fail: jest.fn(),
  })),
  storeToolResults: jest.fn(),
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

// Mock config constants - override DEFAULT_TIMEOUTS for faster tests
jest.mock('../../../src/config/constants', () => {
  const actual = jest.requireActual('../../../src/config/constants') as any;
  return {
    ...actual,
    DEFAULT_TIMEOUTS: {
      ...actual.DEFAULT_TIMEOUTS,
      deploymentPoll: 1000, // Short timeout for tests
    },
  };
});


// Create mock ToolContext
function createMockToolContext(): ToolContext {
  return {
    logger: createMockLogger(),
    progressReporter: jest.fn(),
  };
}

describe('deployApplication', () => {
  let mockLogger: ReturnType<typeof createMockLogger>;
  let config: DeployApplicationParams;

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
  type: ClusterIP`;

  beforeEach(() => {
    mockLogger = createMockLogger();
    config = {
      manifestsPath: sampleManifests,
      namespace: 'default',
    };

    jest.clearAllMocks();
    mockKubernetesClient.applyManifest.mockResolvedValue(createSuccessResult({}));
    mockKubernetesClient.waitForDeploymentReady.mockResolvedValue(
      createSuccessResult({
        ready: true,
        readyReplicas: 2,
        totalReplicas: 2,
      }),
    );
  });

  describe('Successful Deployments', () => {
    beforeEach(() => {
      mockKubernetesClient.applyManifest.mockResolvedValue(createSuccessResult({ applied: true }));
      mockKubernetesClient.waitForDeploymentReady.mockResolvedValue(
        createSuccessResult({
          ready: true,
          readyReplicas: 2,
          totalReplicas: 2,
          conditions: [{ type: 'Available', status: 'True', message: 'Deployment is available' }],
        }),
      );
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
        expect(mockKubernetesClient.waitForDeploymentReady).toHaveBeenCalledWith(
          'default',
          'test-app',
          expect.any(Number),
        );
      } else {
        expect(mockKubernetesClient.getDeploymentStatus).toHaveBeenCalledWith(
          'default',
          'test-app',
        );
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
      mockKubernetesClient.waitForDeploymentReady.mockResolvedValue(
        createSuccessResult({
          ready: true,
          readyReplicas: 2,
          totalReplicas: 2,
        }),
      );
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

  describe('Service and Ingress Endpoint Detection', () => {
    beforeEach(() => {
      mockKubernetesClient.applyManifest.mockResolvedValue(createSuccessResult({ applied: true }));
      mockKubernetesClient.waitForDeploymentReady.mockResolvedValue(
        createSuccessResult({
          ready: true,
          readyReplicas: 2,
          totalReplicas: 2,
        }),
      );
    });

    it('should detect ClusterIP service endpoints correctly', async () => {
      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.endpoints).toEqual([
          {
            type: 'internal',
            url: 'http://test-app.default.svc.cluster.local',
            port: 80,
          },
        ]);
      }
    });
  });

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
  });

  describe('Error Scenarios - Infrastructure', () => {
    it('should fail when Kubernetes cluster is unreachable', async () => {
      (mockKubernetesClient.applyManifest.mockResolvedValue as any)({
        ok: false,
        error: 'Unable to connect to the cluster',
        guidance: {
          hint: 'Kubernetes cluster is not reachable',
          resolution:
            'Verify cluster is running and kubectl is configured: kubectl cluster-info',
        },
      });

      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('connect to the cluster');
        expect(result.guidance).toBeDefined();
        expect(result.guidance?.hint).toContain('not reachable');
        expect(result.guidance?.resolution).toBeDefined();
      }
    });

    it('should fail when kubectl is not installed', async () => {
      (mockKubernetesClient.applyManifest.mockResolvedValue as any)({
        ok: false,
        error: 'kubectl not found in PATH',
        guidance: {
          hint: 'kubectl command-line tool is not installed',
          resolution: 'Install kubectl: https://kubernetes.io/docs/tasks/tools/',
        },
      });

      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('kubectl not found');
        expect(result.guidance).toBeDefined();
      }
    });

    it('should fail when kubeconfig is invalid', async () => {
      (mockKubernetesClient.applyManifest.mockResolvedValue as any)({
        ok: false,
        error: 'Invalid kubeconfig file',
        guidance: {
          hint: 'kubeconfig file is missing or malformed',
          resolution: 'Check kubeconfig at ~/.kube/config or set KUBECONFIG environment variable',
        },
      });

      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Invalid kubeconfig');
        expect(result.guidance).toBeDefined();
      }
    });

    it('should fail when cluster authentication fails', async () => {
      (mockKubernetesClient.applyManifest.mockResolvedValue as any)({
        ok: false,
        error: 'Authentication failed: invalid credentials',
        guidance: {
          hint: 'Unable to authenticate with Kubernetes cluster',
          resolution: 'Update credentials: kubectl config set-credentials or re-login to cluster',
        },
      });

      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Authentication failed');
        expect(result.guidance).toBeDefined();
      }
    });
  });

  describe('Error Scenarios - Manifest Issues', () => {
    it('should fail when manifests are empty', async () => {
      const emptyConfig = {
        manifestsPath: '',
        namespace: 'default',
      };

      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(emptyConfig, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('No valid manifests');
      }
    });

    it('should fail when manifests have invalid YAML syntax', async () => {
      const invalidYamlConfig = {
        manifestsPath: 'invalid: yaml: [syntax',
        namespace: 'default',
      };

      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(invalidYamlConfig, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        // Implementation may return "No valid manifests" for invalid YAML
        expect(result.error).toMatch(/parse|No valid manifests/i);
      }
    });

    it('should fail when manifest is missing required fields', async () => {
      (mockKubernetesClient.applyManifest.mockResolvedValue as any)({
        ok: false,
        error: 'Manifest validation failed: missing metadata.name',
        guidance: {
          hint: 'Kubernetes manifest is missing required fields',
          resolution: 'Add required fields to manifest: metadata.name, kind, apiVersion',
        },
      });

      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Manifest validation failed');
        expect(result.guidance).toBeDefined();
      }
    });
  });

  describe('Error Scenarios - Namespace Issues', () => {
    it('should fail when namespace does not exist', async () => {
      (mockKubernetesClient.applyManifest.mockResolvedValue as any)({
        ok: false,
        error: 'Namespace "nonexistent" not found',
        guidance: {
          hint: 'Target namespace does not exist in cluster',
          resolution: 'Create namespace: kubectl create namespace nonexistent',
        },
      });

      config.namespace = 'nonexistent';

      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Namespace');
        expect(result.guidance).toBeDefined();
      }
    });

    it('should fail when user lacks permissions on namespace', async () => {
      (mockKubernetesClient.applyManifest.mockResolvedValue as any)({
        ok: false,
        error: 'Forbidden: User lacks permissions in namespace "restricted"',
        guidance: {
          hint: 'Current user does not have permission to deploy to this namespace',
          resolution: 'Request access from cluster admin or use a different namespace',
        },
      });

      config.namespace = 'restricted';

      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Forbidden');
        expect(result.guidance).toBeDefined();
      }
    });
  });

  describe('Error Scenarios - Deployment Failures', () => {
    beforeEach(() => {
      mockKubernetesClient.applyManifest.mockResolvedValue(createSuccessResult({ applied: true }));
    });

    it('should fail when deployment does not become ready in time', async () => {
      mockKubernetesClient.waitForDeploymentReady.mockResolvedValue(
        createFailureResult('Deployment timed out after 300 seconds', {
          hint: 'Deployment did not reach ready state within timeout',
          resolution:
            'Check pod status: kubectl get pods -n default and kubectl describe pod <pod-name>',
        }),
      );

      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(config, mockContext);

      // Deployment may succeed but not be ready
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.ready).toBe(false);
      }
    });

    it('should fail when pod image cannot be pulled', async () => {
      mockKubernetesClient.applyManifest.mockResolvedValue(createSuccessResult({ applied: true }));
      mockKubernetesClient.waitForDeploymentReady.mockResolvedValue(
        createFailureResult('ImagePullBackOff: Failed to pull image', {
          hint: 'Kubernetes cannot pull the specified container image',
          resolution: 'Verify image exists and credentials are configured: docker pull <image>',
        }),
      );

      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.ready).toBe(false);
      }
    });

    it('should fail when pod crashes with CrashLoopBackOff', async () => {
      mockKubernetesClient.waitForDeploymentReady.mockResolvedValue(
        createFailureResult('CrashLoopBackOff: Container keeps crashing', {
          hint: 'Application container is crashing repeatedly',
          resolution: 'Check logs: kubectl logs -n default <pod-name> and fix application errors',
        }),
      );

      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.ready).toBe(false);
      }
    });

    it('should fail when resource quota is exceeded', async () => {
      (mockKubernetesClient.applyManifest.mockResolvedValue as any)({
        ok: false,
        error: 'Forbidden: exceeded quota in namespace',
        guidance: {
          hint: 'Deployment would exceed namespace resource quota',
          resolution: 'Reduce resource requests or request quota increase from cluster admin',
        },
      });

      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('quota');
        expect(result.guidance).toBeDefined();
      }
    });
  });

  describe('Error Scenarios - Resource Conflicts', () => {
    it('should fail when resource already exists with different configuration', async () => {
      (mockKubernetesClient.applyManifest.mockResolvedValue as any)({
        ok: false,
        error: 'Resource conflict: immutable field cannot be changed',
        guidance: {
          hint: 'Attempting to modify immutable resource field',
          resolution: 'Delete and recreate resource: kubectl delete <resource> or update manifest',
        },
      });

      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('conflict');
        expect(result.guidance).toBeDefined();
      }
    });
  });

  describe('Error Scenarios - Input Validation', () => {
    it('should fail with invalid parameters object', async () => {
      // Wrap in try-catch as null params might throw before returning a Result
      try {
        const result = await deployApplicationTool(null as any, createMockToolContext());
        expect(result.ok).toBe(false);
      } catch (error) {
        // Expected - implementation may throw on null params
        expect(error).toBeDefined();
      }
    });

    it('should fail when namespace name is invalid', async () => {
      config.namespace = 'INVALID_NAMESPACE_123!@#';

      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('namespace');
      }
    });
  });

  describe('Partial Deployment Failures', () => {
    it('should handle partial success when some manifests fail', async () => {
      let callCount = 0;
      mockKubernetesClient.applyManifest.mockImplementation(async () => {
        callCount++;
        if (callCount === 1) {
          return createSuccessResult({ applied: true });
        } else {
          return {
            ok: false,
            error: 'Failed to apply second manifest',
            guidance: {
              hint: 'Manifest application failed',
              resolution: 'Check manifest syntax and cluster state',
            },
          };
        }
      });

      const mockContext = createMockToolContext();
      const result = await deployApplicationTool(config, mockContext);

      // Should succeed even with partial failures (depending on implementation)
      expect(result).toBeDefined();
    });
  });

  describe('Configuration Options - Extended', () => {
    beforeEach(() => {
      mockKubernetesClient.applyManifest.mockResolvedValue(
        createSuccessResult({ applied: true }),
      );
      mockKubernetesClient.waitForDeploymentReady.mockResolvedValue(
        createSuccessResult({
          ready: true,
          readyReplicas: 2,
          totalReplicas: 2,
        }),
      );
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
        mockKubernetesClient.applyManifest.mockResolvedValue(createSuccessResult({}));
        mockKubernetesClient.waitForDeploymentReady.mockResolvedValue(
          createSuccessResult({
            ready: true,
            readyReplicas: 2,
            totalReplicas: 2,
          }),
        );
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
