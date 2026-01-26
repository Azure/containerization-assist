/**
 * Integration Test: tag-image with Real Docker Images
 *
 * Tests the complete flow of:
 * 1. Pulling a test image (busybox)
 * 2. Tagging with various formats (versions, registries, multi-tag)
 * 3. Verifying tags are applied correctly
 * 4. Validating Docker image metadata
 *
 * Prerequisites:
 * - Docker installed and running
 *
 * Usage:
 *   npm run build
 *   tsx scripts/integration-test-tag-image.ts
 */

import { createToolContext } from '../dist/src/mcp/context.js';
import tagImageTool from '../dist/src/tools/tag-image/tool.js';
import { execSync } from 'child_process';
import { createLogger } from '../dist/src/lib/logger.js';
import { writeFileSync } from 'fs';

const logger = createLogger({ name: 'tag-image-test', level: 'error' });

/**
 * Test case definition
 */
interface TagTestCase {
  name: string;
  sourceImage: string;
  targetTag: string;
  expectedRepository: string;
  expectedTag: string;
  description: string;
}

/**
 * Test result tracking
 */
interface TestResult {
  name: string;
  passed: boolean;
  message: string;
  duration?: number;
}

/**
 * Test cases for various tagging scenarios
 */
const TEST_CASES: TagTestCase[] = [
  {
    name: 'Simple Version Tag',
    sourceImage: 'busybox:latest',
    targetTag: 'test-tag-image:v1.0.0',
    expectedRepository: 'test-tag-image',
    expectedTag: 'v1.0.0',
    description: 'Basic semver tagging',
  },
  {
    name: 'Latest Tag',
    sourceImage: 'busybox:latest',
    targetTag: 'test-tag-image:latest',
    expectedRepository: 'test-tag-image',
    expectedTag: 'latest',
    description: 'Standard latest tag',
  },
  {
    name: 'Git SHA Tag',
    sourceImage: 'busybox:latest',
    targetTag: 'test-tag-image:sha-abc1234',
    expectedRepository: 'test-tag-image',
    expectedTag: 'sha-abc1234',
    description: 'Git commit SHA tag format',
  },
  {
    name: 'Environment Tag',
    sourceImage: 'busybox:latest',
    targetTag: 'test-tag-image:staging-2024.01.15',
    expectedRepository: 'test-tag-image',
    expectedTag: 'staging-2024.01.15',
    description: 'Environment + date tag format',
  },
  {
    name: 'Registry URL Tag',
    sourceImage: 'busybox:latest',
    targetTag: 'localhost:5000/test-tag-image:v1.0.0',
    expectedRepository: 'test-tag-image',
    expectedTag: 'v1.0.0',
    description: 'Tag with registry URL prefix',
  },
  {
    name: 'Nested Repository Tag',
    sourceImage: 'busybox:latest',
    targetTag: 'myorg/myteam/test-tag-image:v2.0.0',
    expectedRepository: 'myorg/myteam/test-tag-image',
    expectedTag: 'v2.0.0',
    description: 'Nested repository path',
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
    return true;
  } catch (error) {
    console.log('   ❌ Docker not found or not running');
    return false;
  }
}

/**
 * Pull a source image for testing
 */
function pullSourceImage(image: string): boolean {
  console.log(`   Pulling ${image}...`);
  try {
    execSync(`docker pull ${image}`, { stdio: 'pipe' });
    console.log(`   ✅ Pulled ${image}`);
    return true;
  } catch (error) {
    console.log(`   ❌ Failed to pull ${image}`);
    return false;
  }
}

/**
 * Verify a tag exists in Docker
 */
function verifyTagExists(fullTag: string): boolean {
  try {
    execSync(`docker inspect ${fullTag}`, { stdio: 'pipe' });
    return true;
  } catch {
    return false;
  }
}

/**
 * Get image ID for a tag
 */
function getImageId(tag: string): string | null {
  try {
    const output = execSync(`docker inspect --format='{{.Id}}' ${tag}`, {
      encoding: 'utf-8',
      stdio: 'pipe',
    });
    return output.trim();
  } catch {
    return null;
  }
}

/**
 * Cleanup test images
 */
function cleanupTestImages(): void {
  console.log('   Removing test images...');
  try {
    // Get all images and filter in JavaScript (avoids shell-specific grep)
    const imagesOutput = execSync('docker images --format "{{.Repository}}:{{.Tag}}"', {
      encoding: 'utf-8',
      stdio: 'pipe',
    });
    
    const imagesToRemove = imagesOutput
      .split('\n')
      .map((img) => img.trim())
      .filter((img) => img && img.includes('test-tag-image'));
    for (const img of imagesToRemove) {
      try {
        execSync(`docker rmi -f ${img}`, { stdio: 'pipe' });
      } catch {
        // Ignore cleanup errors
      }
    }
    console.log(`   ✅ Cleaned up ${imagesToRemove.length} test images`);
  } catch {
    console.log('   ⚠️ Cleanup completed (some images may not exist)');
  }
}

