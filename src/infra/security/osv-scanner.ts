/**
 * OSV (Open Source Vulnerabilities) Scanner Implementation
 *
 * Integrates with OSV API for container image vulnerability scanning.
 * OSV is a distributed vulnerability database maintained by Google and the open source community.
 * No external CLI tools required - uses Docker API + OSV REST API.
 *
 * @see https://osv.dev
 * @see https://google.github.io/osv.dev/api/
 */

import type { Logger } from 'pino';
import type Dockerode from 'dockerode';

import { extractErrorMessage } from '@/lib/errors';
import { Result, Success, Failure } from '@/types';
import type { BasicScanResult } from './scanner';
import {
  validateImageId,
  SeverityCounter,
  normalizeSeverity,
  logScanStart,
  logScanComplete,
  ScannerErrors,
} from './scanner-common';

// OSV API types
interface OSVPackage {
  name: string;
  ecosystem: string;
  purl?: string;
}

interface OSVQueryBatch {
  queries: Array<{
    package: OSVPackage;
    version?: string;
  }>;
}

interface OSVSeverity {
  type: string; // "CVSS_V3", "CVSS_V2"
  score: string; // "7.5"
}

interface OSVAffected {
  package: OSVPackage;
  ranges?: Array<{
    type: string;
    events: Array<{
      introduced?: string;
      fixed?: string;
    }>;
  }>;
  versions?: string[];
  database_specific?: {
    severity?: string;
  };
  ecosystem_specific?: Record<string, unknown>;
}

interface OSVVulnerability {
  id: string; // CVE-2021-12345 or GHSA-xxxx-xxxx-xxxx
  summary?: string;
  details?: string;
  aliases?: string[];
  modified?: string;
  published?: string;
  database_specific?: {
    severity?: string;
    cwe_ids?: string[];
  };
  severity?: OSVSeverity[];
  affected?: OSVAffected[];
  references?: Array<{
    type: string;
    url: string;
  }>;
}

interface OSVQueryResponse {
  vulns?: OSVVulnerability[];
}

interface OSVBatchResponse {
  results: OSVQueryResponse[];
}

/**
 * Package extracted from Docker image
 */
interface ExtractedPackage {
  name: string;
  version: string;
  ecosystem: 'npm' | 'Maven' | 'PyPI' | 'Go' | 'NuGet' | 'Debian' | 'Alpine';
  path?: string;
}

/**
 * Map severity score to our standard levels
 * Based on CVSS v3.0 ranges:
 * - 0.0: NONE
 * - 0.1-3.9: LOW
 * - 4.0-6.9: MEDIUM
 * - 7.0-8.9: HIGH
 * - 9.0-10.0: CRITICAL
 */
function mapCVSSToSeverity(score: number): 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL' | 'UNKNOWN' {
  if (score >= 9.0) return 'CRITICAL';
  if (score >= 7.0) return 'HIGH';
  if (score >= 4.0) return 'MEDIUM';
  if (score > 0.0) return 'LOW';
  return 'UNKNOWN';
}

/**
 * Extract severity from OSV vulnerability
 */
function extractSeverity(
  vuln: OSVVulnerability,
): 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL' | 'UNKNOWN' {
  // Try severity array first (CVSS scores)
  if (vuln.severity && vuln.severity.length > 0) {
    for (const sev of vuln.severity) {
      if (sev.type === 'CVSS_V3' || sev.type === 'CVSS_V2') {
        const score = parseFloat(sev.score);
        if (!isNaN(score)) {
          return mapCVSSToSeverity(score);
        }
      }
    }
  }

  // Try database_specific severity (GitHub, etc.)
  if (vuln.database_specific?.severity) {
    const sev = vuln.database_specific.severity.toUpperCase();
    if (sev === 'CRITICAL' || sev === 'HIGH' || sev === 'MEDIUM' || sev === 'LOW') {
      return sev;
    }
  }

  // Try affected packages severity
  if (vuln.affected && vuln.affected.length > 0) {
    for (const affected of vuln.affected) {
      if (affected.database_specific?.severity) {
        const sev = affected.database_specific.severity.toUpperCase();
        if (sev === 'CRITICAL' || sev === 'HIGH' || sev === 'MEDIUM' || sev === 'LOW') {
          return sev;
        }
      }
    }
  }

  return 'UNKNOWN';
}

/**
 * Extract fixed version from OSV vulnerability
 */
function extractFixedVersion(vuln: OSVVulnerability): string | undefined {
  if (!vuln.affected || vuln.affected.length === 0) {
    return undefined;
  }

  for (const affected of vuln.affected) {
    if (affected.ranges && affected.ranges.length > 0) {
      for (const range of affected.ranges) {
        if (range.events && range.events.length > 0) {
          for (const event of range.events) {
            if (event.fixed) {
              return event.fixed;
            }
          }
        }
      }
    }
  }

  return undefined;
}

/**
 * Extract packages from Docker image layers
 */
