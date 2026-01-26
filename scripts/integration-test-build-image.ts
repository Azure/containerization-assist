/**
 * Integration Test: build-image with Multi-Language Scenarios
 *
 * Tests the complete flow of:
 * 1. Building Node.js application with multi-stage Dockerfile
 * 2. Building Python application with multi-stage Dockerfile
 * 3. Verifying build success, image metadata, and layer counts
 * 4. Testing build arguments injection
 * 5. Validating image sizes are reasonable
 *
 * Prerequisites:
 * - Docker installed and running
 *
 * Usage:
 *   npm run build
 *   tsx scripts/integration-test-build-image.ts
 */

import { createToolContext } from '../dist/src/mcp/context.js';
import buildImageTool from '../dist/src/tools/build-image/tool.js';
import { execSync } from 'child_process';
import { createLogger } from '../dist/src/lib/logger.js';
import { existsSync } from 'fs';
import { join } from 'path';
import { writeFileSync } from 'fs';

const logger = createLogger({ name: 'build-image-test', level: 'error' });

/**
 * Test case definition
 */
interface BuildTestCase {
  name: string;
  dockerContext: string;
  dockerfile?: string;
  tags: string[];
  buildArgs?: Record<string, string>;
  expectedSize?: { min: number; max: number }; // bytes
  expectedLayers?: { min: number; max?: number };
  shouldSucceed: boolean;
  description: string;
}

/**
 * Test result tracking
 */
interface TestResult {
  name: string;
  passed: boolean;
  message: string;
  imageSize?: number;
  layers?: number;
  buildTime?: number;
}

/**
 * Test cases for multi-language builds
 */
const TEST_CASES: BuildTestCase[] = [
  {
    name: 'Java Multi-Stage Build',
    dockerContext: 'test/fixtures/build-scenarios/java',
    tags: ['test-build:java-app'],
    expectedSize: { min: 100_000_000, max: 400_000_000 }, // 100MB - 400MB
    expectedLayers: { min: 5, max: 20 },
    shouldSucceed: true,
    description: 'Tests Java build with Eclipse Temurin JRE and multi-stage',
  },
  {
    name: 'Java with Build Args',
    dockerContext: 'test/fixtures/build-scenarios/java',
    tags: ['test-build:java-args'],
    buildArgs: {
      VERSION: '2.0.0',
    },
    expectedSize: { min: 100_000_000, max: 400_000_000 },
    shouldSucceed: true,
    description: 'Tests Java build argument injection',
  },
  {
    name: '.NET Multi-Stage Build',
    dockerContext: 'test/fixtures/build-scenarios/dotnet',
    tags: ['test-build:dotnet-app'],
    expectedSize: { min: 80_000_000, max: 300_000_000 }, // 80MB - 300MB
    expectedLayers: { min: 5, max: 25 },
    shouldSucceed: true,
    description: 'Tests .NET 8 build with ASP.NET runtime and multi-stage',
  },
  {
    name: '.NET with Build Args',
    dockerContext: 'test/fixtures/build-scenarios/dotnet',
    tags: ['test-build:dotnet-args'],
    buildArgs: {
      VERSION: '3.0.0',
    },
    expectedSize: { min: 80_000_000, max: 300_000_000 },
    shouldSucceed: true,
    description: 'Tests .NET build with version argument',
  },
];

/**
 * Verify Docker is installed and running
 */
function verifyDockerInstalled(): boolean {
  console.log('   Checking Docker...');
  try {
    const output = execSync('docker --version', { encoding: 'utf-8', stdio: 'pipe' });
    console.log(`   ✅ Docker: ${output.trim()}`);
    
    // Verify Docker daemon is running
    execSync('docker info', { stdio: 'pipe' });
    console.log('   ✅ Docker daemon is running');
    return true;
  } catch (error) {
    console.log('   ❌ Docker not found or not running');
    return false;
  }
}

/**
 * Get image metadata (size, layers)
 */
function getImageMetadata(imageTag: string): { size: number; layers: number } | null {
  try {
    // Get image size
    const sizeOutput = execSync(
      `docker inspect --format='{{.Size}}' ${imageTag}`,
      { encoding: 'utf-8', stdio: 'pipe' },
    );
    const size = parseInt(sizeOutput.trim(), 10);

    // Get layer count (count lines in JavaScript instead of shell pipeline)
    const layersOutput = execSync(
      `docker history ${imageTag} --format='{{.ID}}'`,
      { encoding: 'utf-8', stdio: 'pipe' },
    );
    const layers = layersOutput
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => line.length > 0).length;

    return { size, layers };
  } catch {
    return null;
  }
}

