/**
 * Unit Tests: Prepare Cluster Tool (advisory)
 *
 * The tool is read-only: it probes host/cluster state and returns a ClusterPlan
 * describing the commands and manifests the agent should run. These tests assert
 * the plan shape, not side effects.
 */

import { jest } from '@jest/globals';

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

// Mock Kubernetes client (read-only probe surface)
const mockK8sClient = {
  namespaceExists: jest.fn(),
};

const mockTimer = {
  end: jest.fn(),
  error: jest.fn(),
};

jest.mock('@/infra/kubernetes/client', () => ({
  createKubernetesClient: jest.fn(() => mockK8sClient),
}));

// Import after mocks are set up
import { prepareCluster } from '../../../src/tools/prepare-cluster/tool';

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

jest.mock('@/lib/platform', () => ({
  getSystemInfo: jest.fn(() => ({ isWindows: false, isMac: false, isLinux: true })),
  getDownloadOS: jest.fn(() => 'linux'),
  getDownloadArch: jest.fn(() => 'amd64'),
  mapNodeArchToPlatform: jest.fn(() => 'linux/amd64'),
  isPlatformCompatible: jest.fn(() => true),
}));

jest.mock('@/lib/port-utils', () => ({
  findRegistryPort: jest.fn(() => Promise.resolve(6000)),
  isPortAvailable: jest.fn(() => Promise.resolve(true)),
}));

jest.mock('node:child_process', () => ({
  exec: jest.fn(),
}));

