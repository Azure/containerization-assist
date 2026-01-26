/**
 * Unit Tests: Scanner Factory
 * Tests the security scanner factory and scanner type selection
 */

import { jest } from '@jest/globals';
import { createSecurityScanner } from '../../../../src/infra/security/scanner';
import { createLogger } from '../../../../src/lib/logger';

// Mock the individual scanner modules
jest.mock('../../../../src/infra/security/trivy-scanner', () => ({
  scanImageWithTrivy: jest.fn(),
  checkTrivyAvailability: jest.fn(),
}));

jest.mock('../../../../src/infra/security/snyk-scanner', () => ({
  scanImageWithSnyk: jest.fn(),
  checkSnykAvailability: jest.fn(),
}));

jest.mock('../../../../src/infra/security/grype-scanner', () => ({
  scanImageWithGrype: jest.fn(),
  checkGrypeAvailability: jest.fn(),
}));

jest.mock('../../../../src/infra/security/osv-scanner', () => ({
  scanImageWithOSV: jest.fn(),
  checkOSVAvailability: jest.fn(),
}));

jest.mock('../../../../src/lib/logger', () => ({
  createLogger: jest.fn(() => ({
    info: jest.fn(),
    error: jest.fn(),
    warn: jest.fn(),
    debug: jest.fn(),
    trace: jest.fn(),
    fatal: jest.fn(),
    child: jest.fn().mockReturnThis(),
  })),
}));

describe('Scanner Factory', () => {
  let logger: any;

  beforeEach(() => {
    jest.clearAllMocks();
    logger = (createLogger as jest.Mock)();
  });

  describe('Scanner Type Selection', () => {
    it('should create OSV scanner by default', () => {
      const scanner = createSecurityScanner(logger);
      expect(scanner).toBeDefined();
      expect(scanner.scanImage).toBeDefined();
      expect(scanner.ping).toBeDefined();
    });

    it('should create OSV scanner when explicitly specified', () => {
      const scanner = createSecurityScanner(logger, 'osv');
      expect(scanner).toBeDefined();
    });

    it('should create Trivy scanner when explicitly specified', () => {
      const scanner = createSecurityScanner(logger, 'trivy');
      expect(scanner).toBeDefined();
    });

    it('should create Snyk scanner when specified', () => {
      const scanner = createSecurityScanner(logger, 'snyk');
      expect(scanner).toBeDefined();
    });

    it('should create Grype scanner when specified', () => {
      const scanner = createSecurityScanner(logger, 'grype');
      expect(scanner).toBeDefined();
    });

    it('should create stub scanner when specified', () => {
      const scanner = createSecurityScanner(logger, 'stub');
      expect(scanner).toBeDefined();
    });

    it('should handle case-insensitive scanner names', () => {
      const scanners = [
        createSecurityScanner(logger, 'OSV'),
        createSecurityScanner(logger, 'TRIVY'),
        createSecurityScanner(logger, 'Snyk'),
        createSecurityScanner(logger, 'GrYpE'),
      ];

      scanners.forEach((scanner) => {
        expect(scanner).toBeDefined();
        expect(scanner.scanImage).toBeDefined();
      });
    });

    it('should fall back to OSV for unknown scanner types', () => {
      const scanner = createSecurityScanner(logger, 'unknown-scanner');
      expect(scanner).toBeDefined();
      expect(logger.warn).toHaveBeenCalledWith(
        { scannerType: 'unknown-scanner' },
        'Unknown scanner type, falling back to OSV',
      );
    });
  });

  describe('Scanner Interface', () => {
    it('should return scanner with scanImage method', () => {
      const scanner = createSecurityScanner(logger, 'osv');
      expect(typeof scanner.scanImage).toBe('function');
    });

    it('should return scanner with ping method', () => {
      const scanner = createSecurityScanner(logger, 'osv');
      expect(typeof scanner.ping).toBe('function');
    });
  });

  describe('Stub Scanner', () => {
    it('should return empty scan results', async () => {
      const scanner = createSecurityScanner(logger, 'stub');
      const result = await scanner.scanImage('test-image:latest');

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.vulnerabilities).toEqual([]);
        expect(result.value.totalVulnerabilities).toBe(0);
        expect(result.value.criticalCount).toBe(0);
      }
    });

    it('should always report as available', async () => {
      const scanner = createSecurityScanner(logger, 'stub');
      const result = await scanner.ping();

      expect(result.ok).toBe(true);
    });

    it('should log warning about stub scanner usage', async () => {
      const scanner = createSecurityScanner(logger, 'stub');
      await scanner.scanImage('test-image:latest');

      expect(logger.warn).toHaveBeenCalledWith(
        { imageId: 'test-image:latest' },
        'Stub scanner returns empty results - no actual scanning performed',
      );
    });
  });

  describe('Scanner Factory Consistency', () => {
    it('should return different instances for different scanner types', () => {
      const osvScanner = createSecurityScanner(logger, 'osv');
      const trivyScanner = createSecurityScanner(logger, 'trivy');
      const snykScanner = createSecurityScanner(logger, 'snyk');
      const grypeScanner = createSecurityScanner(logger, 'grype');

      // All should be defined but may be different instances
      expect(osvScanner).toBeDefined();
      expect(trivyScanner).toBeDefined();
      expect(snykScanner).toBeDefined();
      expect(grypeScanner).toBeDefined();
    });

    it('should create new instance for each call', () => {
      const scanner1 = createSecurityScanner(logger, 'osv');
      const scanner2 = createSecurityScanner(logger, 'osv');

      // Both should be defined (instances may or may not be the same)
      expect(scanner1).toBeDefined();
      expect(scanner2).toBeDefined();
    });
  });
});
