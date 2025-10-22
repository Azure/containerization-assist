/**
 * Integration tests for Kubernetes fast-fail behavior
 *
 * Tests that the system fails fast with helpful errors when:
 * - Kubeconfig is missing
 * - Kubeconfig is invalid
 * - Cluster is unreachable
 */

import * as fs from 'node:fs';
import * as path from 'node:path';
import pino from 'pino';
import { jest } from '@jest/globals';
import { createTestTempDir } from '../__support__/utilities/tmp-helpers';
import type { DirResult } from 'tmp';
import { createKubernetesClient } from '@/infra/kubernetes/client';
import { discoverAndValidateKubeconfig } from '@/infra/kubernetes/kubeconfig-discovery';

// Mock os module at the top
jest.mock('node:os', () => {
  const actualOs = jest.requireActual('node:os') as typeof import('node:os');
  return {
    ...actualOs,
    homedir: jest.fn(() => actualOs.homedir()),
  };
});

describe('Kubernetes fast-fail integration', () => {
  const logger = pino({ level: 'silent' });
  const originalEnv = process.env.KUBECONFIG;

  beforeEach(() => {
    delete process.env.KUBECONFIG;
    jest.restoreAllMocks();
  });

  afterEach(() => {
    if (originalEnv) {
      process.env.KUBECONFIG = originalEnv;
    } else {
      delete process.env.KUBECONFIG;
    }
    jest.restoreAllMocks();
  });

  describe('missing kubeconfig scenarios', () => {
    it('should fail fast when kubeconfig does not exist', async () => {
      const os = await import('node:os');
      // Mock home directory with no .kube/config
      const mockHome = '/tmp/nonexistent-home';
      jest.mocked(os.homedir).mockReturnValue(mockHome);

      expect(() => createKubernetesClient(logger)).toThrow(/kubeconfig.*not found/i);
    });

    it('should fail fast when KUBECONFIG env var points to missing file', () => {
      process.env.KUBECONFIG = '/tmp/completely-missing.yaml';

      expect(() => createKubernetesClient(logger)).toThrow('KUBECONFIG');
      expect(() => createKubernetesClient(logger)).toThrow('not found');
    });

    it('should provide actionable error message for missing kubeconfig', async () => {
      const os = await import('node:os');
      const mockHome = '/tmp/nonexistent-home';
      jest.mocked(os.homedir).mockReturnValue(mockHome);

      const result = discoverAndValidateKubeconfig();

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeTruthy();
        expect(result.guidance).toBeDefined();
        expect(result.guidance?.hint).toBeTruthy();
        expect(result.guidance?.resolution).toContain('kubectl');
      }
    });
  });

  describe('invalid kubeconfig scenarios', () => {
    let tempDir: DirResult;
    let cleanup: () => Promise<void>;
    let tempKubeconfig: string;

    beforeEach(() => {
      const result = createTestTempDir('kube-test-');
      tempDir = result.dir;
      cleanup = result.cleanup;
      tempKubeconfig = path.join(tempDir.name, 'config');
    });

    afterEach(async () => {
      await cleanup();
    });

    it('should fail fast with invalid YAML in kubeconfig', () => {
      fs.writeFileSync(tempKubeconfig, 'invalid: yaml: {{{{ broken');
      process.env.KUBECONFIG = tempKubeconfig;

      expect(() => createKubernetesClient(logger)).toThrow();
    });

    it('should fail fast when kubeconfig has no current context', () => {
      const invalidConfig = `
apiVersion: v1
kind: Config
clusters: []
users: []
contexts: []
current-context: ""
`;
      fs.writeFileSync(tempKubeconfig, invalidConfig);
      process.env.KUBECONFIG = tempKubeconfig;

      expect(() => createKubernetesClient(logger)).toThrow('current context');
    });

    it('should succeed with valid kubeconfig', () => {
      const validConfig = `
apiVersion: v1
kind: Config
clusters:
- name: test-cluster
  cluster:
    server: https://localhost:6443
    insecure-skip-tls-verify: true
users:
- name: test-user
  user:
    token: test-token
contexts:
- name: test-context
  context:
    cluster: test-cluster
    user: test-user
current-context: test-context
`;
      fs.writeFileSync(tempKubeconfig, validConfig);
      process.env.KUBECONFIG = tempKubeconfig;

      // Should not throw during creation
      expect(() => createKubernetesClient(logger)).not.toThrow();
    });
  });

  describe('unreachable cluster scenarios', () => {
    let tempDir: DirResult;
    let cleanup: () => Promise<void>;
    let tempKubeconfig: string;

    beforeEach(() => {
      const result = createTestTempDir('kube-test-');
      tempDir = result.dir;
      cleanup = result.cleanup;
      tempKubeconfig = path.join(tempDir.name, 'config');

      // Create a valid kubeconfig pointing to unreachable server
      const config = `
apiVersion: v1
kind: Config
clusters:
- name: unreachable-cluster
  cluster:
    server: https://10.255.255.255:6443
    insecure-skip-tls-verify: true
users:
- name: test-user
  user:
    token: test-token
contexts:
- name: test-context
  context:
    cluster: unreachable-cluster
    user: test-user
current-context: test-context
`;
      fs.writeFileSync(tempKubeconfig, config);
      process.env.KUBECONFIG = tempKubeconfig;
    });

    afterEach(async () => {
      await cleanup();
    });

    it('should detect unreachable cluster via ping', async () => {
      const client = createKubernetesClient(logger, undefined, 1000);
      const isReachable = await client.ping();

      expect(isReachable).toBe(false);
    }, 10000);

    it('should timeout quickly for unreachable cluster', async () => {
      const client = createKubernetesClient(logger, undefined, 2000);
      const startTime = Date.now();

      const isReachable = await client.ping();
      const elapsed = Date.now() - startTime;

      expect(isReachable).toBe(false);
      // Should timeout within configured time plus small buffer
      expect(elapsed).toBeLessThan(3000);
    }, 10000);

    it('should provide guidance for connection timeout', async () => {
      const client = createKubernetesClient(logger, undefined, 1000);

      // Capture log output
      const logMessages: Array<{ error: string; hint?: string; resolution?: string }> = [];
      const testLogger = pino({
        level: 'debug',
        hooks: {
          logMethod(args, method) {
            logMessages.push(args[0] as { error: string; hint?: string; resolution?: string });
            return method.apply(this, args);
          },
        },
      });

      const clientWithLogger = createKubernetesClient(testLogger, undefined, 1000);
      await clientWithLogger.ping();

      // Should have logged guidance
      const guidanceLog = logMessages.find((log) => log.hint || log.resolution);
      expect(guidanceLog).toBeDefined();
      if (guidanceLog) {
        expect(guidanceLog.resolution).toBeTruthy();
      }
    }, 10000);
  });

  describe('kubeconfig file permissions', () => {
    let tempDir: DirResult;
    let cleanup: () => Promise<void>;
    let tempKubeconfig: string;

    beforeEach(() => {
      const result = createTestTempDir('kube-test-');
      tempDir = result.dir;
      cleanup = result.cleanup;
      tempKubeconfig = path.join(tempDir.name, 'config');
    });

    afterEach(async () => {
      if (fs.existsSync(tempKubeconfig)) {
        // Restore permissions before cleanup
        try {
          fs.chmodSync(tempKubeconfig, 0o644);
        } catch (e) {
          // Ignore
        }
      }
      await cleanup();
    });

    it('should fail fast when kubeconfig is not readable', () => {
      // Create file with no read permissions
      const validConfig = `
apiVersion: v1
kind: Config
clusters:
- name: test-cluster
  cluster:
    server: https://localhost:6443
users:
- name: test-user
  user:
    token: test-token
contexts:
- name: test-context
  context:
    cluster: test-cluster
    user: test-user
current-context: test-context
`;
      fs.writeFileSync(tempKubeconfig, validConfig);

      // Make it unreadable (only works on Unix-like systems)
      if (process.platform !== 'win32') {
        fs.chmodSync(tempKubeconfig, 0o000);
        process.env.KUBECONFIG = tempKubeconfig;

        expect(() => createKubernetesClient(logger)).toThrow();
      }
    });
  });
});