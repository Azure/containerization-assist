/**
 * Trivy Security Scanner Implementation
 *
 * Integrates with Trivy CLI for container image vulnerability scanning.
 * Trivy is an industry-standard security scanner maintained by Aqua Security.
 *
 * @see https://aquasecurity.github.io/trivy/
 */

import { exec, execFile } from 'node:child_process';
import { promisify } from 'node:util';
import type { Logger } from 'pino';

import { extractErrorMessage } from '@/lib/errors';
import { Result, Success, Failure } from '@/types';
import type { BasicScanResult } from './scanner';
import { DEFAULT_TIMEOUTS, LIMITS } from '@/config/constants';
import {
  validateImageId,
  SeverityCounter,
  normalizeSeverity,
  parseVersion,
  logScanStart,
  logScanComplete,
  ScannerErrors,
} from './scanner-common';

const execAsync = promisify(exec);
const execFileAsync = promisify(execFile);

// Trivy JSON output structures
interface TrivyVulnerability {
  VulnerabilityID: string;
  PkgName: string;
  InstalledVersion: string;
  FixedVersion?: string;
  Severity: string;
  Title?: string;
  Description?: string;
  References?: string[];
  PrimaryURL?: string;
}

interface TrivyResult {
  Target: string;
  Class: string;
  Type: string;
  Vulnerabilities?: TrivyVulnerability[];
}

interface TrivyOutput {
  SchemaVersion: number;
  ArtifactName: string;
  ArtifactType: string;
  Metadata?: {
    ImageID?: string;
    RepoTags?: string[];
    RepoDigests?: string[];
  };
  Results?: TrivyResult[];
}

/**
 * Parse Trivy JSON output to our BasicScanResult format
 */
function parseTrivyOutput(trivyOutput: TrivyOutput, imageId: string): BasicScanResult {
  const vulnerabilities: BasicScanResult['vulnerabilities'] = [];
  const counter = new SeverityCounter();

  // Iterate through all results and their vulnerabilities
  for (const result of trivyOutput.Results || []) {
    for (const vuln of result.Vulnerabilities || []) {
      const severity = normalizeSeverity(vuln.Severity);
      counter.increment(severity);

      // Build vulnerability entry
      const vulnEntry: BasicScanResult['vulnerabilities'][number] = {
        id: vuln.VulnerabilityID,
        severity,
        package: vuln.PkgName,
        version: vuln.InstalledVersion,
        description: vuln.Title || vuln.Description || 'No description available',
      };

      // Only add fixedVersion if it exists (exactOptionalPropertyTypes compliance)
      if (vuln.FixedVersion !== undefined) {
        vulnEntry.fixedVersion = vuln.FixedVersion;
      }

      vulnerabilities.push(vulnEntry);
    }
  }

  return {
    imageId,
    vulnerabilities,
    scanDate: new Date(),
    ...counter.getCounts(),
  };
}

/**
 * Get Trivy version
 * @throws Error if Trivy is not installed or execution fails
 */
async function getTrivyVersion(logger: Logger): Promise<Result<string>> {
  try {
    const { stdout } = await execAsync('trivy --version', { timeout: DEFAULT_TIMEOUTS.trivyVersionCheck });
    // Trivy version output format: "Version: X.Y.Z"
    const version = parseVersion(stdout, /Version:\s*([^\s\n]+)/);
    if (!version) {
      logger.debug({ stdout }, 'Could not parse Trivy version from output');
      return ScannerErrors.versionParseError('Trivy', 'trivy --version');
    }
    return Success(version);
  } catch (error: unknown) {
    const err = error as NodeJS.ErrnoException;
    if (err?.code === 'ETIMEDOUT') {
      logger.error({ error }, 'Trivy version check timed out');
      return ScannerErrors.versionCheckTimeout('Trivy', 'trivy --version');
    }
    throw error;
  }
}

/**
 * Check if Trivy is installed and accessible
 */
export async function checkTrivyAvailability(logger: Logger): Promise<Result<string>> {
  try {
    const versionResult = await getTrivyVersion(logger);
    if (!versionResult.ok) {
      return versionResult;
    }
    return Success(versionResult.value);
  } catch {
    return ScannerErrors.scannerNotInstalled(
      'Trivy',
      'https://aquasecurity.github.io/trivy/latest/getting-started/installation/',
    );
  }
}

/**
 * Scan a Docker image using Trivy
 */
export async function scanImageWithTrivy(
  imageId: string,
  logger: Logger,
): Promise<Result<BasicScanResult>> {
  // Validate imageId to prevent command injection
  if (!validateImageId(imageId)) {
    return ScannerErrors.invalidImageId(imageId);
  }

  // Check if Trivy is available
  const availabilityCheck = await checkTrivyAvailability(logger);
  if (!availabilityCheck.ok) {
    return Failure(availabilityCheck.error, availabilityCheck.guidance);
  }

  const trivyVersion = availabilityCheck.value;
  logScanStart(logger, 'Trivy', trivyVersion, imageId);

  try {
    // Run Trivy scan with JSON output using execFile to prevent command injection
    // --format json: output in JSON format
    // --quiet: suppress progress output
    // --timeout 5m: set timeout to 5 minutes
    const args = ['image', '--format', 'json', '--quiet', '--timeout', '5m', imageId];
    logger.debug({ args }, 'Executing Trivy command');

    const { stdout, stderr } = await execFileAsync('trivy', args, {
      maxBuffer: LIMITS.MAX_SCAN_BUFFER, // 10MB buffer for large scan results
    });

    // Log any warnings from stderr
    if (stderr) {
      logger.debug({ stderr }, 'Trivy stderr output');
    }

    // Validate output size before parsing
    if (stdout.length === 0) {
      return ScannerErrors.emptyOutput('Trivy', imageId);
    }

    // Parse JSON output
    let trivyOutput: TrivyOutput;
    try {
      trivyOutput = JSON.parse(stdout);
    } catch (parseError) {
      return ScannerErrors.jsonParseError(
        'Trivy',
        extractErrorMessage(parseError),
        stdout.substring(0, 200),
      );
    }

    // Parse the Trivy output into our format
    const scanResult = parseTrivyOutput(trivyOutput, imageId);

    logScanComplete(
      logger,
      'Trivy',
      imageId,
      scanResult.totalVulnerabilities,
      scanResult.criticalCount,
      scanResult.highCount,
    );

    return Success(scanResult);
  } catch (error) {
    const errorMessage = extractErrorMessage(error);
    logger.error({ error: errorMessage, imageId }, 'Trivy scan failed');
    return ScannerErrors.scanExecutionError('Trivy', imageId, errorMessage);
  }
}
