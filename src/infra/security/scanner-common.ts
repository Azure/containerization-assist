/**
 * Common Scanner Utilities
 *
 * Shared functionality across security scanner implementations
 * to reduce code duplication and centralize security-critical logic.
 */

import type { Logger } from 'pino';
import { Failure, type Result } from '@/types';

/**
 * Safely quote a value for use in shell commands
 * Uses POSIX single-quote escaping: wraps in '' and escapes internal quotes as '\''
 * This is the standard approach used by shell-quote and similar libraries
 *
 * @example
 * shellQuote("hello") => "'hello'"
 * shellQuote("it's") => "'it'\\''s'"
 * shellQuote("a;rm -rf /") => "'a;rm -rf /'"
 */
function shellQuote(value: string): string {
  // POSIX single-quote escaping: replace ' with '\''
  // This closes the quote, adds escaped quote, then reopens quote
  return `'${value.replace(/'/g, "'\\''")}'`;
}

/**
 * Common error messages and guidance for scanner implementations
 */
export const ScannerErrors = {
  /**
   * Create invalid imageId error
   */
  invalidImageId: (imageId: string): Result<never> =>
    Failure('Invalid imageId format', {
      message: 'ImageId contains invalid characters',
      hint: 'ImageId must contain only alphanumeric characters, dots, colons, slashes, at-signs, underscores, and hyphens',
      resolution: 'Verify the imageId is a valid Docker image identifier',
      details: { imageId },
    }),

  /**
   * Create scanner not installed error
   */
  scannerNotInstalled: (scannerName: string, installUrl: string): Result<never> =>
    Failure(`${scannerName} not installed or not in PATH`, {
      message: `${scannerName} CLI not found`,
      hint: `${scannerName} CLI is required for security scanning`,
      resolution: `Install ${scannerName}: ${installUrl}`,
    }),

  /**
   * Create JSON parse error
   */
  jsonParseError: (scannerName: string, parseError: string, outputPreview: string): Result<never> =>
    Failure(`Failed to parse ${scannerName} output`, {
      message: `${scannerName} output parsing failed`,
      hint: `${scannerName} may have returned invalid JSON`,
      resolution: `Try running ${scannerName} manually to verify`,
      details: {
        parseError,
        outputPreview,
      },
    }),

  /**
   * Create empty output error
   */
  emptyOutput: (scannerName: string, imageId: string): Result<never> =>
    Failure(`${scannerName} returned empty output`, {
      message: 'No scan results received',
      hint: `${scannerName} may not have found the image or encountered an error`,
      resolution: `Verify image exists: docker image inspect ${shellQuote(imageId)}`,
    }),

  /**
   * Create scan execution error
   */
  scanExecutionError: (scannerName: string, imageId: string, errorMessage: string): Result<never> =>
    Failure(`${scannerName} scan failed: ${errorMessage}`, {
      message: 'Security scan execution failed',
      hint: `${scannerName} encountered an error while scanning the image`,
      resolution: `Check image exists and is accessible: docker image ls | grep ${shellQuote(imageId)}`,
      details: { error: errorMessage },
    }),

  /**
   * Create version check timeout error
   */
  versionCheckTimeout: (scannerName: string, command: string): Result<never> =>
    Failure(`${scannerName} version check timed out`, {
      message: 'Command execution timeout',
      hint: `${scannerName} CLI took too long to respond`,
      resolution: `Check if ${scannerName} is functioning correctly: ${command}`,
    }),

  /**
   * Create version parse error
   */
  versionParseError: (scannerName: string, command: string): Result<never> =>
    Failure(`${scannerName} version could not be parsed`, {
      message: `${scannerName} version check failed`,
      hint: `${scannerName} CLI may not be properly configured`,
      resolution: `Try running: ${command}`,
    }),
};

/**
 * Validate imageId against allowlist pattern to prevent command injection
 * Allows: alphanumeric, dots, colons, slashes, at-signs, underscores, and hyphens
 */
export function validateImageId(imageId: string): boolean {
  const allowedPattern = /^[a-zA-Z0-9._:/@-]+$/;
  return allowedPattern.test(imageId);
}

/**
 * Standardized severity type
 */
export type StandardSeverity = 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL' | 'NEGLIGIBLE' | 'UNKNOWN';

/**
 * Normalize severity string to our standard format
 * Handles various case formats (UPPERCASE, lowercase, TitleCase)
 */
export function normalizeSeverity(severity: string): StandardSeverity {
  const normalized = severity.toUpperCase();
  switch (normalized) {
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
 * Severity counter helper
 */
export class SeverityCounter {
  critical = 0;
  high = 0;
  medium = 0;
  low = 0;
  negligible = 0;
  unknown = 0;

  increment(severity: StandardSeverity): void {
    switch (severity) {
      case 'CRITICAL':
        this.critical++;
        break;
      case 'HIGH':
        this.high++;
        break;
      case 'MEDIUM':
        this.medium++;
        break;
      case 'LOW':
        this.low++;
        break;
      case 'NEGLIGIBLE':
        this.negligible++;
        break;
      case 'UNKNOWN':
        this.unknown++;
        break;
    }
  }

  get total(): number {
    return this.critical + this.high + this.medium + this.low + this.negligible + this.unknown;
  }

  getCounts(): {
    criticalCount: number;
    highCount: number;
    mediumCount: number;
    lowCount: number;
    negligibleCount: number;
    unknownCount: number;
    totalVulnerabilities: number;
  } {
    return {
      criticalCount: this.critical,
      highCount: this.high,
      mediumCount: this.medium,
      lowCount: this.low,
      negligibleCount: this.negligible,
      unknownCount: this.unknown,
      totalVulnerabilities: this.total,
    };
  }
}

/**
 * Parse version from scanner output using a regex pattern
 */
export function parseVersion(output: string, pattern: RegExp): string | undefined {
  const match = output.match(pattern);
  return match?.[1]?.trim();
}

/**
 * Log scanner information
 */
export function logScanStart(
  logger: Logger,
  scanner: string,
  version: string,
  imageId: string,
): void {
  logger.info({ scanner, version, imageId }, `Starting ${scanner} scan`);
}

/**
 * Log scan completion
 */
export function logScanComplete(
  logger: Logger,
  scanner: string,
  imageId: string,
  totalVulnerabilities: number,
  criticalCount: number,
  highCount: number,
): void {
  logger.info(
    {
      scanner,
      imageId,
      totalVulnerabilities,
      criticalCount,
      highCount,
    },
    `${scanner} scan completed successfully`,
  );
}
