/**
 * OSV Scanner - vulnerability scanning via Google's OSV API
 *
 * Maven-only scope. Rate-limited batch queries (10 req/sec, max 1000/batch).
 * @see https://osv.dev
 */

import type { Logger } from 'pino';
import type Dockerode from 'dockerode';

import { extractErrorMessage } from '@/lib/errors';
import { Result, Success, Failure } from '@/types';
import type { BasicScanResult } from '../scanner';
import {
  validateImageId,
  SeverityCounter,
  normalizeSeverity,
  logScanStart,
  logScanComplete,
  ScannerErrors,
} from '../scanner-common';

import { extractPackagesFromImage } from './package-extractor';
import { osvRateLimiter, queryOSVBatch, extractSeverity, extractFixedVersion } from './osv-api';

/**
 * Check if OSV API is available
 */
export async function checkOSVAvailability(logger: Logger): Promise<Result<string>> {
  try {
    await osvRateLimiter.acquire();

    const response = await fetch('https://api.osv.dev/v1/query', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        package: { name: 'test-availability-check', ecosystem: 'npm' },
        version: '1.0.0',
      }),
    });

    logger.debug({ status: response.status }, 'OSV API availability check');

    // Accept 2xx (success) and 4xx (client errors like 404 = package not found)
    // 4xx means API is up and responding, just no data for our test package
    if (response.ok || (response.status >= 400 && response.status < 500)) {
      return Success('osv-api-available');
    }

    // 5xx = server errors (API is down or malfunctioning)
    return Failure(`OSV API returned server error: ${response.status}`, {
      message: `OSV API is experiencing issues (HTTP ${response.status})`,
      hint: 'The OSV API server is not responding correctly',
      resolution: 'Wait a few moments and try again, or use an alternative scanner',
      details: { status: response.status, statusText: response.statusText },
    });
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
  if (!validateImageId(imageId)) {
    return ScannerErrors.invalidImageId(imageId);
  }

  const availabilityCheck = await checkOSVAvailability(logger);
  if (!availabilityCheck.ok) {
    return Failure(availabilityCheck.error, availabilityCheck.guidance);
  }

  logScanStart(logger, 'OSV', 'api', imageId);

  try {
    const packages = await extractPackagesFromImage(docker, imageId, logger);

    if (packages.length === 0) {
      logger.warn({ imageId }, 'No packages found - returning empty scan result');
      return Success({
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
      });
    }

    const vulnerabilityMap = await queryOSVBatch(packages, logger);
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
        if (fixedVersion) {
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
