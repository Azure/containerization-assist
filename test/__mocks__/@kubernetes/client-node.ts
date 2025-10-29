/**
 * Manual Jest mock for @kubernetes/client-node
 * Solves ES module loading issues in integration tests
 */

import { jest } from '@jest/globals';

// CoreV1Api mock with all common operations
const createCoreV1ApiMock = () => ({
  // Namespace operations
  listNamespace: jest.fn().mockResolvedValue({
    body: {
      items: [
        { metadata: { name: 'default' }, status: { phase: 'Active' } },
        { metadata: { name: 'kube-system' }, status: { phase: 'Active' } },
      ],
    },
  }),
  createNamespace: jest.fn().mockResolvedValue({
    body: { metadata: { name: 'test-namespace' }, status: { phase: 'Active' } },
  }),
  readNamespace: jest.fn().mockResolvedValue({
    body: { metadata: { name: 'default' }, status: { phase: 'Active' } },
  }),
  deleteNamespace: jest.fn().mockResolvedValue({ body: { status: 'Success' } }),

  // Pod operations
  listNamespacedPod: jest.fn().mockResolvedValue({
    body: {
      items: [
        {
          metadata: { name: 'pod-1', namespace: 'default', uid: 'uid-1' },
          spec: { containers: [{ name: 'main', image: 'app:latest' }] },
          status: {
            phase: 'Running',
            containerStatuses: [{ ready: true, restartCount: 0 }],
            conditions: [{ type: 'Ready', status: 'True' }],
          },
        },
      ],
    },
  }),
  createNamespacedPod: jest.fn().mockResolvedValue({
    body: {
      metadata: { name: 'new-pod', namespace: 'default' },
      status: { phase: 'Pending' },
    },
  }),
  readNamespacedPod: jest.fn().mockResolvedValue({
    body: {
      metadata: { name: 'pod-1', namespace: 'default' },
      status: { phase: 'Running' },
    },
  }),
  deleteNamespacedPod: jest.fn().mockResolvedValue({ body: { status: 'Success' } }),
  readNamespacedPodLog: jest.fn().mockResolvedValue({
    body: 'Container logs here\nApplication started\n',
  }),

  // Service operations
  listNamespacedService: jest.fn().mockResolvedValue({
    body: {
      items: [
        {
          metadata: { name: 'service-1', namespace: 'default' },
          spec: {
            type: 'ClusterIP',
            selector: { app: 'test-app' },
            ports: [{ port: 80, targetPort: 3000 }],
          },
          status: { loadBalancer: {} },
        },
      ],
    },
  }),
  createNamespacedService: jest.fn().mockResolvedValue({
    body: {
      metadata: { name: 'new-service', namespace: 'default' },
      spec: { type: 'ClusterIP' },
    },
  }),
  readNamespacedService: jest.fn().mockResolvedValue({
    body: {
      metadata: { name: 'service-1', namespace: 'default' },
      spec: { type: 'ClusterIP' },
    },
  }),
  deleteNamespacedService: jest.fn().mockResolvedValue({ body: { status: 'Success' } }),

  // ConfigMap operations
  listNamespacedConfigMap: jest.fn().mockResolvedValue({
    body: { items: [] },
  }),
  createNamespacedConfigMap: jest.fn().mockResolvedValue({
    body: {
      metadata: { name: 'config-1', namespace: 'default' },
      data: { 'app.properties': 'key=value' },
    },
  }),

  // Secret operations
  listNamespacedSecret: jest.fn().mockResolvedValue({
    body: { items: [] },
  }),
  createNamespacedSecret: jest.fn().mockResolvedValue({
    body: {
      metadata: { name: 'secret-1', namespace: 'default' },
      type: 'Opaque',
      data: { password: 'base64encoded' },
    },
  }),

  // Event operations
  listNamespacedEvent: jest.fn().mockResolvedValue({
    body: {
      items: [
        {
          metadata: { name: 'event-1', namespace: 'default' },
          type: 'Normal',
          reason: 'Created',
          message: 'Created pod: pod-1',
        },
      ],
    },
  }),
});

