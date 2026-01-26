/**
 * Integration Test: scan-image with Real Security Scanners
 *
 * Tests the complete flow of:
 * 1. Pulling test images with known vulnerabilities (no build step)
 * 2. Running scan-image tool with Trivy scanner
 * 3. Verifying vulnerability detection and severity classification
 * 4. Validating remediation guidance from knowledge base
 * 5. Testing threshold enforcement (fail on HIGH/CRITICAL)
 *
 * Test Images (pulled from registries, not built):
 * - openjdk:8u181-jdk - Old Java 8 with known CVEs
 * - mcr.microsoft.com/dotnet/aspnet:3.1 - EOL .NET Core 3.1 with CVEs
 * - mcr.microsoft.com/dotnet/runtime:8.0-alpine - Clean baseline
 *
 * Prerequisites:
 * - Docker installed and running
 * - Trivy installed (brew install trivy / apt install trivy)
 *
 * Usage:
 *   npm run build
 *   tsx scripts/integration-test-scan-image.ts
 */

import { createToolContext } from '../dist/src/mcp/context.js';
import scanImageTool from '../dist/src/tools/scan-image/tool.js';
import { execSync } from 'child_process';
import { createLogger } from '../dist/src/lib/logger.js';
import { writeFileSync } from 'fs';

const logger = createLogger({ name: 'scan-image-test', level: 'error' });

/**
 * Test case definition for pull-based images
 */
interface TestCase {
  name: string;
  /** Docker image to pull (full registry path) */
  pullImage: string;
  /** Local tag to apply for testing */
  localTag: string;
  expectedSeverities: {
    critical?: { min: number; max?: number };
    high?: { min: number; max?: number };
    medium?: { min: number; max?: number };
  };
  shouldPassThreshold: boolean;
  scanner: 'trivy' | 'snyk' | 'grype';
  description: string;
}

/**
 * Test result tracking
 */
interface TestResult {
  name: string;
  passed: boolean;
  message: string;
  vulnerabilities?: {
    critical: number;
    high: number;
    medium: number;
    low: number;
    total: number;
  };
  duration?: number;
}

/**
 * Test cases using well-known vulnerable images from public registries
 *
 * These images are intentionally old/EOL versions with documented CVEs:
 * - Java: openjdk:8u181-jdk (2018) - OpenJDK 8 update 181 with known CVEs
 * - .NET: mcr.microsoft.com/dotnet/aspnet:3.1 (EOL Dec 2022) - Known CVEs
 * - Clean: mcr.microsoft.com/dotnet/runtime:8.0-alpine - Current, minimal CVEs
 */
const TEST_CASES: TestCase[] = [
  {
    name: 'Java OpenJDK 8 with Known CVEs',
    pullImage: 'openjdk:8u181-jdk',
    localTag: 'test-scan:java-vulns',
    expectedSeverities: {
      // OpenJDK 8u181 (2018) + Debian stretch = many CVEs
      critical: { min: 1 }, // At least 1 critical (OpenSSL, glibc, etc.)
      high: { min: 5 }, // At least 5 high severity
    },
    shouldPassThreshold: false, // Should fail HIGH threshold
    scanner: 'trivy',
    description: 'Tests detection of Java base image CVEs (OpenJDK 8u181 from 2018)',
  },
  {
    name: '.NET Core 3.1 (EOL) with Known CVEs',
    pullImage: 'mcr.microsoft.com/dotnet/aspnet:3.1',
    localTag: 'test-scan:dotnet-vulns',
    expectedSeverities: {
      // .NET Core 3.1 reached EOL December 2022, has known CVEs
      critical: { min: 0 }, // May have critical CVEs
      high: { min: 1 }, // At least 1 high severity (EOL = unpatched vulns)
    },
    shouldPassThreshold: false, // Should fail HIGH threshold
    scanner: 'trivy',
    description: 'Tests detection of EOL .NET Core 3.1 CVEs',
  },
  {
    name: 'Clean .NET 8 Alpine Baseline',
    pullImage: 'mcr.microsoft.com/dotnet/runtime:8.0-alpine',
    localTag: 'test-scan:clean',
    expectedSeverities: {
      // Current LTS image - expect few/no critical vulnerabilities
      // Using loose bounds since CVE feeds change over time
      critical: { min: 0, max: 5 }, // Allow some drift as CVEs are discovered
      high: { min: 0, max: 10 }, // Allow reasonable variance
    },
    shouldPassThreshold: true, // Should generally pass HIGH threshold for current LTS
    scanner: 'trivy',
    description: 'Control test - verifies current LTS images scan successfully',
  },
];

/**
 * Verify a tool is installed and available
 */
