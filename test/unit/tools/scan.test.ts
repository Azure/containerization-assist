/**
 * Unit Tests: Image Scanning Tool
 * Tests the scan image tool functionality with mock security scanner
 */

import { jest } from '@jest/globals';

// Jest mocks must be at the top to ensure proper hoisting

// Create a shared mock scanner that we can access in tests
const mockSecurityScannerInstance: any = {
  scanImage: jest.fn(),
  ping: jest.fn(),
};

jest.mock('../../../src/infra/security/scanner', () => ({
  createSecurityScanner: jest.fn(() => mockSecurityScannerInstance),
}));

jest.mock('../../../src/lib/logger', () => ({
  createTimer: jest.fn(() => ({
    end: jest.fn(),
    error: jest.fn(),
  })),
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

jest.mock('../../../src/knowledge', () => ({
  getKnowledgeForCategory: jest.fn().mockReturnValue([
    {
      pattern: 'vulnerability',
      template: 'Mock remediation guidance',
      confidence: 0.9,
    },
  ]),
}));

import { scanImage } from '../../../src/tools/scan/tool';
import type { ScanImageParams } from '../../../src/tools/scan/schema';
import { createLogger } from '../../../src/lib/logger';

// Get the mocked instances after imports
const mockSessionManager = {
  create: jest.fn(),
  get: jest.fn(),
  update: jest.fn(),
};
const mockLogger = (createLogger as jest.Mock)();

// Create session facade mock
const mockSessionFacade = {
  id: 'test-session-123',
  get: jest.fn(),
  set: jest.fn(),
  pushStep: jest.fn(),
};

function createMockToolContext() {
  return {
    logger: mockLogger,
    sessionManager: mockSessionManager,
    session: mockSessionFacade,
  } as any;
}

// Test helper functions
const createSuccessResult = <T>(value: T) => ({ ok: true, value }) as const;
const createFailureResult = (error: string) => ({ ok: false, error }) as const;

describe('scanImage', () => {
  let config: ScanImageParams;

  beforeEach(() => {
    config = {
      imageId: 'sha256:mock-image-id',
      scanner: 'trivy',
      severity: 'HIGH',
    };

    // Reset all mocks
    jest.clearAllMocks();
    mockSessionManager.update.mockResolvedValue(true);
  });

  describe('Basic Functionality', () => {
    beforeEach(() => {
      // Default scan result with vulnerabilities - BasicScanResult format
      mockSecurityScannerInstance.scanImage.mockResolvedValue(
        createSuccessResult({
          imageId: 'sha256:mock-image-id',
          vulnerabilities: [
            {
              id: 'CVE-2023-1234',
              severity: 'HIGH' as const,
              package: 'test-package',
              version: '1.0.0',
              description: 'A high severity security issue',
              fixedVersion: '1.2.0',
            },
          ],
          totalVulnerabilities: 1,
          criticalCount: 0,
          highCount: 1,
          mediumCount: 0,
          lowCount: 0,
          scanDate: new Date('2023-01-01T12:00:00Z'),
        }) as any,
      );
    });

    it('should successfully scan image and return results', async () => {
      const mockContext = createMockToolContext();
      const result = await scanImage(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.success).toBe(true);
        expect(result.value.vulnerabilities.high).toBe(1);
        expect(result.value.vulnerabilities.total).toBe(1);
        expect(result.value.passed).toBe(false); // Has high vulnerability with high threshold
        expect(result.value.scanTime).toBe('2023-01-01T12:00:00.000Z');
      }

      // Verify scanner was called with correct image ID
      expect(mockSecurityScannerInstance.scanImage).toHaveBeenCalledWith('sha256:mock-image-id');
    });

    it('should pass scan with no vulnerabilities', async () => {
      mockSecurityScannerInstance.scanImage.mockResolvedValue(
        createSuccessResult({
          vulnerabilities: [],
          criticalCount: 0,
          highCount: 0,
          mediumCount: 0,
          lowCount: 0,
          totalVulnerabilities: 0,
          scanDate: new Date('2023-01-01T12:00:00Z'),
          imageId: 'sha256:mock-image-id',
        }) as any,
      );

      const mockContext = createMockToolContext();
      const result = await scanImage(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.passed).toBe(true);
        expect(result.value.vulnerabilities.total).toBe(0);
      }
    });

    it('should respect severity threshold settings', async () => {
      config.severity = 'CRITICAL';

      // Only high vulnerability, threshold is critical
      const mockContext = createMockToolContext();
      const result = await scanImage(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.success).toBe(true); // Should pass since high < critical
      }
    });

    it('should use default scanner and threshold when not specified', async () => {
      const minimalConfig: ScanImageParams = {
        imageId: 'sha256:mock-image-id',
      };

      const mockContext = createMockToolContext();
      const result = await scanImage(minimalConfig, mockContext);

      expect(result.ok).toBe(true);
      expect(mockSecurityScannerInstance.scanImage).toHaveBeenCalled();
    });
  });

  describe('Error Handling', () => {
    it('should return error when no imageId provided', async () => {
      const configWithoutImage: ScanImageParams = {
        scanner: 'trivy',
      } as any; // Cast to bypass type checking for test

      const mockContext = createMockToolContext();
      const result = await scanImage(configWithoutImage, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('No image specified. Provide imageId parameter.');
      }
    });

    it('should handle scanner failures', async () => {
      mockSecurityScannerInstance.scanImage.mockResolvedValue(
        createFailureResult('Scanner failed to analyze image') as any,
      );

      const mockContext = createMockToolContext();
      const result = await scanImage(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Failed to scan image: Scanner failed to analyze image');
      }
    });

    it('should handle exceptions during scan process', async () => {
      mockSecurityScannerInstance.scanImage.mockRejectedValue(new Error('Scanner crashed') as any);

      const mockContext = createMockToolContext();
      const result = await scanImage(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Scanner crashed');
      }
    });
  });

  describe('Vulnerability Counting', () => {
    it('should correctly count vulnerabilities by severity', async () => {
      mockSessionFacade.get.mockImplementation((key: string) => {
        if (key === 'results') {
          return {
            'build-image': {
              imageId: 'sha256:mock-image-id',
            },
          };
        }
        return undefined;
      });

      mockSecurityScannerInstance.scanImage.mockResolvedValue(
        createSuccessResult({
          vulnerabilities: [
            {
              id: 'CVE-1',
              severity: 'CRITICAL' as const,
              package: 'pkg1',
              version: '1.0',
              description: 'Critical issue',
            },
            {
              id: 'CVE-2',
              severity: 'HIGH' as const,
              package: 'pkg2',
              version: '1.0',
              description: 'High issue',
            },
            {
              id: 'CVE-3',
              severity: 'HIGH' as const,
              package: 'pkg3',
              version: '1.0',
              description: 'High issue',
            },
            {
              id: 'CVE-4',
              severity: 'MEDIUM' as const,
              package: 'pkg4',
              version: '1.0',
              description: 'Medium issue',
            },
            {
              id: 'CVE-5',
              severity: 'LOW' as const,
              package: 'pkg5',
              version: '1.0',
              description: 'Low issue',
            },
          ],
          criticalCount: 1,
          highCount: 2,
          mediumCount: 1,
          lowCount: 1,
          totalVulnerabilities: 5,
          scanDate: new Date('2023-01-01T12:00:00Z'),
          imageId: 'sha256:mock-image-id',
        }) as any,
      );

      const mockContext = createMockToolContext();
      const result = await scanImage(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.vulnerabilities).toEqual({
          critical: 1,
          high: 2,
          medium: 1,
          low: 1,
          unknown: 0,
          total: 5,
        });
      }
    });
  });

  describe('Scanner Configuration', () => {
    beforeEach(() => {
      mockSecurityScannerInstance.scanImage.mockResolvedValue(
        createSuccessResult({
          vulnerabilities: [],
          criticalCount: 0,
          highCount: 0,
          mediumCount: 0,
          lowCount: 0,
          totalVulnerabilities: 0,
          scanDate: new Date('2023-01-01T12:00:00Z'),
          imageId: 'sha256:mock-image-id',
        }) as any,
      );
    });

    it('should support different scanner types', async () => {
      // Test each scanner type
      const scannerTypes: Array<'trivy' | 'snyk' | 'grype'> = ['trivy', 'snyk', 'grype'];

      for (const scanner of scannerTypes) {
        config.scanner = scanner;
        const mockContext = createMockToolContext();
        const result = await scanImage(config, mockContext);

        expect(result.ok).toBe(true);
        // Verify the scanner was created with the correct type
        // (Implementation detail: scanner type is passed to createSecurityScanner)
      }
    });

    it('should support different severity thresholds', async () => {
      const thresholds: Array<'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL'> = [
        'LOW',
        'MEDIUM',
        'HIGH',
        'CRITICAL',
      ];

      for (const threshold of thresholds) {
        config.severity = threshold;
        const mockContext = createMockToolContext();
        const result = await scanImage(config, mockContext);

        expect(result.ok).toBe(true);
      }
    });
  });
});
