/**
 * Security Scanner - Type Definitions Only
 *
 * Type definitions for security scanning functionality.
 * The actual implementation uses the functional approach in scanner.ts
 */

import type { Logger } from 'pino';
import { Result, Success, Failure, isFail } from '../domain/types';

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

  // Mock implementation - replace with actual scanner integration
  const result: SecurityScanResult = {
    vulnerabilities: [],
    summary: { total: 0, critical: 0, high: 0, medium: 0, low: 0, unknown: 0 },
    passed: true,
  };

  return Success(result);
}

interface CommandExecutor {
  execute(
    command: string,
    args: string[],
    options?: any,
  ): Promise<Result<{ stdout: string; stderr: string; exitCode: number }>>;
}

/**
 * Security Scanner class for vulnerability and secret detection
 */
export class SecurityScanner {
  private commandExecutor: CommandExecutor;
  private logger: Logger;

  constructor(commandExecutor: CommandExecutor, logger: Logger) {
    this.commandExecutor = commandExecutor;
    this.logger = logger;
  }

  /**
   * Scan Docker image for vulnerabilities
   */
  async scanImage(imageId: string, options?: ScanOptions): Promise<Result<SecurityScanResult>> {
    try {
      const validationResult = this.validateScanOptions(options);
      if (isFail(validationResult)) {
        return validationResult;
      }

      this.logger.info({ imageId, options }, 'Starting security scan');

      const args = ['image', '--format', 'json', imageId];

      if (options?.minSeverity) {
        args.splice(2, 0, '--severity', this.getSeverityFilter(options.minSeverity));
      }

      if (options?.skipUnfixed) {
        args.splice(-1, 0, '--ignore-unfixed');
      }

      const execOptions = {
        timeout: options?.timeout || 120000,
      };

      const result = await Promise.race([
        this.commandExecutor.execute('trivy', args, execOptions),
        this.createTimeoutPromise(options?.timeout || 120000),
      ]);

      if (isFail(result)) {
        return Failure(`Security scan failed: ${result.error}`);
      }

      if (result.value.stderr) {
        this.logger.warn({ stderr: result.value.stderr }, 'Scanner warnings');
      }

      const parseResult = this.parseTrivyOutput(result.value.stdout);
      if (isFail(parseResult)) {
        return parseResult;
      }
      const scanResult = parseResult.value;

      this.logger.info(
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
      this.logger.error({ error, imageId }, 'Security scan failed');
      return Failure(
        `Security scan failed: ${error instanceof Error ? error.message : 'Unknown error'}`,
      );
    }
  }

  /**
   * Scan filesystem for vulnerabilities
   */
  async scanFilesystem(path: string, options?: ScanOptions): Promise<Result<SecurityScanResult>> {
    try {
      this.logger.info({ path }, 'Starting filesystem scan');

      const args = ['fs', '--format', 'json', path];

      const execOptions = {
        timeout: options?.timeout || 120000,
      };

      const result = await this.commandExecutor.execute('trivy', args, execOptions);

      if (isFail(result)) {
        return Failure(`Filesystem scan failed: ${result.error}`);
      }

      const parseResult = this.parseTrivyOutput(result.value.stdout);
      if (isFail(parseResult)) {
        return parseResult;
      }
      return Success(parseResult.value);
    } catch (error) {
      return Failure(
        `Filesystem scan failed: ${error instanceof Error ? error.message : 'Unknown error'}`,
      );
    }
  }

  /**
   * Scan for secrets in code
   */
  async scanSecrets(path: string, options?: ScanOptions): Promise<Result<SecretScanResult>> {
    try {
      this.logger.info({ path }, 'Starting secret scan');

      const args = ['fs', '--format', 'json', '--scanners', 'secret', path];

      const execOptions = {
        timeout: options?.timeout || 120000,
      };

      const result = await this.commandExecutor.execute('trivy', args, execOptions);

      if (isFail(result)) {
        return Failure(`Secret scan failed: ${result.error}`);
      }

      const parseResult = this.parseSecretOutput(result.value.stdout);
      if (isFail(parseResult)) {
        return parseResult;
      }
      return Success(parseResult.value);
    } catch (error) {
      return Failure(
        `Secret scan failed: ${error instanceof Error ? error.message : 'Unknown error'}`,
      );
    }
  }

  /**
   * Generate comprehensive security report
   */
  async generateReport(imageId: string, sourcePath: string): Promise<Result<SecurityReport>> {
    try {
      const [vulnerabilityResult, secretResult] = await Promise.all([
        this.scanImage(imageId),
        this.scanSecrets(sourcePath),
      ]);

      if (isFail(vulnerabilityResult)) {
        return Failure(`Vulnerability scan failed: ${vulnerabilityResult.error}`);
      }

      if (isFail(secretResult)) {
        return Failure(`Secret scan failed: ${secretResult.error}`);
      }

      const riskScore = this.calculateRiskScore(vulnerabilityResult.value, secretResult.value);
      const highestSeverity = this.getHighestSeverity(
        vulnerabilityResult.value,
        secretResult.value,
      );

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
      return Failure(
        `Report generation failed: ${error instanceof Error ? error.message : 'Unknown error'}`,
      );
    }
  }

  /**
   * Get scanner version
   */
  async getScannerVersion(): Promise<Result<string>> {
    try {
      const result = await this.commandExecutor.execute('trivy', ['--version']);

      if (isFail(result)) {
        return Failure(`Failed to get scanner version: ${result.error}`);
      }

      return Success(result.value.stdout.trim());
    } catch (error) {
      return Failure(
        `Failed to get scanner version: ${error instanceof Error ? error.message : 'Unknown error'}`,
      );
    }
  }

  /**
   * Update vulnerability database
   */
  async updateDatabase(): Promise<Result<void>> {
    try {
      const result = await this.commandExecutor.execute('trivy', ['image', '--download-db-only'], {
        timeout: 300000, // 5 minutes for database update
      });

      if (isFail(result)) {
        return Failure(`Failed to update vulnerability database: ${result.error}`);
      }

      return Success(undefined);
    } catch (error) {
      return Failure(
        `Failed to update vulnerability database: ${error instanceof Error ? error.message : 'Unknown error'}`,
      );
    }
  }

  private validateScanOptions(options?: ScanOptions): Result<void> {
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

  private getSeverityFilter(minSeverity: string): string {
    const severityLevels = {
      LOW: 'CRITICAL,HIGH,MEDIUM,LOW',
      MEDIUM: 'CRITICAL,HIGH,MEDIUM',
      HIGH: 'CRITICAL,HIGH',
      CRITICAL: 'CRITICAL',
    };

    return severityLevels[minSeverity as keyof typeof severityLevels] || 'CRITICAL,HIGH,MEDIUM,LOW';
  }

  private parseTrivyOutput(output: string): Result<SecurityScanResult> {
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
              const severity = (vuln.Severity?.toUpperCase() ||
                'UNKNOWN') as VulnerabilityFinding['severity'];

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
      return Failure(
        `Failed to parse scan results: ${error instanceof Error ? error.message : 'Unknown error'}`,
      );
    }
  }

  private parseSecretOutput(output: string): Result<SecretScanResult> {
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
      return Failure(
        `Failed to parse secret scan results: ${error instanceof Error ? error.message : 'Unknown error'}`,
      );
    }
  }