// Closure-based execAsync mock; tests override (global as any).mockExecAsync.
jest.mock('node:util', () => {
  let execAsyncMock: any = null;

  return {
    promisify: jest.fn(() => {
      if (!execAsyncMock) {
        execAsyncMock = jest.fn(async () => ({ stdout: '', stderr: '' }));
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

/**
 * Default execAsync behavior for a "kind ready" host:
 * kind installed, cluster exists, registry running on port 6000, linux/amd64 nodes.
 */
function mockKindReadyExec() {
  (global as any).mockExecAsync.mockImplementation(async (cmd: string) => {
    if (cmd.includes('kubectl get nodes') && cmd.includes('architecture')) {
      return { stdout: 'amd64', stderr: '' };
    }
    if (cmd.includes('kubectl get nodes') && cmd.includes('operatingSystem')) {
      return { stdout: 'linux', stderr: '' };
    }
    if (cmd.includes('kind version')) {
      return { stdout: 'kind v0.20.0 go1.20.5 linux/amd64', stderr: '' };
    }
    if (cmd.includes('kind get clusters')) {
      return { stdout: 'containerization-assist\n', stderr: '' };
    }
    if (cmd.includes('docker ps') && cmd.includes('ca-registry')) {
      return { stdout: 'ca-registry', stderr: '' };
    }
    if (cmd.includes('docker inspect ca-registry')) {
      return { stdout: '6000', stderr: '' };
    }
    return { stdout: '', stderr: '' };
  });
}

/**
 * execAsync behavior for an "empty kind" host:
 * kind NOT installed, no cluster, no registry. kubectl node probe fails.
 */
function mockKindEmptyExec() {
  (global as any).mockExecAsync.mockImplementation(async (cmd: string) => {
    if (cmd.includes('kind version')) {
      throw new Error('kind: command not found');
    }
    if (cmd.includes('kubectl get nodes')) {
      throw new Error('no cluster');
    }
    if (cmd.includes('docker ps') && cmd.includes('ca-registry')) {
      return { stdout: '', stderr: '' };
    }
    return { stdout: '', stderr: '' };
  });
}

describe('prepareCluster (advisory)', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    if ((global as any).mockExecAsync) {
      (global as any).mockExecAsync.mockReset();
      (global as any).mockExecAsync.mockResolvedValue({ stdout: '', stderr: '' });
    }
  });

  describe('Plan shape', () => {
    beforeEach(() => {
      mockK8sClient.namespaceExists.mockResolvedValue(true);
      mockKindReadyExec();
    });

    it('returns a review-and-decide plan with no files and a confidence score', async () => {
      const result = await prepareCluster(
        { clusterType: 'kind', namespace: 'default', targetPlatform: 'linux/amd64' },
        createMockToolContext(),
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const plan = result.value;
        expect(plan.nextAction.action).toBe('review-and-decide');
        expect(plan.nextAction.files).toEqual([]);
        expect(typeof plan.nextAction.instruction).toBe('string');
        expect(plan.confidence).toBeGreaterThan(0);
        expect(plan.confidence).toBeLessThanOrEqual(1);
        expect(typeof plan.summary).toBe('string');
      }
    });

    it('never mutates host state (no docker run / kind create / kubectl apply emitted as execAsync)', async () => {
      await prepareCluster(
        { clusterType: 'kind', namespace: 'default', targetPlatform: 'linux/amd64' },
        createMockToolContext(),
      );

      const calls = ((global as any).mockExecAsync.mock.calls as Array<[string]>).map((c) => c[0]);
      expect(calls.some((c) => c.startsWith('docker run'))).toBe(false);
      expect(calls.some((c) => c.includes('kind create cluster'))).toBe(false);
      expect(calls.some((c) => c.includes('kubectl apply'))).toBe(false);
      expect(calls.some((c) => c.includes('docker network connect kind'))).toBe(false);
    });
  });

  describe('Kind cluster — already prepared', () => {
    beforeEach(() => {
      mockK8sClient.namespaceExists.mockResolvedValue(true);
      mockKindReadyExec();
    });

    it('detects satisfied state and emits no required setup commands', async () => {
      const result = await prepareCluster(
        { clusterType: 'kind', namespace: 'default', targetPlatform: 'linux/amd64' },
        createMockToolContext(),
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const { detected, recommendations } = result.value;
        expect(result.value.clusterType).toBe('kind');
        expect(detected.kindInstalled).toBe(true);
        expect(detected.clusterExists).toBe(true);
        expect(detected.registryRunning).toBe(true);

        // Only optional commands (e.g. network connect, export kubeconfig) remain.
        const required = recommendations.setupCommands.filter((c) => !c.optional);
        const installCmd = required.find((c) => c.command.includes('kind.sigs.k8s.io/dl'));
        const runRegistry = required.find((c) => c.command.startsWith('docker run'));
        const createCluster = required.find((c) => c.command.includes('kind create cluster'));
        expect(installCmd).toBeUndefined();
        expect(runRegistry).toBeUndefined();
        expect(createCluster).toBeUndefined();
      }
    });

    it('always includes the local-registry-hosting ConfigMap manifest for kind', async () => {
      const result = await prepareCluster(
        { clusterType: 'kind', namespace: 'default', targetPlatform: 'linux/amd64' },
        createMockToolContext(),
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const configMap = result.value.recommendations.manifests.find(
          (m) => m.kind === 'ConfigMap' && m.namespace === 'kube-public',
        );
        expect(configMap).toBeDefined();
        expect(configMap?.yaml).toContain('local-registry-hosting');
      }
    });
  });

  describe('Kind cluster — nothing installed', () => {
    beforeEach(() => {
      mockK8sClient.namespaceExists.mockResolvedValue(false);
      mockKindEmptyExec();
    });

    it('emits install, registry-run, and cluster-create commands when state is unsatisfied', async () => {
      const result = await prepareCluster(
        { clusterType: 'kind', namespace: 'default', targetPlatform: 'linux/amd64' },
        createMockToolContext(),
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const { detected, recommendations } = result.value;
        expect(detected.kindInstalled).toBe(false);
        expect(detected.clusterExists).toBe(false);
        expect(detected.registryRunning).toBe(false);

        const commands = recommendations.setupCommands.map((c) => c.command);
        expect(commands.some((c) => c.includes('kind.sigs.k8s.io/dl'))).toBe(true);
        expect(commands.some((c) => c.startsWith('docker run'))).toBe(true);
        expect(commands.some((c) => c.includes('kind create cluster'))).toBe(true);
        expect(commands.some((c) => c.includes('kind export kubeconfig'))).toBe(true);
      }
    });

    it('embeds the kind cluster config (with registry mirror) in the create command', async () => {
      const result = await prepareCluster(
        { clusterType: 'kind', namespace: 'default', targetPlatform: 'linux/amd64' },
        createMockToolContext(),
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const createCmd = result.value.recommendations.setupCommands.find((c) =>
          c.command.includes('kind create cluster'),
        );
        expect(createCmd).toBeDefined();
        expect(createCmd?.command).toContain('containerdConfigPatches');
        expect(createCmd?.command).toContain('localhost:6000');
      }
    });

    it('marks the registry network-connect command as optional', async () => {
      const result = await prepareCluster(
        { clusterType: 'kind', namespace: 'default', targetPlatform: 'linux/amd64' },
        createMockToolContext(),
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const connectCmd = result.value.recommendations.setupCommands.find((c) =>
          c.command.includes('docker network connect kind'),
        );
        expect(connectCmd).toBeDefined();
        expect(connectCmd?.optional).toBe(true);
      }
    });
  });

  describe('Generic cluster', () => {
    beforeEach(() => {
      mockK8sClient.namespaceExists.mockResolvedValue(true);
      (global as any).mockExecAsync.mockImplementation(async (cmd: string) => {
        if (cmd.includes('kubectl get nodes') && cmd.includes('architecture')) {
          return { stdout: 'amd64', stderr: '' };
        }
        if (cmd.includes('kubectl get nodes') && cmd.includes('operatingSystem')) {
          return { stdout: 'linux', stderr: '' };
        }
        return { stdout: '', stderr: '' };
      });
    });

    it('does not probe kind/registry and emits a ServiceAccount manifest', async () => {
      const result = await prepareCluster(
        { clusterType: 'generic', namespace: 'app-ns', targetPlatform: 'linux/amd64' },
        createMockToolContext(),
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const { detected, recommendations } = result.value;
        expect(result.value.clusterType).toBe('generic');
        expect(detected.kindInstalled).toBe(false);
        expect(detected.clusterExists).toBe(false);
        expect(detected.registryRunning).toBe(false);

        const sa = recommendations.manifests.find(
          (m) => m.kind === 'ServiceAccount' && m.namespace === 'app-ns',
        );
        expect(sa).toBeDefined();

        // No kind probes should have run.
        const calls = ((global as any).mockExecAsync.mock.calls as Array<[string]>).map(
          (c) => c[0],
        );
        expect(calls.some((c) => c.includes('kind version'))).toBe(false);
        expect(calls.some((c) => c.includes('kind get clusters'))).toBe(false);
      }
    });

    it('emits an optional create-namespace command when the namespace is missing', async () => {
      mockK8sClient.namespaceExists.mockResolvedValue(false);

      const result = await prepareCluster(
        { clusterType: 'generic', namespace: 'missing-ns', targetPlatform: 'linux/amd64' },
        createMockToolContext(),
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const createNs = result.value.recommendations.setupCommands.find((c) =>
          c.command.includes('kubectl create namespace missing-ns'),
        );
        expect(createNs).toBeDefined();
        expect(createNs?.optional).toBe(true);
        expect(result.value.detected.namespaceExists).toBe(false);
      }
    });
  });

  describe('Cluster type inference', () => {
    it('infers kind from environment=development when clusterType omitted', async () => {
      mockK8sClient.namespaceExists.mockResolvedValue(true);
      mockKindReadyExec();

      const result = await prepareCluster(
        { environment: 'development', namespace: 'default', targetPlatform: 'linux/amd64' },
        createMockToolContext(),
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.clusterType).toBe('kind');
      }
    });

    it('infers generic from environment=production when clusterType omitted', async () => {
      mockK8sClient.namespaceExists.mockResolvedValue(true);

      const result = await prepareCluster(
        { environment: 'production', namespace: 'default', targetPlatform: 'linux/amd64' },
        createMockToolContext(),
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.clusterType).toBe('generic');
      }
    });

    it('lets explicit clusterType override environment-based inference', async () => {
      mockK8sClient.namespaceExists.mockResolvedValue(true);

      const result = await prepareCluster(
        {
          clusterType: 'generic',
          environment: 'development',
          namespace: 'override-ns',
          targetPlatform: 'linux/amd64',
        },
        createMockToolContext(),
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.clusterType).toBe('generic');
      }
    });
  });

  describe('Platform guidance', () => {
    it('reports compatible guidance when cluster platform matches target', async () => {
      mockK8sClient.namespaceExists.mockResolvedValue(true);
      mockKindReadyExec();

      const result = await prepareCluster(
        { clusterType: 'kind', namespace: 'default', targetPlatform: 'linux/amd64' },
        createMockToolContext(),
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const guidance = result.value.recommendations.platformGuidance;
        expect(guidance.target).toBe('linux/amd64');
        expect(guidance.cluster).toBe('linux/amd64');
        expect(guidance.compatible).toBe(true);
      }
    });

    it('reports undetected cluster platform when no cluster exists', async () => {
      mockK8sClient.namespaceExists.mockResolvedValue(false);
      mockKindEmptyExec();

      const result = await prepareCluster(
        { clusterType: 'kind', namespace: 'default', targetPlatform: 'linux/amd64' },
        createMockToolContext(),
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const guidance = result.value.recommendations.platformGuidance;
        expect(guidance.cluster).toBeNull();
        expect(guidance.compatible).toBe(false);
        expect(result.value.detected.clusterPlatform).toBeNull();
      }
    });
  });

  describe('Error handling', () => {
    it('returns a failure when the namespace is invalid', async () => {
      const result = await prepareCluster(
        { clusterType: 'generic', namespace: 'Invalid_NS', targetPlatform: 'linux/amd64' },
        createMockToolContext(),
      );

      expect(result.ok).toBe(false);
    });
  });
});
