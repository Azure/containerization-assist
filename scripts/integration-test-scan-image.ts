/**
 * Integration Test: scan-image with Real Security Scanners
 *
 * Tests the complete flow of:
 * 1. Pulling/building test images with known vulnerabilities
 * 2. Running scan-image tool with Trivy scanner
 * 3. Verifying vulnerability detection and severity classification
 * 4. Validating remediation guidance from knowledge base
 * 5. Testing threshold enforcement (fail on HIGH/CRITICAL)
 * 6. Verifying zero-vulnerability baseline with scratch image
 *
 * Test Images:
 * - openjdk:8u181-jdk - Old Java 8 with known CVEs (pulled)
 * - mcr.microsoft.com/dotnet/aspnet:3.1 - EOL .NET Core 3.1 with CVEs (pulled)
 * - mcr.microsoft.com/dotnet/runtime:8.0-alpine - Current LTS baseline (pulled)
 * - FROM scratch - Empty image with zero packages (built locally)
 *
 * Prerequisites:
 * - Docker installed and running
 * - Trivy installed (brew install trivy / apt install trivy)
 *
 * Reliability Features:
 * - Automatic retry with exponential backoff for transient Docker Hub failures
 * - Handles HTTP 500 errors from Docker registries
 * - Up to 3 attempts per image pull
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
 * Test case definition
 */
interface TestCase {
  name: string;
  /** Docker image to pull (full registry path). Omit if using buildInline. */
  pullImage?: string;
  /** Inline Dockerfile content to build instead of pulling */
  buildInline?: string;
  /** Local tag to apply for testing */
  localTag: string;
  expectedSeverities: {
    critical?: { min: number; max?: number };
    high?: { min: number; max?: number };
    medium?: { min: number; max?: number };
  };
  shouldPassThreshold: boolean;
  scanner: 'trivy' | 'snyk' | 'grype' | 'osv';
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
  };
  duration?: number;
}

/**
 * Test cases using well-known vulnerable images from public registries
 * and locally-built images for zero-vulnerability baselines.
 *
 * These images are intentionally old/EOL versions with documented CVEs:
 * - Java: openjdk:8u181-jdk (2018) - OpenJDK 8 update 181 with known CVEs
 * - .NET: mcr.microsoft.com/dotnet/aspnet:3.1 (EOL Dec 2022) - Known CVEs
 * - .NET 8 Alpine: Current LTS - may accumulate CVEs over time
 * - Scratch: Empty image - guaranteed zero vulnerabilities
 */
const TEST_CASES: TestCase[] = [
  {
    name: 'OSV Java OpenJDK 8 with Known CVEs',
    pullImage: 'openjdk:8u181-jdk',
    localTag: 'test-scan:java-vulns',
    expectedSeverities: {
      // OpenJDK 8u181 (2018) + Debian stretch = many CVEs
      critical: { min: 1 }, // At least 1 critical (OpenSSL, glibc, etc.)
      high: { min: 5 }, // At least 5 high severity
    },
    shouldPassThreshold: false, // Should fail HIGH threshold
    scanner: 'osv',
    description: 'Tests detection of Java base image CVEs (OpenJDK 8u181 from 2018)',
  },
  {
    name: 'OSV Alpine 3.9 with Known CVEs',
    pullImage: 'alpine:3.9',
    localTag: 'test-scan:alpine-vulns',
    expectedSeverities: {
      // Alpine 3.9 (January 2019) - old version, minimal base (14 packages)
      // OSV now correctly maps binary packages (libcrypto1.1, libssl1.1) to source package (openssl)
      // Finds 6 unique OpenSSL CVEs: CVE-2020-1971, CVE-2021-23839, CVE-2021-23840,
      // CVE-2021-23841, CVE-2021-3449, CVE-2021-3450
      high: { min: 2 }, // CVE-2021-23840, CVE-2021-3450
      medium: { min: 3 }, // CVE-2020-1971, CVE-2021-23841, CVE-2021-3449
    },
    shouldPassThreshold: false, // Will fail due to 2 HIGH severity OpenSSL CVEs
    scanner: 'osv',
    description: 'Tests detection of Alpine package CVEs (Alpine 3.9 from 2019)',
  },
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
    name: '.NET 8 Alpine LTS Baseline',
    pullImage: 'mcr.microsoft.com/dotnet/runtime:8.0-alpine',
    localTag: 'test-scan:clean',
    expectedSeverities: {
      // Current LTS image - may accumulate CVEs over time as new vulnerabilities are disclosed.
      // We don't control this image so we use loose bounds and don't assert pass/fail threshold.
      critical: { min: 0, max: 20 },
      high: { min: 0, max: 50 },
    },
    shouldPassThreshold: false, // LTS images can accumulate CVEs; don't assume clean
    scanner: 'trivy',
    description: 'Verifies LTS image scans complete successfully (CVE counts may vary)',
  },
  {
    name: 'Scratch Image Zero Vulnerabilities',
    buildInline: 'FROM scratch\nCOPY <<EOF /empty\nEOF',
    localTag: 'test-scan:scratch',
    expectedSeverities: {
      // scratch + single empty file via heredoc: no OS, no packages, no binaries — zero vulnerabilities
      critical: { min: 0, max: 0 },
      high: { min: 0, max: 0 },
      medium: { min: 0, max: 0 },
    },
    shouldPassThreshold: true, // Guaranteed — nothing scannable
    scanner: 'trivy',
    description: 'Control test - scratch image with empty heredoc file, zero vulnerabilities',
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
 * Implements retry logic for transient Docker Hub failures (HTTP 500 errors)
 */
function pullImage(remoteImage: string, localTag: string, maxRetries = 3): boolean {
  console.log(`   Pulling ${remoteImage}...`);

  for (let attempt = 1; attempt <= maxRetries; attempt++) {
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
      const errorMessage = error instanceof Error ? error.message : String(error);
      const stderrMessage =
        error && typeof error === 'object' && 'stderr' in error
          ? String((error as { stderr?: unknown }).stderr ?? '')
          : '';
      const combinedError = `${errorMessage}\n${stderrMessage}`;
      const isRetryable =
        combinedError.includes('500') || combinedError.includes('Internal Server Error');

      if (attempt < maxRetries && isRetryable) {
        console.log(`   ⚠️  Attempt ${attempt}/${maxRetries} failed (transient error), retrying...`);
        // Exponential backoff: 2s, 4s, 8s
        const backoffMs = Math.pow(2, attempt) * 1000;
        Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, backoffMs);
      } else {
        console.log(`   ❌ Failed to pull ${remoteImage}${attempt > 1 ? ` after ${attempt} attempts` : ''}`);
        if (error instanceof Error) {
          console.log(`      Error: ${error.message}`);
        }
        return false;
      }
    }
  }

  return false;
}

