// OSV Scanner using Google's OSV API (https://osv.dev)
// Supports Maven packages with rate-limited batch queries

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

    const isAvailable = response.ok || (response.status >= 400 && response.status < 500);
    if (isAvailable) return Success('osv-api-available');

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

    logger.info(
      {
        packageCount: packages.length,
        ecosystems: [...new Set(packages.map((p) => p.ecosystem))],
        samplePackages: packages
          .slice(0, 10)
          .map((p) => ({ name: p.name, version: p.version, ecosystem: p.ecosystem })),
      },
      'Extracted packages for OSV scan',
    );

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
    logger.info({ vulnCount: vulnerabilityMap.size }, 'OSV query completed');
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
