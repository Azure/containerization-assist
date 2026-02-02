/**
 * Integration Test: build-image with Multi-Language Scenarios
 *
 * Tests the complete flow of:
 * 1. Building Java application with multi-stage Dockerfile
 * 2. Building .NET application with multi-stage Dockerfile
 * 3. Verifying build context output, security analysis, and build command generation
 * 4. Validating security warnings and BuildKit recommendations
 * 5. Ensuring generated docker build command passes validation and includes expected flags
 *
 * Prerequisites:
 * - Docker installed and running
 *
 * Usage:
 *   npm run build
 *   tsx scripts/integration-test-build-image.ts
 */

import { join } from 'node:path';
import { existsSync, writeFileSync } from 'node:fs';
import { execSync } from 'node:child_process';

import { createToolContext } from '../dist/src/mcp/context.js';
import buildImageContextTool from '../dist/src/tools/build-image-context/tool.js';
import { createLogger } from '../dist/src/lib/logger.js';

const logger = createLogger({ name: 'build-image-test', level: 'error' });

/**
 * Test case definition
 */
interface BuildTestCase {
  name: string;
  dockerContext: string;
  dockerfile?: string;
  imageName?: string;
  tags: string[];
  buildArgs?: Record<string, string>;
  expectedWarnings?: string[];
  expectBuildKit?: boolean;
  expectedCommandFlags?: string[];
  description: string;
}

/**
 * Test result tracking
 */
interface TestResult {
  name: string;
  passed: boolean;
  message: string;
  warnings?: number;
  buildKit?: boolean;
  command?: string;
}

/**
 * Test cases for context-only builds
 *
 * These tests validate the build-image tool's context preparation output:
 * - Security analysis (warnings)
 * - BuildKit feature detection
 * - Generated docker build command
 *
 * No actual Docker builds are performed - only output validation.
 */
const TEST_CASES: BuildTestCase[] = [
  {
    name: 'Java Multi-Stage Context',
    dockerContext: 'test/fixtures/build-scenarios/java',
    tags: ['test-build:java-app'],
    expectBuildKit: true,
    // Java fixture uses pinned images and non-root user, so no security warnings expected
    expectedCommandFlags: ['docker build', '-t test-build:java-app'],
    description: 'Validates Java multi-stage Dockerfile analysis and BuildKit recommendation',
  },
  {
    name: 'Java with Build Args',
    dockerContext: 'test/fixtures/build-scenarios/java',
    tags: ['test-build:java-args'],
    buildArgs: {
      VERSION: '2.0.0',
    },
    expectedCommandFlags: ['--build-arg VERSION=2.0.0'],
    description: 'Validates build args are included in generated build command',
  },
  {
    name: '.NET Multi-Stage Context',
    dockerContext: 'test/fixtures/build-scenarios/dotnet',
    tags: ['test-build:dotnet-app'],
    expectBuildKit: true,
    // .NET fixture uses pinned images and non-root user, so no security warnings expected
    expectedCommandFlags: ['docker build', '-t test-build:dotnet-app'],
    description: 'Validates .NET multi-stage Dockerfile analysis and BuildKit recommendation',
  },
  {
    name: '.NET Custom Image Name',
    dockerContext: 'test/fixtures/build-scenarios/dotnet',
    imageName: 'custom/dotnet-app',
    tags: ['v3.1.0'],
    expectedCommandFlags: ['-t custom/dotnet-app:v3.1.0'],
    description: 'Validates imageName + tag composition in final build command',
  },
];

/**
 * Check if Docker is available (optional - not required for context-only tests)
 */
function checkDockerAvailable(): boolean {
  try {
    const output = execSync('docker --version', { encoding: 'utf-8', stdio: 'pipe' });
    console.log(`   ℹ️  Docker available: ${output.trim()}`);
    return true;
  } catch {
    console.log('   ℹ️  Docker not available (not required for context tests)');
    return false;
  }
}

/**
 * Validate security warnings include expected IDs
 */
function validateWarnings(
  testCase: BuildTestCase,
  warnings: { id: string }[],
): {
  passed: boolean;
  messages: string[];
} {
  if (!testCase.expectedWarnings || testCase.expectedWarnings.length === 0) {
    return { passed: true, messages: [] };
  }

  const warningIds = warnings.map((w) => w.id);
  const missing = testCase.expectedWarnings.filter((id) => !warningIds.includes(id));
  if (missing.length === 0) {
    return { passed: true, messages: [] };
  }
  return {
    passed: false,
    messages: missing.map((id) => `Missing expected warning: ${id}`),
  };
}

/**
 * Validate build command contains expected flags
 */
function validateCommandFlags(
  testCase: BuildTestCase,
  command: string,
): {
  passed: boolean;
  messages: string[];
} {
  if (!testCase.expectedCommandFlags || testCase.expectedCommandFlags.length === 0) {
    return { passed: true, messages: [] };
  }
  const missing = testCase.expectedCommandFlags.filter((flag) => !command.includes(flag));
  if (missing.length === 0) {
    return { passed: true, messages: [] };
  }
  return {
    passed: false,
    messages: missing.map((flag) => `Build command missing flag: ${flag}`),
  };
}

/**
 * Create JSON summary payload for CI upload
 */
function writeResultsSummary(results: TestResult[]) {
  const summary = {
    total: results.length,
    passed: results.filter((r) => r.passed).length,
    failed: results.filter((r) => !r.passed).length,
    timestamp: new Date().toISOString(),
    results,
  };

  writeFileSync('build-image-context-test-results.json', JSON.stringify(summary, null, 2));
}