  private calculateRiskScore(
    vulnResult: SecurityScanResult,
    secretResult: SecretScanResult,
  ): number {
    const vulnerabilityScore =
      vulnResult.summary.critical * 10 +
      vulnResult.summary.high * 7 +
      vulnResult.summary.medium * 5 +
      vulnResult.summary.low * 2;

    const secretScore =
      secretResult.summary.high * 8 +
      secretResult.summary.medium * 5 +
      secretResult.summary.low * 2;

    return vulnerabilityScore + secretScore;
  }

  private getHighestSeverity(
    vulnResult: SecurityScanResult,
    secretResult: SecretScanResult,
  ): string {
    if (vulnResult.summary.critical > 0) return 'CRITICAL';
    if (vulnResult.summary.high > 0 || secretResult.summary.high > 0) return 'HIGH';
    if (vulnResult.summary.medium > 0 || secretResult.summary.medium > 0) return 'MEDIUM';
    if (vulnResult.summary.low > 0 || secretResult.summary.low > 0) return 'LOW';
    return 'NONE';
  }

  private createTimeoutPromise(timeout: number): Promise<Result<any>> {
    return new Promise((_, reject) => {
      const timer = setTimeout(() => {
        reject(new Error(`Operation timed out after ${timeout}ms`));
      }, timeout);
      // Prevent this timer from keeping the Node.js process alive
      timer.unref();
    });
  }
}