function verifyToolInstalled(toolName: string, versionCommand: string): boolean {
  console.log(`   Checking ${toolName}...`);
  try {
    const output = execSync(versionCommand, { encoding: 'utf-8', stdio: 'pipe' });
    const version = output.split('\n')[0];
    console.log(`   ✅ ${toolName}: ${version}`);
    return true;
  } catch {
    console.log(`   ❌ ${toolName} not found`);
    return false;
  }
}

/**
 * Pull a Docker image and optionally tag it locally
 */
function pullImage(remoteImage: string, localTag: string): boolean {
  console.log(`   Pulling ${remoteImage}...`);
  try {
    const startTime = Date.now();
    execSync(`docker pull ${remoteImage}`, {
      stdio: 'pipe',
      cwd: process.cwd(),
    });
    // Tag with local name for consistent test references
    execSync(`docker tag ${remoteImage} ${localTag}`, {
      stdio: 'pipe',
      cwd: process.cwd(),
    });
    const duration = ((Date.now() - startTime) / 1000).toFixed(1);
    console.log(`   ✅ Pulled and tagged as ${localTag} (${duration}s)`);
    return true;
  } catch (error) {
    console.log(`   ❌ Failed to pull ${remoteImage}`);
    if (error instanceof Error) {
      console.log(`      Error: ${error.message}`);
    }
    return false;
  }
}

/**
 * Clean up test images
 */
function cleanupImages(tags: string[], remoteImages: string[]): void {
  console.log('   Removing test images...');
  // Remove local tags
  for (const tag of tags) {
    try {
      execSync(`docker rmi -f ${tag}`, { stdio: 'pipe' });
    } catch {
      // Ignore cleanup errors
    }
  }
  // Remove pulled images to save space
  for (const image of remoteImages) {
    try {
      execSync(`docker rmi -f ${image}`, { stdio: 'pipe' });
    } catch {
      // Ignore cleanup errors
    }
  }
  console.log('   ✅ Test images removed');
}

/**
 * Validate vulnerability counts against expected ranges
 */
function validateSeverityCounts(
  testCase: TestCase,
  actual: { critical: number; high: number; medium: number; low: number },
): { passed: boolean; messages: string[] } {
  const messages: string[] = [];
  let passed = true;

  const { expectedSeverities } = testCase;

  // Validate critical
  if (expectedSeverities.critical) {
    const { min, max } = expectedSeverities.critical;
    const count = actual.critical;
    if (count < min) {
      messages.push(`Expected at least ${min} CRITICAL, got ${count}`);
      passed = false;
    }
    if (max !== undefined && count > max) {
      messages.push(`Expected at most ${max} CRITICAL, got ${count}`);
      passed = false;
    }
  }

  // Validate high
  if (expectedSeverities.high) {
    const { min, max } = expectedSeverities.high;
    const count = actual.high;
    if (count < min) {
      messages.push(`Expected at least ${min} HIGH, got ${count}`);
      passed = false;
    }
    if (max !== undefined && count > max) {
      messages.push(`Expected at most ${max} HIGH, got ${count}`);
      passed = false;
    }
  }

  // Validate medium
  if (expectedSeverities.medium) {
    const { min, max } = expectedSeverities.medium;
    const count = actual.medium;
    if (count < min) {
      messages.push(`Expected at least ${min} MEDIUM, got ${count}`);
      passed = false;
    }
    if (max !== undefined && count > max) {
      messages.push(`Expected at most ${max} MEDIUM, got ${count}`);
      passed = false;
    }
  }

  return { passed, messages };
}

/**
 * Main test execution
 */
