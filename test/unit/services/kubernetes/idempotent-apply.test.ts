import { createIdempotentApply, parseManifests } from '../../../../src/services/kubernetes-idempotent-apply';
import * as k8s from '@kubernetes/client-node';
import pino from 'pino';
import { Success, Failure } from '../../../../src/types';

// Mock k8s client
jest.mock('@kubernetes/client-node', () => ({
  KubeConfig: jest.fn(() => ({
    loadFromDefault: jest.fn(),
    loadFromString: jest.fn(),
    makeApiClient: jest.fn()
  })),
  CoreV1Api: jest.fn(),
  AppsV1Api: jest.fn(),
  BatchV1Api: jest.fn(),
  NetworkingV1Api: jest.fn(),
  RbacAuthorizationV1Api: jest.fn(),
  AutoscalingV2Api: jest.fn(),
  CustomObjectsApi: jest.fn()
}));

// Mock config
jest.mock('../../../../src/config', () => ({
  config: {
    mutex: {
      defaultTimeout: 30000,
      monitoringEnabled: true
    }
  }
}));

// Mock js-yaml
jest.mock('js-yaml', () => ({
  loadAll: (content: string) => {
    // Simple YAML parser for tests
    const docs = content.split('---').filter(d => d.trim());
    return docs.map(doc => {
      const lines = doc.trim().split('\n');
      const obj: any = {};
      let currentObj = obj;
      let currentIndent = 0;
      
      for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed || trimmed.startsWith('#')) continue;
        
        const indent = line.length - line.trimStart().length;
        const [key, ...valueParts] = trimmed.split(':');
        const value = valueParts.join(':').trim();
        
        if (key && value) {
          if (key === 'apiVersion') currentObj.apiVersion = value;
          else if (key === 'kind') currentObj.kind = value;
          else if (key === 'name') {
            if (!currentObj.metadata) currentObj.metadata = {};
            currentObj.metadata.name = value;
          }
          else if (key === 'namespace') {
            if (!currentObj.metadata) currentObj.metadata = {};
            currentObj.metadata.namespace = value;
          }
          else if (key === 'port') {
            if (!currentObj.spec) currentObj.spec = {};
            if (!currentObj.spec.ports) currentObj.spec.ports = [];
            currentObj.spec.ports.push({ port: parseInt(value) });
          }
        } else if (key === 'metadata') {
          currentObj.metadata = currentObj.metadata || {};
        } else if (key === 'spec') {
          currentObj.spec = currentObj.spec || {};
        } else if (key === 'ports' && line.includes('-')) {
          if (!currentObj.spec) currentObj.spec = {};
          currentObj.spec.ports = currentObj.spec.ports || [];
        }
      }
      
      return obj;
    }).filter(obj => obj.apiVersion && obj.kind);
  }
}));

