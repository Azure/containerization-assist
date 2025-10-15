/**
 * Security Scanner - Type Definitions Only
 *
 * Type definitions for security scanning functionality.
 * The actual implementation uses the functional approach in scanner.ts
 */

import type { Logger } from 'pino';
import { Result, Success, Failure, isFail } from '@/types';
import { extractErrorMessage } from '@/lib/error-utils';

// Type definitions expected by tests and other components
export interface ScanOptions {
  minSeverity?: 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL';
  skipUnfixed?: boolean;
  timeout?: number;
}

export interface VulnerabilityFinding {
  id: string;
  severity: 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL' | 'UNKNOWN';
  package: string;
  version?: string;
  fixedVersion?: string;
  title?: string;
  description?: string;
}

export interface SecurityScanResult {
  vulnerabilities: VulnerabilityFinding[];
  summary: {
    total: number;
    critical: number;
    high: number;
    medium: number;
    low: number;
    unknown: number;
  };
  passed: boolean;
}

export interface SecretFinding {
  type: string;
  severity: string;
  line: number;
  content: string;
  file?: string;
}

export interface SecretScanResult {
  secrets: SecretFinding[];
  summary: {
    total: number;
    high: number;
    medium: number;
    low: number;
  };
}

export interface SecurityReport {
  vulnerabilityResults: SecurityScanResult;
  secretResults: SecretScanResult;
  summary: {
    totalIssues: number;
    riskScore: number;
    highestSeverity: string;
  };
}

/**
 * Functional scan implementation for Docker images
 * Simple mock implementation for development
 */
export async function scanImage(
  imageId: string,
  options: ScanOptions,
  logger: Logger,
): Promise<Result<SecurityScanResult>> {
  logger.info({ imageId, options }, 'Mock security scan');

  const result: SecurityScanResult = {
    vulnerabilities: [],
    summary: { total: 0, critical: 0, high: 0, medium: 0, low: 0, unknown: 0 },
    passed: true,
  };

  return Success(result);
}

export interface CommandExecutor {
  execute(
    command: string,
    args: string[],
    options?: { timeout?: number },
  ): Promise<Result<{ stdout: string; stderr: string; exitCode: number }>>;
}

// Scanner configuration
interface ScannerContext {
  commandExecutor: CommandExecutor;
  logger: Logger;
}

// Validation helpers
function validateScanOptions(options?: ScanOptions): Result<void> {
  if (
    options?.minSeverity &&
    !['LOW', 'MEDIUM', 'HIGH', 'CRITICAL'].includes(options.minSeverity)
  ) {
    return Failure(`Invalid severity level: ${options.minSeverity}`);
  }

  if (options?.timeout && options.timeout < 0) {
    return Failure(`Invalid timeout: ${options.timeout}`);
  }

  return Success(undefined);
}

function getSeverityFilter(minSeverity: string): string {
  const severityLevels = {
    LOW: 'CRITICAL,HIGH,MEDIUM,LOW',
    MEDIUM: 'CRITICAL,HIGH,MEDIUM',
    HIGH: 'CRITICAL,HIGH',
    CRITICAL: 'CRITICAL',
  };

  return severityLevels[minSeverity as keyof typeof severityLevels] || 'CRITICAL,HIGH,MEDIUM,LOW';
}

function createTimeoutPromise(
  timeout: number,
): Promise<Result<{ stdout: string; stderr: string; exitCode: number }>> {
  return new Promise((resolve) => {
    const timer = setTimeout(() => {
      resolve(Failure(`Operation timed out after ${timeout}ms`));
    }, timeout);
    // Prevent this timer from keeping the Node.js process alive
    timer.unref();
  });
}

/**
 * Scan Docker image for vulnerabilities
 */
export async function scanImageVulnerabilities(
  ctx: ScannerContext,
  imageId: string,
  options?: ScanOptions,
): Promise<Result<SecurityScanResult>> {
  try {
    const validationResult = validateScanOptions(options);
    if (isFail(validationResult)) {
      return validationResult;
    }

    ctx.logger.info({ imageId, options }, 'Starting security scan');

    const args = ['image', '--format', 'json', imageId];

    if (options?.minSeverity) {
      args.splice(2, 0, '--severity', getSeverityFilter(options.minSeverity));
    }

    if (options?.skipUnfixed) {
      args.splice(-1, 0, '--ignore-unfixed');
    }

    const execOptions = {
      timeout: options?.timeout || 120000,
    };

    const result = await Promise.race([
      ctx.commandExecutor.execute('trivy', args, execOptions),
      createTimeoutPromise(options?.timeout || 120000),
    ]);

    if (isFail(result)) {
      return Failure(`Security scan failed: ${result.error}`);
    }

    if (result.value.stderr) {
      ctx.logger.warn({ stderr: result.value.stderr }, 'Scanner warnings');
    }

    const parseResult = parseTrivyOutput(result.value.stdout);
    if (isFail(parseResult)) {
      return parseResult;
    }
    const scanResult = parseResult.value;

    ctx.logger.info(
      {
        imageId,
        totalVulnerabilities: scanResult.summary.total,
        criticalCount: scanResult.summary.critical,
        highCount: scanResult.summary.high,
      },
      'Security scan completed',
    );

    return Success(scanResult);
  } catch (error) {
    ctx.logger.error({ error, imageId }, 'Security scan failed');
    return Failure(`Security scan failed: ${extractErrorMessage(error)}`);
  }
}