/**
 * Format bytes to human-readable size
 */
function formatSize(bytes: number): string {
  if (bytes >= 1_000_000_000) {
    return `${(bytes / 1_000_000_000).toFixed(2)}GB`;
  }
  if (bytes >= 1_000_000) {
    return `${(bytes / 1_000_000).toFixed(2)}MB`;
  }
  if (bytes >= 1_000) {
    return `${(bytes / 1_000).toFixed(2)}KB`;
  }
  return `${bytes}B`;
}

/**
 * Cleanup test images
 */
function cleanupTestImages(tags: string[]): void {
  console.log('   Removing test images...');
  for (const tag of tags) {
    try {
      execSync(`docker rmi -f ${tag}`, { stdio: 'pipe' });
    } catch {
      // Ignore cleanup errors
    }
  }
  console.log(`   ✅ Cleaned up ${tags.length} test images`);
}

/**
 * Validate image against expected constraints
 */
function validateImage(
  testCase: BuildTestCase,
  metadata: { size: number; layers: number },
): { passed: boolean; messages: string[] } {
  const messages: string[] = [];
  let passed = true;

  // Validate size
  if (testCase.expectedSize) {
    const { min, max } = testCase.expectedSize;
    if (metadata.size < min) {
      messages.push(`Image size ${formatSize(metadata.size)} is smaller than expected min ${formatSize(min)}`);
      passed = false;
    }
    if (metadata.size > max) {
      messages.push(`Image size ${formatSize(metadata.size)} exceeds expected max ${formatSize(max)}`);
      passed = false;
    }
  }

  // Validate layers
  if (testCase.expectedLayers) {
    const { min, max } = testCase.expectedLayers;
    if (metadata.layers < min) {
      messages.push(`Layer count ${metadata.layers} is less than expected min ${min}`);
      passed = false;
    }
    if (max !== undefined && metadata.layers > max) {
      messages.push(`Layer count ${metadata.layers} exceeds expected max ${max}`);
      passed = false;
    }
  }

  return { passed, messages };
}

/**
 * Main test execution
 */
