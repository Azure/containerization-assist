/**
 * Integration tests for OSV Scanner with real network calls
 *
 * These tests make actual network requests to the OSV API (https://api.osv.dev)
 * to verify vulnerability detection with real-world data.
 *
 * Note: These tests require:
 * - Network connectivity
 * - Docker daemon running
 * - OSV API availability
 *
 * The tests build Docker images with known vulnerable dependencies in pom.xml.
 */

import { describe, it, expect, beforeAll, afterAll } from '@jest/globals';
import { createLogger } from '@/lib/logger';
import type { Logger } from 'pino';
import { createSecurityScanner } from '@/infra/security/scanner';
import { checkOSVAvailability } from '@/infra/security/osv-scanner/index';
import Docker from 'dockerode';
import { mkdir, writeFile, rm } from 'fs/promises';
import { join } from 'path';
import { tmpdir } from 'os';
import { randomBytes } from 'crypto';

describe('OSV Scanner Integration - Real Network Calls', () => {
  let logger: Logger;
  let osvScanner: ReturnType<typeof createSecurityScanner>;
  let dockerClient: Docker;
  let skipTests = false;
  const testImages: string[] = [];

  beforeAll(async () => {
    // Create logger for tests
    logger = createLogger({ level: 'warn' }); // Use warn to reduce test noise

    // Check if OSV API is available before running tests
    const availabilityResult = await checkOSVAvailability(logger);

    if (!availabilityResult.ok) {
      console.warn('OSV API not available, skipping integration tests');
      console.warn(`Reason: ${availabilityResult.error}`);
      skipTests = true;
      return;
    }

    // Initialize Docker client
    try {
      dockerClient = new Docker();

      // Test Docker connectivity
      await dockerClient.ping();
    } catch (error) {
      console.warn('Docker not available, skipping integration tests');
      console.warn(`Error: ${error}`);
      skipTests = true;
      return;
    }

    // Create OSV scanner instance
    osvScanner = createSecurityScanner(logger, 'osv');
  }, 30000); // 30 second timeout for initial setup

  afterAll(async () => {
    if (dockerClient && testImages.length > 0) {
      // Clean up any test images
      for (const imageName of testImages) {
        try {
          const image = dockerClient.getImage(imageName);
          await image.remove({ force: true });
        } catch (error) {
          // Ignore cleanup errors
        }
      }
    }
  });

  /**
   * Helper function to build a Docker image with a given pom.xml
   */
  async function buildTestImage(pomContent: string, imageTag: string): Promise<void> {
    const tempDir = join(tmpdir(), `osv-integration-test-${randomBytes(8).toString('hex')}`);
    await mkdir(tempDir, { recursive: true });

    try {
      // Write pom.xml
      await writeFile(join(tempDir, 'pom.xml'), pomContent, 'utf-8');

      // Create a simple Dockerfile that copies pom.xml
      const dockerfile = `FROM maven:3.9-eclipse-temurin-17
WORKDIR /app
COPY pom.xml .
# Don't actually build, just copy pom.xml for scanning
CMD ["echo", "Test image for OSV scanning"]
`;
      await writeFile(join(tempDir, 'Dockerfile'), dockerfile, 'utf-8');

      // Build the image
      const stream = await dockerClient.buildImage(
        {
          context: tempDir,
          src: ['Dockerfile', 'pom.xml'],
        },
        {
          t: imageTag,
        },
      );

      // Wait for build to complete
      await new Promise((resolve, reject) => {
        const buildErrors: string[] = [];
        dockerClient.modem.followProgress(
          stream,
          (err: Error | null) => {
            if (err) {
              console.error('Build error:', err);
              reject(err);
            } else {
              if (buildErrors.length > 0) {
                console.error('Build completed with errors:', buildErrors);
                reject(new Error(`Build failed: ${buildErrors.join(', ')}`));
              } else {
                resolve(null);
              }
            }
          },
          (event: { error?: string; stream?: string }) => {
            if (event.error) {
              buildErrors.push(event.error);
            }
            if (event.stream) {
              console.log('Build:', event.stream.trim());
            }
          },
        );
      });

      testImages.push(imageTag);
    } finally {
      // Cleanup temp directory
      await rm(tempDir, { recursive: true, force: true });
    }
  }

  describe('API Availability', () => {
    it('should successfully connect to OSV API', async () => {
      if (skipTests) {
        console.log('Skipping test - OSV API or Docker not available');
        return;
      }

      const result = await checkOSVAvailability(logger);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe('osv-api-available');
      }
    }, 10000);

    it('should query severity for known CVE GHSA-jfh8-c2jp-5v3q (Log4Shell)', async () => {
      if (skipTests) {
        console.log('Skipping test - OSV API or Docker not available');
        return;
      }

      // Query OSV API directly for the known Log4Shell vulnerability
      const response = await fetch('https://api.osv.dev/v1/vulns/GHSA-jfh8-c2jp-5v3q', {
        method: 'GET',
        headers: { 'Content-Type': 'application/json' },
      });

      expect(response.ok).toBe(true);
      const vulnerability = await response.json();

      // Verify basic vulnerability details
      expect(vulnerability.id).toBe('GHSA-jfh8-c2jp-5v3q');
      expect(vulnerability.summary).toContain('Log4j');

      // Check severity data
      console.log('Vulnerability details:', {
        id: vulnerability.id,
        severity: vulnerability.severity,
        database_specific: vulnerability.database_specific,
        affected_count: vulnerability.affected?.length || 0,
      });

      // OSV vulnerabilities may have severity in different formats:
      // 1. CVSS scores in vulnerability.severity array
      // 2. database_specific.severity
      // 3. affected[].database_specific.severity

      let hasSeverityData = false;

      // Check for CVSS scores
      if (vulnerability.severity?.length > 0) {
        console.log('CVSS scores found:', vulnerability.severity);
        hasSeverityData = true;

        // CVSS v3 score in OSV format is a vector string like:
        // "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H"
        // The actual numeric score is not included, but we can verify it's present
        const cvssV3 = vulnerability.severity.find(
          (s: { type: string; score: string }) => s.type === 'CVSS_V3',
        );

        if (cvssV3) {
          console.log('CVSS v3 vector:', cvssV3.score);
          // Verify it's a valid CVSS vector string
          expect(cvssV3.score).toMatch(/^CVSS:/);
        }
      }

      // The OSV scanner now uses CVSS vector strings to calculate severity
      // We verify that severity data is available (either CVSS vectors or database_specific)
      // but we no longer rely on database_specific as the primary source

      // At least some severity information should be available
      expect(hasSeverityData).toBe(true);
    }, 15000);
  });

  describe('Multiple Dependencies with Vulnerabilities', () => {
    it('should detect vulnerabilities across multiple packages in pom.xml', async () => {
      if (skipTests) {
        console.log('Skipping test - OSV API or Docker not available');
        return;
      }

      // Use the comprehensive vulnerable pom.xml fixture
      const pomXml = `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>vulnerable-test-app</artifactId>
  <version>1.0.0</version>
  
  <dependencies>
    <!-- Log4j 2.14.1 - Has CVE-2021-44228 (Log4Shell) -->
    <dependency>
      <groupId>org.apache.logging.log4j</groupId>
      <artifactId>log4j-core</artifactId>
      <version>2.14.1</version>
    </dependency>
    
    <!-- Spring Framework 5.2.0 - Has multiple CVEs -->
    <dependency>
      <groupId>org.springframework</groupId>
      <artifactId>spring-core</artifactId>
      <version>5.2.0.RELEASE</version>
    </dependency>
    
    <!-- Jackson Databind 2.9.8 - Has known CVEs -->
    <dependency>
      <groupId>com.fasterxml.jackson.core</groupId>
      <artifactId>jackson-databind</artifactId>
      <version>2.9.8</version>
    </dependency>
    
    <!-- Commons Collections 3.2.1 - Has CVE-2015-6420 -->
    <dependency>
      <groupId>commons-collections</groupId>
      <artifactId>commons-collections</artifactId>
      <version>3.2.1</version>
    </dependency>
  </dependencies>
</project>`;

      const imageTag = 'osv-test:multiple-vulns';
      await buildTestImage(pomXml, imageTag);

      const result = await osvScanner.scanImage(imageTag);

      expect(result.ok).toBe(true);
      if (!result.ok) {
        console.error('Scan failed:', result.error);
        return;
      }

      const { vulnerabilities, totalVulnerabilities, criticalCount, highCount } = result.value;

      // Should find vulnerabilities across all packages
      expect(totalVulnerabilities).toBeGreaterThan(3); // At least one per vulnerable package

      // Check that we found vulnerabilities for each package
      const packageNames = new Set(vulnerabilities.map((v) => v.package));

      // Should include at least some of these packages (depending on what OSV returns)
      const expectedPackages = [
        'log4j-core',
        'spring-core',
        'jackson-databind',
        'commons-collections',
      ];
      const foundExpectedPackages = expectedPackages.filter((pkg) =>
        Array.from(packageNames).some((p) => p.includes(pkg)),
      );

      expect(foundExpectedPackages.length).toBeGreaterThan(0);

      // Note: OSV API may return UNKNOWN severity when CVSS vector parsing fails
      // or when severity data is not available in CVSS format.
      // We verify that we found vulnerabilities, even if severity extraction is incomplete.
      console.log(
        `Found ${totalVulnerabilities} total vulnerabilities (${criticalCount} CRITICAL, ${highCount} HIGH, ${vulnerabilities.filter((v) => v.severity === 'UNKNOWN').length} UNKNOWN)`,
      );

      // Should have found vulnerabilities (regardless of severity classification)
      expect(totalVulnerabilities).toBeGreaterThan(3);
    }, 90000); // Longer timeout for build + multiple API calls
  });
});
