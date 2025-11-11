import {
  AnalysisResult,
  DockerBuildResult,
  DockerfileResult,
  ScanResult,
  K8sManifestResult,
  DeploymentResult,
} from '../../../src/types';

import type { Logger } from '../../../src/lib/logger';
import type { Result } from '../../../src/types';
import type { AppConfig } from '../../../src/config/app-config';
import { jest } from '@jest/globals';



export function createMockAnalysisResult(overrides?: Partial<AnalysisResult>): AnalysisResult {
  return {
    language: 'javascript',
    language_version: '18.0.0',
    framework: 'express',
    framework_version: '4.18.0',
    build_system: {
      type: 'npm',
      build_file: 'package.json',
      build_command: 'npm run build',
    },
    dependencies: [
      { name: 'express', version: '4.18.0', type: 'runtime' },
      { name: 'pino', version: '8.0.0', type: 'runtime' },
      { name: 'zod', version: '3.21.0', type: 'runtime' },
      { name: '@/types/node', version: '18.0.0', type: 'dev' },
    ],
    has_tests: true,
    test_framework: 'jest',
    required_ports: [3000],
    env_variables: {
      NODE_ENV: 'production',
      PORT: '3000',
    },
    docker_compose_exists: false,
    ...overrides,
  };
}

export function createMockDockerfileResult(
  overrides?: Partial<DockerfileResult>,
): DockerfileResult {
  return {
    content: `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
EXPOSE 3000
CMD ["node", "index"]`,
    path: './Dockerfile',
    base_image: 'node:18-alpine',
    stages: ['production'],
    optimizations: ['multistage', 'layer-caching'],
    multistage: false,
    ...overrides,
  };
}

export function createMockDockerBuildResult(
  overrides?: Partial<DockerBuildResult>,
): DockerBuildResult {
  return {
    image_id: `sha256:${'a'.repeat(64)}`,
    image_tag: 'test-app:latest',
    size_bytes: 52428800, // 50MB
    layers: [
      { id: `sha256:${'b'.repeat(64)}`, size: 5242880, command: 'FROM node:18-alpine' },
      { id: `sha256:${'c'.repeat(64)}`, size: 1048576, command: 'WORKDIR /app' },
      { id: `sha256:${'d'.repeat(64)}`, size: 41943040, command: 'RUN npm ci' },
      { id: `sha256:${'e'.repeat(64)}`, size: 4194304, command: 'COPY . .' },
    ],
    build_duration_ms: 45000,
    build_args: {},
    cache_used: true,
    ...overrides,
  };
}

export function createMockScanResult(overrides?: Partial<ScanResult>): ScanResult {
  return {
    scanner: 'trivy',
    vulnerabilities: [
      {
        id: 'CVE-2023-1234',
        severity: 'high',
        package: 'openssl',
        version: '1.1.1k',
        fixed_version: '1.1.1l',
        description: 'Buffer overflow in OpenSSL',
      },
      {
        id: 'CVE-2023-5678',
        severity: 'medium',
        package: 'zlib',
        version: '1.2.11',
        fixed_version: '1.2.12',
        description: 'Memory corruption in zlib',
      },
    ],
    summary: {
      critical: 0,
      high: 1,
      medium: 1,
      low: 0,
      total: 2,
    },
    scan_duration_ms: 12000,
    ...overrides,
  };
}

export function createMockK8sManifestResult(
  overrides?: Partial<K8sManifestResult>,
): K8sManifestResult {
  return {
    manifests: [
      {
        kind: 'Deployment',
        name: 'test-app',
        namespace: 'default',
        content: `apiVersion: apps/v1
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
        image: test-app:latest
        ports:
        - containerPort: 3000`,
        file_path: './test/fixtures/k8s/deployment.yaml',
      },
      {
        kind: 'Service',
        name: 'test-app-service',
        namespace: 'default',
        content: `apiVersion: v1
kind: Service
metadata:
  name: test-app-service
  namespace: default
spec:
  selector:
    app: test-app
  ports:
  - port: 80
    targetPort: 3000
  type: ClusterIP`,
        file_path: './test/fixtures/k8s/service.yaml',
      },
    ],
    deployment_strategy: 'rolling',
    replicas: 2,
    resources: {
      requests: {
        cpu: '100m',
        memory: '128Mi',
      },
      limits: {
        cpu: '500m',
        memory: '512Mi',
      },
    },
    ...overrides,
  };
}

