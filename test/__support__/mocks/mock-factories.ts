/**
 * Simple Mock Factories - De-Enterprise Refactoring
 *
 * Replaces MockRegistry (287 lines) + MockFactories (1,473 lines) enterprise patterns
 * with simple object factories (~300 lines total).
 * Removes complex initialization/cleanup lifecycles, configuration methods, and scoped registries.
 */

import { jest } from '@jest/globals';
import { randomBytes } from 'node:crypto';

/**
 * Generate a simple random ID (replaces nanoid dependency for testing)
 */
function generateId(length = 21): string {
  return randomBytes(Math.ceil(length / 2))
    .toString('hex')
    .slice(0, length);
}

/**
 * Simple Docker mock - no complex factory patterns
 */
export const mockDocker = () => ({
  buildImage: jest.fn().mockResolvedValue({ 
    imageId: 'sha256:test123',
    tags: ['test:latest'],
    logs: ['Step 1/5 : FROM node:18'],
    success: true,
  }),
  
  pushImage: jest.fn().mockResolvedValue({
    registry: 'docker.io',
    repository: 'test/app',
    tag: 'latest',
    digest: 'sha256:abc123',
    success: true,
  }),
  
  tagImage: jest.fn().mockResolvedValue({
    sourceImage: 'sha256:test123',
    targetTag: 'test:v1.0',
    success: true,
  }),
  
  ping: jest.fn().mockResolvedValue({ ok: true }),
  
  info: jest.fn().mockResolvedValue({
    containers: 5,
    images: 10,
    serverVersion: '20.10.0',
  }),
});

/**
 * Simple Kubernetes mock - no complex client patterns
 */
export const mockK8s = () => ({
  applyManifest: jest.fn().mockResolvedValue({ success: true }),

  getDeploymentStatus: jest.fn().mockResolvedValue({
    ready: true,
    readyReplicas: 3,
    totalReplicas: 3,
  }),

  listPods: jest.fn().mockResolvedValue([
    { name: 'test-pod-1', status: 'Running' },
    { name: 'test-pod-2', status: 'Running' },
  ]),

  ping: jest.fn().mockResolvedValue(true),
  namespaceExists: jest.fn().mockResolvedValue(true),
  checkPermissions: jest.fn().mockResolvedValue(true),
  checkIngressController: jest.fn().mockResolvedValue(true),
});

/**
 * Simple security scanner mock - no strategy patterns
 */
export const mockScanner = () => ({
  scanImage: jest.fn().mockResolvedValue({
    vulnerabilities: {
      critical: 0,
      high: 1,
      medium: 3,
      low: 5,
      unknown: 0,
      total: 9,
    },
    scanTime: new Date().toISOString(),
    passed: true,
  }),
  
  ping: jest.fn().mockResolvedValue({ ok: true }),
});

/**
 * Simple workflow results - no factory complexity
 */
export const mockAnalysisResult = () => ({
  language: 'javascript',
  language_version: '18.0.0',
  framework: 'express',
  framework_version: '4.18.0',
  build_system: { name: 'npm', version: '8.0.0' },
  dependencies: ['express@4.18.0', 'cors@2.8.5'],
  projectType: 'web-app',
  hasDockerfile: false,
  hasPackageJson: true,
  estimatedBuildTime: 120,
});

export const mockDockerfile = () => ({
  content: `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
EXPOSE 3000
USER node
CMD ["npm", "start"]`,
  size: 150,
  layers: 8,
  estimatedSize: 250 * 1024 * 1024, // 250MB
});

export const mockBuildResult = () => ({
  imageId: `sha256:${generateId()}`,
  tag: 'test:latest',
  tags: ['test:latest', 'test:v1.0'],
  size: 245 * 1024 * 1024, // 245MB  
  layers: 8,
  buildTime: 45000, // 45 seconds
  logs: [
    'Step 1/8 : FROM node:18-alpine',
    'Step 2/8 : WORKDIR /app',
    'Step 3/8 : COPY package*.json ./',
    'Step 4/8 : RUN npm ci --only=production',
  ],
  success: true,
});

export const mockScanResult = () => ({
  vulnerabilities: [
    {
      id: 'CVE-2023-1234',
      severity: 'HIGH' as const,
      package: 'npm',
      version: '8.0.0',
      fixedVersion: '8.1.0',
      description: 'Vulnerability in npm package manager',
    },
  ],
  summary: {
    critical: 0,
    high: 1,
    medium: 2,
    low: 3,
    unknown: 0,
    total: 6,
  },
  scanTime: new Date().toISOString(),
  metadata: {
    image: 'sha256:test123',
    scanner: 'trivy',
    version: '0.35.0',
  },
});