/**
 * Main test execution
 */
async function main() {
  console.log('🏷️  Testing tag-image with Real Docker Images\n');
  console.log('='.repeat(60));

  const results: TestResult[] = [];
  let passCount = 0;
  let failCount = 0;

  // ─────────────────────────────────────────────────────────────
  // Step 1: Verify Prerequisites
  // ─────────────────────────────────────────────────────────────
  console.log('\n📋 Step 1: Verifying prerequisites...\n');

  if (!verifyDockerInstalled()) {
    console.error('\n❌ Docker is required but not installed or not running.');
    process.exit(1);
  }

  // ─────────────────────────────────────────────────────────────
  // Step 2: Pull Source Image
  // ─────────────────────────────────────────────────────────────
  console.log('\n📦 Step 2: Preparing source image...\n');

  const sourceImage = 'busybox:latest';
  if (!pullSourceImage(sourceImage)) {
    console.error(`\n❌ Failed to pull source image: ${sourceImage}`);
    process.exit(1);
  }

  const sourceImageId = getImageId(sourceImage);
  console.log(`   Source image ID: ${sourceImageId?.substring(0, 20)}...`);

  // ─────────────────────────────────────────────────────────────
  // Step 3: Run Tagging Tests
  // ─────────────────────────────────────────────────────────────
  console.log('\n🏷️  Step 3: Running tag tests...\n');

  const ctx = createToolContext(logger);

  for (const testCase of TEST_CASES) {
    console.log(`\n   📊 Testing: ${testCase.name}`);
    console.log(`      Description: ${testCase.description}`);
    console.log(`      Target: ${testCase.targetTag}`);

    const startTime = Date.now();

    try {
      const result = await tagImageTool.handler(
        {
          imageId: testCase.sourceImage,
          tag: testCase.targetTag,
        },
        ctx,
      );

      const duration = Date.now() - startTime;

      if (!result.ok) {
        console.log(`      ❌ Tag failed: ${result.error}`);
        results.push({
          name: testCase.name,
          passed: false,
          message: `Tagging error: ${result.error}`,
          duration,
        });
        failCount++;
        continue;
      }

      // Verify the tag was actually applied
      const tagExists = verifyTagExists(testCase.targetTag);
      if (!tagExists) {
        console.log(`      ❌ Tag was not applied to Docker`);
        results.push({
          name: testCase.name,
          passed: false,
          message: 'Tag not found in Docker after operation',
          duration,
        });
        failCount++;
        continue;
      }

      // Verify the image IDs match
      const taggedImageId = getImageId(testCase.targetTag);
      const sourceId = getImageId(testCase.sourceImage);
      
      if (taggedImageId !== sourceId) {
        console.log(`      ❌ Image ID mismatch`);
        console.log(`         Source: ${sourceId}`);
        console.log(`         Tagged: ${taggedImageId}`);
        results.push({
          name: testCase.name,
          passed: false,
          message: 'Image ID does not match source image',
          duration,
        });
        failCount++;
        continue;
      }

      // Verify result structure
      if (!result.value.success || !result.value.tags || result.value.tags.length === 0) {
        console.log(`      ❌ Invalid result structure`);
        results.push({
          name: testCase.name,
          passed: false,
          message: 'Result missing success flag or tags array',
          duration,
        });
        failCount++;
        continue;
      }

      console.log(`      ✅ PASSED (${(duration / 1000).toFixed(2)}s)`);
      console.log(`         Tags applied: ${result.value.tags.join(', ')}`);
      console.log(`         Summary: ${result.value.summary}`);

      results.push({
        name: testCase.name,
        passed: true,
        message: 'All validations passed',
        duration,
      });
      passCount++;
    } catch (error) {
      const duration = Date.now() - startTime;
      console.log(`      ❌ Error: ${error instanceof Error ? error.message : 'Unknown error'}`);
      results.push({
        name: testCase.name,
        passed: false,
        message: `Exception: ${error instanceof Error ? error.message : 'Unknown error'}`,
        duration,
      });
      failCount++;
    }
  }

  // ─────────────────────────────────────────────────────────────
  // Step 4: Cleanup
  // ─────────────────────────────────────────────────────────────
  console.log('\n🧹 Step 4: Cleaning up...\n');
  cleanupTestImages();

  // ─────────────────────────────────────────────────────────────
  // Step 5: Generate Summary
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
    const duration = result.duration ? ` (${(result.duration / 1000).toFixed(2)}s)` : '';
    console.log(`   ${status} ${result.name}${duration}`);
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
      durationMs: r.duration,
    })),
  };

  writeFileSync('tag-image-test-results.json', JSON.stringify(resultsJson, null, 2));
  console.log('\n   Results written to tag-image-test-results.json');

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