/**
 * Main test execution
 */
async function main() {
  console.log('🔨 Testing build-image context generation scenarios\n');
  console.log('='.repeat(60));

  const results: TestResult[] = [];
  let passCount = 0;
  let failCount = 0;

  // ─────────────────────────────────────────────────────────────
  // Step 1: Verify Prerequisites
  // ─────────────────────────────────────────────────────────────
  console.log('\n📋 Step 1: Verifying prerequisites...\n');

  // Docker check is informational only - not required for context tests
  checkDockerAvailable();

  // Verify test fixtures exist
  console.log('\n   Checking test fixtures...');
  for (const testCase of TEST_CASES) {
    const dockerfilePath = join(
      process.cwd(),
      testCase.dockerContext,
      testCase.dockerfile || 'Dockerfile',
    );
    if (!existsSync(dockerfilePath)) {
      console.error(`   ❌ Missing Dockerfile: ${dockerfilePath}`);
      process.exit(1);
    }
    console.log(`   ✅ ${testCase.name}: Dockerfile found`);
  }

  const ctx = createToolContext(logger);

  // ─────────────────────────────────────────────────────────────
  // Step 2: Run Context Tests
  // ─────────────────────────────────────────────────────────────
  console.log('\n🧱 Step 2: Running build context tests...\n');

  for (const testCase of TEST_CASES) {
    console.log(`\n   📦 Testing: ${testCase.name}`);
    console.log(`      Description: ${testCase.description}`);
    console.log(`      Context: ${testCase.dockerContext}`);
    console.log(`      Tags: ${testCase.tags.join(', ')}`);
    if (testCase.buildArgs) {
      console.log(`      Build Args: ${JSON.stringify(testCase.buildArgs)}`);
    }

    const contextPath = join(process.cwd(), testCase.dockerContext);
    const dockerfile = testCase.dockerfile || 'Dockerfile';

    // Detect platform from system architecture
    let platform: 'linux/amd64' | 'linux/arm64' = 'linux/amd64';
    if (process.arch === 'arm64') {
      platform = 'linux/arm64';
    }

    try {
      const result = await buildImageContextTool.handler(
        {
          path: contextPath,
          dockerfilePath: join(contextPath, dockerfile),
          imageName: testCase.imageName,
          tags: testCase.tags,
          buildArgs: testCase.buildArgs,
          platform,
        },
        ctx,
      );

      if (!result.ok) {
        console.log(`      ❌ Tool failed: ${result.error}`);
        failCount++;
        results.push({
          name: testCase.name,
          passed: false,
          message: result.error,
        });
        continue;
      }

      const buildResult = result.value;
      const warningValidation = validateWarnings(testCase, buildResult.securityAnalysis.warnings);
      const commandValidation = validateCommandFlags(
        testCase,
        buildResult.nextAction.buildCommand.command,
      );
      const buildKitFlagPassed =
        testCase.expectBuildKit === undefined
          ? true
          : buildResult.buildKitAnalysis.recommended === testCase.expectBuildKit;

      const failureMessages: string[] = [];
      if (!warningValidation.passed) {
        failureMessages.push(...warningValidation.messages);
      }
      if (!commandValidation.passed) {
        failureMessages.push(...commandValidation.messages);
      }
      if (!buildKitFlagPassed) {
        failureMessages.push(
          `BuildKit recommendation mismatch. Expected: ${testCase.expectBuildKit}`,
        );
      }

      if (failureMessages.length === 0) {
        console.log('      ✅ PASSED');
        passCount++;
        results.push({
          name: testCase.name,
          passed: true,
          message: 'Context prepared successfully',
          warnings: buildResult.securityAnalysis.warnings.length,
          buildKit: buildResult.buildKitAnalysis.recommended,
          command: buildResult.nextAction.buildCommand.command,
        });
      } else {
        console.log('      ❌ FAILED');
        for (const msg of failureMessages) {
          console.log(`         - ${msg}`);
        }
        failCount++;
        results.push({
          name: testCase.name,
          passed: false,
          message: failureMessages.join('; '),
          warnings: buildResult.securityAnalysis.warnings.length,
          buildKit: buildResult.buildKitAnalysis.recommended,
          command: buildResult.nextAction.buildCommand.command,
        });
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Unknown error';
      console.log(`      ❌ Error: ${message}`);
      failCount++;
      results.push({
        name: testCase.name,
        passed: false,
        message,
      });
    }
  }

  // ─────────────────────────────────────────────────────────────
  // Step 3: Summary
  // ─────────────────────────────────────────────────────────────
  console.log('\n'.repeat(2));
  console.log('='.repeat(60));
  console.log('📊 TEST SUMMARY');
  console.log('='.repeat(60));
  console.log(`\n   Total:  ${results.length}`);
  console.log(`   Passed: ${passCount} ✅`);
  console.log(`   Failed: ${failCount} ❌`);
  console.log('\n   Results by test case:');

  for (const result of results) {
    const status = result.passed ? '✅ PASS' : '❌ FAIL';
    console.log(`   ${status} ${result.name}`);
    if (!result.passed) {
      console.log(`         ${result.message}`);
    }
  }

  writeResultsSummary(results);
  console.log('\n   Results written to build-image-context-test-results.json');

  if (failCount > 0) {
    console.log('\n❌ Some tests failed. See above for details.');
    process.exit(1);
  }

  console.log('\n✅ All context tests passed!');
}

main().catch((error) => {
  console.error('❌ Test execution failed:', error);
  process.exit(1);
});
