/**
 * Unit Tests: Verify Deployment Tool
 * Tests the verify-deployment tool functionality with Kubernetes client mocking
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

// Mock Kubernetes client
const mockK8sClient = {
  waitForDeploymentReady: jest.fn(),
  getDeploymentStatus: jest.fn(),
  listServices: jest.fn(),
  listIngresses: jest.fn(),
  getService: jest.fn(),
};

jest.mock('../../../src/infra/kubernetes/client', () => ({
  createKubernetesClient: jest.fn(() => mockK8sClient),
}));

// Mock lib modules
jest.mock('../../../src/lib/logger', () => ({
  createTimer: jest.fn(() => ({
    end: jest.fn(),
    error: jest.fn(),
  })),
  createLogger: jest.fn(() => createMockLogger()),
}));

function createMockToolContext() {
  return {
    logger: createMockLogger(),
  } as any;
}

// Import these after mocks are set up
import { default as verifyDeploymentTool } from '../../../src/tools/verify-deployment/tool';
import type { VerifyDeploymentParams } from '../../../src/tools/verify-deployment/schema';

describe('verify-deployment', () => {
  let mockLogger: ReturnType<typeof createMockLogger>;
  let config: VerifyDeploymentParams;

  beforeEach(() => {
    mockLogger = createMockLogger();
    config = {
      deploymentName: 'test-app',
      namespace: 'production',
      checks: ['pods', 'services', 'health'],
    };

    // Reset all mocks
    jest.clearAllMocks();

    // Default successful deployment status
    mockK8sClient.waitForDeploymentReady.mockResolvedValue(
      createSuccessResult({
        ready: true,
        readyReplicas: 2,
        totalReplicas: 2,
      }),
    );

    mockK8sClient.getDeploymentStatus.mockResolvedValue(
      createSuccessResult({
        readyReplicas: 2,
        totalReplicas: 2,
        ready: true,
      }),
    );
  });

  describe('Happy Path', () => {
    it('should successfully verify a healthy deployment', async () => {
      const mockContext = createMockToolContext();
      const result = await verifyDeploymentTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.success).toBe(true);
        expect(result.value.ready).toBe(true);
        expect(result.value.deploymentName).toBe('test-app');
        expect(result.value.namespace).toBe('production');
        expect(result.value.status.readyReplicas).toBe(2);
        expect(result.value.status.totalReplicas).toBe(2);
        expect(result.value.healthCheck?.status).toBe('healthy');
        expect(result.value.workflowHints?.nextStep).toBe('ops');
      }
    });

    it('should use default namespace when not provided', async () => {
      delete config.namespace;

      const mockContext = createMockToolContext();
      const result = await verifyDeploymentTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.namespace).toBe('default');
      }
    });

    it('should use default checks when not provided', async () => {
      delete config.checks;

      const mockContext = createMockToolContext();
      const result = await verifyDeploymentTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      expect(mockK8sClient.waitForDeploymentReady).toHaveBeenCalled();
    });

    it('should handle minimal configuration', async () => {
      const minimalConfig: VerifyDeploymentParams = {
        deploymentName: 'minimal-app',
      };

      const mockContext = createMockToolContext();
      const result = await verifyDeploymentTool.handler(minimalConfig, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.deploymentName).toBe('minimal-app');
        expect(result.value.namespace).toBe('default');
      }
    });
  });

  describe('Error Handling', () => {
    it('should fail when deploymentName is not provided', async () => {
      config.deploymentName = '' as any;

      const mockContext = createMockToolContext();
      const result = await verifyDeploymentTool.handler(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Deployment name is required');
      }
    });

    it('should handle deployment not ready', async () => {
      mockK8sClient.waitForDeploymentReady.mockResolvedValue(
        createFailureResult('Deployment not ready: waiting for replicas'),
      );

      mockK8sClient.getDeploymentStatus.mockResolvedValue(
        createSuccessResult({
          readyReplicas: 1,
          totalReplicas: 3,
          ready: false,
        }),
      );

      const mockContext = createMockToolContext();
      const result = await verifyDeploymentTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.success).toBe(false);
        expect(result.value.ready).toBe(false);
        expect(result.value.status.readyReplicas).toBe(1);
        expect(result.value.status.totalReplicas).toBe(3);
        expect(result.value.healthCheck?.status).toBe('unknown');
        expect(result.value.workflowHints?.nextStep).toBe('fix-deployment-issues');
      }
    });

    it('should handle Kubernetes client errors', async () => {
      mockK8sClient.waitForDeploymentReady.mockRejectedValue(
        new Error('Failed to connect to Kubernetes cluster'),
      );

      const mockContext = createMockToolContext();
      const result = await verifyDeploymentTool.handler(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Failed to connect to Kubernetes cluster');
      }
    });

    it('should handle invalid parameters', async () => {
      const mockContext = createMockToolContext();
      const result = await verifyDeploymentTool.handler(null as any, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Invalid parameters');
      }
    });
  });

  describe('Deployment Status', () => {
    it('should handle deployment with no ready replicas', async () => {
      mockK8sClient.waitForDeploymentReady.mockResolvedValue(
        createSuccessResult({
          ready: false,
          readyReplicas: 0,
          totalReplicas: 2,
        }),
      );

      mockK8sClient.getDeploymentStatus.mockResolvedValue(
        createSuccessResult({
          readyReplicas: 0,
          totalReplicas: 2,
          ready: false,
        }),
      );

      const mockContext = createMockToolContext();
      const result = await verifyDeploymentTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.ready).toBe(false);
        expect(result.value.status.readyReplicas).toBe(0);
        expect(result.value.healthCheck?.status).toBe('unknown');
      }
    });

    it('should handle deployment with partial readiness', async () => {
      mockK8sClient.waitForDeploymentReady.mockResolvedValue(
        createSuccessResult({
          ready: false,
          readyReplicas: 2,
          totalReplicas: 5,
        }),
      );

      mockK8sClient.getDeploymentStatus.mockResolvedValue(
        createSuccessResult({
          readyReplicas: 2,
          totalReplicas: 5,
          ready: false,
        }),
      );

      const mockContext = createMockToolContext();
      const result = await verifyDeploymentTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.ready).toBe(false);
        expect(result.value.status.readyReplicas).toBe(2);
        expect(result.value.status.totalReplicas).toBe(5);
      }
    });
  });

  describe('Check Types', () => {
    it('should handle pods check', async () => {
      config.checks = ['pods'];

      const mockContext = createMockToolContext();
      const result = await verifyDeploymentTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      expect(mockK8sClient.waitForDeploymentReady).toHaveBeenCalled();
    });

    it('should handle services check', async () => {
      config.checks = ['services'];

      const mockContext = createMockToolContext();
      const result = await verifyDeploymentTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
    });

    it('should handle ingress check', async () => {
      config.checks = ['ingress'];

      const mockContext = createMockToolContext();
      const result = await verifyDeploymentTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
    });

    it('should handle multiple check types', async () => {
      config.checks = ['pods', 'services', 'ingress', 'health'];

      const mockContext = createMockToolContext();
      const result = await verifyDeploymentTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
    });
  });

  describe('Health Check', () => {
    it('should include health check status in result', async () => {
      const mockContext = createMockToolContext();
      const result = await verifyDeploymentTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.healthCheck).toBeDefined();
        expect(result.value.healthCheck?.status).toBe('healthy');
        expect(result.value.healthCheck?.message).toBeDefined();
      }
    });

    it('should mark health as unhealthy when deployment is not ready', async () => {
      mockK8sClient.waitForDeploymentReady.mockResolvedValue(
        createFailureResult('Timeout waiting for deployment'),
      );

      mockK8sClient.getDeploymentStatus.mockResolvedValue(
        createSuccessResult({
          readyReplicas: 0,
          totalReplicas: 3,
          ready: false,
        }),
      );

      const mockContext = createMockToolContext();
      const result = await verifyDeploymentTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.healthCheck?.status).toBe('unknown');
      }
    });
  });

  describe('Workflow Hints', () => {
    it('should provide next steps for successful deployment', async () => {
      const mockContext = createMockToolContext();
      const result = await verifyDeploymentTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.workflowHints?.nextStep).toBe('ops');
        expect(result.value.workflowHints?.message).toContain('successful');
      }
    });

    it('should provide next steps for failed deployment', async () => {
      mockK8sClient.waitForDeploymentReady.mockResolvedValue(
        createSuccessResult({
          ready: false,
          readyReplicas: 0,
          totalReplicas: 2,
        }),
      );

      const mockContext = createMockToolContext();
      const result = await verifyDeploymentTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.workflowHints?.nextStep).toBe('fix-deployment-issues');
        expect(result.value.workflowHints?.message).toContain('found issues');
      }
    });
  });

  describe('Endpoints', () => {
    beforeEach(() => {
      // Default to empty services and ingresses
      mockK8sClient.listServices.mockResolvedValue(createSuccessResult([]));
      mockK8sClient.listIngresses.mockResolvedValue(createSuccessResult([]));
    });

    it('should include endpoints in result', async () => {
      const mockContext = createMockToolContext();
      const result = await verifyDeploymentTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.endpoints).toBeDefined();
        expect(Array.isArray(result.value.endpoints)).toBe(true);
      }
    });

    it('should handle deployment with no endpoints', async () => {
      const mockContext = createMockToolContext();
      const result = await verifyDeploymentTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.endpoints).toEqual([]);
      }
    });

    describe('Service Discovery', () => {
      it('should discover LoadBalancer service endpoints with IP', async () => {
        config.checks = ['services'];
        mockK8sClient.listServices.mockResolvedValue(
          createSuccessResult([
            {
              metadata: { name: 'test-service' },
              spec: {
                type: 'LoadBalancer',
                ports: [{ port: 8080 }],
              },
              status: {
                loadBalancer: {
                  ingress: [{ ip: '203.0.113.1' }],
                },
              },
            },
          ]),
        );

        const mockContext = createMockToolContext();
        const result = await verifyDeploymentTool.handler(config, mockContext);

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.endpoints).toHaveLength(1);
          expect(result.value.endpoints[0]).toMatchObject({
            type: 'external',
            url: 'http://203.0.113.1',
            port: 8080,
          });
        }
      });

      it('should discover LoadBalancer service endpoints with hostname', async () => {
        config.checks = ['services'];
        mockK8sClient.listServices.mockResolvedValue(
          createSuccessResult([
            {
              metadata: { name: 'test-service' },
              spec: {
                type: 'LoadBalancer',
                ports: [{ port: 80 }],
              },
              status: {
                loadBalancer: {
                  ingress: [{ hostname: 'example.com' }],
                },
              },
            },
          ]),
        );

        const mockContext = createMockToolContext();
        const result = await verifyDeploymentTool.handler(config, mockContext);

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.endpoints).toHaveLength(1);
          expect(result.value.endpoints[0]).toMatchObject({
            type: 'external',
            url: 'http://example.com',
            port: 80,
          });
        }
      });

      it('should discover NodePort service endpoints', async () => {
        config.checks = ['services'];
        mockK8sClient.listServices.mockResolvedValue(
          createSuccessResult([
            {
              metadata: { name: 'test-service' },
              spec: {
                type: 'NodePort',
                ports: [{ port: 8080, nodePort: 30080 }],
              },
            },
          ]),
        );

        const mockContext = createMockToolContext();
        const result = await verifyDeploymentTool.handler(config, mockContext);

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.endpoints).toHaveLength(1);
          expect(result.value.endpoints[0]).toMatchObject({
            type: 'external',
            url: '<node-ip>',
            port: 30080,
          });
        }
      });

      it('should discover ClusterIP service endpoints when includeClusterIP is enabled', async () => {
        config.checks = ['services'];
        mockK8sClient.listServices.mockResolvedValue(
          createSuccessResult([
            {
              metadata: { name: 'test-service' },
              spec: {
                type: 'ClusterIP',
                clusterIP: '10.0.0.1',
                ports: [{ port: 8080 }],
              },
            },
          ]),
        );

        const mockContext = createMockToolContext();
        const result = await verifyDeploymentTool.handler(config, mockContext);

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.endpoints).toHaveLength(1);
          expect(result.value.endpoints[0]).toMatchObject({
            type: 'internal',
            url: 'http://10.0.0.1',
            port: 8080,
          });
        }
      });

      it('should discover multiple service endpoints', async () => {
        config.checks = ['services'];
        mockK8sClient.listServices.mockResolvedValue(
          createSuccessResult([
            {
              metadata: { name: 'service-1' },
              spec: {
                type: 'LoadBalancer',
                ports: [{ port: 80 }],
              },
              status: {
                loadBalancer: {
                  ingress: [{ ip: '203.0.113.1' }],
                },
              },
            },
            {
              metadata: { name: 'service-2' },
              spec: {
                type: 'NodePort',
                ports: [{ port: 8080, nodePort: 30080 }],
              },
            },
          ]),
        );

        const mockContext = createMockToolContext();
        const result = await verifyDeploymentTool.handler(config, mockContext);

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.endpoints).toHaveLength(2);
        }
      });

      it('should handle service discovery errors gracefully', async () => {
        config.checks = ['services'];
        mockK8sClient.listServices.mockResolvedValue(
          createFailureResult('Failed to list services'),
        );

        const mockContext = createMockToolContext();
        const result = await verifyDeploymentTool.handler(config, mockContext);

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.endpoints).toEqual([]);
        }
      });
    });

    describe('Ingress Discovery', () => {
      it('should discover HTTP ingress endpoints', async () => {
        config.checks = ['ingress'];
        mockK8sClient.listIngresses.mockResolvedValue(
          createSuccessResult([
            {
              metadata: { name: 'test-ingress' },
              spec: {
                rules: [
                  {
                    host: 'example.com',
                    http: {
                      paths: [{ path: '/api' }],
                    },
                  },
                ],
              },
            },
          ]),
        );

        const mockContext = createMockToolContext();
        const result = await verifyDeploymentTool.handler(config, mockContext);

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.endpoints).toHaveLength(1);
          expect(result.value.endpoints[0]).toMatchObject({
            type: 'external',
            url: 'http://example.com/api',
            port: 80,
          });
        }
      });

      it('should discover HTTPS ingress endpoints with TLS', async () => {
        config.checks = ['ingress'];
        mockK8sClient.listIngresses.mockResolvedValue(
          createSuccessResult([
            {
              metadata: { name: 'test-ingress' },
              spec: {
                tls: [
                  {
                    hosts: ['example.com'],
                  },
                ],
                rules: [
                  {
                    host: 'example.com',
                    http: {
                      paths: [{ path: '/' }],
                    },
                  },
                ],
              },
            },
          ]),
        );

        const mockContext = createMockToolContext();
        const result = await verifyDeploymentTool.handler(config, mockContext);

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.endpoints).toHaveLength(1);
          expect(result.value.endpoints[0]).toMatchObject({
            type: 'external',
            url: 'https://example.com/',
            port: 443,
          });
        }
      });

      it('should discover ingress endpoints with LoadBalancer IP', async () => {
        config.checks = ['ingress'];
        mockK8sClient.listIngresses.mockResolvedValue(
          createSuccessResult([
            {
              metadata: { name: 'test-ingress' },
              spec: {
                rules: [],
              },
              status: {
                loadBalancer: {
                  ingress: [{ ip: '203.0.113.1' }],
                },
              },
            },
          ]),
        );

        const mockContext = createMockToolContext();
        const result = await verifyDeploymentTool.handler(config, mockContext);

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.endpoints).toHaveLength(1);
          expect(result.value.endpoints[0]).toMatchObject({
            type: 'external',
            url: 'http://203.0.113.1',
            port: 80,
          });
        }
      });

      it('should discover multiple ingress endpoints', async () => {
        config.checks = ['ingress'];
        mockK8sClient.listIngresses.mockResolvedValue(
          createSuccessResult([
            {
              metadata: { name: 'ingress-1' },
              spec: {
                rules: [
                  {
                    host: 'example.com',
                    http: {
                      paths: [{ path: '/api' }, { path: '/v2' }],
                    },
                  },
                ],
              },
            },
          ]),
        );

        const mockContext = createMockToolContext();
        const result = await verifyDeploymentTool.handler(config, mockContext);

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.endpoints).toHaveLength(2);
        }
      });

      it('should handle ingress discovery errors gracefully', async () => {
        config.checks = ['ingress'];
        mockK8sClient.listIngresses.mockResolvedValue(
          createFailureResult('Failed to list ingresses'),
        );

        const mockContext = createMockToolContext();
        const result = await verifyDeploymentTool.handler(config, mockContext);

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.endpoints).toEqual([]);
        }
      });
    });

    describe('Combined Service and Ingress Discovery', () => {
      it('should discover both service and ingress endpoints', async () => {
        config.checks = ['services', 'ingress'];
        mockK8sClient.listServices.mockResolvedValue(
          createSuccessResult([
            {
              metadata: { name: 'test-service' },
              spec: {
                type: 'LoadBalancer',
                ports: [{ port: 8080 }],
              },
              status: {
                loadBalancer: {
                  ingress: [{ ip: '203.0.113.1' }],
                },
              },
            },
          ]),
        );
        mockK8sClient.listIngresses.mockResolvedValue(
          createSuccessResult([
            {
              metadata: { name: 'test-ingress' },
              spec: {
                rules: [
                  {
                    host: 'example.com',
                    http: {
                      paths: [{ path: '/' }],
                    },
                  },
                ],
              },
            },
          ]),
        );

        const mockContext = createMockToolContext();
        const result = await verifyDeploymentTool.handler(config, mockContext);

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.endpoints).toHaveLength(2);
        }
      });
    });
  });

  describe('Status Conditions', () => {
    it('should include deployment conditions in status', async () => {
      const mockContext = createMockToolContext();
      const result = await verifyDeploymentTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.status.conditions).toBeDefined();
        expect(Array.isArray(result.value.status.conditions)).toBe(true);
        expect(result.value.status.conditions.length).toBeGreaterThan(0);
        expect(result.value.status.conditions[0]).toHaveProperty('type');
        expect(result.value.status.conditions[0]).toHaveProperty('status');
        expect(result.value.status.conditions[0]).toHaveProperty('message');
      }
    });

    it('should mark condition as False when deployment not ready', async () => {
      mockK8sClient.waitForDeploymentReady.mockResolvedValue(
        createSuccessResult({
          ready: false,
          readyReplicas: 1,
          totalReplicas: 3,
        }),
      );

      const mockContext = createMockToolContext();
      const result = await verifyDeploymentTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        const availableCondition = result.value.status.conditions.find((c) => c.type === 'Available');
        expect(availableCondition?.status).toBe('False');
      }
    });
  });
});
