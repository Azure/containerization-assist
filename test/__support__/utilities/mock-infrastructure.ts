import { jest } from '@jest/globals';
import type { Logger } from 'pino';

/**
 * Mock Infrastructure Factory
 * Provides mock implementations for infrastructure dependencies
 */
export function createMockInfrastructure() {
  return {
    docker: createMockDockerClient(),
    kubernetes: createMockKubernetesClient(),
    logger: createMockLogger(),
    filesystem: createMockFilesystem(),
  };
}

/**
 * Mock Docker Client
 */
export function createMockDockerClient() {
  return {
    buildImage: jest.fn().mockResolvedValue({ 
      imageId: 'sha256:mock-image-id',
      logs: ['Building image...', 'Image built successfully'] 
    }),
    pushImage: jest.fn().mockResolvedValue({ 
      digest: 'sha256:mock-digest',
      size: 123456789 
    }),
    tagImage: jest.fn().mockResolvedValue(true),
    listImages: jest.fn().mockResolvedValue([
      { id: 'image1', tags: ['app:latest'] },
      { id: 'image2', tags: ['app:v1.0'] }
    ]),
    removeImage: jest.fn().mockResolvedValue(true),
    pullImage: jest.fn().mockResolvedValue({ 
      layers: ['layer1', 'layer2'] 
    }),
    inspectImage: jest.fn().mockResolvedValue({
      Config: { ExposedPorts: { '3000/tcp': {} } },
      Size: 123456789
    }),
  };
}

/**
 * Mock Kubernetes Client
 */
export function createMockKubernetesClient() {
  return {
    applyManifest: jest.fn().mockResolvedValue({ 
      applied: true,
      resources: ['deployment/app', 'service/app'] 
    }),
    deleteManifest: jest.fn().mockResolvedValue({ 
      deleted: true,
      resources: ['deployment/app', 'service/app'] 
    }),
    getNamespace: jest.fn().mockResolvedValue({ 
      name: 'test-namespace',
      status: 'Active' 
    }),
    createNamespace: jest.fn().mockResolvedValue({ 
      name: 'test-namespace',
      created: true 
    }),
    listPods: jest.fn().mockResolvedValue([
      { name: 'app-pod-1', status: 'Running' },
      { name: 'app-pod-2', status: 'Running' }
    ]),
    getDeployment: jest.fn().mockResolvedValue({
      name: 'app',
      replicas: 2,
      ready: 2
    }),
  };
}

/**
 * Mock Logger
 */
export function createMockLogger(): Logger {
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

/**
 * Mock Filesystem
 */
export function createMockFilesystem(files: Record<string, string | object> = {}) {
  const mockFiles = new Map(
    Object.entries(files).map(([path, content]) => [
      path,
      typeof content === 'string' ? content : JSON.stringify(content, null, 2)
    ])
  );

  return {
    readFile: jest.fn().mockImplementation((path: string) => {
      if (mockFiles.has(path)) {
        return Promise.resolve(mockFiles.get(path));
      }
      return Promise.reject(new Error(`File not found: ${path}`));
    }),
    writeFile: jest.fn().mockImplementation((path: string, content: string) => {
      mockFiles.set(path, content);
      return Promise.resolve();
    }),
    exists: jest.fn().mockImplementation((path: string) => {
      return Promise.resolve(mockFiles.has(path));
    }),
    readdir: jest.fn().mockImplementation((path: string) => {
      const files = Array.from(mockFiles.keys())
        .filter(file => file.startsWith(path))
        .map(file => file.substring(path.length + 1).split('/')[0])
        .filter((file, index, arr) => arr.indexOf(file) === index);
      return Promise.resolve(files);
    }),
    mkdir: jest.fn().mockResolvedValue(undefined),
    stat: jest.fn().mockImplementation((path: string) => {
      if (mockFiles.has(path)) {
        return Promise.resolve({ 
          isFile: () => true,
          isDirectory: () => false,
          size: mockFiles.get(path)?.length || 0
        });
      }
      return Promise.reject(new Error(`File not found: ${path}`));
    }),
    addFile: (path: string, content: string | object) => {
      mockFiles.set(path, typeof content === 'string' ? content : JSON.stringify(content, null, 2));
    },
    getFile: (path: string) => mockFiles.get(path),
    getAllFiles: () => Object.fromEntries(mockFiles.entries()),
  };
}

/**
 * Result Type Helpers for Testing
 */
export function createSuccessResult<T>(value: T) {
  return {
    ok: true as const,
    value,
  };
}

export function createFailureResult(error: string) {
  return {
    ok: false as const,
    error,
  };
}

/**
 * Test Data Builders
 */
export function createMockPackageJson(overrides: any = {}) {
  return {
    name: 'test-app',
    version: '1.0.0',
    scripts: {
      start: 'node index.js',
      build: 'npm run build',
    },
    dependencies: {
      express: '^4.18.0',
    },
    ...overrides,
  };
}

export function createMockDockerfile(baseImage = 'node:18-alpine') {
  return `FROM ${baseImage}
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
EXPOSE 3000
CMD ["npm", "start"]`;
}

export function createMockKubernetesManifest(appName = 'test-app') {
  return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${appName}
spec:
  replicas: 2
  selector:
    matchLabels:
      app: ${appName}
  template:
    metadata:
      labels:
        app: ${appName}
    spec:
      containers:
      - name: ${appName}
        image: ${appName}:latest
        ports:
        - containerPort: 3000
---
apiVersion: v1
kind: Service
metadata:
  name: ${appName}
spec:
  selector:
    app: ${appName}
  ports:
  - port: 80
    targetPort: 3000
  type: ClusterIP`;
}

export {};