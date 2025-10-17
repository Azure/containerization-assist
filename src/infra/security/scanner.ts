import type { Logger } from 'pino';
import { Result, Success, Failure } from '@/types';
import { extractErrorMessage } from '@/lib/error-utils';
import { scanImageWithTrivy, checkTrivyAvailability } from './trivy-scanner';

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
 * Create a Trivy-based security scanner
 */
function createTrivyScanner(logger: Logger): SecurityScanner {
  return {
    async scanImage(imageId: string): Promise<Result<BasicScanResult>> {
      return scanImageWithTrivy(imageId, logger);
    },

    async ping(): Promise<Result<boolean>> {
      const result = await checkTrivyAvailability(logger);
      if (result.ok) {
        logger.debug({ version: result.value }, 'Trivy scanner available');
        return Success(true);
      }
      return Failure(result.error, result.guidance);
    },
  };
}

/**
 * Create a stub scanner that returns empty results
 * Used when no real scanner is configured
 */
function createStubScanner(logger: Logger): SecurityScanner {
  return {
    async scanImage(imageId: string): Promise<Result<BasicScanResult>> {
      try {
        logger.info({ imageId, scanner: 'stub' }, 'Starting stub security scan');

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

        logger.warn(
          { imageId },
          'Stub scanner returns empty results - no actual scanning performed',
        );

        return Success(result);
      } catch (error) {
        const errorMessage = extractErrorMessage(error);
        logger.error({ error: errorMessage, imageId }, 'Security scan failed');

        return Failure(errorMessage);
      }
    },

    async ping(): Promise<Result<boolean>> {
      logger.debug('Checking stub scanner availability');
      return Success(true);
    },
  };
}

/**
 * Create a security scanner based on the specified type
 *
 * @param logger - Logger instance
 * @param scannerType - Type of scanner to create ('trivy', 'stub', or undefined for 'trivy')
 * @returns SecurityScanner instance
 */
export const createSecurityScanner = (
  logger: Logger,
  scannerType?: string,
): SecurityScanner => {
  const type = (scannerType || 'trivy').toLowerCase();

  switch (type) {
    case 'trivy':
      return createTrivyScanner(logger);
    case 'stub':
      return createStubScanner(logger);
    default:
      logger.warn({ scannerType: type }, 'Unknown scanner type, falling back to Trivy');
      return createTrivyScanner(logger);
  }
};
