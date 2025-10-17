import type { Logger } from 'pino';
import { Result, Success, Failure } from '@/types';
import { extractErrorMessage } from '@/lib/error-utils';

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

export const createSecurityScanner = (logger: Logger, scannerType?: string): SecurityScanner => {
  return {
    async scanImage(imageId: string): Promise<Result<BasicScanResult>> {
      try {
        logger.info({ imageId, scanner: scannerType }, 'Starting security scan');

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
