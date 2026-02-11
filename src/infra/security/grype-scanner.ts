/**
 * Grype Security Scanner Implementation
 *
 * Integrates with Grype CLI for container image vulnerability scanning.
 * Grype is an open-source vulnerability scanner maintained by Anchore.
 *
 * @see https://github.com/anchore/grype
 */

import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import type { Logger } from 'pino';

import { extractErrorMessage } from '@/lib/errors';
import { Result, Success, Failure } from '@/types';
import type { BasicScanResult } from './scanner';
import { LIMITS } from '@/config/constants';
import {
  validateImageId,
  SeverityCounter,
  normalizeSeverity,
  parseVersion,
  logScanStart,
  logScanComplete,
  ScannerErrors,
} from './scanner-common';

const execFileAsync = promisify(execFile);

// Grype JSON output structures
interface GrypeVulnerability {
  id: string;
  dataSource?: string;
  namespace?: string;
  severity: string; // TitleCase: Critical, High, Medium, Low, Negligible
  urls?: string[];
  description?: string;
  cvss?: Array<{
    version: string;
    vector: string;
    metrics: {
      baseScore: number;
      exploitabilityScore?: number;
      impactScore?: number;
    };
  }>;
  fix?: {
    versions: string[];
    state: string; // "fixed", "not-fixed", "wont-fix", "unknown"
  };
}

interface GrypeArtifact {
  id?: string;
  name: string;
  version: string;
  type?: string;
  locations?: Array<{
    path: string;
    layerID?: string;
  }>;
  language?: string;
  licenses?: string[];
  cpes?: string[];
  purl?: string;
}

interface GrypeMatch {
  vulnerability: GrypeVulnerability;
  relatedVulnerabilities?: Array<{
    id: string;
    dataSource?: string;
    namespace?: string;
  }>;
  matchDetails?: Array<{
    type: string;
    matcher: string;
  }>;
  artifact: GrypeArtifact;
}

interface GrypeOutput {
  matches?: GrypeMatch[];
  source?: {
    type: string;
    target: {
      userInput: string;
      imageID?: string;
      manifestDigest?: string;
      tags?: string[];
    };
  };
  distro?: {
    name: string;
    version: string;
  };
  descriptor?: {
    name: string;
    version: string;
  };
}

/**
 * Parse Grype JSON output to our BasicScanResult format
 */
function parseGrypeOutput(grypeOutput: GrypeOutput, imageId: string): BasicScanResult {
  const vulnerabilities: BasicScanResult['vulnerabilities'] = [];
  const counter = new SeverityCounter();

  // Grype uses matches array with nested vulnerability objects
  for (const match of grypeOutput.matches || []) {
    const vuln = match.vulnerability;
    const artifact = match.artifact;

    const severity = normalizeSeverity(vuln.severity);
    counter.increment(severity);

    // Build vulnerability entry
    const vulnEntry: BasicScanResult['vulnerabilities'][number] = {
      id: vuln.id,
      severity,
      package: artifact.name,
      version: artifact.version,
      description: vuln.description || 'No description available',
    };

    // Add fixedVersion if available (from fix.versions array)
    if (vuln.fix?.versions && vuln.fix.versions.length > 0 && vuln.fix.state === 'fixed') {
      const fixedVersion = vuln.fix.versions[0];
      if (fixedVersion) {
        vulnEntry.fixedVersion = fixedVersion;
      }
    }

    vulnerabilities.push(vulnEntry);
  }

  return {
    imageId,
    vulnerabilities,
    scanDate: new Date(),
    ...counter.getCounts(),
  };
}

/**
 * Get Grype version
 * @throws Error if Grype is not installed or execution fails
 */
async function getGrypeVersion(logger: Logger): Promise<Result<string>> {
  try {
    const { stdout } = await execFileAsync('grype', ['version'], { timeout: 5000 });
    // Grype version output format: multi-line with "Version: X.Y.Z"
    const version = parseVersion(stdout, /Version:\s*([^\s\n]+)/);
    if (!version) {
      logger.debug({ stdout }, 'Could not parse Grype version from output');
      return ScannerErrors.versionParseError('Grype', 'grype version');
    }
    return Success(version);
  } catch (error: unknown) {
    const err = error as NodeJS.ErrnoException;
    if (err?.code === 'ETIMEDOUT') {
      logger.error({ error }, 'Grype version check timed out');
      return ScannerErrors.versionCheckTimeout('Grype', 'grype version');
    }
    throw error;
  }
}

/**
 * Check if Grype is installed and accessible
 */
export async function checkGrypeAvailability(logger: Logger): Promise<Result<string>> {
  try {
    const versionResult = await getGrypeVersion(logger);
    if (!versionResult.ok) {
      return versionResult;
    }
    return Success(versionResult.value);
  } catch {
    return ScannerErrors.scannerNotInstalled(
      'Grype',
      'brew install grype or https://github.com/anchore/grype#installation',
    );
  }
}

/**
 * Scan a Docker image using Grype
 */
export async function scanImageWithGrype(
  imageId: string,
  logger: Logger,
): Promise<Result<BasicScanResult>> {
  // Validate imageId to prevent command injection
  if (!validateImageId(imageId)) {
    return ScannerErrors.invalidImageId(imageId);
  }

  // Check if Grype is available
  const availabilityCheck = await checkGrypeAvailability(logger);
  if (!availabilityCheck.ok) {
    return Failure(availabilityCheck.error, availabilityCheck.guidance);
  }

  const grypeVersion = availabilityCheck.value;
  logScanStart(logger, 'Grype', grypeVersion, imageId);

  try {
    // Run Grype scan with JSON output
    // -o json: output in JSON format
    // -q: quiet mode (suppress non-essential output)
    const args = [imageId, '-o', 'json', '-q'];
    logger.debug({ args }, 'Executing Grype command');

    const { stdout, stderr } = await execFileAsync('grype', args, {
      maxBuffer: LIMITS.MAX_SCAN_BUFFER,
    });

    // Log any warnings from stderr
    if (stderr) {
      logger.debug({ stderr }, 'Grype stderr output');
    }

    // Validate output size before parsing
    if (stdout.length === 0) {
      return ScannerErrors.emptyOutput('Grype', imageId);
    }

    // Parse JSON output
    let grypeOutput: GrypeOutput;
    try {
      grypeOutput = JSON.parse(stdout);
    } catch (parseError) {
      return ScannerErrors.jsonParseError(
        'Grype',
        extractErrorMessage(parseError),
        stdout.substring(0, 200),
      );
    }

    // Parse the Grype output into our format
    const scanResult = parseGrypeOutput(grypeOutput, imageId);

    logScanComplete(
      logger,
      'Grype',
      imageId,
      scanResult.totalVulnerabilities,
      scanResult.criticalCount,
      scanResult.highCount,
    );

    return Success(scanResult);
  } catch (error) {
    const errorMessage = extractErrorMessage(error);
    logger.error({ error: errorMessage, imageId }, 'Grype scan failed');
    return ScannerErrors.scanExecutionError('Grype', imageId, errorMessage);
  }
}
