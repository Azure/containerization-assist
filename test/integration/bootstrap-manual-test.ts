#!/usr/bin/env tsx
/**
 * Manual Bootstrap Integration Test
 *
 * ⚠️  MANUAL TEST - NOT RUN BY AUTOMATED TEST SUITE ⚠️
 *
 * This file is excluded from automated test runs (jest.config.js).
 * Run manually with: npx tsx test/integration/bootstrap-manual-test.ts
 *
 * Purpose: Validates the bootstrap helper in a real execution environment
 * without Jest's ESM compatibility issues. Useful for local verification
 * and debugging during development.
 */

import { bootstrap } from '../../src/cli/bootstrap.js';
import { createLogger } from '../../src/lib/logger.js';

async function runTests(): Promise<void> {
  console.log('🧪 Bootstrap Manual Integration Test\n');

  let passCount = 0;
  let failCount = 0;

  const test = (name: string, fn: () => boolean | Promise<boolean>) => async () => {
    try {
      const result = await fn();
      if (result) {
        console.log(`  ✅ ${name}`);
        passCount++;
      } else {
        console.log(`  ❌ ${name} - assertion failed`);
        failCount++;
      }
    } catch (error) {
      console.log(`  ❌ ${name} - ${error instanceof Error ? error.message : String(error)}`);
      failCount++;
    }
  };

  // Test 1: Basic bootstrap
  await test('should bootstrap with minimal config', async () => {
    const logger = createLogger({ name: 'test', level: 'silent' });
    const result = await bootstrap({
      appName: 'test-app',
      version: '1.0.0',
      logger,
      quiet: true,
    });

    const hasApp = result.app !== undefined;
    const hasShutdown = typeof result.shutdown === 'function';

    await result.app.stop();

    return hasApp && hasShutdown;
  })();

  // Test 2: MCP_MODE setting
  await test('should set MCP_MODE environment variable', async () => {
    delete process.env.MCP_MODE;
    const logger = createLogger({ name: 'test', level: 'silent' });

    const result = await bootstrap({
      appName: 'test-mcp',
      version: '1.0.0',
      logger,
      quiet: true,
    });

    const modeset = process.env.MCP_MODE === 'true';

    await result.app.stop();

    return modeset;
  })();

  // Test 3: Health check
  await test('should provide functional health check', async () => {
    const logger = createLogger({ name: 'test', level: 'silent' });

    const result = await bootstrap({
      appName: 'test-health',
      version: '1.0.0',
      logger,
      quiet: true,
    });

    const health = result.app.healthCheck();
    const isHealthy =
      health.status === 'healthy' &&
      typeof health.tools === 'number' &&
      health.tools > 0 &&
      typeof health.message === 'string';

    await result.app.stop();

    return isHealthy;
  })();

  // Test 4: List tools
  await test('should list available tools', async () => {
    const logger = createLogger({ name: 'test', level: 'silent' });

    const result = await bootstrap({
      appName: 'test-tools',
      version: '1.0.0',
      logger,
      quiet: true,
    });

    const tools = result.app.listTools();
    const hasCoreTools =
      Array.isArray(tools) &&
      tools.length > 0 &&
      tools.some((t) => t.name === 'analyze-repo');

    await result.app.stop();

    return hasCoreTools;
  })();

  // Test 5: Idempotent stop
  await test('should support multiple stop calls', async () => {
    const logger = createLogger({ name: 'test', level: 'silent' });

    const result = await bootstrap({
      appName: 'test-stop',
      version: '1.0.0',
      logger,
      quiet: true,
    });

    await result.app.stop();
    await result.app.stop(); // Second call should not throw

    return true;
  })();

  // Test 6: Shutdown hook
  await test('should call onShutdown hook', async () => {
    let hookCalled = false;
    const logger = createLogger({ name: 'test', level: 'silent' });

    const result = await bootstrap({
      appName: 'test-hook',
      version: '1.0.0',
      logger,
      onShutdown: async () => {
        hookCalled = true;
      },
      quiet: true,
    });

    await result.shutdown('SIGTERM');

    return hookCalled;
  })();

  // Test 7: Configuration options
  await test('should accept full configuration', async () => {
    const logger = createLogger({ name: 'test', level: 'silent' });

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

    const health = result.app.healthCheck();
    const isValid = health.status === 'healthy';

    await result.app.stop();

    return isValid;
  })();

  // Summary
  console.log('\n📊 Test Summary:');
  console.log(`  ✅ Passed: ${passCount}`);
  console.log(`  ❌ Failed: ${failCount}`);
  console.log(`  📈 Total: ${passCount + failCount}`);

  if (failCount > 0) {
    console.log('\n❌ Some tests failed');
    process.exit(1);
  } else {
    console.log('\n✅ All tests passed');
    process.exit(0);
  }
}

// Run tests
runTests().catch((error) => {
  console.error('❌ Test suite failed:', error);
  process.exit(1);
});
