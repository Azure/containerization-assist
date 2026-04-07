import type { Logger } from 'pino';
import Docker from 'dockerode';
import { Result, Success, Failure } from '@/types';
import { extractErrorMessage } from '@/lib/errors';
import { scanImageWithTrivy, checkTrivyAvailability } from './trivy-scanner';
import { scanImageWithSnyk, checkSnykAvailability } from './snyk-scanner';
import { scanImageWithGrype, checkGrypeAvailability } from './grype-scanner';
import { scanImageWithOSV, checkOSVAvailability } from './osv-scanner/index';
import {
  autoDetectDockerSocket,
  dockerHostToOptions,
  toDockerHostURI,
} from '@/infra/docker/socket-validation';

export interface SecurityScanner {
  scanImage: (imageId: string) => Promise<Result<BasicScanResult>>;
  ping: () => Promise<Result<boolean>>;
}

export interface BasicScanResult {
  imageId: string;
  vulnerabilities: Array<{
    id: string;
    severity: 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL' | 'NEGLIGIBLE' | 'UNKNOWN';
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
  negligibleCount: number;
  unknownCount: number;
  scanDate: Date;
}

/**
 * Create a Trivy-based security scanner
 */
function createTrivyScanner(logger: Logger, dockerHost: string): SecurityScanner {
  return {
    async scanImage(imageId: string): Promise<Result<BasicScanResult>> {
      return scanImageWithTrivy(imageId, logger, dockerHost);
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
 * Create a Snyk-based security scanner
 */
function createSnykScanner(logger: Logger, dockerHost: string): SecurityScanner {
  return {
    async scanImage(imageId: string): Promise<Result<BasicScanResult>> {
      return scanImageWithSnyk(imageId, logger, dockerHost);
    },

    async ping(): Promise<Result<boolean>> {
      const result = await checkSnykAvailability(logger);
      if (result.ok) {
        logger.debug({ version: result.value }, 'Snyk scanner available');
        return Success(true);
      }
      return Failure(result.error, result.guidance);
    },
  };
}

/**
 * Create a Grype-based security scanner
 */
function createGrypeScanner(logger: Logger, dockerHost: string): SecurityScanner {
  return {
    async scanImage(imageId: string): Promise<Result<BasicScanResult>> {
      return scanImageWithGrype(imageId, logger, dockerHost);
    },

    async ping(): Promise<Result<boolean>> {
      const result = await checkGrypeAvailability(logger);
      if (result.ok) {
        logger.debug({ version: result.value }, 'Grype scanner available');
        return Success(true);
      }
      return Failure(result.error, result.guidance);
    },
  };
}

/**
 * Create an OSV-based security scanner
 * Uses OSV API (no external CLI required)
 */
function createOSVScanner(logger: Logger, dockerHost: string): SecurityScanner {
  // Create Docker client targeting the resolved daemon endpoint
  const opts = dockerHostToOptions(dockerHost);
  const docker = new Docker(opts);
  logger.debug({ dockerHost, opts }, 'Created Docker client for OSV scanner');

  return {
    async scanImage(imageId: string): Promise<Result<BasicScanResult>> {
      return scanImageWithOSV(docker, imageId, logger);
    },

    async ping(): Promise<Result<boolean>> {
      const result = await checkOSVAvailability(logger);
      if (result.ok) {
        logger.debug('OSV scanner available');
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
          negligibleCount: 0,
          unknownCount: 0,
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
 * @param scannerType - Type of scanner to create ('osv', 'trivy', 'snyk', 'grype', 'stub', or undefined for 'osv')
 * @param dockerHost - Optional Docker daemon endpoint to target a specific Docker context
 * @returns SecurityScanner instance
 */
export const createSecurityScanner = (
  logger: Logger,
  scannerType?: string,
  dockerHost?: string,
): SecurityScanner => {
  const type = (scannerType || 'osv').toLowerCase();

  // Stub scanner needs no Docker connection — return early to skip detection
  if (type === 'stub') {
    return createStubScanner(logger);
  }

  // Ensure every scanner talks to the same Docker daemon the server uses
  const effectiveDockerHost = dockerHost ?? toDockerHostURI(autoDetectDockerSocket());
  logger.debug({ effectiveDockerHost, explicit: !!dockerHost }, 'Scanner Docker host resolved');

  switch (type) {
    case 'osv':
      return createOSVScanner(logger, effectiveDockerHost);
    case 'trivy':
      return createTrivyScanner(logger, effectiveDockerHost);
    case 'snyk':
      return createSnykScanner(logger, effectiveDockerHost);
    case 'grype':
      return createGrypeScanner(logger, effectiveDockerHost);
    default:
      logger.warn({ scannerType: type }, 'Unknown scanner type, falling back to OSV');
      return createOSVScanner(logger, effectiveDockerHost);
  }
};