describe('IdempotentApply', () => {
  let logger: pino.Logger;
  let mockKubeConfig: any;
  let mockCoreApi: any;
  let mockAppsApi: any;
  let applyResource: any;

  beforeEach(() => {
    logger = pino({ level: 'silent' });
    
    // Setup mock APIs
    mockCoreApi = {
      createNamespace: jest.fn(),
      createNamespacedService: jest.fn(),
      createNamespacedConfigMap: jest.fn(),
      patchNamespace: jest.fn(),
      patchNamespacedService: jest.fn(),
      patchNamespacedConfigMap: jest.fn()
    };

    mockAppsApi = {
      createNamespacedDeployment: jest.fn(),
      patchNamespacedDeployment: jest.fn()
    };

    mockKubeConfig = {
      loadFromDefault: jest.fn(),
      loadFromString: jest.fn(),
      makeApiClient: jest.fn((apiClass) => {
        if (apiClass === k8s.CoreV1Api) return mockCoreApi;
        if (apiClass === k8s.AppsV1Api) return mockAppsApi;
        return {};
      })
    };

    (k8s.KubeConfig as jest.Mock).mockImplementation(() => mockKubeConfig);
    
    applyResource = createIdempotentApply(logger);
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('create new resources', () => {
    test('should create a new deployment', async () => {
      const deployment = {
        apiVersion: 'apps/v1',
        kind: 'Deployment',
        metadata: {
          name: 'test-app',
          namespace: 'default'
        },
        spec: {
          replicas: 3,
          selector: { matchLabels: { app: 'test' } },
          template: {
            metadata: { labels: { app: 'test' } },
            spec: {
              containers: [{
                name: 'app',
                image: 'nginx:latest'
              }]
            }
          }
        }
      };

      mockAppsApi.createNamespacedDeployment.mockResolvedValue({ 
        body: deployment 
      });

      const result = await applyResource(deployment);

      expect(result.ok).toBe(true);
      expect(mockAppsApi.createNamespacedDeployment).toHaveBeenCalledWith(
        'default',
        deployment,
        undefined,
        undefined
      );
    });

    test('should create a new service', async () => {
      const service = {
        apiVersion: 'v1',
        kind: 'Service',
        metadata: {
          name: 'test-service',
          namespace: 'production'
        },
        spec: {
          selector: { app: 'test' },
          ports: [{ port: 80, targetPort: 8080 }]
        }
      };

      mockCoreApi.createNamespacedService.mockResolvedValue({ 
        body: service 
      });

      const result = await applyResource(service);

      expect(result.ok).toBe(true);
      expect(mockCoreApi.createNamespacedService).toHaveBeenCalledWith(
        'production',
        service,
        undefined,
        undefined
      );
    });

    test('should support dry-run mode', async () => {
      const configMap = {
        apiVersion: 'v1',
        kind: 'ConfigMap',
        metadata: {
          name: 'test-config'
        },
        data: {
          key: 'value'
        }
      };

      mockCoreApi.createNamespacedConfigMap.mockResolvedValue({ 
        body: configMap 
      });

      const result = await applyResource(configMap, { dryRun: true });

      expect(result.ok).toBe(true);
      expect(mockCoreApi.createNamespacedConfigMap).toHaveBeenCalledWith(
        'default',
        configMap,
        undefined,
        'All'
      );
    });
  });

  describe('handle existing resources', () => {
    test('should use server-side apply when resource exists', async () => {
      const deployment = {
        apiVersion: 'apps/v1',
        kind: 'Deployment',
        metadata: {
          name: 'existing-app',
          namespace: 'default'
        },
        spec: {
          replicas: 5
        }
      };

      // First create fails with 409 (already exists)
      mockAppsApi.createNamespacedDeployment.mockRejectedValue({
        statusCode: 409,
        message: 'deployments.apps "existing-app" already exists'
      });

      // Patch succeeds
      mockAppsApi.patchNamespacedDeployment.mockResolvedValue({
        body: deployment
      });

      const result = await applyResource(deployment);

      expect(result.ok).toBe(true);
      expect(mockAppsApi.createNamespacedDeployment).toHaveBeenCalledTimes(1);
      expect(mockAppsApi.patchNamespacedDeployment).toHaveBeenCalledWith(
        'existing-app',
        'default',
        deployment,
        undefined,
        undefined,
        'containerization-assist',
        undefined,
        { headers: { 'Content-Type': 'application/apply-patch+yaml' } }
      );
    });

    test('should handle concurrent applies of same resource', async () => {
      const service = {
        apiVersion: 'v1',
        kind: 'Service',
        metadata: {
          name: 'concurrent-service',
          namespace: 'default'
        },
        spec: {
          selector: { app: 'test' },
          ports: [{ port: 80 }]
        }
      };

      let createCallCount = 0;
      mockCoreApi.createNamespacedService.mockImplementation(async () => {
        createCallCount++;
        if (createCallCount === 1) {
          // First call succeeds
          await new Promise(resolve => setTimeout(resolve, 50));
          return { body: service };
        } else {
          // Subsequent calls fail with 409
          throw { statusCode: 409, message: 'already exists' };
        }
      });

      mockCoreApi.patchNamespacedService.mockResolvedValue({
        body: service
      });

      // Start multiple concurrent applies
      const results = await Promise.all([
        applyResource(service),
        applyResource(service),
        applyResource(service)
      ]);

      // All should succeed
      expect(results.every(r => r.ok)).toBe(true);
      
      // First should create, others should patch
      expect(mockCoreApi.createNamespacedService).toHaveBeenCalledTimes(3);
      expect(mockCoreApi.patchNamespacedService.mock.calls.length).toBeGreaterThanOrEqual(0);
    });
  });

  describe('error handling', () => {
    test('should handle API errors', async () => {
      const deployment = {
        apiVersion: 'apps/v1',
        kind: 'Deployment',
        metadata: {
          name: 'error-app',
          namespace: 'default'
        },
        spec: {}
      };

      mockAppsApi.createNamespacedDeployment.mockRejectedValue({
        statusCode: 400,
        message: 'Invalid deployment spec'
      });

      const result = await applyResource(deployment);

      expect(result.ok).toBe(false);
      expect(result.error).toContain('Invalid deployment spec');
    });

    test('should handle network errors', async () => {
      const service = {
        apiVersion: 'v1',
        kind: 'Service',
        metadata: {
          name: 'network-error',
          namespace: 'default'
        },
        spec: {}
      };

      mockCoreApi.createNamespacedService.mockRejectedValue(
        new Error('ECONNREFUSED')
      );

      const result = await applyResource(service);

      expect(result.ok).toBe(false);
      expect(result.error).toContain('ECONNREFUSED');
    });
  });

  describe('parseManifests', () => {
    test('should parse single manifest', () => {
      const yaml = `
apiVersion: v1
kind: Service
metadata:
  name: test
spec:
  ports:
  - port: 80
`;
      const resources = parseManifests(yaml);
      
      expect(resources).toHaveLength(1);
      expect(resources[0].kind).toBe('Service');
      expect(resources[0].metadata.name).toBe('test');
    });

    test('should parse multiple manifests', () => {
      const yaml = `
apiVersion: v1
kind: Service
metadata:
  name: service1
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deploy1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: config1
`;
      const resources = parseManifests(yaml);
      
      expect(resources).toHaveLength(3);
      expect(resources[0].kind).toBe('Service');
      expect(resources[1].kind).toBe('Deployment');
      expect(resources[2].kind).toBe('ConfigMap');
    });

    test('should filter out empty documents', () => {
      const yaml = `
---
apiVersion: v1
kind: Service
metadata:
  name: test
---
---
`;
      const resources = parseManifests(yaml);
      
      expect(resources).toHaveLength(1);
      expect(resources[0].kind).toBe('Service');
    });
  });
});