async function main() {
  console.log('🔨 Testing build-image with Multi-Language Scenarios\n');
  console.log('='.repeat(60));

  const results: TestResult[] = [];
  let passCount = 0;
  let failCount = 0;
  const builtTags: string[] = [];

  // ─────────────────────────────────────────────────────────────
  // Step 1: Verify Prerequisites
  // ─────────────────────────────────────────────────────────────
  console.log('\n📋 Step 1: Verifying prerequisites...\n');

  if (!verifyDockerInstalled()) {
    console.error('\n❌ Docker is required but not installed or running.');
    process.exit(1);
  }

  // Verify test fixtures exist
  console.log('\n   Checking test fixtures...');
  for (const testCase of TEST_CASES) {
    const dockerfilePath = join(process.cwd(), testCase.dockerContext, 'Dockerfile');
    if (!existsSync(dockerfilePath)) {
      console.error(`   ❌ Missing Dockerfile: ${dockerfilePath}`);
      process.exit(1);
    }
    console.log(`   ✅ ${testCase.name}: Dockerfile found`);
  }

  // ─────────────────────────────────────────────────────────────
  // Step 2: Run Build Tests
  // ─────────────────────────────────────────────────────────────
  console.log('\n🔨 Step 2: Running build tests...\n');

  const ctx = createToolContext(logger);

  for (const testCase of TEST_CASES) {
    console.log(`\n   📦 Building: ${testCase.name}`);
    console.log(`      Description: ${testCase.description}`);
    console.log(`      Context: ${testCase.dockerContext}`);
    console.log(`      Tags: ${testCase.tags.join(', ')}`);
    if (testCase.buildArgs) {
      console.log(`      Build Args: ${JSON.stringify(testCase.buildArgs)}`);
    }

    const startTime = Date.now();

    try {
      const contextPath = join(process.cwd(), testCase.dockerContext);
      
      // Detect platform
      let platform: 'linux/amd64' | 'linux/arm64' = 'linux/amd64';
      try {
        const arch = execSync('uname -m', { encoding: 'utf-8' }).trim();
        if (arch === 'arm64' || arch === 'aarch64') {
          platform = 'linux/arm64';
        }
      } catch {
        // Default to amd64
      }
      
      const result = await buildImageTool.handler(
        {
          path: contextPath,
          tags: testCase.tags,
          buildArgs: testCase.buildArgs,
          platform,
          strictPlatformValidation: false,
        },
        ctx,
      );

      const buildTime = Date.now() - startTime;

      if (!result.ok) {
        if (testCase.shouldSucceed) {
          console.log(`      ❌ Build failed: ${result.error}`);
          results.push({
            name: testCase.name,
            passed: false,
            message: `Build error: ${result.error}`,
            buildTime,
          });
          failCount++;
        } else {
          console.log(`      ✅ Build failed as expected`);
          results.push({
            name: testCase.name,
            passed: true,
            message: 'Build failed as expected',
            buildTime,
          });
          passCount++;
        }
        continue;
      }

      if (!testCase.shouldSucceed) {
        console.log(`      ❌ Build should have failed but succeeded`);
        results.push({
          name: testCase.name,
          passed: false,
          message: 'Expected build to fail',
          buildTime,
        });
        failCount++;
        continue;
      }

      // Track built images for cleanup
      builtTags.push(...testCase.tags);

      const buildResult = result.value;
      console.log(`      Build completed in ${(buildTime / 1000).toFixed(1)}s`);
      console.log(`      Image ID: ${buildResult.imageId.substring(0, 20)}...`);
      console.log(`      Image size: ${formatSize(buildResult.size)}`);
      console.log(`      Layers: ${buildResult.layers || 'N/A'}`);

      // Get additional metadata from Docker
      const metadata = getImageMetadata(testCase.tags[0]);
      if (metadata) {
        console.log(`      Docker size: ${formatSize(metadata.size)}`);
        console.log(`      Docker layers: ${metadata.layers}`);
      }

      // Validate image constraints
      const validation = validateImage(testCase, metadata || { size: buildResult.size, layers: buildResult.layers || 0 });

      if (validation.passed) {
        console.log(`      ✅ PASSED`);
        results.push({
          name: testCase.name,
          passed: true,
          message: 'Build successful, all validations passed',
          imageSize: metadata?.size || buildResult.size,
          layers: metadata?.layers || buildResult.layers,
          buildTime,
        });
        passCount++;
      } else {
        console.log(`      ❌ FAILED`);
        for (const msg of validation.messages) {
          console.log(`         - ${msg}`);
        }
        results.push({
          name: testCase.name,
          passed: false,
          message: validation.messages.join('; '),
          imageSize: metadata?.size || buildResult.size,
          layers: metadata?.layers || buildResult.layers,
          buildTime,
        });
        failCount++;
      }
    } catch (error) {
      const buildTime = Date.now() - startTime;
      console.log(`      ❌ Error: ${error instanceof Error ? error.message : 'Unknown error'}`);
      results.push({
        name: testCase.name,
        passed: false,
        message: `Exception: ${error instanceof Error ? error.message : 'Unknown error'}`,
        buildTime,
      });
      failCount++;
    }
  }

  // ─────────────────────────────────────────────────────────────
  // Step 3: Cleanup
  // ─────────────────────────────────────────────────────────────
  console.log('\n🧹 Step 3: Cleaning up...\n');
  cleanupTestImages(builtTags);

  // ─────────────────────────────────────────────────────────────
  // Step 4: Generate Summary
  // ─────────────────────────────────────────────────────────────
  console.log('\n' + '='.repeat(60));
  console.log('📊 TEST SUMMARY');
  console.log('='.repeat(60));
  console.log(`\n   Total:  ${results.length}`);
  console.log(`   Passed: ${passCount} ✅`);
  console.log(`   Failed: ${failCount} ❌`);
  console.log('\n   Results by test case:');

  for (const result of results) {
    const status = result.passed ? '✅ PASS' : '❌ FAIL';
    const time = result.buildTime ? ` (${(result.buildTime / 1000).toFixed(1)}s)` : '';
    const size = result.imageSize ? ` ${formatSize(result.imageSize)}` : '';
    console.log(`   ${status} ${result.name}${time}${size}`);
    if (!result.passed) {
      console.log(`         ${result.message}`);
    }
  }

  // Write results to JSON for CI/CD reporting
  const resultsJson = {
    total: results.length,
    passed: passCount,
    failed: failCount,
    timestamp: new Date().toISOString(),
    results: results.map((r) => ({
      name: r.name,
      passed: r.passed,
      message: r.message,
      imageSize: r.imageSize,
      layers: r.layers,
      buildTimeMs: r.buildTime,
    })),
  };

  writeFileSync('build-image-test-results.json', JSON.stringify(resultsJson, null, 2));
  console.log('\n   Results written to build-image-test-results.json');

  console.log('\n' + '='.repeat(60));

  if (failCount > 0) {
    console.log('❌ Some tests failed. See above for details.');
    process.exit(1);
  }

  console.log('✅ All tests passed!');
}

main().catch((error) => {
  console.error('❌ Test execution failed:', error);
  process.exit(1);
});
