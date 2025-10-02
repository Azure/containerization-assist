import { describe, it, expect, beforeEach, afterEach } from '@jest/globals';
import {
  loadEnvironmentConfig,
  applyOptionsToEnvironment,
  createRuntimeConfig,
  createBootstrapConfig,
  getConfigSummary,
  type CLIOptions,
} from '../../../src/cli/config-loader';
import { createLogger } from '../../../src/lib/logger';

describe('Config Loader', () => {
  let originalEnv: NodeJS.ProcessEnv;

  beforeEach(() => {
    originalEnv = { ...process.env };
    // Clean environment for predictable tests
    delete process.env.LOG_LEVEL;
    delete process.env.WORKSPACE_DIR;
    delete process.env.DOCKER_SOCKET;
    delete process.env.K8S_NAMESPACE;
    delete process.env.NODE_ENV;
    delete process.env.MCP_MODE;
    delete process.env.MCP_QUIET;
    delete process.env.POLICY_PATH;
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  describe('loadEnvironmentConfig', () => {
    it('should load default values when env vars not set', () => {
      const config = loadEnvironmentConfig();

      expect(config.logLevel).toBe('info');
      expect(config.workspaceDir).toBe(process.cwd());
      expect(config.dockerSocket).toBeDefined(); // Platform-specific
      expect(config.k8sNamespace).toBe('default');
      expect(config.nodeEnv).toBe('production');
      expect(config.mcpMode).toBe(false);
      expect(config.mcpQuiet).toBe(false);
      expect(config.policyPath).toBeUndefined();
    });

    it('should load values from environment variables', () => {
      process.env.LOG_LEVEL = 'debug';
      process.env.WORKSPACE_DIR = '/custom/workspace';
      process.env.DOCKER_SOCKET = '/custom/docker.sock';
      process.env.K8S_NAMESPACE = 'custom-namespace';
      process.env.NODE_ENV = 'development';
      process.env.MCP_MODE = 'true';
      process.env.MCP_QUIET = 'true';
      process.env.POLICY_PATH = '/custom/policy.yaml';

      const config = loadEnvironmentConfig();

      expect(config.logLevel).toBe('debug');
      expect(config.workspaceDir).toBe('/custom/workspace');
      expect(config.dockerSocket).toBe('/custom/docker.sock');
      expect(config.k8sNamespace).toBe('custom-namespace');
      expect(config.nodeEnv).toBe('development');
      expect(config.mcpMode).toBe(true);
      expect(config.mcpQuiet).toBe(true);
      expect(config.policyPath).toBe('/custom/policy.yaml');
    });

    it('should handle MCP_MODE string values correctly', () => {
      process.env.MCP_MODE = 'false';
      const config1 = loadEnvironmentConfig();
      expect(config1.mcpMode).toBe(false);

      process.env.MCP_MODE = 'true';
      const config2 = loadEnvironmentConfig();
      expect(config2.mcpMode).toBe(true);

      process.env.MCP_MODE = 'anything';
      const config3 = loadEnvironmentConfig();
      expect(config3.mcpMode).toBe(false);
    });
  });

  describe('applyOptionsToEnvironment', () => {
    it('should apply CLI options to environment', () => {
      const options: CLIOptions = {
        logLevel: 'debug',
        workspace: '/cli/workspace',
        dockerSocket: '/cli/docker.sock',
        k8sNamespace: 'cli-namespace',
        dev: true,
      };

      applyOptionsToEnvironment(options);

      expect(process.env.LOG_LEVEL).toBe('debug');
      expect(process.env.WORKSPACE_DIR).toBe('/cli/workspace');
      expect(process.env.DOCKER_SOCKET).toBe('/cli/docker.sock');
      expect(process.env.K8S_NAMESPACE).toBe('cli-namespace');
      expect(process.env.NODE_ENV).toBe('development');
    });

    it('should not set undefined options', () => {
      const options: CLIOptions = {
        logLevel: 'info',
        // Other options undefined
      };

      applyOptionsToEnvironment(options);

      expect(process.env.LOG_LEVEL).toBe('info');
      expect(process.env.WORKSPACE_DIR).toBeUndefined();
      expect(process.env.DOCKER_SOCKET).toBeUndefined();
    });

    it('should handle empty options object', () => {
      applyOptionsToEnvironment({});

      // No environment variables should be set
      expect(process.env.LOG_LEVEL).toBeUndefined();
      expect(process.env.WORKSPACE_DIR).toBeUndefined();
    });
  });

  describe('createRuntimeConfig', () => {
    it('should create runtime config with defaults', () => {
      const logger = createLogger({ name: 'test', level: 'silent' });
      const config = createRuntimeConfig(logger);

      expect(config.logger).toBe(logger);
      expect(config.policyPath).toBe('config/policy.yaml');
      expect(config.policyEnvironment).toBe('production');
    });

    it('should use CLI options when provided', () => {
      const logger = createLogger({ name: 'test', level: 'silent' });
      const options: CLIOptions = {
        config: '/custom/policy.yaml',
        dev: true,
      };

      const config = createRuntimeConfig(logger, options);

      expect(config.logger).toBe(logger);
      expect(config.policyPath).toBe('/custom/policy.yaml');
      expect(config.policyEnvironment).toBe('development');
    });

    it('should prefer environment variable for policy path', () => {
      process.env.POLICY_PATH = '/env/policy.yaml';
      const logger = createLogger({ name: 'test', level: 'silent' });

      const config = createRuntimeConfig(logger, {});

      expect(config.policyPath).toBe('/env/policy.yaml');
    });

    it('should use production environment when dev is false', () => {
      process.env.NODE_ENV = 'production';
      const logger = createLogger({ name: 'test', level: 'silent' });
      const options: CLIOptions = { dev: false };

      const config = createRuntimeConfig(logger, options);

      expect(config.policyEnvironment).toBe('production');
    });
  });

  describe('createBootstrapConfig', () => {
    it('should create bootstrap config with all fields', () => {
      const logger = createLogger({ name: 'test', level: 'silent' });
      const options: CLIOptions = {
        config: '/custom/policy.yaml',
        dev: true,
        workspace: '/custom/workspace',
      };

      const config = createBootstrapConfig(
        'test-app',
        '1.0.0',
        logger,
        options,
        5,
      );

      expect(config.appName).toBe('test-app');
      expect(config.version).toBe('1.0.0');
      expect(config.logger).toBe(logger);
      expect(config.policyPath).toBe('/custom/policy.yaml');
      expect(config.policyEnvironment).toBe('development');
      expect(config.transport).toEqual({ transport: 'stdio' });
      expect(config.workspace).toBe('/custom/workspace');
      expect(config.devMode).toBe(true);
      expect(config.toolCount).toBe(5);
    });

    it('should handle minimal configuration', () => {
      const logger = createLogger({ name: 'test', level: 'silent' });

      const config = createBootstrapConfig('minimal-app', '1.0.0', logger);

      expect(config.appName).toBe('minimal-app');
      expect(config.version).toBe('1.0.0');
      expect(config.logger).toBe(logger);
      expect(config.transport).toEqual({ transport: 'stdio' });
      expect(config.toolCount).toBe(0);
    });

    it('should respect MCP_QUIET environment variable', () => {
      process.env.MCP_QUIET = 'true';
      const logger = createLogger({ name: 'test', level: 'silent' });

      const config = createBootstrapConfig('test-app', '1.0.0', logger);

      expect(config.quiet).toBe(true);
    });

    it('should use workspace from options when provided', () => {
      const logger = createLogger({ name: 'test', level: 'silent' });
      const options: CLIOptions = {
        workspace: '/option/workspace',
      };

      const config = createBootstrapConfig('test-app', '1.0.0', logger, options);

      expect(config.workspace).toBe('/option/workspace');
    });

    it('should fallback to environment workspace when option not provided', () => {
      process.env.WORKSPACE_DIR = '/env/workspace';
      const logger = createLogger({ name: 'test', level: 'silent' });

      const config = createBootstrapConfig('test-app', '1.0.0', logger, {});

      expect(config.workspace).toBe('/env/workspace');
    });

    it('should only set devMode when dev option is explicitly provided', () => {
      const logger = createLogger({ name: 'test', level: 'silent' });

      // Without dev option
      const config1 = createBootstrapConfig('test-app', '1.0.0', logger, {});
      expect(config1.devMode).toBeUndefined();

      // With dev: false
      const config2 = createBootstrapConfig('test-app', '1.0.0', logger, { dev: false });
      expect(config2.devMode).toBe(false);

      // With dev: true
      const config3 = createBootstrapConfig('test-app', '1.0.0', logger, { dev: true });
      expect(config3.devMode).toBe(true);
    });
  });

  describe('getConfigSummary', () => {
    it('should return current configuration summary', () => {
      process.env.LOG_LEVEL = 'debug';
      process.env.WORKSPACE_DIR = '/test/workspace';
      process.env.DOCKER_SOCKET = '/test/docker.sock';
      process.env.K8S_NAMESPACE = 'test-namespace';
      process.env.NODE_ENV = 'development';

      const summary = getConfigSummary();

      expect(summary.logLevel).toBe('debug');
      expect(summary.workspace).toBe('/test/workspace');
      expect(summary.dockerSocket).toBe('/test/docker.sock');
      expect(summary.k8sNamespace).toBe('test-namespace');
      expect(summary.nodeEnv).toBe('development');
    });

    it('should return defaults when env vars not set', () => {
      const summary = getConfigSummary();

      expect(summary.logLevel).toBe('info');
      expect(summary.workspace).toBe(process.cwd());
      expect(summary.dockerSocket).toBeDefined();
      expect(summary.k8sNamespace).toBe('default');
      expect(summary.nodeEnv).toBe('production');
    });
  });
});