/**
 * Build a Docker image from inline Dockerfile content
 */
function buildImage(dockerfileContent: string, localTag: string): boolean {
  console.log(`   Building ${localTag} from inline Dockerfile...`);
  try {
    const startTime = Date.now();
    execSync(`echo '${dockerfileContent}' | docker build -t ${localTag} -`, {
      stdio: 'pipe',
      cwd: process.cwd(),
    });
    const duration = ((Date.now() - startTime) / 1000).toFixed(1);
    console.log(`   ✅ Built ${localTag} (${duration}s)`);
    return true;
  } catch (error) {
    console.log(`   ❌ Failed to build ${localTag}`);
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
    console.error(
      '   - Linux: https://aquasecurity.github.io/trivy/latest/getting-started/installation/',
    );
    process.exit(1);
  }

  // List test images
  console.log('\n   Test images:');
  for (const testCase of TEST_CASES) {
    if (testCase.pullImage) {
      console.log(`   - pull ${testCase.pullImage} → ${testCase.localTag}`);
    } else if (testCase.buildInline) {
      console.log(`   - build inline → ${testCase.localTag}`);
    }
  }

  // ─────────────────────────────────────────────────────────────
  // Step 2: Prepare Test Images (pull or build)
  // ─────────────────────────────────────────────────────────────
  console.log('\n📦 Step 2: Preparing test images...\n');

  const readyTags: string[] = [];
  const pulledRemotes: string[] = [];

  for (const testCase of TEST_CASES) {
    if (testCase.pullImage) {
      const success = pullImage(testCase.pullImage, testCase.localTag);
      if (success) {
        readyTags.push(testCase.localTag);
        pulledRemotes.push(testCase.pullImage);
      } else {
        results.push({
          name: testCase.name,
          passed: false,
          message: `Failed to pull Docker image: ${testCase.pullImage}`,
        });
        failCount++;
      }
    } else if (testCase.buildInline) {
      const success = buildImage(testCase.buildInline, testCase.localTag);
      if (success) {
        readyTags.push(testCase.localTag);
        // Track base images used in multi-stage builds for cleanup
        const baseImageMatch = testCase.buildInline?.match(/--from=(\S+)/);
        if (baseImageMatch) {
          pulledRemotes.push(baseImageMatch[1]);
        }
      } else {
        results.push({
          name: testCase.name,
          passed: false,
          message: `Failed to build Docker image: ${testCase.localTag}`,
        });
        failCount++;
      }
    }
  }

  if (readyTags.length === 0) {
    console.error('\n❌ No images were prepared successfully. Aborting tests.');
    process.exit(1);
  }

  console.log(`\n   ✅ Prepared ${readyTags.length}/${TEST_CASES.length} images`);

  // ─────────────────────────────────────────────────────────────
  // Step 3: Run Security Scans
  // ─────────────────────────────────────────────────────────────
  console.log('\n🔍 Step 3: Running security scans...\n');

  const ctx = createToolContext(logger);

  for (const testCase of TEST_CASES) {
    // Skip if image wasn't prepared
    if (!readyTags.includes(testCase.localTag)) {
      continue;
    }

    console.log(`\n   📊 Scanning: ${testCase.name}`);
    console.log(`      Description: ${testCase.description}`);
    const imageSource = testCase.pullImage ? `from ${testCase.pullImage}` : 'built inline';
    console.log(`      Image: ${testCase.localTag} (${imageSource})`);
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
        console.log(
          `      Remediation guidance: ${scanResult.remediationGuidance.length} recommendations`,
        );
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
  cleanupImages(readyTags, pulledRemotes);

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
      sourceImage: tc.pullImage ?? 'built inline',
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
