/**
 * Tests for kubeconfig discovery utilities
 */

import * as path from 'node:path';

// Mock modules first
jest.mock('node:fs');
jest.mock('node:os');
jest.mock('@kubernetes/client-node', () => ({
  KubeConfig: jest.fn(),
}));

import * as fs from 'node:fs';
import * as os from 'node:os';
import { KubeConfig } from '@kubernetes/client-node';
import {
  discoverKubeconfigPath,
  validateKubeconfig,
  discoverAndValidateKubeconfig,
  isInCluster,
} from '@/infra/kubernetes/kubeconfig-discovery';

// Get mocked versions
const mockedFs = jest.mocked(fs);
const mockedOs = jest.mocked(os);
const MockedKubeConfig = jest.mocked(KubeConfig);

describe('kubeconfig-discovery', () => {
  const originalEnv = process.env.KUBECONFIG;

  beforeEach(() => {
    jest.resetAllMocks();
    delete process.env.KUBECONFIG;
  });

  afterEach(() => {
    if (originalEnv) {
      process.env.KUBECONFIG = originalEnv;
    } else {
      delete process.env.KUBECONFIG;
    }
  });

  describe('discoverKubeconfigPath', () => {
    it('should find kubeconfig from KUBECONFIG env var', () => {
      const mockPath = '/tmp/test-kubeconfig';
      process.env.KUBECONFIG = mockPath;
      mockedFs.existsSync.mockReturnValue(true);

      const result = discoverKubeconfigPath();

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe(mockPath);
      }
    });

    it('should handle multiple paths in KUBECONFIG (colon-separated)', () => {
      const validPath = '/tmp/valid-kubeconfig';
      const invalidPath = '/tmp/invalid-kubeconfig';
      process.env.KUBECONFIG = `${invalidPath}:${validPath}`;

      mockedFs.existsSync.mockImplementation((p) => p === validPath);

      const result = discoverKubeconfigPath();

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe(validPath);
      }
    });

    it('should fail if KUBECONFIG points to non-existent file', () => {
      const mockPath = '/tmp/nonexistent-kubeconfig';
      process.env.KUBECONFIG = mockPath;
      mockedFs.existsSync.mockReturnValue(false);

      const result = discoverKubeconfigPath();

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('KUBECONFIG');
        expect(result.error).toContain('not found');
        expect(result.guidance).toBeDefined();
        expect(result.guidance?.hint).toBeDefined();
        expect(result.guidance?.resolution).toBeDefined();
      }
    });

    it('should find kubeconfig at default location ~/.kube/config', () => {
      const mockHome = '/home/testuser';
      const expectedPath = path.join(mockHome, '.kube', 'config');

      mockedOs.homedir.mockReturnValue(mockHome);
      mockedFs.existsSync.mockImplementation((p) => p === expectedPath);

      const result = discoverKubeconfigPath();

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe(expectedPath);
      }
    });

    it('should fail if no kubeconfig found', () => {
      const mockHome = '/home/testuser';
      mockedOs.homedir.mockReturnValue(mockHome);
      mockedFs.existsSync.mockReturnValue(false);

      const result = discoverKubeconfigPath();

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('not found');
        expect(result.guidance?.hint).toContain('KUBECONFIG');
        expect(result.guidance?.resolution).toContain('kubectl');
      }
    });
  });

  describe('validateKubeconfig', () => {
    beforeEach(() => {
      // Setup mock KubeConfig class
      const mockKubeConfig = {
        loadFromFile: jest.fn(),
        getCurrentContext: jest.fn(),
        getCurrentCluster: jest.fn(),
        getCurrentUser: jest.fn(),
      };
      MockedKubeConfig.mockImplementation(() => mockKubeConfig as any);
    });

    it('should fail if file does not exist', () => {
      const testPath = '/tmp/nonexistent.yaml';
      mockedFs.existsSync.mockReturnValue(false);

      const result = validateKubeconfig(testPath);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('does not exist');
        expect(result.guidance?.details).toHaveProperty('configPath', testPath);
      }
    });

    it('should fail if file is not readable', () => {
      const testPath = '/tmp/unreadable.yaml';
      mockedFs.existsSync.mockReturnValue(true);
      mockedFs.accessSync.mockImplementation(() => {
        throw new Error('Permission denied');
      });

      const result = validateKubeconfig(testPath);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('not readable');
        expect(result.guidance?.hint).toContain('permissions');
      }
    });

    it('should fail if kubeconfig parsing fails', () => {
      const testPath = '/tmp/invalid.yaml';
      mockedFs.existsSync.mockReturnValue(true);
      mockedFs.accessSync.mockReturnValue(undefined);

      const mockKc = new KubeConfig();
      (mockKc.loadFromFile as jest.Mock).mockImplementation(() => {
        throw new Error('Failed to parse');
      });

      const result = validateKubeconfig(testPath);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('parse');
      }
    });

    it('should fail if no current context is set', () => {
      const testPath = '/tmp/no-context.yaml';
      mockedFs.existsSync.mockReturnValue(true);
      mockedFs.accessSync.mockReturnValue(undefined);

      const mockKc = new KubeConfig();
      (mockKc.loadFromFile as jest.Mock).mockReturnValue(undefined);
      (mockKc.getCurrentContext as jest.Mock).mockReturnValue('');

      const result = validateKubeconfig(testPath);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('No current context');
      }
    });

    it('should succeed with valid kubeconfig', () => {
      const testPath = '/tmp/valid.yaml';
      mockedFs.existsSync.mockReturnValue(true);
      mockedFs.accessSync.mockReturnValue(undefined);

      const mockKc = new KubeConfig();
      (mockKc.loadFromFile as jest.Mock).mockReturnValue(undefined);
      (mockKc.getCurrentContext as jest.Mock).mockReturnValue('test-context');
      (mockKc.getCurrentCluster as jest.Mock).mockReturnValue({ name: 'test-cluster' });
      (mockKc.getCurrentUser as jest.Mock).mockReturnValue({ name: 'test-user' });

      const result = validateKubeconfig(testPath);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.path).toBe(testPath);
        expect(result.value.contextName).toBe('test-context');
        expect(result.value.clusterName).toBe('test-cluster');
        expect(result.value.user).toBe('test-user');
      }
    });
  });

  describe('discoverAndValidateKubeconfig', () => {
    beforeEach(() => {
      const mockKubeConfig = {
        loadFromFile: jest.fn(),
        getCurrentContext: jest.fn().mockReturnValue('test-context'),
        getCurrentCluster: jest.fn().mockReturnValue({ name: 'test-cluster' }),
        getCurrentUser: jest.fn().mockReturnValue({ name: 'test-user' }),
      };
      MockedKubeConfig.mockImplementation(() => mockKubeConfig as any);
    });

    it('should return discovery error if path not found', () => {
      mockedOs.homedir.mockReturnValue('/home/testuser');
      mockedFs.existsSync.mockReturnValue(false);

      const result = discoverAndValidateKubeconfig();

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('not found');
      }
    });

    it('should return validation error if file exists but is invalid', () => {
      const mockHome = '/home/testuser';
      const defaultPath = path.join(mockHome, '.kube', 'config');

      mockedOs.homedir.mockReturnValue(mockHome);
      mockedFs.existsSync.mockImplementation((p) => p === defaultPath);
      mockedFs.accessSync.mockImplementation(() => {
        throw new Error('Permission denied');
      });

      const result = discoverAndValidateKubeconfig();

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('not readable');
      }
    });

    it('should succeed when kubeconfig is found and valid', () => {
      const mockHome = '/home/testuser';
      const defaultPath = path.join(mockHome, '.kube', 'config');

      mockedOs.homedir.mockReturnValue(mockHome);
      mockedFs.existsSync.mockImplementation((p) => p === defaultPath);
      mockedFs.accessSync.mockReturnValue(undefined);

      const result = discoverAndValidateKubeconfig();

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.path).toBe(defaultPath);
        expect(result.value.contextName).toBe('test-context');
      }
    });
  });

  describe('isInCluster', () => {
    it('should return true when service account files exist', () => {
      mockedFs.existsSync.mockImplementation((p) =>
        p === '/var/run/secrets/kubernetes.io/serviceaccount/token' ||
        p === '/var/run/secrets/kubernetes.io/serviceaccount/ca.crt'
          ? true
          : false,
      );

      expect(isInCluster()).toBe(true);
    });

    it('should return false when service account files do not exist', () => {
      mockedFs.existsSync.mockReturnValue(false);

      expect(isInCluster()).toBe(false);
    });

    it('should return false when only token exists', () => {
      mockedFs.existsSync.mockImplementation(
        (p) => p === '/var/run/secrets/kubernetes.io/serviceaccount/token',
      );

      expect(isInCluster()).toBe(false);
    });
  });
});