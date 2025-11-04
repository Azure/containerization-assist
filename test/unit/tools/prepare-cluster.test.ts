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
const mockK8sClient = {
  ping: jest.fn(),
  namespaceExists: jest.fn(),
  ensureNamespace: jest.fn(),
  applyManifest: jest.fn(),
  checkIngressController: jest.fn(),
  checkPermissions: jest.fn(),
};

const mockTimer = {
  end: jest.fn(),
  error: jest.fn(),
};


jest.mock('@/infra/kubernetes/client', () => ({
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
  createStandardizedToolTracker: jest.fn(() => ({
    complete: jest.fn(),
    fail: jest.fn(),
  })),
}));

jest.mock('@/lib/errors', () => ({
  extractErrorMessage: jest.fn((error) => error.message || String(error)),
  ERROR_MESSAGES: {},
}));

jest.mock('@/lib/platform', () => ({
  getSystemInfo: jest.fn(() => ({ isWindows: false, isMac: false, isLinux: true })),
  getDownloadOS: jest.fn(() => 'linux'),
  getDownloadArch: jest.fn(() => 'amd64'),
  mapNodeArchToPlatform: jest.fn(() => 'linux/amd64'),
  isPlatformCompatible: jest.fn(() => true),
}));

jest.mock('@/lib/file-utils', () => ({
  downloadFile: jest.fn(),
  makeExecutable: jest.fn(),
  createTempFile: jest.fn(() => Promise.resolve('/tmp/test-config.yaml')),
  deleteTempFile: jest.fn(),
}));

jest.mock('@/lib/port-utils', () => ({
  findRegistryPort: jest.fn(() => Promise.resolve(6000)),
  isPortAvailable: jest.fn(() => Promise.resolve(true)),
}));

jest.mock('node:child_process', () => ({
  exec: jest.fn((cmd, callback) => {
    // Mock kubectl commands for platform detection
    if (callback) {
      callback(null, { stdout: 'amd64', stderr: '' }, '');
    }
  }),
}));

// Use closure pattern to store mock reference
jest.mock('node:util', () => {
  let execAsyncMock: any = null;

  return {
    promisify: jest.fn(() => {
      // Create mock on first call
      if (!execAsyncMock) {
        execAsyncMock = jest.fn(async (cmd: string) => {
          if (typeof cmd === 'string') {
            if (cmd.includes('kubectl get nodes') && cmd.includes('architecture')) {
              return { stdout: 'amd64', stderr: '' };
            }
            if (cmd.includes('kubectl get nodes') && cmd.includes('operatingSystem')) {
              return { stdout: 'linux', stderr: '' };
            }
            if (cmd.includes('kind get clusters')) {
              return { stdout: '', stderr: '' };
            }
            if (cmd.includes('kind version')) {
              return { stdout: 'kind v0.20.0 go1.20.5 linux/amd64', stderr: '' };
            }
            if (cmd.includes('docker ps') && cmd.includes('ca-registry')) {
              return { stdout: '', stderr: '' };
            }
            if (cmd.includes('docker network ls')) {
              return { stdout: 'kind\n', stderr: '' };
            }
            if (cmd.includes('kubectl get nodes --no-headers')) {
              return { stdout: 'node1   Ready   control-plane   1m   v1.27.3\n', stderr: '' };
            }
            if (cmd.includes('kubectl run') && cmd.includes('nslookup')) {
              return { stdout: 'Address 1: 172.18.0.3 ca-registry\nDNS_SUCCESS', stderr: '' };
            }
            if (cmd.includes('kubectl run') && cmd.includes('curl')) {
              return { stdout: 'success', stderr: '' };
            }
            // Handle curl health checks - MUST come after kubectl run checks to avoid false matches
            if (cmd.includes('curl') && cmd.includes('/v2/')) {
              return { stdout: '{}', stderr: '' };
            }
            // Handle kubectl delete pod for test cleanup
            if (cmd.includes('kubectl delete pod')) {
              return { stdout: '', stderr: '' };
            }
            // Handle docker network connect
            if (cmd.includes('docker network connect')) {
              return { stdout: '', stderr: '' };
            }
            if (cmd.includes('docker exec') && cmd.includes('config.toml')) {
              return {
                stdout: `
[plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:6000"]
  endpoint = ["http://ca-registry:5000"]
`,
                stderr: '',
              };
            }
            if (cmd.includes('docker inspect ca-registry')) {
              if (cmd.includes('State.Status')) {
                return { stdout: 'running', stderr: '' };
              }
              if (cmd.includes('NetworkSettings.Networks')) {
                return { stdout: 'bridge kind', stderr: '' };
              }
              if (cmd.includes('NetworkSettings.Ports')) {
                return { stdout: '6000', stderr: '' };
              }
            }
          }
          return { stdout: '', stderr: '' };
        });
        // Store reference globally so tests can access it
        (global as any).mockExecAsync = execAsyncMock;
      }
      return execAsyncMock;
    }),
  };
});

