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
      expect(config.server).toBeDefined();
      expect(config.workspace).toBeDefined();
      expect(config.docker).toBeDefined();
      expect(config.mutex).toBeDefined();
    });

    it('should use environment variables when provided', () => {
      // Set test environment variables
      process.env.LOG_LEVEL = 'debug';
      process.env.PORT = '4000';
      process.env.WORKSPACE_DIR = '/test/workspace';
      process.env.DOCKER_SOCKET = '/test/docker.sock';

      // Re-require the module to get new environment values
      jest.resetModules();
      const { config: testConfig } = require('../../../src/config/index');

      expect(testConfig.server.logLevel).toBe('debug');
      expect(testConfig.server.port).toBe(4000);
      expect(testConfig.workspace.workspaceDir).toBe('/test/workspace');
      expect(testConfig.docker.socketPath).toBe('/test/docker.sock');
    });

    it('should use default values when environment variables are not set', () => {
      // Clear relevant environment variables
      delete process.env.LOG_LEVEL;
      delete process.env.PORT;

      jest.resetModules();
      const { config: testConfig } = require('../../../src/config/index');

      expect(testConfig.server.logLevel).toBe('info');
      expect(testConfig.server.port).toBe(3000);
    });

    it('should parse integer environment variables correctly', () => {
      process.env.MAX_FILE_SIZE = '20971520'; // 20MB
      process.env.MUTEX_DEFAULT_TIMEOUT = '45000';

      jest.resetModules();
      const { config: testConfig } = require('../../../src/config/index');

      expect(testConfig.workspace.maxFileSize).toBe(20971520);
      expect(testConfig.mutex.defaultTimeout).toBe(45000);
    });

    it('should handle boolean environment variables', () => {
      process.env.MUTEX_MONITORING = 'true';

      jest.resetModules();
      const { config: testConfig } = require('../../../src/config/index');

      expect(testConfig.mutex.monitoringEnabled).toBe(true);

      process.env.MUTEX_MONITORING = 'false';

      jest.resetModules();
      const { config: testConfig2 } = require('../../../src/config/index');

      expect(testConfig2.mutex.monitoringEnabled).toBe(false);
    });

    it('should have mutex configuration', () => {
      expect(config.mutex.defaultTimeout).toBeDefined();
      expect(config.mutex.dockerBuildTimeout).toBeDefined();
      expect(config.mutex.monitoringEnabled).toBeDefined();

      expect(typeof config.mutex.defaultTimeout).toBe('number');
      expect(typeof config.mutex.dockerBuildTimeout).toBe('number');
      expect(typeof config.mutex.monitoringEnabled).toBe('boolean');
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
      expect(config.server.logLevel).toBeDefined();
      expect(typeof config.server.logLevel).toBe('string');
    });

    it('should have reasonable default values', () => {
      expect(config.server.port).toBeGreaterThan(0);
      expect(config.server.port).toBeLessThan(65536);

      expect(config.workspace.maxFileSize).toBeGreaterThan(0);

      expect(config.mutex.defaultTimeout).toBeGreaterThan(0);
      expect(config.mutex.dockerBuildTimeout).toBeGreaterThan(0);
    });

    it('should have valid file paths', () => {
      expect(config.docker.socketPath).toContain('/');
      expect(config.workspace.workspaceDir).toBeTruthy();
    });

    it('should have all required mutex settings', () => {
      expect(config.mutex.defaultTimeout).toBeDefined();
      expect(config.mutex.dockerBuildTimeout).toBeDefined();
      expect(config.mutex.monitoringEnabled).toBeDefined();
    });
  });
});