// AppsV1Api mock with deployment operations
const createAppsV1ApiMock = () => ({
  // Deployment operations
  listNamespacedDeployment: jest.fn().mockResolvedValue({
    body: {
      items: [
        {
          metadata: { name: 'deployment-1', namespace: 'default', generation: 1 },
          spec: {
            replicas: 2,
            selector: { matchLabels: { app: 'test-app' } },
            template: {
              metadata: { labels: { app: 'test-app' } },
              spec: {
                containers: [{ name: 'main', image: 'app:latest' }],
              },
            },
          },
          status: {
            observedGeneration: 1,
            replicas: 2,
            updatedReplicas: 2,
            readyReplicas: 2,
            availableReplicas: 2,
            conditions: [
              { type: 'Progressing', status: 'True', reason: 'NewReplicaSetAvailable' },
              { type: 'Available', status: 'True', reason: 'MinimumReplicasAvailable' },
            ],
          },
        },
      ],
    },
  }),
  createNamespacedDeployment: jest.fn().mockResolvedValue({
    body: {
      metadata: { name: 'new-deployment', namespace: 'default' },
      spec: { replicas: 1 },
      status: { replicas: 0, readyReplicas: 0 },
    },
  }),
  readNamespacedDeployment: jest.fn().mockResolvedValue({
    body: {
      metadata: { name: 'deployment-1', namespace: 'default' },
      status: { replicas: 2, readyReplicas: 2 },
    },
  }),
  patchNamespacedDeployment: jest.fn().mockResolvedValue({
    body: {
      metadata: { name: 'deployment-1', namespace: 'default' },
      spec: { replicas: 3 },
    },
  }),
  deleteNamespacedDeployment: jest.fn().mockResolvedValue({ body: { status: 'Success' } }),
  readNamespacedDeploymentScale: jest.fn().mockResolvedValue({
    body: {
      metadata: { name: 'deployment-1', namespace: 'default' },
      spec: { replicas: 2 },
      status: { replicas: 2 },
    },
  }),
  patchNamespacedDeploymentScale: jest.fn().mockResolvedValue({
    body: {
      metadata: { name: 'deployment-1', namespace: 'default' },
      spec: { replicas: 5 },
    },
  }),

  // StatefulSet operations
  listNamespacedStatefulSet: jest.fn().mockResolvedValue({
    body: { items: [] },
  }),
  createNamespacedStatefulSet: jest.fn().mockResolvedValue({
    body: {
      metadata: { name: 'statefulset-1', namespace: 'default' },
      spec: { replicas: 1 },
    },
  }),

  // DaemonSet operations
  listNamespacedDaemonSet: jest.fn().mockResolvedValue({
    body: { items: [] },
  }),
  createNamespacedDaemonSet: jest.fn().mockResolvedValue({
    body: {
      metadata: { name: 'daemonset-1', namespace: 'default' },
      spec: {},
    },
  }),

  // ReplicaSet operations
  listNamespacedReplicaSet: jest.fn().mockResolvedValue({
    body: {
      items: [
        {
          metadata: { name: 'rs-1', namespace: 'default', ownerReferences: [] },
          spec: { replicas: 2 },
          status: { replicas: 2, readyReplicas: 2 },
        },
      ],
    },
  }),
});

// NetworkingV1Api mock
const createNetworkingV1ApiMock = () => ({
  listNamespacedIngress: jest.fn().mockResolvedValue({
    body: {
      items: [
        {
          metadata: { name: 'ingress-1', namespace: 'default' },
          spec: {
            rules: [
              {
                host: 'test.example.com',
                http: {
                  paths: [
                    {
                      path: '/',
                      pathType: 'Prefix',
                      backend: { service: { name: 'service-1', port: { number: 80 } } },
                    },
                  ],
                },
              },
            ],
          },
          status: { loadBalancer: { ingress: [{ ip: '10.0.0.1' }] } },
        },
      ],
    },
  }),
  createNamespacedIngress: jest.fn().mockResolvedValue({
    body: {
      metadata: { name: 'new-ingress', namespace: 'default' },
      spec: { rules: [] },
    },
  }),
});

// BatchV1Api mock
const createBatchV1ApiMock = () => ({
  listNamespacedJob: jest.fn().mockResolvedValue({
    body: {
      items: [
        {
          metadata: { name: 'job-1', namespace: 'default' },
          spec: { completions: 1, parallelism: 1 },
          status: { succeeded: 1, conditions: [{ type: 'Complete', status: 'True' }] },
        },
      ],
    },
  }),
  createNamespacedJob: jest.fn().mockResolvedValue({
    body: {
      metadata: { name: 'new-job', namespace: 'default' },
      spec: { completions: 1 },
      status: { active: 1 },
    },
  }),
  listNamespacedCronJob: jest.fn().mockResolvedValue({
    body: { items: [] },
  }),
});