export function createMockDeploymentResult(
  overrides?: Partial<DeploymentResult>,
): DeploymentResult {
  return {
    namespace: 'default',
    deployment_name: 'test-app',
    service_name: 'test-app-service',
    endpoints: [
      {
        type: 'internal',
        url: 'http://test-app-service.default.svc.cluster.local',
        port: 80,
      },
    ],
    status: {
      ready_replicas: 2,
      total_replicas: 2,
      conditions: [
        {
          type: 'Available',
          status: 'True',
          reason: 'MinimumReplicasAvailable',
          message: 'Deployment has minimum availability.',
        },
      ],
    },
    deployment_duration_ms: 30000,
    ready: true,
    ...overrides,
  };
}

export const SAMPLE_DOCKERFILES = {
  node: `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
EXPOSE 3000
CMD ["node", "index"]`,

  python: `FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 8000
CMD ["python", "app.py"]`,

  multistage: `FROM node:18-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:18-alpine
WORKDIR /app
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/package*.json ./
RUN npm ci --only=production
EXPOSE 3000
CMD ["node", "dist/index"]`,
};

export const VALID_TOOL_INPUTS = {
  analyze_repository: {
    repo_path: '/test/repo',
    deep_scan: false,
  },
  generate_dockerfile: {
    base_image: 'node:18-alpine',
    port: 3000,
  },
  build_image: {
    dockerfile_path: './Dockerfile',
    image_name: 'test-app',
    tag: 'latest',
  },
  scan_image: {
    scanner: 'trivy',
  },
  tag_image: {
    tags: ['latest', 'v1.0.0'],
    registry: 'docker.io',
  },
  push_image: {
  },
  generate_k8s_manifests: {
    namespace: 'default',
    replicas: 2,
  },
  prepare_cluster: {
    cluster_name: 'test-cluster',
    namespace: 'default',
  },
  verify_deploy: {
  },
};

export const INVALID_TOOL_INPUTS = {
  analyze_repository: {
    deep_scan: 'not-a-boolean', // Wrong type
  },
  generate_dockerfile: {
    port: 'not-a-number', // Wrong type
  },
  build_image: {
    dockerfile_path: null, // Wrong type
    image_name: '',
    tag: [],
  },
};
/**
 * Mock Logger Implementation
 */
export function createMockLogger(): jest.Mocked<Logger> {
  const mockLogger = {
    debug: jest.fn(),
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    trace: jest.fn(),
    fatal: jest.fn(),
    child: jest.fn(),
  } as jest.Mocked<Logger>;

  // Make child return a new mock logger with the same interface
  mockLogger.child.mockImplementation(() => mockLogger);

  return mockLogger;
}

/**
 * Mock Configuration Factory
 */
export function createMockConfig(overrides?: Partial<AppConfig>): AppConfig {
  return {
    server: {
      nodeEnv: 'test',
      logLevel: 'error',
      port: 3000,
      host: 'localhost',
    },
    mcp: {
      name: 'containerization-assist',
      version: '1.0.0',
      storePath: ':memory:',
      enableMetrics: false,
      enableEvents: false,
    },
    services: {
      docker: {
        socketPath: '/var/run/docker.sock',
        host: 'localhost',
        port: 2375,
        registry: 'docker.io',
        timeout: 30000,
        buildArgs: {},
      },
      kubernetes: {
        namespace: 'test',
        kubeconfig: '',
        timeout: 30000,
      },
    },
    workspace: {
      workspaceDir: '/tmp/test',
      tempDir: '/tmp/test/tmp',
      maxFileSize: 1048576,
    },
    logging: {
      level: 'error',
    },
    cache: {
      ttl: 3600,
      maxSize: 100,
    },
    security: {
      scanTimeout: 30000,
      failOnCritical: false,
    },
    ...overrides,
  };
}

/**
 * Test helpers for async result patterns
 */
export async function expectOk<T>(resultPromise: Promise<Result<T>>): Promise<T> {
  const result = await resultPromise;
  expect(result.kind).toBe('ok');
  if (result.kind === 'ok') {
    return result.value;
  }
  throw new Error('Expected Ok result');
}

export async function expectFail<T>(resultPromise: Promise<Result<T>>): Promise<string> {
  const result = await resultPromise;
  expect(result.kind).toBe('fail');
  if (result.kind === 'fail') {
    return result.error;
  }
  throw new Error('Expected Fail result');
}

