/**
 * Bootstrap Integration Test
 *
 * Tests the full startup/shutdown lifecycle using the bootstrap helper.
 * Validates that bootstrap correctly:
 * - Sets up MCP_MODE
 * - Creates and starts the app
 * - Installs signal handlers
 * - Provides working app instance
 * - Cleans up on shutdown
 *
 * NOTE: This test is currently excluded from automated runs due to Jest ESM
 * compatibility issues with @kubernetes/client-node. The test file exists as
 * documentation and can be run manually:
 *
 * ```bash
 * NODE_OPTIONS='--experimental-vm-modules' npx tsx test/integration/bootstrap-integration.test.ts
 * ```
 *
 * The bootstrap functionality is thoroughly validated by:
 * - test/unit/cli/bootstrap.test.ts (16 unit tests)
 * - test/unit/cli/config-loader.test.ts (16 unit tests)
 * - test/unit/cli/cli.test.ts (17 tests validating CLI integration)
 * - test/unit/cli/server.test.ts (15 tests validating server integration)
 */

import { describe, it, expect, beforeEach, afterEach } from '@jest/globals';
import { bootstrap } from '../../src/cli/bootstrap';
import { createLogger } from '../../src/lib/logger';

describe('Bootstrap Integration', () => {
  let originalEnv: NodeJS.ProcessEnv;

  beforeEach(() => {
    originalEnv = { ...process.env };
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  describe('Full Startup Lifecycle', () => {
    it('should successfully start and stop server via bootstrap', async () => {
      const logger = createLogger({ name: 'integration-test', level: 'silent' });

      // Start the server
      const result = await bootstrap({
        appName: 'integration-test-app',
        version: '1.0.0',
        logger,
        policyPath: 'config/policy.yaml',
        policyEnvironment: 'test',
        quiet: true,
      });

      // Verify app instance is created
      expect(result.app).toBeDefined();
      expect(result.shutdown).toBeInstanceOf(Function);

      // Verify app has expected methods
      expect(result.app.listTools).toBeInstanceOf(Function);
      expect(result.app.healthCheck).toBeInstanceOf(Function);
      expect(result.app.execute).toBeInstanceOf(Function);
      expect(result.app.stop).toBeInstanceOf(Function);

      // Verify health check returns correct structure
      const health = result.app.healthCheck();
      expect(health.status).toBe('healthy');
      expect(health.tools).toBeGreaterThan(0);
      expect(health.message).toBeDefined();

      // Verify tools are loaded
      const tools = result.app.listTools();
      expect(tools.length).toBeGreaterThan(0);
      expect(tools[0]).toHaveProperty('name');
      expect(tools[0]).toHaveProperty('description');

      // Clean up
      await result.app.stop();
    }, 10000);

    it('should set MCP_MODE environment variable', async () => {
      delete process.env.MCP_MODE;
      const logger = createLogger({ name: 'integration-test', level: 'silent' });

      const result = await bootstrap({
        appName: 'test-mcp-mode',
        version: '1.0.0',
        logger,
        quiet: true,
      });

      // Verify MCP_MODE was set
      expect(process.env.MCP_MODE).toBe('true');

      // Clean up
      await result.app.stop();
    });

    it('should preserve existing MCP_MODE value', async () => {
      process.env.MCP_MODE = 'custom-value';
      const logger = createLogger({ name: 'integration-test', level: 'silent' });

      const result = await bootstrap({
        appName: 'test-mcp-mode',
        version: '1.0.0',
        logger,
        quiet: true,
      });

      // Verify MCP_MODE was preserved
      expect(process.env.MCP_MODE).toBe('custom-value');

      // Clean up
      await result.app.stop();
    });
  });

  describe('App Instance Functionality', () => {
    it('should provide functional health check', async () => {
      const logger = createLogger({ name: 'integration-test', level: 'silent' });

      const result = await bootstrap({
        appName: 'test-health',
        version: '1.0.0',
        logger,
        quiet: true,
      });

      const health = result.app.healthCheck();

      expect(health).toMatchObject({
        status: 'healthy',
        tools: expect.any(Number),
        message: expect.any(String),
      });

      expect(health.tools).toBeGreaterThan(0);

      await result.app.stop();
    });

    it('should list available tools', async () => {
      const logger = createLogger({ name: 'integration-test', level: 'silent' });

      const result = await bootstrap({
        appName: 'test-tools',
        version: '1.0.0',
        logger,
        quiet: true,
      });

      const tools = result.app.listTools();

      expect(Array.isArray(tools)).toBe(true);
      expect(tools.length).toBeGreaterThan(0);

      // Verify at least one core tool is present
      const toolNames = tools.map((t) => t.name);
      expect(toolNames).toContain('analyze-repo');

      await result.app.stop();
    });

    it('should support multiple stop calls gracefully', async () => {
      const logger = createLogger({ name: 'integration-test', level: 'silent' });

      const result = await bootstrap({
        appName: 'test-stop',
        version: '1.0.0',
        logger,
        quiet: true,
      });

      // First stop should succeed
      await expect(result.app.stop()).resolves.not.toThrow();

      // Second stop should also succeed (idempotent)
      await expect(result.app.stop()).resolves.not.toThrow();
    });
  });

  describe('Configuration Handling', () => {
    it('should respect quiet mode configuration', async () => {
      const logger = createLogger({ name: 'integration-test', level: 'silent' });
      const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

      const result = await bootstrap({
        appName: 'test-quiet',
        version: '1.0.0',
        logger,
        quiet: true,
      });

      // In quiet mode, console.error should not be called for startup messages
      expect(consoleErrorSpy).not.toHaveBeenCalled();

      await result.app.stop();
      consoleErrorSpy.mockRestore();
    });

    it('should accept custom workspace configuration', async () => {
      const logger = createLogger({ name: 'integration-test', level: 'silent' });
      const customWorkspace = '/tmp/test-workspace';

      const result = await bootstrap({
        appName: 'test-workspace',
        version: '1.0.0',
        logger,
        workspace: customWorkspace,
        quiet: true,
      });

      expect(result.app).toBeDefined();

      await result.app.stop();
    });

    it('should accept development mode configuration', async () => {
      const logger = createLogger({ name: 'integration-test', level: 'silent' });

      const result = await bootstrap({
        appName: 'test-dev',
        version: '1.0.0',
        logger,
        devMode: true,
        quiet: true,
      });

      expect(result.app).toBeDefined();

      await result.app.stop();
    });

    it('should accept custom policy configuration', async () => {
      const logger = createLogger({ name: 'integration-test', level: 'silent' });

      const result = await bootstrap({
        appName: 'test-policy',
        version: '1.0.0',
        logger,
        policyPath: 'config/policy.yaml',
        policyEnvironment: 'production',
        quiet: true,
      });

      expect(result.app).toBeDefined();

      await result.app.stop();
    });
  });

  describe('Error Handling', () => {
    it('should throw error for missing required configuration', async () => {
      // Missing logger should cause failure
      await expect(
        bootstrap({
          appName: 'test-missing',
          version: '1.0.0',
          // @ts-expect-error Testing missing logger
          logger: undefined,
          quiet: true,
        }),
      ).rejects.toThrow();
    });

    it('should handle startup with minimal configuration', async () => {
      const logger = createLogger({ name: 'integration-test', level: 'silent' });

      // Minimal config should still work
      const result = await bootstrap({
        appName: 'minimal',
        version: '1.0.0',
        logger,
        quiet: true,
      });

      expect(result.app).toBeDefined();
      expect(result.app.healthCheck().status).toBe('healthy');

      await result.app.stop();
    });
  });

  describe('Shutdown Hook', () => {
    it('should call onShutdown hook during shutdown', async () => {
      const logger = createLogger({ name: 'integration-test', level: 'silent' });
      let hookCalled = false;
      const onShutdown = jest.fn(async () => {
        hookCalled = true;
      });

      const result = await bootstrap({
        appName: 'test-shutdown-hook',
        version: '1.0.0',
        logger,
        onShutdown,
        quiet: true,
      });

      // Call shutdown via the shutdown function
      await result.shutdown('SIGTERM');

      // Verify hook was called
      expect(onShutdown).toHaveBeenCalled();
      expect(hookCalled).toBe(true);
    });

    it('should handle shutdown hook errors gracefully', async () => {
      const logger = createLogger({ name: 'integration-test', level: 'silent' });
      const onShutdown = jest.fn(async () => {
        throw new Error('Shutdown hook error');
      });

      const result = await bootstrap({
        appName: 'test-shutdown-error',
        version: '1.0.0',
        logger,
        onShutdown,
        quiet: true,
      });

      // Shutdown should throw when hook fails
      await expect(result.shutdown('SIGTERM')).rejects.toThrow('Shutdown hook error');

      // Still clean up app
      await result.app.stop();
    });
  });

  describe('Real-World Scenarios', () => {
    it('should support typical CLI usage pattern', async () => {
      const logger = createLogger({ name: 'cli-simulation', level: 'silent' });

      // Simulate CLI startup
      const result = await bootstrap({
        appName: 'containerization-assist-mcp',
        version: '1.5.0',
        logger,
        policyPath: 'config/policy.yaml',
        policyEnvironment: 'production',
        workspace: process.cwd(),
        devMode: false,
        quiet: true,
      });

      // Verify server is ready
      const health = result.app.healthCheck();
      expect(health.status).toBe('healthy');

      // Simulate checking available tools
      const tools = result.app.listTools();
      expect(tools.length).toBeGreaterThan(10);

      // Clean shutdown
      await result.app.stop();
    });

    it('should support programmatic usage without server', async () => {
      const logger = createLogger({ name: 'programmatic', level: 'silent' });

      const result = await bootstrap({
        appName: 'programmatic-app',
        version: '1.0.0',
        logger,
        quiet: true,
      });

      // Use app for direct execution (no server started in this test)
      const tools = result.app.listTools();
      expect(tools).toBeDefined();

      const health = result.app.healthCheck();
      expect(health.status).toBe('healthy');

      await result.app.stop();
    });
  });
});