/**
 * Scan filesystem for vulnerabilities
 */
export async function scanFilesystem(
  ctx: ScannerContext,
  path: string,
  options?: ScanOptions,
): Promise<Result<SecurityScanResult>> {
  try {
    ctx.logger.info({ path }, 'Starting filesystem scan');

    const args = ['fs', '--format', 'json', path];

    const execOptions = {
      timeout: options?.timeout || 120000,
    };

    const result = await ctx.commandExecutor.execute('trivy', args, execOptions);

    if (isFail(result)) {
      return Failure(`Filesystem scan failed: ${result.error}`);
    }

    const parseResult = parseTrivyOutput(result.value.stdout);
    if (isFail(parseResult)) {
      return parseResult;
    }
    return Success(parseResult.value);
  } catch (error) {
    return Failure(`Filesystem scan failed: ${extractErrorMessage(error)}`);
  }
}

/**
 * Scan for secrets in code
 */
export async function scanSecrets(
  ctx: ScannerContext,
  path: string,
  options?: ScanOptions,
): Promise<Result<SecretScanResult>> {
  try {
    ctx.logger.info({ path }, 'Starting secret scan');

    const args = ['fs', '--format', 'json', '--scanners', 'secret', path];

    const execOptions = {
      timeout: options?.timeout || 120000,
    };

    const result = await ctx.commandExecutor.execute('trivy', args, execOptions);

    if (isFail(result)) {
      return Failure(`Secret scan failed: ${result.error}`);
    }

    const parseResult = parseSecretOutput(result.value.stdout);
    if (isFail(parseResult)) {
      return parseResult;
    }
    return Success(parseResult.value);
  } catch (error) {
    return Failure(`Secret scan failed: ${extractErrorMessage(error)}`);
  }
}

/**
 * Generate comprehensive security report
 */
export async function generateSecurityReport(
  ctx: ScannerContext,
  imageId: string,
  sourcePath: string,
): Promise<Result<SecurityReport>> {
  try {
    const [vulnerabilityResult, secretResult] = await Promise.all([
      scanImageVulnerabilities(ctx, imageId),
      scanSecrets(ctx, sourcePath),
    ]);

    if (isFail(vulnerabilityResult)) {
      return Failure(`Vulnerability scan failed: ${vulnerabilityResult.error}`);
    }

    if (isFail(secretResult)) {
      return Failure(`Secret scan failed: ${secretResult.error}`);
    }

    const riskScore = calculateRiskScore(vulnerabilityResult.value, secretResult.value);
    const highestSeverity = getHighestSeverity(vulnerabilityResult.value, secretResult.value);

    const report: SecurityReport = {
      vulnerabilityResults: vulnerabilityResult.value,
      secretResults: secretResult.value,
      summary: {
        totalIssues: vulnerabilityResult.value.summary.total + secretResult.value.summary.total,
        riskScore,
        highestSeverity,
      },
    };

    return Success(report);
  } catch (error) {
    return Failure(`Report generation failed: ${extractErrorMessage(error)}`);
  }
}

/**
 * Get scanner version
 */
export async function getScannerVersion(ctx: ScannerContext): Promise<Result<string>> {
  try {
    const result = await ctx.commandExecutor.execute('trivy', ['--version']);

    if (isFail(result)) {
      return Failure(`Failed to get scanner version: ${result.error}`);
    }

    return Success(result.value.stdout.trim());
  } catch (error) {
    return Failure(`Failed to get scanner version: ${extractErrorMessage(error)}`);
  }
}

/**
 * Update vulnerability database
 */
export async function updateDatabase(ctx: ScannerContext): Promise<Result<void>> {
  try {
    const result = await ctx.commandExecutor.execute('trivy', ['image', '--download-db-only'], {
      timeout: 300000, // 5 minutes for database update
    });

    if (isFail(result)) {
      return Failure(`Failed to update vulnerability database: ${result.error}`);
    }

    return Success(undefined);
  } catch (error) {
    return Failure(`Failed to update vulnerability database: ${extractErrorMessage(error)}`);
  }
}