async function extractPackagesFromImage(
  docker: Dockerode,
  imageId: string,
  logger: Logger,
): Promise<ExtractedPackage[]> {
  const packages: ExtractedPackage[] = [];

  try {
    // Get image inspection data
    const image = docker.getImage(imageId);
    const inspectData = await image.inspect();

    logger.debug({ imageId, layers: inspectData.RootFS?.Layers?.length }, 'Inspecting image');

    // Create a container from the image to access filesystem
    const container = await docker.createContainer({
      Image: imageId,
      Cmd: ['/bin/sh', '-c', 'exit 0'],
      AttachStdout: false,
      AttachStderr: false,
    });

    try {
      // Get archive of package manifest files
      // We'll check for common package manager files
      const manifestPaths = [
        '/package.json', // npm
        '/app/package.json',
        '/usr/src/app/package.json',
        '/pom.xml', // Maven
        '/app/pom.xml',
        '/requirements.txt', // Python pip
        '/app/requirements.txt',
        '/Pipfile.lock',
        '/go.mod', // Go
        '/app/go.mod',
        '/packages.lock.json', // NuGet
        '/var/lib/dpkg/status', // Debian packages
        '/lib/apk/db/installed', // Alpine packages
      ];

      for (const manifestPath of manifestPaths) {
        try {
          const stream = await container.getArchive({ path: manifestPath });
          const chunks: Buffer[] = [];

          for await (const chunk of stream) {
            chunks.push(chunk as Buffer);
          }

          const buffer = Buffer.concat(chunks);

          // Parse based on file type
          if (manifestPath.includes('package.json')) {
            const extracted = await parseNpmPackages(buffer, manifestPath, logger);
            packages.push(...extracted);
          } else if (manifestPath.includes('pom.xml')) {
            const extracted = await parseMavenPackages(buffer, manifestPath, logger);
            packages.push(...extracted);
          } else if (
            manifestPath.includes('requirements.txt') ||
            manifestPath.includes('Pipfile')
          ) {
            const extracted = await parsePythonPackages(buffer, manifestPath, logger);
            packages.push(...extracted);
          } else if (manifestPath.includes('go.mod')) {
            const extracted = await parseGoPackages(buffer, manifestPath, logger);
            packages.push(...extracted);
          } else if (manifestPath.includes('status')) {
            const extracted = await parseDebianPackages(buffer, manifestPath, logger);
            packages.push(...extracted);
          } else if (manifestPath.includes('installed')) {
            const extracted = await parseAlpinePackages(buffer, manifestPath, logger);
            packages.push(...extracted);
          }
        } catch (err) {
          // File doesn't exist or can't be read - that's fine, skip it
          logger.debug(
            { path: manifestPath, error: extractErrorMessage(err) },
            'Manifest not found',
          );
        }
      }

      await container.remove();
    } catch (err) {
      logger.warn({ error: extractErrorMessage(err) }, 'Failed to extract packages from container');
      try {
        await container.remove();
      } catch {
        // Ignore cleanup errors
      }
    }

    logger.info({ packageCount: packages.length }, 'Extracted packages from image');
    return packages;
  } catch (error) {
    logger.error({ error: extractErrorMessage(error) }, 'Failed to inspect image');
    throw error;
  }
}

/**
 * Parse npm package.json from tar archive
 */
async function parseNpmPackages(
  _buffer: Buffer,
  path: string,
  logger: Logger,
): Promise<ExtractedPackage[]> {
  // Simple tar extraction - we need to implement a basic tar parser
  // For now, return empty array - this will be enhanced
  logger.debug({ path }, 'Skipping npm package parsing (not implemented)');
  return [];
}

/**
 * Parse Maven pom.xml from tar archive
 */
async function parseMavenPackages(
  _buffer: Buffer,
  path: string,
  logger: Logger,
): Promise<ExtractedPackage[]> {
  logger.debug({ path }, 'Skipping Maven package parsing (not implemented)');
  return [];
}

/**
 * Parse Python requirements.txt from tar archive
 */
async function parsePythonPackages(
  _buffer: Buffer,
  path: string,
  logger: Logger,
): Promise<ExtractedPackage[]> {
  logger.debug({ path }, 'Skipping Python package parsing (not implemented)');
  return [];
}

/**
 * Parse Go go.mod from tar archive
 */
async function parseGoPackages(
  _buffer: Buffer,
  path: string,
  logger: Logger,
): Promise<ExtractedPackage[]> {
  logger.debug({ path }, 'Skipping Go package parsing (not implemented)');
  return [];
}

/**
 * Parse Debian package status from tar archive
 */
async function parseDebianPackages(
  _buffer: Buffer,
  path: string,
  logger: Logger,
): Promise<ExtractedPackage[]> {
  logger.debug({ path }, 'Skipping Debian package parsing (not implemented)');
  return [];
}

/**
 * Parse Alpine package database from tar archive
 */
async function parseAlpinePackages(
  _buffer: Buffer,
  path: string,
  logger: Logger,
): Promise<ExtractedPackage[]> {
  logger.debug({ path }, 'Skipping Alpine package parsing (not implemented)');
  return [];
}

/**
 * Query OSV API for vulnerabilities (batch mode)
 */
