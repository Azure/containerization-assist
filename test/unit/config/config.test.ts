import { describe, it, expect, beforeEach, afterEach } from '@jest/globals';
import {
  createDefaultConfig,
  createConfiguration,
} from '../../../src/config/config';
import os from 'os';
import path from 'path';

describe('Configuration Module', () => {
  let originalEnv: Record<string, string | undefined>;

  beforeEach(() => {
    originalEnv = { ...process.env };
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  describe('createDefaultConfig', () => {
    it('should create configuration with all required sections', () => {
      const config = createDefaultConfig();

      expect(config).toBeDefined();
      expect(config.logLevel).toBe('info');
      expect(config.workspaceDir).toBe(process.cwd());
      expect(config.server).toBeDefined();
      expect(config.session).toBeDefined();
      expect(config.mcp).toBeDefined();
      expect(config.docker).toBeDefined();
      expect(config.kubernetes).toBeDefined();
      expect(config.workspace).toBeDefined();
      expect(config.logging).toBeDefined();
      expect(config.workflow).toBeDefined();
    });

    it('should have proper server configuration', () => {
      const config = createDefaultConfig();

      expect(config.server.nodeEnv).toBe('development');
      expect(config.server.logLevel).toBe('info');
      expect(config.server.port).toBe(3000); // Default port for javascript
      expect(config.server.host).toBe('localhost');
    });

    it('should have proper session configuration', () => {
      const config = createDefaultConfig();

      expect(config.session.store).toBe('memory');
      expect(config.session.ttl).toBe(86400); // 24h
      expect(config.session.maxSessions).toBe(1000);
      expect(config.session.persistencePath).toBe('./data/sessions.db');
    });

    it('should have proper MCP configuration', () => {
      const config = createDefaultConfig();

      expect(config.mcp.name).toBe('containerization-assist');
      expect(config.mcp.version).toBe('1.0.0');
      expect(config.mcp.storePath).toBe('./data/sessions.db');
      expect(config.mcp.sessionTTL).toBe('24h');
      expect(config.mcp.maxSessions).toBe(100);
      expect(config.mcp.enableMetrics).toBe(true);
      expect(config.mcp.enableEvents).toBe(true);
    });

    it('should have proper Docker configuration', () => {
      const config = createDefaultConfig();

      expect(config.docker.socketPath).toBe('/var/run/docker.sock');
      expect(config.docker.host).toBe('localhost');
      expect(config.docker.port).toBe(2375);
      expect(config.docker.registry).toBe('docker.io');
      expect(config.docker.timeout).toBe(60000);
      expect(config.docker.buildArgs).toEqual({});
    });

    it('should have proper Kubernetes configuration', () => {
      const config = createDefaultConfig();

      expect(config.kubernetes.namespace).toBe('default');
      expect(config.kubernetes.kubeconfig).toBe(path.join(os.homedir(), '.kube', 'config'));
      expect(config.kubernetes.timeout).toBe(30000);
    });

    it('should have proper workspace configuration', () => {
      const config = createDefaultConfig();

      expect(config.workspace.workspaceDir).toBe(process.cwd());
      expect(config.workspace.tempDir).toBe(os.tmpdir());
      expect(config.workspace.cleanupOnExit).toBe(true);
    });

    it('should have proper workflow configuration', () => {
      const config = createDefaultConfig();

      expect(config.workflow.mode).toBe('interactive');
    });
  });

  describe('createConfiguration', () => {
    it('should apply environment variable overrides', () => {
      process.env.NODE_ENV = 'production';
      process.env.LOG_LEVEL = 'warn';
      process.env.PORT = '9000';
      process.env.HOST = '0.0.0.0';
      process.env.MCP_STORE_PATH = '/custom/sessions.db';
      process.env.MAX_SESSIONS = '500';
      process.env.DOCKER_HOST = 'tcp://docker-host:2376';
      process.env.DOCKER_REGISTRY = 'my-registry.com';
      process.env.K8S_NAMESPACE = 'production';
      process.env.KUBECONFIG = '/custom/kubeconfig';

      const config = createConfiguration();

      expect(config.server.nodeEnv).toBe('production');
      expect(config.server.logLevel).toBe('warn');
      expect(config.server.port).toBe(9000);
      expect(config.server.host).toBe('0.0.0.0');
      expect(config.mcp.storePath).toBe('/custom/sessions.db');
      expect(config.mcp.maxSessions).toBe(500);
      expect(config.docker.socketPath).toBe('tcp://docker-host:2376');
      expect(config.docker.registry).toBe('my-registry.com');
      expect(config.kubernetes.namespace).toBe('production');
      expect(config.kubernetes.kubeconfig).toBe('/custom/kubeconfig');
    });

    it('should handle DOCKER_SOCKET environment variable', () => {
      process.env.DOCKER_SOCKET = '/custom/docker.sock';

      const config = createConfiguration();

      expect(config.docker.socketPath).toBe('/custom/docker.sock');
    });

    it('should handle KUBE_NAMESPACE environment variable', () => {
      process.env.KUBE_NAMESPACE = 'kube-namespace';

      const config = createConfiguration();

      expect(config.kubernetes.namespace).toBe('kube-namespace');
    });

    it('should handle empty string environment variables', () => {
      process.env.KUBECONFIG = '';

      const config = createConfiguration();

      expect(config.kubernetes.kubeconfig).toBe('');
    });

    it('should handle invalid integer environment variables gracefully', () => {
      const consoleSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});

      process.env.PORT = 'invalid';
      process.env.MAX_SESSIONS = 'not-a-number';

      const config = createConfiguration();

      // Should fall back to defaults
      expect(config.server.port).toBe(3000); // Default port
      expect(config.mcp.maxSessions).toBe(100); // Default max sessions

      // Should have warned about invalid values
      expect(consoleSpy).toHaveBeenCalledWith(
        expect.stringContaining('Invalid MAX_SESSIONS')
      );

      consoleSpy.mockRestore();
    });

    it('should prefer DOCKER_HOST over DOCKER_SOCKET', () => {
      process.env.DOCKER_HOST = 'tcp://host1:2376';
      process.env.DOCKER_SOCKET = '/path/to/socket';

      const config = createConfiguration();

      expect(config.docker.socketPath).toBe('tcp://host1:2376');
    });

    it('should use KUBE_NAMESPACE or K8S_NAMESPACE', () => {
      process.env.KUBE_NAMESPACE = 'kube-ns';
      process.env.K8S_NAMESPACE = 'k8s-ns';

      const config = createConfiguration();

      // Should use one of the namespace environment variables
      expect(['kube-ns', 'k8s-ns']).toContain(config.kubernetes.namespace);
    });
  });



  describe('configuration validation', () => {
    it('should have consistent configuration between default and environment', () => {
      const defaultConfig = createDefaultConfig();
      const envConfig = createConfiguration();

      // Check that structure is consistent
      expect(Object.keys(defaultConfig)).toEqual(Object.keys(envConfig));
      expect(Object.keys(defaultConfig.server)).toEqual(Object.keys(envConfig.server));
      expect(Object.keys(defaultConfig.docker)).toEqual(Object.keys(envConfig.docker));
      expect(Object.keys(defaultConfig.kubernetes)).toEqual(Object.keys(envConfig.kubernetes));
    });

    it('should have valid port ranges', () => {
      const config = createDefaultConfig();

      expect(config.server.port).toBeGreaterThan(0);
      expect(config.server.port).toBeLessThan(65536);
      expect(config.docker.port).toBeGreaterThan(0);
      expect(config.docker.port).toBeLessThan(65536);
    });

    it('should have positive timeout values', () => {
      const config = createDefaultConfig();

      expect(config.docker.timeout).toBeGreaterThan(0);
      expect(config.kubernetes.timeout).toBeGreaterThan(0);
      expect(config.session.ttl).toBeGreaterThan(0);
    });

    it('should have reasonable session limits', () => {
      const config = createDefaultConfig();

      expect(config.session.maxSessions).toBeGreaterThan(0);
      expect(config.mcp.maxSessions).toBeGreaterThan(0);
    });
  });
});
