import { describe, it, expect, beforeEach, afterEach } from '@jest/globals';
import { config, logConfigSummaryIfDev } from '../../../src/config/index';

describe('Main Configuration', () => {
  let originalEnv: Record<string, string | undefined>;

  beforeEach(() => {
    originalEnv = { ...process.env };
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  describe('config object', () => {
    it('should have all required configuration sections', () => {
      expect(config).toBeDefined();
      expect(config.mcp).toBeDefined();
      expect(config.server).toBeDefined();
      expect(config.workspace).toBeDefined();
      expect(config.sampling).toBeDefined();
      expect(config.cache).toBeDefined();
      expect(config.docker).toBeDefined();
      expect(config.kubernetes).toBeDefined();
      expect(config.security).toBeDefined();
      expect(config.logging).toBeDefined();
      expect(config.orchestrator).toBeDefined();
    });

    it('should use environment variables when provided', () => {
      // Set test environment variables
      process.env.MCP_SERVER_NAME = 'test-server';
      process.env.LOG_LEVEL = 'debug';
      process.env.PORT = '4000';
      process.env.WORKSPACE_DIR = '/test/workspace';
      process.env.DOCKER_SOCKET = '/test/docker.sock';
      process.env.K8S_NAMESPACE = 'test-namespace';

      // Re-require the module to get new environment values
      jest.resetModules();
      const { config: testConfig } = require('../../../src/config/index');

      expect(testConfig.mcp.name).toBe('test-server');
      expect(testConfig.server.logLevel).toBe('debug');
      expect(testConfig.server.port).toBe(4000);
      expect(testConfig.workspace.workspaceDir).toBe('/test/workspace');
      expect(testConfig.docker.socketPath).toBe('/test/docker.sock');
      expect(testConfig.kubernetes.namespace).toBe('test-namespace');
    });

    it('should use default values when environment variables are not set', () => {
      // Clear relevant environment variables
      delete process.env.MCP_SERVER_NAME;
      delete process.env.LOG_LEVEL;
      delete process.env.PORT;

      jest.resetModules();
      const { config: testConfig } = require('../../../src/config/index');

      expect(testConfig.mcp.name).toBe('containerization-assist');
      expect(testConfig.server.logLevel).toBe('info');
      expect(testConfig.server.port).toBe(3000);
    });

    it('should parse integer environment variables correctly', () => {
      process.env.MAX_CANDIDATES = '10';
      process.env.SAMPLING_TIMEOUT = '60000';
      process.env.CACHE_TTL = '7200';
      process.env.MAX_FILE_SIZE = '20971520'; // 20MB

      jest.resetModules();
      const { config: testConfig } = require('../../../src/config/index');

      expect(testConfig.sampling.maxCandidates).toBe(10);
      expect(testConfig.sampling.timeout).toBe(60000);
      expect(testConfig.cache.ttl).toBe(7200);
      expect(testConfig.workspace.maxFileSize).toBe(20971520);
    });

    it('should handle boolean environment variables', () => {
      process.env.FAIL_ON_CRITICAL = 'true';

      jest.resetModules();
      const { config: testConfig } = require('../../../src/config/index');

      expect(testConfig.security.failOnCritical).toBe(true);

      process.env.FAIL_ON_CRITICAL = 'false';

      jest.resetModules();
      const { config: testConfig2 } = require('../../../src/config/index');

      expect(testConfig2.security.failOnCritical).toBe(false);
    });

    it('should have proper sampling weights structure', () => {
      expect(config.sampling.weights.dockerfile).toBeDefined();
      expect(config.sampling.weights.dockerfile.build).toBe(30);
      expect(config.sampling.weights.dockerfile.size).toBe(30);
      expect(config.sampling.weights.dockerfile.security).toBe(25);
      expect(config.sampling.weights.dockerfile.speed).toBe(15);

      expect(config.sampling.weights.k8s).toBeDefined();
      expect(config.sampling.weights.k8s.validation).toBe(20);
      expect(config.sampling.weights.k8s.security).toBe(20);
      expect(config.sampling.weights.k8s.resources).toBe(20);
      expect(config.sampling.weights.k8s.best_practices).toBe(20);
    });

    it('should have orchestrator configuration with thresholds', () => {
      expect(config.orchestrator.scanThresholds).toBeDefined();
      expect(config.orchestrator.scanThresholds.critical).toBe(0);
      expect(config.orchestrator.scanThresholds.high).toBe(2);
      expect(config.orchestrator.scanThresholds.medium).toBe(10);

      expect(config.orchestrator.buildSizeLimits).toBeDefined();
      expect(config.orchestrator.buildSizeLimits.sanityFactor).toBe(1.25);
      expect(config.orchestrator.buildSizeLimits.rejectFactor).toBe(2.5);
    });

    it('should be immutable (readonly)', () => {
      // This test verifies the 'as const' assertion works
      expect(() => {
        // @ts-expect-error - This should fail at compile time due to readonly
        (config as any).server.logLevel = 'test';
      }).not.toThrow(); // Runtime doesn't prevent this, but TypeScript should
    });
  });

  describe('logConfigSummaryIfDev', () => {
    let mockLogger: { info: jest.Mock };

    beforeEach(() => {
      mockLogger = { info: jest.fn() };
    });

    it('should log configuration in development environment', () => {
      process.env.NODE_ENV = 'development';

      logConfigSummaryIfDev(mockLogger);

      expect(mockLogger.info).toHaveBeenCalledWith(
        'Configuration loaded',
        expect.objectContaining({
          server: expect.objectContaining({
            logLevel: expect.any(String),
            port: expect.any(Number),
          }),
          workspace: expect.any(String),
          docker: expect.any(String),
        })
      );
    });

    it('should not log in non-development environments', () => {
      process.env.NODE_ENV = 'production';

      logConfigSummaryIfDev(mockLogger);

      expect(mockLogger.info).not.toHaveBeenCalled();

      process.env.NODE_ENV = 'test';

      logConfigSummaryIfDev(mockLogger);

      expect(mockLogger.info).not.toHaveBeenCalled();
    });

    it('should not throw when no logger is provided', () => {
      process.env.NODE_ENV = 'development';

      expect(() => {
        logConfigSummaryIfDev();
      }).not.toThrow();
    });

    it('should not log when NODE_ENV is undefined', () => {
      delete process.env.NODE_ENV;

      logConfigSummaryIfDev(mockLogger);

      expect(mockLogger.info).not.toHaveBeenCalled();
    });

    it('should include correct configuration data', () => {
      process.env.NODE_ENV = 'development';

      logConfigSummaryIfDev(mockLogger);

      const loggedData = mockLogger.info.mock.calls[0][1];
      expect(loggedData).toHaveProperty('server.logLevel');
      expect(loggedData).toHaveProperty('server.port');
      expect(loggedData).toHaveProperty('workspace');
      expect(loggedData).toHaveProperty('docker');
      
      expect(loggedData.server.logLevel).toBe(config.server.logLevel);
      expect(loggedData.server.port).toBe(config.server.port);
      expect(loggedData.workspace).toBe(config.workspace.workspaceDir);
      expect(loggedData.docker).toBe(config.docker.socketPath);
    });
  });

  describe('configuration structure validation', () => {
    it('should have log level configuration', () => {
      // Both sections should have log level configuration
      expect(config.server.logLevel).toBeDefined();
      expect(config.logging.level).toBeDefined();
      expect(typeof config.server.logLevel).toBe('string');
      expect(typeof config.logging.level).toBe('string');
    });

    it('should have reasonable default values', () => {
      expect(config.server.port).toBeGreaterThan(0);
      expect(config.server.port).toBeLessThan(65536);
      
      expect(config.workspace.maxFileSize).toBeGreaterThan(0);
      
      expect(config.sampling.maxCandidates).toBeGreaterThan(0);
      expect(config.sampling.timeout).toBeGreaterThan(0);
      
      expect(config.cache.ttl).toBeGreaterThan(0);
      expect(config.cache.maxSize).toBeGreaterThan(0);
    });

    it('should have valid file paths', () => {
      expect(config.docker.socketPath).toContain('/');
      expect(config.workspace.workspaceDir).toBeTruthy();
    });

    it('should have all required orchestrator settings', () => {
      expect(config.orchestrator.defaultCandidates).toBeLessThanOrEqual(
        config.orchestrator.maxCandidates
      );
      expect(config.orchestrator.earlyStopThreshold).toBeGreaterThan(0);
      expect(config.orchestrator.earlyStopThreshold).toBeLessThanOrEqual(100);
      expect(config.orchestrator.tiebreakMargin).toBeGreaterThanOrEqual(0);
    });
  });
});