/**
 * Mock Core Services Implementation
 */
export function createMockCoreServices(): {
  docker: any;
  kubernetes: any;
  ai: any;
  logger: jest.Mocked<Logger>;
  progress?: any;
} {
  const mockLogger = createMockLogger();

  return {
    docker: {
      build: jest.fn().mockResolvedValue(createMockDockerBuildResult()),
      scan: jest.fn().mockResolvedValue(createMockScanResult()),
      push: jest.fn().mockResolvedValue(undefined),
      tag: jest.fn().mockResolvedValue(undefined),
      health: jest.fn().mockResolvedValue({ healthy: true, version: '20.10.17' }),
      initialize: jest.fn().mockResolvedValue(undefined),
    },
    kubernetes: {
      deploy: jest.fn().mockResolvedValue({ success: true, resources: [] }),
      generateManifests: jest.fn().mockResolvedValue([]),
      checkClusterAccess: jest.fn().mockResolvedValue(true),
      verifyDeployment: jest.fn().mockResolvedValue({ ready: true }),
      prepareCluster: jest.fn().mockResolvedValue(undefined),
      initialize: jest.fn().mockResolvedValue(undefined),
    },
    ai: {
      generateDockerfile: jest
        .fn()
        .mockResolvedValue('FROM node:18-alpine\nWORKDIR /app\nCMD ["node", "index"]'),
      enhanceManifests: jest.fn().mockImplementation((manifests) => Promise.resolve(manifests)),
      analyzeRepository: jest.fn().mockResolvedValue(createMockAnalysisResult()),
      fixDockerfile: jest
        .fn()
        .mockResolvedValue('FROM node:18-alpine\nWORKDIR /app\nCMD ["node", "index"]'),
      isAvailable: jest.fn().mockReturnValue(true),
      initialize: jest.fn().mockResolvedValue(undefined),
    },
    logger: mockLogger,
    progress: {
      emit: jest.fn().mockResolvedValue(undefined),
    },
  };
}

/**
 * Individual mock service factories
 */
export function createMockDockerClient() {
  return {
    build: jest.fn().mockResolvedValue(createMockDockerBuildResult()),
    buildImage: jest.fn().mockResolvedValue(createMockDockerBuildResult()),
    getImage: jest.fn().mockResolvedValue({
      ok: true,
      value: {
        Id: 'sha256:mock-image-id',
        RepoTags: ['test:latest'],
        Size: 52428800,
        Created: Date.now() / 1000,
      },
    }),
    inspectImage: jest.fn().mockResolvedValue({
      ok: true,
      value: {
        Id: 'sha256:mock-image-id',
        RepoTags: ['test:latest'],
        Size: 52428800,
        Created: Date.now() / 1000,
      },
    }),
    scan: jest.fn().mockResolvedValue(createMockScanResult()),
    push: jest.fn().mockResolvedValue(undefined),
    tag: jest.fn().mockResolvedValue(undefined),
    tagImage: jest.fn().mockResolvedValue({ ok: true, value: undefined }),
    pushImage: jest.fn().mockResolvedValue({ ok: true, value: { digest: 'sha256:mock-digest' } }),
    health: jest.fn().mockResolvedValue({ healthy: true, version: '20.10.17' }),
    initialize: jest.fn().mockResolvedValue(undefined),
  };
}

/**
 * Enhanced ESM-Compatible Dockerode Mock Factory
 * Complete Docker API surface coverage for robust testing
 */