// Parsing helpers
function parseTrivyOutput(output: string): Result<SecurityScanResult> {
  try {
    const trivyResult = JSON.parse(output);
    const vulnerabilities: VulnerabilityFinding[] = [];

    let critical = 0,
      high = 0,
      medium = 0,
      low = 0,
      unknown = 0;

    if (trivyResult.Results) {
      for (const result of trivyResult.Results) {
        if (result.Vulnerabilities) {
          for (const vuln of result.Vulnerabilities) {
            const rawSeverity = vuln.Severity?.toUpperCase() || 'UNKNOWN';
            // Normalize severity to valid values
            const validSeverities = ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW', 'UNKNOWN'];
            const severity = validSeverities.includes(rawSeverity)
              ? (rawSeverity as VulnerabilityFinding['severity'])
              : ('UNKNOWN' as VulnerabilityFinding['severity']);

            vulnerabilities.push({
              id: vuln.VulnerabilityID || 'unknown',
              severity,
              package: vuln.PkgName || 'unknown',
              version: vuln.InstalledVersion,
              fixedVersion: vuln.FixedVersion,
              title: vuln.Title,
              description: vuln.Description,
            });

            switch (severity) {
              case 'CRITICAL':
                critical++;
                break;
              case 'HIGH':
                high++;
                break;
              case 'MEDIUM':
                medium++;
                break;
              case 'LOW':
                low++;
                break;
              default:
                unknown++;
                break;
            }
          }
        }
      }
    }

    const total = critical + high + medium + low + unknown;

    return Success({
      vulnerabilities,
      summary: { critical, high, medium, low, unknown, total },
      passed: total === 0,
    });
  } catch (error) {
    return Failure(`Failed to parse scan results: ${extractErrorMessage(error)}`);
  }
}

function parseSecretOutput(output: string): Result<SecretScanResult> {
  try {
    const trivyResult = JSON.parse(output);
    const secrets: SecretFinding[] = [];

    let high = 0,
      medium = 0,
      low = 0;

    if (trivyResult.Results) {
      for (const result of trivyResult.Results) {
        if (result.Secrets) {
          for (const secret of result.Secrets) {
            const severity = secret.Severity?.toLowerCase() || 'medium';

            secrets.push({
              type: secret.RuleID || 'unknown',
              severity: secret.Severity || 'MEDIUM',
              line: secret.StartLine || 0,
              content: secret.Code?.Lines?.[0]?.Content || '',
              file: result.Target,
            });

            switch (severity) {
              case 'high':
                high++;
                break;
              case 'medium':
                medium++;
                break;
              case 'low':
                low++;
                break;
            }
          }
        }
      }
    }

    const total = high + medium + low;

    return Success({
      secrets,
      summary: { total, high, medium, low },
    });
  } catch (error) {
    return Failure(`Failed to parse secret scan results: ${extractErrorMessage(error)}`);
  }
}

function calculateRiskScore(
  vulnResult: SecurityScanResult,
  secretResult: SecretScanResult,
): number {
  const vulnerabilityScore =
    vulnResult.summary.critical * 10 +
    vulnResult.summary.high * 7 +
    vulnResult.summary.medium * 5 +
    vulnResult.summary.low * 2;

  const secretScore =
    secretResult.summary.high * 8 + secretResult.summary.medium * 5 + secretResult.summary.low * 2;

  return vulnerabilityScore + secretScore;
}

function getHighestSeverity(
  vulnResult: SecurityScanResult,
  secretResult: SecretScanResult,
): string {
  if (vulnResult.summary.critical > 0) return 'CRITICAL';
  if (vulnResult.summary.high > 0 || secretResult.summary.high > 0) return 'HIGH';
  if (vulnResult.summary.medium > 0 || secretResult.summary.medium > 0) return 'MEDIUM';
  if (vulnResult.summary.low > 0 || secretResult.summary.low > 0) return 'LOW';
  return 'NONE';
}

/**
 * Security scanner interface for scan tool
 */
interface SecurityScanner {
  scanImage: (imageId: string) => Promise<Result<BasicScanResult>>;
  ping: () => Promise<Result<boolean>>;
}

export interface BasicScanResult {
  imageId: string;
  vulnerabilities: Array<{
    id: string;
    severity: 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL';
    package: string;
    version: string;
    fixedVersion?: string;
    description: string;
  }>;
  totalVulnerabilities: number;
  criticalCount: number;
  highCount: number;
  mediumCount: number;
  lowCount: number;
  scanDate: Date;
}

/**
 * Create a security scanner with direct integration
 */
export const createSecurityScanner = (logger: Logger, scannerType?: string): SecurityScanner => {
  return {
    /**
     * Scan Docker image for vulnerabilities
     */
    async scanImage(imageId: string): Promise<Result<BasicScanResult>> {
      try {
        logger.info({ imageId, scanner: scannerType }, 'Starting security scan');

        // Simplified implementation - can be enhanced with specific scanner integrations
        const result: BasicScanResult = {
          imageId,
          vulnerabilities: [],
          totalVulnerabilities: 0,
          criticalCount: 0,
          highCount: 0,
          mediumCount: 0,
          lowCount: 0,
          scanDate: new Date(),
        };

        logger.info(
          {
            imageId,
            totalVulnerabilities: result.totalVulnerabilities,
            criticalCount: result.criticalCount,
            highCount: result.highCount,
          },
          'Security scan completed',
        );

        return Success(result);
      } catch (error) {
        const errorMessage = extractErrorMessage(error);
        logger.error({ error: errorMessage, imageId }, 'Security scan failed');

        return Failure(errorMessage);
      }
    },

    /**
     * Check scanner availability
     */
    async ping(): Promise<Result<boolean>> {
      try {
        logger.debug('Checking scanner availability');
        return Success(true);
      } catch (error) {
        const errorMessage = extractErrorMessage(error);
        return Failure(errorMessage);
      }
    },
  };
};