export const mockK8sManifest = () => ({
  apiVersion: 'apps/v1',
  kind: 'Deployment',
  metadata: {
    name: 'test-app',
    namespace: 'default',
  },
  spec: {
    replicas: 3,
    selector: {
      matchLabels: { app: 'test-app' },
    },
    template: {
      metadata: {
        labels: { app: 'test-app' },
      },
      spec: {
        containers: [{
          name: 'test-app',
          image: 'test:latest',
          ports: [{ containerPort: 3000 }],
          resources: {
            limits: { cpu: '500m', memory: '512Mi' },
            requests: { cpu: '100m', memory: '128Mi' },
          },
          livenessProbe: {
            httpGet: { path: '/health', port: 3000 },
            initialDelaySeconds: 30,
          },
          readinessProbe: {
            httpGet: { path: '/ready', port: 3000 },
            initialDelaySeconds: 5,
          },
        }],
        securityContext: {
          runAsNonRoot: true,
          runAsUser: 1000,
        },
      },
    },
  },
});

/**
 * Simple logger mock - no complex logging framework
 */
export const mockLogger = () => ({
  debug: jest.fn(),
  info: jest.fn(),
  warn: jest.fn(),
  error: jest.fn(),
  trace: jest.fn(),
  child: jest.fn().mockReturnThis(),
});

/**
 * Quick setup for common test scenarios
 */
export const setupMockFactories = () => ({
  docker: mockDocker(),
  k8s: mockK8s(),
  scanner: mockScanner(),
  logger: mockLogger(),
});

/**
 * Setup mocks for specific failure scenarios
 */
export const setupFailureMocks = () => ({
  docker: {
    ...mockDocker(),
    buildImage: jest.fn().mockRejectedValue(new Error('Docker build failed')),
  },
  k8s: {
    ...mockK8s(),
    applyManifest: jest.fn().mockRejectedValue(new Error('K8s apply failed')),
  },
  scanner: {
    ...mockScanner(),
    scanImage: jest.fn().mockRejectedValue(new Error('Security scan failed')),
  },
  logger: mockLogger(),
});

/**
 * Setup mocks for network error scenarios
 */
export const setupNetworkErrorMocks = () => {
  const networkError = new Error('getaddrinfo ENOTFOUND docker.io');
  (networkError as any).code = 'ENOTFOUND';

  return {
    docker: {
      ...mockDocker(),
      pushImage: jest.fn().mockRejectedValue(networkError),
      ping: jest.fn().mockRejectedValue(networkError),
    },
    k8s: {
      ...mockK8s(),
      applyManifest: jest.fn().mockRejectedValue(networkError),
    },
    scanner: {
      ...mockScanner(),
      scanImage: jest.fn().mockRejectedValue(networkError),
    },
    logger: mockLogger(),
  };
};

// ===== VS CODE MOCK FACTORIES =====

/**
 * Interface for cancellation token test trigger.
 */
export interface MockCancellationToken {
  isCancellationRequested: boolean;
  onCancellationRequested: (listener: () => void) => { dispose: () => void };
  /** Test helper to trigger cancellation - not part of real CancellationToken */
  _trigger: () => void;
}

/**
 * Create a mock VS Code CancellationToken.
 *
 * @param options.cancelled - Whether the token should start in cancelled state
 * @returns Mock token with _trigger helper for testing
 *
 * @example
 * ```typescript
 * const mockToken = createMockCancellationToken();
 * const { signal, dispose } = createAbortSignalFromToken(mockToken);
 *
 * expect(signal.aborted).toBe(false);
 * mockToken._trigger();
 * expect(signal.aborted).toBe(true);
 *
 * dispose();
 * ```
 */
export function createMockCancellationToken(
  options: { cancelled?: boolean } = {}
): MockCancellationToken {
  let listener: (() => void) | null = null;

  return {
    isCancellationRequested: options.cancelled ?? false,
    onCancellationRequested: (callback: () => void) => {
      listener = callback;
      return { dispose: () => { listener = null; } };
    },
    _trigger: () => { listener?.(); },
  };
}

/**
 * Create mock invocation options for a tool.
 *
 * @param input - The tool input parameters
 * @returns Mock invocation options object
 */
export function createMockInvocationOptions<T>(input: T): { input: T } {
  return { input };
}