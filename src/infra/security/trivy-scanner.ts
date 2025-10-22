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
 * Map Trivy severity to our standardized severity levels
 * Preserves UNKNOWN and NEGLIGIBLE as separate levels to maintain fidelity
 * between uncertain severity (UNKNOWN) and low-risk findings (NEGLIGIBLE)
 */
function mapTrivySeverity(
  trivySeverity: string,
): 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL' | 'NEGLIGIBLE' | 'UNKNOWN' {
  const severity = trivySeverity.toUpperCase();
  switch (severity) {
    case 'CRITICAL':
      return 'CRITICAL';
    case 'HIGH':
      return 'HIGH';
    case 'MEDIUM':
      return 'MEDIUM';
    case 'LOW':
      return 'LOW';
    case 'NEGLIGIBLE':
      return 'NEGLIGIBLE';
    case 'UNKNOWN':
      return 'UNKNOWN';
    default:
      return 'UNKNOWN';
  }
}

/**
 * Parse Trivy JSON output to our BasicScanResult format
 */
function parseTrivyOutput(trivyOutput: TrivyOutput, imageId: string): BasicScanResult {
  const vulnerabilities: BasicScanResult['vulnerabilities'] = [];
  let criticalCount = 0;
  let highCount = 0;
  let mediumCount = 0;
  let lowCount = 0;
  let negligibleCount = 0;
  let unknownCount = 0;

  // Iterate through all results and their vulnerabilities
  for (const result of trivyOutput.Results || []) {
    for (const vuln of result.Vulnerabilities || []) {
      const severity = mapTrivySeverity(vuln.Severity);

      // Count by severity
      switch (severity) {
        case 'CRITICAL':
          criticalCount++;
          break;
        case 'HIGH':
          highCount++;
          break;
        case 'MEDIUM':
          mediumCount++;
          break;
        case 'LOW':
          lowCount++;
          break;
        case 'NEGLIGIBLE':
          negligibleCount++;
          break;
        case 'UNKNOWN':
          unknownCount++;
          break;
      }

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
    totalVulnerabilities: vulnerabilities.length,
    criticalCount,
    highCount,
    mediumCount,
    lowCount,
    negligibleCount,
    unknownCount,
    scanDate: new Date(),
  };
}

/**
 * Validate imageId against allowlist pattern to prevent command injection
 * Allows: alphanumeric, dots, colons, slashes, at-signs, underscores, and hyphens
 */
function validateImageId(imageId: string): boolean {
  const allowedPattern = /^[a-zA-Z0-9._:/@-]+$/;
  return allowedPattern.test(imageId);
}

/**
 * Get Trivy version
 * @throws Error if Trivy is not installed or execution fails
 */
async function getTrivyVersion(logger: Logger): Promise<string | undefined> {
  try {
    const { stdout } = await execAsync('trivy --version', { timeout: DEFAULT_TIMEOUTS.trivyVersionCheck });
    // Trivy version output format: "Version: X.Y.Z"
    const match = stdout.match(/Version:\s*([^\s\n]+)/);
    if (!match) {
      logger.debug({ stdout }, 'Could not parse Trivy version from output');
    }
    return match ? match[1] : undefined;
  } catch (error: unknown) {
    const err = error as NodeJS.ErrnoException;
    if (err?.code === 'ETIMEDOUT') {
      logger.error(
        { error },
        'Trivy version check timed out. The trivy process may be unresponsive or misconfigured.',
      );
      return undefined;
    }
    throw error;
  }
}

/**
 * Check if Trivy is installed and accessible
 */
export async function checkTrivyAvailability(logger: Logger): Promise<Result<string>> {
  try {
    const version = await getTrivyVersion(logger);
    if (!version) {
      return Failure('Trivy is installed but version could not be determined', {
        message: 'Trivy version check failed',
        hint: 'Trivy CLI may not be properly configured',
        resolution: 'Try running: trivy --version',
      });
    }
    return Success(version);
  } catch (error) {
    return Failure('Trivy not installed or not in PATH', {
      message: 'Trivy CLI not found',
      hint: 'Trivy CLI is required for security scanning',
      resolution:
        'Install Trivy: https://aquasecurity.github.io/trivy/latest/getting-started/installation/',
      details: { error: extractErrorMessage(error) },
    });
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
    return Failure('Invalid imageId format', {
      message: 'ImageId contains invalid characters',
      hint: 'ImageId must contain only alphanumeric characters, dots, colons, slashes, at-signs, underscores, and hyphens',
      resolution: 'Verify the imageId is a valid Docker image identifier',
      details: { imageId },
    });
  }

  // Check if Trivy is available
  const availabilityCheck = await checkTrivyAvailability(logger);
  if (!availabilityCheck.ok) {
    return Failure(availabilityCheck.error, availabilityCheck.guidance);
  }

  const trivyVersion = availabilityCheck.value;
  logger.info({ trivyVersion, imageId }, 'Starting Trivy scan');

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

    // Parse JSON output
    let trivyOutput: TrivyOutput;
    try {
      trivyOutput = JSON.parse(stdout);
    } catch (parseError) {
      return Failure('Failed to parse Trivy output', {
        message: 'Trivy output parsing failed',
        hint: 'Trivy may have returned invalid JSON',
        resolution: `Try running Trivy manually to verify: trivy image ${imageId}`,
        details: {
          parseError: extractErrorMessage(parseError),
          outputPreview: stdout.substring(0, 200),
        },
      });
    }

    // Parse the Trivy output into our format
    const scanResult = parseTrivyOutput(trivyOutput, imageId);

    logger.info(
      {
        imageId,
        totalVulnerabilities: scanResult.totalVulnerabilities,
        criticalCount: scanResult.criticalCount,
        highCount: scanResult.highCount,
      },
      'Trivy scan completed successfully',
    );

    return Success(scanResult);
  } catch (error) {
    const errorMessage = extractErrorMessage(error);
    logger.error({ error: errorMessage, imageId }, 'Trivy scan failed');

    return Failure(`Trivy scan failed: ${errorMessage}`, {
      message: 'Security scan execution failed',
      hint: 'Trivy encountered an error while scanning the image',
      resolution: `Check image exists and is accessible: docker image ls | grep ${imageId}`,
      details: { error: errorMessage },
    });
  }
}