export function createMockDockerode() {
  const mockImage = (imageId: string = 'sha256:mock-image-id') => ({
    id: imageId,
    tag: jest.fn().mockImplementation((options: any, callback?: any) => {
      if (callback) {
        callback(null);
      } else {
        return Promise.resolve();
      }
    }),
    push: jest.fn().mockImplementation((options: any, callback?: any) => {
      const mockStream = {
        on: jest.fn().mockImplementation((event: string, handler: any) => {
          if (event === 'data') {
            // Simulate push progress events
            setTimeout(() => {
              handler(
                Buffer.from(
                  JSON.stringify({ status: 'Pushing', id: 'layer1', progress: '[=>    ]' }),
                ),
              );
              handler(Buffer.from(JSON.stringify({ status: 'Pushed', id: 'layer1' })));
            }, 5);
          }
          if (event === 'end') {
            setTimeout(handler, 10);
          }
          if (event === 'error') {
            // Don't trigger error by default
          }
          return mockStream;
        }),
        pipe: jest.fn(),
        removeAllListeners: jest.fn(),
        destroy: jest.fn(),
      } as any;

      if (callback) {
        callback(null, mockStream);
      } else {
        return Promise.resolve(mockStream);
      }
    }),
    inspect: jest.fn().mockResolvedValue({
      Id: imageId,
      RepoTags: ['test:latest'],
      RepoDigests: [],
      Size: 52428800,
      Created: Date.now() / 1000,
      Config: {
        ExposedPorts: { '3000/tcp': {} },
        Env: ['NODE_ENV=production'],
        Cmd: ['node', 'index.js'],
        WorkingDir: '/app',
        Labels: {},
      },
      Architecture: 'amd64',
      Os: 'linux',
      RootFS: {
        Type: 'layers',
        Layers: ['sha256:layer1', 'sha256:layer2'],
      },
    }),
    remove: jest.fn().mockResolvedValue([{ Deleted: imageId }]),
    history: jest
      .fn()
      .mockResolvedValue([
        {
          Id: 'layer1',
          Created: Date.now() / 1000,
          CreatedBy: '/bin/sh -c #(nop) FROM node:18',
          Size: 0,
        },
      ]),
    get: jest.fn().mockImplementation((imageId: string) => mockImage(imageId)),
  });

  const mockContainer = (containerId: string = 'mock-container-id') => ({
    id: containerId,
    start: jest.fn().mockResolvedValue(undefined),
    stop: jest.fn().mockResolvedValue(undefined),
    remove: jest.fn().mockResolvedValue(undefined),
    inspect: jest.fn().mockResolvedValue({
      Id: containerId,
      State: { Running: true, Status: 'running' },
      Config: { Image: 'test:latest' },
    }),
    logs: jest.fn().mockResolvedValue({} as NodeJS.ReadableStream),
    exec: jest.fn().mockResolvedValue({
      start: jest.fn().mockResolvedValue(undefined),
    }),
  });

  return {
    ping: jest.fn().mockResolvedValue(undefined),
    info: jest.fn().mockResolvedValue({
      OperatingSystem: 'Docker Desktop',
      Architecture: 'x86_64',
      Containers: 0,
      Images: 0,
      ServerVersion: '20.10.17',
      MemTotal: 8589934592,
      NCPU: 4,
    }),
    version: jest.fn().mockResolvedValue({
      Version: '20.10.17',
      ApiVersion: '1.41',
      MinAPIVersion: '1.12',
      GoVersion: 'go1.17.8',
    }),

    // Image operations
    buildImage: jest.fn().mockImplementation((context: any, options: any) => {
      const mockStream = {
        on: jest.fn().mockImplementation((event: string, handler: any) => {
          if (event === 'data') {
            // Simulate build progress
            setTimeout(() => {
              handler(Buffer.from(JSON.stringify({ stream: 'Step 1/5 : FROM node:18-alpine\n' })));
              handler(Buffer.from(JSON.stringify({ aux: { ID: 'sha256:mock-build-id' } })));
            }, 10);
          } else if (event === 'end') {
            setTimeout(handler, 20);
          }
          return mockStream;
        }),
        pipe: jest.fn(),
      } as any;

      return Promise.resolve(mockStream);
    }),

    listImages: jest.fn().mockResolvedValue([
      {
        Id: 'sha256:mock-image-1',
        RepoTags: ['test:latest'],
        Size: 52428800,
        Created: Date.now() / 1000,
      },
    ]),

    getImage: jest.fn().mockImplementation((imageId: string) => mockImage(imageId)),

    // Container operations
    listContainers: jest.fn().mockResolvedValue([
      {
        Id: 'mock-container-1',
        Names: ['/test-container'],
        Image: 'test:latest',
        State: 'running',
        Status: 'Up 2 minutes',
      },
    ]),

    getContainer: jest.fn().mockImplementation((containerId: string) => mockContainer(containerId)),

    createContainer: jest.fn().mockResolvedValue(mockContainer()),

    // Network operations
    listNetworks: jest
      .fn()
      .mockResolvedValue([{ Id: 'mock-network-1', Name: 'bridge', Driver: 'bridge' }]),

    // Volume operations
    listVolumes: jest.fn().mockResolvedValue({
      Volumes: [
        {
          Name: 'mock-volume-1',
          Driver: 'local',
          Mountpoint: '/var/lib/docker/volumes/mock-volume-1',
        },
      ],
    }),

    // System operations
    df: jest.fn().mockResolvedValue({
      LayersSize: 1000000,
      Images: [{ Id: 'mock-image-1', Size: 52428800 }],
      Containers: [{ Id: 'mock-container-1', SizeRw: 1024 }],
      Volumes: [{ Name: 'mock-volume-1', Size: 2048 }],
    }),

    // Enhanced modem with comprehensive progress handling
    modem: {
      followProgress: jest
        .fn()
        .mockImplementation((stream: any, onFinish: any, onProgress?: any) => {
          const events = [
            { status: 'Pulling from library/node', id: '18-alpine' },
            { status: 'Pull complete', id: '18-alpine' },
            { aux: { ID: 'sha256:mock-final-id' } },
          ];

          if (onProgress) {
            events.forEach((event, index) => {
              setTimeout(() => onProgress(event), index * 5);
            });
          }

          setTimeout(() => onFinish(null, events), events.length * 5 + 10);
        }),

      demuxStream: jest.fn().mockImplementation((stream: any, stdout: any, stderr: any) => {
        // Mock demux implementation for exec streams
        setTimeout(() => {
          if (stdout?.write) {
            stdout.write('Mock stdout output\n');
          }
        }, 5);
      }),
    },

    // Plugin operations (for completeness)
    listPlugins: jest.fn().mockResolvedValue([]),

    // Secret operations (for swarm mode)
    listSecrets: jest.fn().mockResolvedValue([]),

    // Config operations (for swarm mode)
    listConfigs: jest.fn().mockResolvedValue([]),
  };
}