function createMockToolContext() {
  return {
    logger: createMockLogger(),
  } as any;
}

describe('prepareCluster', () => {
  let config: PrepareClusterParams;

  beforeEach(() => {
    config = {
      namespace: 'test-namespace',
      environment: 'production',
      targetPlatform: 'linux/amd64',
    };

    // Reset all mocks
    jest.clearAllMocks();
    mockK8sClient.ensureNamespace.mockResolvedValue({ success: true });
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
      mockK8sClient.ensureNamespace.mockResolvedValue({
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

  describe('Development environment with local registry', () => {
    let devConfig: PrepareClusterParams;

    beforeEach(() => {
      devConfig = {
        namespace: 'default',
        environment: 'development',
        targetPlatform: 'linux/amd64',
      };

      // Mock successful cluster operations
      mockK8sClient.ping.mockResolvedValue(true);
      mockK8sClient.namespaceExists.mockResolvedValue(true);
      mockK8sClient.checkPermissions.mockResolvedValue(true);
      mockK8sClient.applyManifest.mockResolvedValue({ ok: true });

      // Reset execAsync mock to default behavior
      (global as any).mockExecAsync.mockImplementation(async (cmd: string) => {
        if (cmd.includes('kubectl get nodes') && cmd.includes('architecture')) {
          return { stdout: 'amd64', stderr: '' };
        }
        if (cmd.includes('kubectl get nodes') && cmd.includes('operatingSystem')) {
          return { stdout: 'linux', stderr: '' };
        }
        if (cmd.includes('kind get clusters')) {
          return { stdout: 'containerization-assist\n', stderr: '' };
        }
        if (cmd.includes('kind version')) {
          return { stdout: 'kind v0.20.0 go1.20.5 linux/amd64', stderr: '' };
        }
        if (cmd.includes('docker ps -a') && cmd.includes('ca-registry')) {
          return { stdout: 'ca-registry\n', stderr: '' };
        }
        if (cmd.includes('docker ps') && cmd.includes('ca-registry') && !cmd.includes('-a')) {
          return { stdout: 'ca-registry\n', stderr: '' };
        }
        if (cmd.includes('docker network ls')) {
          return { stdout: 'kind\n', stderr: '' };
        }
        if (cmd.includes('kubectl get nodes --no-headers')) {
          return { stdout: 'node1   Ready   control-plane   1m   v1.27.3\n', stderr: '' };
        }
        // Handle DNS resolution test (matches registry-dns-test-<timestamp>)
        if (cmd.includes('kubectl run') && cmd.includes('nslookup')) {
          return { stdout: 'Address 1: 172.18.0.3 ca-registry\nDNS_SUCCESS', stderr: '' };
        }
        // Handle registry reachability test with curl (matches registry-test-<timestamp>)
        if (cmd.includes('kubectl run') && cmd.includes('curl')) {
          return { stdout: 'success', stderr: '' };
        }
        // Handle curl health checks - MUST come after kubectl run checks to avoid false matches
        if (cmd.includes('curl') && cmd.includes('/v2/')) {
          return { stdout: '{}', stderr: '' };
        }
        // Handle kubectl delete pod for test cleanup
        if (cmd.includes('kubectl delete pod')) {
          return { stdout: '', stderr: '' };
        }
        // Handle docker network connect
        if (cmd.includes('docker network connect')) {
          return { stdout: '', stderr: '' };
        }
        if (cmd.includes('docker exec') && cmd.includes('config.toml')) {
          return {
            stdout: `
[plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:6000"]
  endpoint = ["http://ca-registry:5000"]
`,
            stderr: '',
          };
        }
        if (cmd.includes('docker inspect ca-registry')) {
          if (cmd.includes('State.Status')) {
            return { stdout: 'running', stderr: '' };
          }
          if (cmd.includes('NetworkSettings.Networks')) {
            return { stdout: 'bridge kind', stderr: '' };
          }
          if (cmd.includes('NetworkSettings.Ports')) {
            return { stdout: '6000', stderr: '' };
          }
        }
        if (cmd.includes('kind export kubeconfig')) {
          return { stdout: '', stderr: '' };
        }
        return { stdout: '', stderr: '' };
      });
    });

    it('should setup kind cluster and local registry in development environment', async () => {
      const mockContext = createMockToolContext();
      const result = await prepareCluster(devConfig, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.checks.kindInstalled).toBe(true);
        expect(result.value.checks.kindClusterCreated).toBe(true);
        expect(result.value.checks.localRegistryCreated).toBe(true);
        expect(result.value.localRegistryUrl).toBe('localhost:6000');
        expect(result.value.localRegistry).toBeDefined();
        if (result.value.localRegistry) {
          expect(result.value.localRegistry.externalUrl).toBe('localhost:6000');
          expect(result.value.localRegistry.internalEndpoint).toBe('ca-registry:5000');
          expect(result.value.localRegistry.containerName).toBe('ca-registry');
          expect(result.value.localRegistry.healthy).toBe(true);
          expect(result.value.localRegistry.reachableFromCluster).toBe(true);
        }
      }
    });

    it('should validate registry DNS resolution', async () => {
      const mockContext = createMockToolContext();
      const result = await prepareCluster(devConfig, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        // Should not have DNS warnings when DNS resolution succeeds
        expect(result.value.warnings?.some(w => w.includes('DNS resolution failed'))).toBeFalsy();
      }
    });

    it('should warn when registry DNS resolution fails', async () => {
      // Mock DNS resolution failure
      (global as any).mockExecAsync.mockImplementation(async (cmd: string) => {
        if (cmd.includes('kubectl run') && cmd.includes('nslookup')) {
          return { stdout: 'DNS_FAILED', stderr: '' };
        }
        // Keep other mocks working
        if (cmd.includes('kubectl get nodes') && cmd.includes('architecture')) {
          return { stdout: 'amd64', stderr: '' };
        }
        if (cmd.includes('kind get clusters')) {
          return { stdout: 'containerization-assist\n', stderr: '' };
        }
        if (cmd.includes('docker ps') && cmd.includes('ca-registry')) {
          return { stdout: 'ca-registry\n', stderr: '' };
        }
        if (cmd.includes('kubectl run') && cmd.includes('curl')) {
          return { stdout: 'success', stderr: '' };
        }
        // Handle curl health checks - MUST come after kubectl run checks to avoid false matches
        if (cmd.includes('curl') && cmd.includes('/v2/')) {
          return { stdout: '{}', stderr: '' };
        }
        // Handle kubectl delete pod for test cleanup
        if (cmd.includes('kubectl delete pod')) {
          return { stdout: '', stderr: '' };
        }
        // Handle docker network connect
        if (cmd.includes('docker network connect')) {
          return { stdout: '', stderr: '' };
        }
        if (cmd.includes('docker inspect ca-registry') && cmd.includes('State.Status')) {
          return { stdout: 'running', stderr: '' };
        }
        if (cmd.includes('docker network ls')) {
          return { stdout: 'kind\n', stderr: '' };
        }
        if (cmd.includes('docker exec') && cmd.includes('config.toml')) {
          return {
            stdout: `
[plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:6000"]
  endpoint = ["http://ca-registry:5000"]
`,
            stderr: '',
          };
        }
        return { stdout: '', stderr: '' };
      });

      const mockContext = createMockToolContext();
      const result = await prepareCluster(devConfig, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.warnings).toBeDefined();
        expect(result.value.warnings?.some(w => w.includes('DNS resolution failed'))).toBe(true);
      }
    });

    it('should validate containerd mirror configuration', async () => {
      const mockContext = createMockToolContext();
      const result = await prepareCluster(devConfig, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        // Should not have containerd config warnings when validation succeeds
        expect(result.value.warnings?.some(w => w.includes('Containerd registry mirror'))).toBeFalsy();
      }
    });

    it('should warn when containerd config validation fails', async () => {
      // Mock invalid containerd config
      (global as any).mockExecAsync.mockImplementation(async (cmd: string) => {
        if (cmd.includes('docker exec') && cmd.includes('config.toml')) {
          // Return config without proper mirror configuration
          return { stdout: '[plugins."io.containerd.grpc.v1.cri".registry]', stderr: '' };
        }
        // Keep other mocks working
        if (cmd.includes('kubectl get nodes') && cmd.includes('architecture')) {
          return { stdout: 'amd64', stderr: '' };
        }
        if (cmd.includes('kind get clusters')) {
          return { stdout: 'containerization-assist\n', stderr: '' };
        }
        if (cmd.includes('docker ps') && cmd.includes('ca-registry')) {
          return { stdout: 'ca-registry\n', stderr: '' };
        }
        if (cmd.includes('kubectl run')) {
          return { stdout: 'success\nDNS_SUCCESS', stderr: '' };
        }
        // Handle curl health checks - MUST come after kubectl run checks to avoid false matches
        if (cmd.includes('curl') && cmd.includes('/v2/')) {
          return { stdout: '{}', stderr: '' };
        }
        // Handle kubectl delete pod for test cleanup
        if (cmd.includes('kubectl delete pod')) {
          return { stdout: '', stderr: '' };
        }
        // Handle docker network connect
        if (cmd.includes('docker network connect')) {
          return { stdout: '', stderr: '' };
        }
        if (cmd.includes('docker inspect ca-registry') && cmd.includes('State.Status')) {
          return { stdout: 'running', stderr: '' };
        }
        if (cmd.includes('docker network ls')) {
          return { stdout: 'kind\n', stderr: '' };
        }
        return { stdout: '', stderr: '' };
      });

      const mockContext = createMockToolContext();
      const result = await prepareCluster(devConfig, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.warnings).toBeDefined();
        expect(result.value.warnings?.some(w => w.includes('Containerd registry mirror'))).toBe(true);
      }
    });

    it('should handle stopped registry by restarting it', async () => {
      // Mock registry exists but is stopped
      let registryStarted = false;
      (global as any).mockExecAsync.mockImplementation(async (cmd: string) => {
        if (cmd.includes('docker ps -a') && cmd.includes('ca-registry')) {
          return { stdout: 'ca-registry\n', stderr: '' };
        }
        if (cmd.includes('docker ps') && cmd.includes('ca-registry') && !cmd.includes('-a')) {
          // Not running initially, running after start
          return { stdout: registryStarted ? 'ca-registry\n' : '', stderr: '' };
        }
        if (cmd.includes('docker start ca-registry')) {
          registryStarted = true;
          return { stdout: 'ca-registry', stderr: '' };
        }
        if (cmd.includes('docker inspect ca-registry') && cmd.includes('State.Status')) {
          return { stdout: registryStarted ? 'running' : 'exited', stderr: '' };
        }
        // Keep other mocks working
        if (cmd.includes('kubectl get nodes') && cmd.includes('architecture')) {
          return { stdout: 'amd64', stderr: '' };
        }
        if (cmd.includes('kind get clusters')) {
          return { stdout: 'containerization-assist\n', stderr: '' };
        }
        if (cmd.includes('docker network ls')) {
          return { stdout: 'kind\n', stderr: '' };
        }
        // Handle DNS resolution test (matches registry-dns-test-<timestamp>)
        if (cmd.includes('kubectl run') && cmd.includes('nslookup')) {
          return { stdout: 'Address 1: 172.18.0.3 ca-registry\nDNS_SUCCESS', stderr: '' };
        }
        // Handle registry reachability test with curl (matches registry-test-<timestamp>)
        if (cmd.includes('kubectl run') && cmd.includes('curl')) {
          return { stdout: 'success', stderr: '' };
        }
        // Handle curl health checks - MUST come after kubectl run checks to avoid false matches
        if (cmd.includes('curl') && cmd.includes('/v2/')) {
          return { stdout: '{}', stderr: '' };
        }
        // Handle kubectl delete pod for test cleanup
        if (cmd.includes('kubectl delete pod')) {
          return { stdout: '', stderr: '' };
        }
        // Handle docker network connect
        if (cmd.includes('docker network connect')) {
          return { stdout: '', stderr: '' };
        }
        if (cmd.includes('docker exec') && cmd.includes('config.toml')) {
          return {
            stdout: `
[plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:6000"]
  endpoint = ["http://ca-registry:5000"]
`,
            stderr: '',
          };
        }
        if (cmd.includes('docker inspect ca-registry') && cmd.includes('NetworkSettings.Networks')) {
          return { stdout: 'bridge kind', stderr: '' };
        }
        if (cmd.includes('docker inspect ca-registry') && cmd.includes('NetworkSettings.Ports')) {
          return { stdout: '6000', stderr: '' };
        }
        return { stdout: '', stderr: '' };
      });

      const mockContext = createMockToolContext();
      const result = await prepareCluster(devConfig, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.checks.localRegistryCreated).toBe(true);
        expect(registryStarted).toBe(true);
      }
    }, 30000);

    it('should validate registry health with exponential backoff', async () => {
      let healthCheckAttempts = 0;
      (global as any).mockExecAsync.mockImplementation(async (cmd: string) => {
        if (cmd.includes('docker inspect ca-registry') && cmd.includes('State.Status')) {
          return { stdout: 'running', stderr: '' };
        }
        // Keep other mocks working
        if (cmd.includes('kubectl get nodes') && cmd.includes('architecture')) {
          return { stdout: 'amd64', stderr: '' };
        }
        if (cmd.includes('kind get clusters')) {
          return { stdout: 'containerization-assist\n', stderr: '' };
        }
        if (cmd.includes('docker ps') && cmd.includes('ca-registry')) {
          return { stdout: 'ca-registry\n', stderr: '' };
        }
        if (cmd.includes('docker network ls')) {
          return { stdout: 'kind\n', stderr: '' };
        }
        // Handle DNS resolution test (matches registry-dns-test-<timestamp>)
        if (cmd.includes('kubectl run') && cmd.includes('nslookup')) {
          return { stdout: 'Address 1: 172.18.0.3 ca-registry\nDNS_SUCCESS', stderr: '' };
        }
        // Handle registry reachability test with curl (matches registry-test-<timestamp>)
        if (cmd.includes('kubectl run') && cmd.includes('curl')) {
          return { stdout: 'success', stderr: '' };
        }
        // Handle curl health checks - MUST come after kubectl run checks to avoid false matches
        if (cmd.includes('curl') && cmd.includes('/v2/')) {
          healthCheckAttempts++;
          // Fail first 2 attempts, succeed on 3rd
          if (healthCheckAttempts < 3) {
            return { stdout: 'failed', stderr: '' };
          }
          return { stdout: '{}', stderr: '' };
        }
        // Handle kubectl delete pod for test cleanup
        if (cmd.includes('kubectl delete pod')) {
          return { stdout: '', stderr: '' };
        }
        // Handle docker network connect
        if (cmd.includes('docker network connect')) {
          return { stdout: '', stderr: '' };
        }
        if (cmd.includes('docker exec') && cmd.includes('config.toml')) {
          return {
            stdout: `
[plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:6000"]
  endpoint = ["http://ca-registry:5000"]
`,
            stderr: '',
          };
        }
        if (cmd.includes('docker inspect ca-registry') && cmd.includes('NetworkSettings.Networks')) {
          return { stdout: 'bridge kind', stderr: '' };
        }
        if (cmd.includes('docker inspect ca-registry') && cmd.includes('NetworkSettings.Ports')) {
          return { stdout: '6000', stderr: '' };
        }
        return { stdout: '', stderr: '' };
      });

      const mockContext = createMockToolContext();
      const result = await prepareCluster(devConfig, mockContext);

      expect(result.ok).toBe(true);
      // Should have retried multiple times with exponential backoff
      expect(healthCheckAttempts).toBeGreaterThanOrEqual(3);
    });
  });
});