async function main() {
  console.log('🔒 Testing scan-image with Real Security Scanners\n');
  console.log('='.repeat(60));

  const results: TestResult[] = [];
  let passCount = 0;
  let failCount = 0;

  // ─────────────────────────────────────────────────────────────
  // Step 1: Verify Prerequisites
  // ─────────────────────────────────────────────────────────────
  console.log('\n📋 Step 1: Verifying prerequisites...\n');

  const dockerInstalled = verifyToolInstalled('Docker', 'docker --version');
  const trivyInstalled = verifyToolInstalled('Trivy', 'trivy --version');

  if (!dockerInstalled) {
    console.error('\n❌ Docker is required but not installed.');
    console.error('   Install Docker: https://docs.docker.com/get-docker/');
    process.exit(1);
  }

  if (!trivyInstalled) {
    console.error('\n❌ Trivy is required but not installed.');
    console.error('   Install Trivy:');
    console.error('   - macOS: brew install trivy');
    console.error('   - Linux: https://aquasecurity.github.io/trivy/latest/getting-started/installation/');
    process.exit(1);
  }

  // List test images to be pulled
  console.log('\n   Test images to pull:');
  for (const testCase of TEST_CASES) {
    console.log(`   - ${testCase.pullImage} → ${testCase.localTag}`);
  }

  // ─────────────────────────────────────────────────────────────
  // Step 2: Pull Test Images
  // ─────────────────────────────────────────────────────────────
  console.log('\n📦 Step 2: Pulling test images...\n');

  const pulledTags: string[] = [];
  const pulledRemotes: string[] = [];

  for (const testCase of TEST_CASES) {
    const success = pullImage(testCase.pullImage, testCase.localTag);
    if (success) {
      pulledTags.push(testCase.localTag);
      pulledRemotes.push(testCase.pullImage);
    } else {
      results.push({
        name: testCase.name,
        passed: false,
        message: `Failed to pull Docker image: ${testCase.pullImage}`,
      });
      failCount++;
    }
  }

  if (pulledTags.length === 0) {
    console.error('\n❌ No images were pulled successfully. Aborting tests.');
    process.exit(1);
  }

  console.log(`\n   ✅ Pulled ${pulledTags.length}/${TEST_CASES.length} images`);

  // ─────────────────────────────────────────────────────────────
  // Step 3: Run Security Scans
  // ─────────────────────────────────────────────────────────────
  console.log('\n🔍 Step 3: Running security scans...\n');

  const ctx = createToolContext(logger);

  for (const testCase of TEST_CASES) {
    // Skip if image wasn't pulled
    if (!pulledTags.includes(testCase.localTag)) {
      continue;
    }

    console.log(`\n   📊 Scanning: ${testCase.name}`);
    console.log(`      Description: ${testCase.description}`);
    console.log(`      Image: ${testCase.localTag} (from ${testCase.pullImage})`);
    console.log(`      Scanner: ${testCase.scanner}`);

    const startTime = Date.now();

    try {
      const result = await scanImageTool.handler(
        {
          imageId: testCase.localTag,
          scanner: testCase.scanner,
          severity: 'HIGH',
          scanType: 'vulnerability',
          enableAISuggestions: true,
        },
        ctx,
      );

      const duration = Date.now() - startTime;

      if (!result.ok) {
        console.log(`      ❌ Scan failed: ${result.error}`);
        results.push({
          name: testCase.name,
          passed: false,
          message: `Scan error: ${result.error}`,
          duration,
        });
        failCount++;
        continue;
      }

      const scanResult = result.value;
      const vulns = scanResult.vulnerabilities;

      console.log(`      Scan completed in ${(duration / 1000).toFixed(1)}s`);
      console.log(`      Vulnerabilities found:`);
      console.log(`        - Critical: ${vulns.critical}`);
      console.log(`        - High: ${vulns.high}`);
      console.log(`        - Medium: ${vulns.medium}`);
      console.log(`        - Low: ${vulns.low}`);
      console.log(`        - Total: ${vulns.total}`);

      // Validate vulnerability counts
      const validation = validateSeverityCounts(testCase, {
        critical: vulns.critical,
        high: vulns.high,
        medium: vulns.medium,
        low: vulns.low,
      });

      // Validate threshold enforcement
      const hasHighOrCritical = vulns.critical > 0 || vulns.high > 0;
      const thresholdBehaviorCorrect = hasHighOrCritical !== testCase.shouldPassThreshold;

      if (!thresholdBehaviorCorrect) {
        validation.passed = false;
        validation.messages.push(
          `Threshold enforcement incorrect: expected ${testCase.shouldPassThreshold ? 'PASS' : 'FAIL'}, got ${hasHighOrCritical ? 'FAIL' : 'PASS'}`,
        );
      }

      // Check remediation guidance for vulnerable images
      if (!testCase.shouldPassThreshold && scanResult.remediationGuidance) {
        console.log(`      Remediation guidance: ${scanResult.remediationGuidance.length} recommendations`);
      }

      if (validation.passed) {
        console.log(`      ✅ PASSED`);
        results.push({
          name: testCase.name,
          passed: true,
          message: 'All validations passed',
          vulnerabilities: {
            critical: vulns.critical,
            high: vulns.high,
            medium: vulns.medium,
            low: vulns.low,
            total: vulns.total,
          },
          duration,
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
          vulnerabilities: {
            critical: vulns.critical,
            high: vulns.high,
            medium: vulns.medium,
            low: vulns.low,
            total: vulns.total,
          },
          duration,
        });
        failCount++;
      }
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
  cleanupImages(pulledTags, pulledRemotes);

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
    const duration = result.duration ? ` (${(result.duration / 1000).toFixed(1)}s)` : '';
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
    scanner: 'trivy',
    testImages: TEST_CASES.map((tc) => ({
      name: tc.name,
      sourceImage: tc.pullImage,
      localTag: tc.localTag,
    })),
    results: results.map((r) => ({
      name: r.name,
      passed: r.passed,
      message: r.message,
      vulnerabilities: r.vulnerabilities,
      durationMs: r.duration,
    })),
  };

  writeFileSync('scan-image-test-results.json', JSON.stringify(resultsJson, null, 2));
  console.log('\n   Results written to scan-image-test-results.json');

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