export function createMockKubernetesClient() {
  return {
    deploy: jest.fn().mockResolvedValue({ success: true, resources: [] }),
    generateManifests: jest.fn().mockResolvedValue([]),
    checkClusterAccess: jest.fn().mockResolvedValue(true),
    verifyDeployment: jest.fn().mockResolvedValue({ ready: true }),
    prepareCluster: jest.fn().mockResolvedValue(undefined),
    initialize: jest.fn().mockResolvedValue(undefined),
    applyManifest: jest.fn().mockResolvedValue({ success: true, message: 'Applied successfully' }),
  };
}

/**
 * Comprehensive Kubernetes Client Mock Factory
 * Full k8s-client-node API coverage for robust testing
 */
export function createComprehensiveK8sMock() {
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
    loadFromFile: jest.fn(),
    loadFromString: jest.fn(),
    loadFromCluster: jest.fn(),
    makeApiClient: jest.fn().mockImplementation((ApiClass: any) => {
      const apiName = ApiClass.name || ApiClass.constructor?.name || '';
      switch (apiName) {
        case 'CoreV1Api':
          return createCoreV1ApiMock();
        case 'AppsV1Api':
          return createAppsV1ApiMock();
        case 'NetworkingV1Api':
          return createNetworkingV1ApiMock();
        case 'BatchV1Api':
          return createBatchV1ApiMock();
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

  return {
    KubeConfig: jest.fn().mockImplementation(() => mockKubeConfig),
    CoreV1Api: jest.fn().mockImplementation(() => createCoreV1ApiMock()),
    AppsV1Api: jest.fn().mockImplementation(() => createAppsV1ApiMock()),
    NetworkingV1Api: jest.fn().mockImplementation(() => createNetworkingV1ApiMock()),
    BatchV1Api: jest.fn().mockImplementation(() => createBatchV1ApiMock()),

    // Additional utilities
    Config: {
      defaultClient: mockKubeConfig,
      fromKubeconfig: jest.fn().mockReturnValue(mockKubeConfig),
    },

    // Watch API mock
    Watch: jest.fn().mockImplementation(() => ({
      watch: jest.fn().mockImplementation((path, params, eventType, handler) => {
        // Simulate watch events
        setTimeout(() => {
          handler('ADDED', { metadata: { name: 'watched-resource' } });
        }, 10);
        return Promise.resolve({ abort: jest.fn() });
      }),
    })),

    // Metrics API mock
    Metrics: jest.fn().mockImplementation(() => ({
      getPodMetrics: jest.fn().mockResolvedValue({
        items: [
          {
            metadata: { name: 'pod-1' },
            containers: [{ name: 'main', usage: { cpu: '10m', memory: '64Mi' } }],
          },
        ],
      }),
    })),
  };
}

export function createMockAIService() {
  return {
    generateDockerfile: jest
      .fn()
      .mockResolvedValue('FROM node:18-alpine\nWORKDIR /app\nCMD ["node", "index"]'),
    enhanceManifests: jest.fn().mockImplementation((manifests) => Promise.resolve(manifests)),
    analyzeRepository: jest.fn().mockResolvedValue(createMockAnalysisResult()),
    fixDockerfile: jest
      .fn()
      .mockResolvedValue('FROM node:18-alpine\nWORKDIR /app\nCMD ["node", "index"]'),
    isAvailable: jest.fn().mockReturnValue(true),
    initialize: jest.fn().mockResolvedValue(undefined),
  };
}

/**
 * Enhanced Infrastructure Mock Factories with ESM Support
 */

/**
 * DockerClient Mock Factory for service-level testing
 */
export function createMockDockerClientForService() {
  return {
    initialize: jest.fn().mockResolvedValue(undefined),
    build: jest.fn().mockResolvedValue({
      image_id: 'sha256:mock-build-result',
      image_tag: 'test-app:latest',
      size_bytes: 52428800,
      layers: [],
      build_duration_ms: 45000,
      build_args: {},
      cache_used: true,
    }),
    scan: jest.fn().mockResolvedValue({
      scanner: 'trivy',
      vulnerabilities: [],
      summary: { critical: 0, high: 0, medium: 0, low: 0, total: 0 },
      scan_duration_ms: 12000,
    }),
    tag: jest.fn().mockResolvedValue(undefined),
    push: jest.fn().mockResolvedValue({ digest: 'sha256:mock-digest' }),
    ping: jest.fn().mockResolvedValue(undefined),
    info: jest.fn().mockResolvedValue({
      os: 'Docker Desktop',
      arch: 'x86_64',
      containers: 0,
      images: 0,
      serverVersion: '20.10.17',
    }),

    // Additional methods needed by DockerService
    listImages: jest
      .fn()
      .mockResolvedValue([
        {
          Id: 'sha256:mock-image',
          RepoTags: ['test:latest'],
          Size: 100000,
          Created: Date.now() / 1000,
        },
      ]),
    removeImage: jest.fn().mockResolvedValue(undefined),
    imageExists: jest.fn().mockResolvedValue(true),
    listContainers: jest
      .fn()
      .mockResolvedValue([
        {
          Id: 'mock-container',
          Names: ['/test'],
          Image: 'test:latest',
          State: 'running',
          Status: 'Up 5 minutes',
        },
      ]),
    health: jest.fn().mockResolvedValue({
      available: true,
      version: '20.10.17',
      trivyAvailable: true,
      systemInfo: { os: 'Docker Desktop', arch: 'x86_64' },
    }),
  };
}

/**
 * Docker Service Mock Factory
 */
export function createMockDockerService() {
  return {
    initialize: jest.fn().mockResolvedValue(undefined),
    buildImage: jest.fn().mockResolvedValue(createMockDockerBuildResult()),
    scanImage: jest.fn().mockResolvedValue(createMockScanResult()),
    tagImage: jest.fn().mockResolvedValue(undefined),
    pushImage: jest.fn().mockResolvedValue(undefined),
    getSystemInfo: jest.fn().mockResolvedValue({
      os: 'Docker Desktop',
      arch: 'x86_64',
      containers: 0,
      images: 0,
      serverVersion: '20.10.17',
    }),
    isHealthy: jest.fn().mockResolvedValue(true),
  };
}

/**
 * TrivyScanner Mock Factory
 */
export function createMockTrivyScanner() {
  return {
    initialize: jest.fn().mockResolvedValue(undefined),
    scanImage: jest.fn().mockResolvedValue({
      scanner: 'trivy',
      vulnerabilities: [
        {
          id: 'CVE-2023-1234',
          severity: 'medium',
          package: 'test-package',
          version: '1.0.0',
          fixed_version: '1.0.1',
          description: 'Test vulnerability',
        },
      ],
      summary: { critical: 0, high: 0, medium: 1, low: 0, total: 1 },
      scan_duration_ms: 5000,
    }),
    isAvailable: jest.fn().mockReturnValue(true),
    getVersion: jest.fn().mockResolvedValue('0.45.0'),
  };
}

/**
 * Enhanced AI Service Mock Factory
 */
export function createMockAIServiceEnhanced() {
  return {
    // Core AI operations
    generateDockerfile: jest.fn().mockResolvedValue({
      content: createMockDockerfileResult().content,
      reasoning: 'Generated based on detected Node.js application',
      confidence: 0.95,
    }),

    analyzeRepository: jest.fn().mockResolvedValue(createMockAnalysisResult()),

    enhanceManifests: jest
      .fn()
      .mockImplementation((manifests) =>
        Promise.resolve(manifests.map((m: any) => ({ ...m, enhanced: true }))),
      ),

    fixDockerfile: jest.fn().mockResolvedValue({
      content: 'FROM node:18-alpine\nWORKDIR /app\nCMD ["node", "index"]',
      fixes: ['Added WORKDIR for better organization'],
      improved: true,
    }),

    // Structured generation with schemas
    generateStructured: jest.fn().mockImplementation((request: any, schema: any) => {
      // Return mock data that conforms to the expected schema
      if (schema.safeParse) {
        // Zod schema detected
        const mockData = {
          language: 'javascript',
          framework: 'express',
          dependencies: ['express', 'pino'],
        };
        return Promise.resolve(mockData);
      }
      return Promise.resolve({});
    }),

    // Service management
    initialize: jest.fn().mockResolvedValue(undefined),
    isAvailable: jest.fn().mockReturnValue(true),
    getModel: jest.fn().mockReturnValue('test-model'),
    getUsage: jest.fn().mockResolvedValue({
      requests: 42,
      tokens: 1337,
      cost: 0.05,
    }),
  };
}

/**
 * Mock Registry for centralized mock management
 */
export class MockRegistry {
  private static mocks = new Map<string, any>();

  static register(name: string, mock: any): void {
    this.mocks.set(name, mock);
  }

  static get<T>(name: string): T {
    return this.mocks.get(name) as T;
  }

  static reset(): void {
    // Reset all registered mocks
    this.mocks.forEach((mock) => {
      if (mock && typeof mock === 'object') {
        Object.values(mock).forEach((fn) => {
          if (jest.isMockFunction(fn)) {
            (fn as jest.Mock).mockReset();
          }
        });
      }
    });
  }

  static cleanup(): void {
    this.mocks.clear();
  }

  static setupDefaults(): void {
    this.register('dockerode', createMockDockerode());
    this.register('dockerClient', createMockDockerClientForService());
    this.register('dockerService', createMockDockerService());
    this.register('kubernetesClient', createMockKubernetesClient());
    this.register('trivyScanner', createMockTrivyScanner());
    this.register('aiService', createMockAIServiceEnhanced());
    this.register('logger', createMockLogger());
    this.register('config', createMockConfig());
  }
}

/**
 * Enhanced Test Environment Setup
 */
export function setupTestEnvironment() {
  MockRegistry.setupDefaults();

  return {
    beforeEach: () => {
      jest.clearAllMocks();
      MockRegistry.reset();
    },
    afterEach: () => {
      // Optional cleanup
    },
    afterAll: () => {
      MockRegistry.cleanup();
    },
  };
}

/**
 * Test data generators
 */
export const TestData = {
  workflowId: () => `test-workflow-${Date.now()}`,
  imageId: () => `sha256:${Math.random().toString(36).substring(2, 66)}`,
  imageName: (name = 'test-app') => `${name}:${Date.now()}`,
  timestamp: () => new Date().toISOString(),
  uuid: () => Math.random().toString(36).substring(2, 15),

  // Enhanced generators for infrastructure testing
  dockerImageInfo: () => ({
    Id: TestData.imageId(),
    RepoTags: [TestData.imageName()],
    Size: Math.floor(Math.random() * 100000000),
    Created: Date.now() / 1000,
  }),

  containerInfo: () => ({
    Id: TestData.uuid(),
    Names: [`/test-container-${TestData.uuid()}`],
    Image: TestData.imageName(),
    State: 'running',
    Status: 'Up 5 minutes',
  }),

  kubernetesResource: (kind = 'Deployment') => ({
    apiVersion: 'apps/v1',
    kind,
    metadata: {
      name: `test-${kind.toLowerCase()}`,
      namespace: 'default',
    },
    spec: {},
  }),
};