// KubeConfig mock
const mockKubeConfig = {
  loadFromDefault: jest.fn(),
  loadFromFile: jest.fn((filePath: string) => {
    // Actually try to read and parse the file for integration tests
    const fs = require('node:fs');
    const yaml = require('js-yaml');
    try {
      const content = fs.readFileSync(filePath, 'utf8');
      const config = yaml.load(content) as any;

      // Update mock return values based on loaded config
      if (config) {
        mockKubeConfig.getCurrentContext = jest.fn().mockReturnValue(config['current-context'] || '');
        if (config.contexts && config.contexts.length > 0) {
          mockKubeConfig.getContexts = jest.fn().mockReturnValue(config.contexts);
        }
        if (config.clusters && config.clusters.length > 0) {
          mockKubeConfig.getClusters = jest.fn().mockReturnValue(config.clusters);
          mockKubeConfig.getCurrentCluster = jest.fn().mockReturnValue(config.clusters[0]);
        }
        if (config.users && config.users.length > 0) {
          mockKubeConfig.getUsers = jest.fn().mockReturnValue(config.users);
          mockKubeConfig.getCurrentUser = jest.fn().mockReturnValue(config.users[0]);
        }
      }
    } catch (error) {
      // Re-throw parse errors
      throw error;
    }
  }),
  loadFromString: jest.fn(),
  loadFromCluster: jest.fn(),
  makeApiClient: jest.fn().mockImplementation((ApiClass: any) => {
    if (!ApiClass) {
      return {};
    }
    const apiName = ApiClass.name || ApiClass.constructor?.name || '';
    switch (apiName) {
      case 'ObjectCoreV1Api':
        return createCoreV1ApiMock();
      case 'ObjectAppsV1Api':
        return createAppsV1ApiMock();
      case 'ObjectNetworkingV1Api':
        return createNetworkingV1ApiMock();
      case 'ObjectBatchV1Api':
        return createBatchV1ApiMock();
      case 'ObjectAuthorizationV1Api':
        return { createSelfSubjectAccessReview: jest.fn().mockResolvedValue({ status: { allowed: true } }) };
      default:
        return {};
    }
  }),
  getCurrentContext: jest.fn().mockReturnValue('default'),
  setCurrentContext: jest.fn(),
  getCurrentCluster: jest
    .fn()
    .mockReturnValue({ name: 'local', server: 'https://localhost:6443' }),
  getCurrentUser: jest.fn().mockReturnValue({ name: 'admin' }),
  getContexts: jest.fn().mockReturnValue([{ name: 'default' }]),
  getClusters: jest.fn().mockReturnValue([{ name: 'local', server: 'https://localhost:6443' }]),
  getUsers: jest.fn().mockReturnValue([{ name: 'admin' }]),
  contexts: [{ name: 'default', cluster: 'local', user: 'admin' }],
  clusters: [{ name: 'local', server: 'https://localhost:6443', skipTLSVerify: false }],
  users: [{ name: 'admin', token: 'mock-token' }],
};

// Kubernetes Object API mock
const mockKubernetesObjectApi = {
  create: jest.fn().mockResolvedValue({
    body: { metadata: { name: 'created-resource' }, status: 'Success' },
  }),
  read: jest.fn().mockResolvedValue({
    body: { metadata: { name: 'read-resource' } },
  }),
  patch: jest.fn().mockResolvedValue({
    body: { metadata: { name: 'patched-resource' } },
  }),
  replace: jest.fn().mockResolvedValue({
    body: { metadata: { name: 'replaced-resource' } },
  }),
  delete: jest.fn().mockResolvedValue({
    body: { status: 'Success' },
  }),
};

// Export mocked modules
export const KubeConfig = jest.fn().mockImplementation(() => mockKubeConfig);
export const CoreV1Api = jest.fn().mockImplementation(() => createCoreV1ApiMock());
export const AppsV1Api = jest.fn().mockImplementation(() => createAppsV1ApiMock());
export const NetworkingV1Api = jest.fn().mockImplementation(() => createNetworkingV1ApiMock());
export const BatchV1Api = jest.fn().mockImplementation(() => createBatchV1ApiMock());
export const KubernetesObjectApi = jest.fn().mockImplementation(() => ({
  ...mockKubernetesObjectApi,
}));

// Attach static methods to KubernetesObjectApi
(KubernetesObjectApi as any).makeApiClient = jest.fn().mockImplementation(() => ({
  ...mockKubernetesObjectApi,
}));

// Additional utilities
export const Config = {
  defaultClient: mockKubeConfig,
  fromKubeconfig: jest.fn().mockReturnValue(mockKubeConfig),
};

// Watch API mock
export const Watch = jest.fn().mockImplementation(() => ({
  watch: jest.fn().mockImplementation((path, params, eventType, handler) => {
    // Simulate watch events
    setTimeout(() => {
      handler('ADDED', { metadata: { name: 'watched-resource' } });
    }, 10);
    return Promise.resolve({ abort: jest.fn() });
  }),
}));

// Metrics API mock
export const Metrics = jest.fn().mockImplementation(() => ({
  getPodMetrics: jest.fn().mockResolvedValue({
    items: [
      {
        metadata: { name: 'pod-1' },
        containers: [{ name: 'main', usage: { cpu: '10m', memory: '64Mi' } }],
      },
    ],
  }),
}));