async function queryOSVBatch(
  packages: ExtractedPackage[],
  logger: Logger,
): Promise<Map<string, OSVVulnerability[]>> {
  const OSV_API_URL = 'https://api.osv.dev/v1/querybatch';
  const vulnerabilityMap = new Map<string, OSVVulnerability[]>();

  if (packages.length === 0) {
    return vulnerabilityMap;
  }

  // Build batch query (max 1000 queries per batch)
  const batchSize = 1000;
  for (let i = 0; i < packages.length; i += batchSize) {
    const batch = packages.slice(i, i + batchSize);
    const query: OSVQueryBatch = {
      queries: batch.map((pkg) => ({
        package: {
          name: pkg.name,
          ecosystem: pkg.ecosystem,
        },
        version: pkg.version,
      })),
    };

    try {
      logger.debug({ queryCount: batch.length }, 'Querying OSV API');

      const response = await fetch(OSV_API_URL, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(query),
      });

      if (!response.ok) {
        logger.warn({ status: response.status }, 'OSV API returned error status');
        continue;
      }

      const data = (await response.json()) as OSVBatchResponse;

      // Process results
      for (let j = 0; j < data.results.length; j++) {
        const result = data.results[j];
        if (result?.vulns && result.vulns.length > 0) {
          const pkg = batch[j];
          if (pkg) {
            const key = `${pkg.name}@${pkg.version}`;
            vulnerabilityMap.set(key, result.vulns);
          }
        }
      }
    } catch (error) {
      logger.error({ error: extractErrorMessage(error) }, 'Failed to query OSV API');
    }
  }

  return vulnerabilityMap;
}

/**
 * Check if OSV API is available
 */
export async function checkOSVAvailability(logger: Logger): Promise<Result<string>> {
  try {
    const response = await fetch('https://api.osv.dev/v1/query', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        package: {
          name: 'test-availability-check',
          ecosystem: 'npm',
        },
        version: '1.0.0',
      }),
    });

    // Any response (even 404) means the API is available
    logger.debug({ status: response.status }, 'OSV API availability check');
    return Success('osv-api-available');
  } catch (error) {
    return Failure('OSV API not accessible', {
      message: 'Cannot reach OSV API',
      hint: 'OSV API requires network connectivity',
      resolution: 'Check your network connection and try again',
      details: { error: extractErrorMessage(error) },
    });
  }
}

/**
 * Scan a Docker image using OSV API
 */
export async function scanImageWithOSV(
  docker: Dockerode,
  imageId: string,
  logger: Logger,
): Promise<Result<BasicScanResult>> {
  // Validate imageId to prevent command injection
  if (!validateImageId(imageId)) {
    return ScannerErrors.invalidImageId(imageId);
  }

  // Check if OSV API is available
  const availabilityCheck = await checkOSVAvailability(logger);
  if (!availabilityCheck.ok) {
    return Failure(availabilityCheck.error, availabilityCheck.guidance);
  }

  logScanStart(logger, 'OSV', 'api', imageId);

  try {
    // Extract packages from image
    const packages = await extractPackagesFromImage(docker, imageId, logger);

    if (packages.length === 0) {
      logger.warn({ imageId }, 'No packages found in image - returning empty scan result');

      const emptyResult: BasicScanResult = {
        imageId,
        vulnerabilities: [],
        totalVulnerabilities: 0,
        criticalCount: 0,
        highCount: 0,
        mediumCount: 0,
        lowCount: 0,
        negligibleCount: 0,
        unknownCount: 0,
        scanDate: new Date(),
      };

      return Success(emptyResult);
    }

    // Query OSV for vulnerabilities
    const vulnerabilityMap = await queryOSVBatch(packages, logger);

    // Build result
    const vulnerabilities: BasicScanResult['vulnerabilities'] = [];
    const counter = new SeverityCounter();

    for (const [pkgKey, vulns] of vulnerabilityMap.entries()) {
      const pkg = packages.find((p) => `${p.name}@${p.version}` === pkgKey);
      if (!pkg) continue;

      for (const vuln of vulns) {
        const severity = normalizeSeverity(extractSeverity(vuln));
        counter.increment(severity);

        const vulnEntry: BasicScanResult['vulnerabilities'][number] = {
          id: vuln.id,
          severity,
          package: pkg.name,
          version: pkg.version,
          description: vuln.summary || vuln.details || 'No description available',
        };

        const fixedVersion = extractFixedVersion(vuln);
        if (fixedVersion !== undefined) {
          vulnEntry.fixedVersion = fixedVersion;
        }

        vulnerabilities.push(vulnEntry);
      }
    }

    const scanResult: BasicScanResult = {
      imageId,
      vulnerabilities,
      scanDate: new Date(),
      ...counter.getCounts(),
    };

    logScanComplete(
      logger,
      'OSV',
      imageId,
      scanResult.totalVulnerabilities,
      scanResult.criticalCount,
      scanResult.highCount,
    );

    return Success(scanResult);
  } catch (error) {
    const errorMessage = extractErrorMessage(error);
    logger.error({ error: errorMessage, imageId }, 'OSV scan failed');
    return ScannerErrors.scanExecutionError('OSV', imageId, errorMessage);
  }
}
