/**
 * Unit Tests: Image Scanning Tool
 * Tests the scan image tool functionality with mock security scanner
 */

import { jest } from '@jest/globals';
import { scanImage, type ScanImageConfig } from '../../../src/tools/scan/tool';
import { createMockLogger, createSuccessResult, createFailureResult } from '../../__support__/utilities/mock-infrastructure';

// Mock lib modules following analyze-repo pattern
const mockSessionManager = {
  create: jest.fn().mockResolvedValue({
    "sessionId": "test-session-123",
    "workflow_state": {},
    "metadata": {},
    "completed_steps": [],
    "errors": {},
    "current_step": null,
    "createdAt": "2025-09-08T11:12:40.362Z",
    "updatedAt": "2025-09-08T11:12:40.362Z"
  }),
  get: jest.fn(),
  update: jest.fn(),
};

const mockSecurityScanner = {
  scanImage: jest.fn(),
};

const mockTimer = {
  end: jest.fn(),
  error: jest.fn(),
};

jest.mock('@lib/session', () => ({
  createSessionManager: jest.fn(() => mockSessionManager),
}));

jest.mock('@lib/scanner', () => ({
  createSecurityScanner: jest.fn(() => mockSecurityScanner),
}));

jest.mock('@lib/logger', () => ({
  createTimer: jest.fn(() => mockTimer),
  createLogger: jest.fn(() => createMockLogger()),
}));

describe('scanImage', () => {
  let mockLogger: ReturnType<typeof createMockLogger>;
  let config: ScanImageConfig;

  beforeEach(() => {
    mockLogger = createMockLogger();
    config = {
      sessionId: 'test-session-123',
      scanner: 'trivy',
      severityThreshold: 'high',
    };

    // Reset all mocks
    jest.clearAllMocks();
    mockSessionManager.update.mockResolvedValue(true);
  });

  describe('Basic Functionality', () => {
    beforeEach(() => {
      // Session with valid build result
      mockSessionManager.get.mockResolvedValue({
        build_result: {
          imageId: 'sha256:mock-image-id',
        },
        repo_path: '/test/repo',
      });

      // Default scan result with vulnerabilities
      mockSecurityScanner.scanImage.mockResolvedValue(createSuccessResult({
        vulnerabilities: [
          {
            id: 'CVE-2023-1234',
            severity: 'HIGH',
            package: 'test-package',
            version: '1.0.0',
            description: 'A high severity security issue',
            fixedVersion: '1.2.0',
          },
        ],
        criticalCount: 0,
        highCount: 1,
        mediumCount: 0,
        lowCount: 0,
        totalVulnerabilities: 1,
        scanDate: new Date('2023-01-01T12:00:00Z'),
        imageId: 'sha256:mock-image-id',
      }));
    });

    it('should successfully scan image and return results', async () => {
      const result = await scanImage(config, { logger: mockLogger, sessionManager: mockSessionManager });

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.success).toBe(true);
        expect(result.value.sessionId).toBe('test-session-123');
        expect(result.value.vulnerabilities.high).toBe(1);
        expect(result.value.vulnerabilities.total).toBe(1);
        expect(result.value.passed).toBe(false); // Has high vulnerability with high threshold
        expect(result.value.scanTime).toBe('2023-01-01T12:00:00.000Z');
      }

      // Verify scanner was called with correct image ID
      expect(mockSecurityScanner.scanImage).toHaveBeenCalledWith('sha256:mock-image-id');
      
      // Verify session was updated
      expect(mockSessionManager.update).toHaveBeenCalledWith(
        'test-session-123',
        expect.objectContaining({
          scan_result: expect.objectContaining({
            success: false, // Failed due to vulnerability above threshold
          }),
          completed_steps: expect.arrayContaining(['scan']),
        })
      );
    });

    it('should pass scan with no vulnerabilities', async () => {
      mockSecurityScanner.scanImage.mockResolvedValue(createSuccessResult({
        vulnerabilities: [],
        criticalCount: 0,
        highCount: 0,
        mediumCount: 0,
        lowCount: 0,
        totalVulnerabilities: 0,
        scanDate: new Date('2023-01-01T12:00:00Z'),
        imageId: 'sha256:mock-image-id',
      }));

      const result = await scanImage(config, { logger: mockLogger, sessionManager: mockSessionManager });

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.passed).toBe(true);
        expect(result.value.vulnerabilities.total).toBe(0);
      }
    });

    it('should respect severity threshold settings', async () => {
      config.severityThreshold = 'critical';
      
      // Only high vulnerability, threshold is critical
      const result = await scanImage(config, { logger: mockLogger, sessionManager: mockSessionManager });

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.success).toBe(true); // Should pass since high < critical
      }
    });

    it('should use default scanner and threshold when not specified', async () => {
      const minimalConfig: ScanImageConfig = {
        sessionId: 'test-session-123',
      };

      const result = await scanImage(minimalConfig, { logger: mockLogger, sessionManager: mockSessionManager });

      expect(result.ok).toBe(true);
      expect(mockSecurityScanner.scanImage).toHaveBeenCalled();
    });
  });

  describe('Error Handling', () => {
    it('should auto-create session when not found', async () => {
      mockSessionManager.get.mockResolvedValue(null);
      mockSessionManager.create.mockResolvedValue({
      "sessionId": "test-session-123",
      "workflow_state": {},
      "metadata": {},
      "completed_steps": [],
      "errors": {},
      "current_step": null,
      "createdAt": "2025-09-08T11:12:40.362Z",
      "updatedAt": "2025-09-08T11:12:40.362Z"
});

      const result = await scanImage(config, { logger: mockLogger, sessionManager: mockSessionManager });

      expect(mockSessionManager.get).toHaveBeenCalledWith('test-session-123');
      expect(mockSessionManager.create).toHaveBeenCalledWith('test-session-123');
    });

    it('should return error when no build result exists', async () => {
      mockSessionManager.get.mockResolvedValue({
        repo_path: '/test/repo',
      });

      const result = await scanImage(config, { logger: mockLogger, sessionManager: mockSessionManager });

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('No image specified. Provide imageId parameter or ensure session has built image from build-image tool.');
      }
    });

    it('should handle scanner failures', async () => {
      mockSessionManager.get.mockResolvedValue({
        build_result: {
          imageId: 'sha256:mock-image-id',
        },
        repo_path: '/test/repo',
      });

      mockSecurityScanner.scanImage.mockResolvedValue(
        createFailureResult('Scanner failed to analyze image')
      );

      const result = await scanImage(config, { logger: mockLogger, sessionManager: mockSessionManager });

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Failed to scan image: Scanner failed to analyze image');
      }
    });

    it('should handle exceptions during scan process', async () => {
      mockSessionManager.get.mockResolvedValue({
        build_result: {
          imageId: 'sha256:mock-image-id',
        },
        repo_path: '/test/repo',
      });

      mockSecurityScanner.scanImage.mockRejectedValue(new Error('Scanner crashed'));

      const result = await scanImage(config, { logger: mockLogger, sessionManager: mockSessionManager });

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Scanner crashed');
      }
    });
  });

  describe('Vulnerability Counting', () => {
    it('should correctly count vulnerabilities by severity', async () => {
      mockSessionManager.get.mockResolvedValue({
        build_result: {
          imageId: 'sha256:mock-image-id',
        },
        repo_path: '/test/repo',
      });

      mockSecurityScanner.scanImage.mockResolvedValue(createSuccessResult({
        vulnerabilities: [
          { id: 'CVE-1', severity: 'CRITICAL', package: 'pkg1', version: '1.0', description: 'Critical issue' },
          { id: 'CVE-2', severity: 'HIGH', package: 'pkg2', version: '1.0', description: 'High issue' },
          { id: 'CVE-3', severity: 'HIGH', package: 'pkg3', version: '1.0', description: 'High issue' },
          { id: 'CVE-4', severity: 'MEDIUM', package: 'pkg4', version: '1.0', description: 'Medium issue' },
          { id: 'CVE-5', severity: 'LOW', package: 'pkg5', version: '1.0', description: 'Low issue' },
        ],
        criticalCount: 1,
        highCount: 2,
        mediumCount: 1,
        lowCount: 1,
        totalVulnerabilities: 5,
        scanDate: new Date('2023-01-01T12:00:00Z'),
        imageId: 'sha256:mock-image-id',
      }));

      const result = await scanImage(config, { logger: mockLogger, sessionManager: mockSessionManager });

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
      mockSessionManager.get.mockResolvedValue({
        build_result: {
          imageId: 'sha256:mock-image-id',
        },
        repo_path: '/test/repo',
      });

      mockSecurityScanner.scanImage.mockResolvedValue(createSuccessResult({
        vulnerabilities: [],
        criticalCount: 0,
        highCount: 0,
        mediumCount: 0,
        lowCount: 0,
        totalVulnerabilities: 0,
        scanDate: new Date('2023-01-01T12:00:00Z'),
        imageId: 'sha256:mock-image-id',
      }));
    });

    it('should support different scanner types', async () => {
      // Test each scanner type
      const scannerTypes: Array<'trivy' | 'snyk' | 'grype'> = ['trivy', 'snyk', 'grype'];
      
      for (const scanner of scannerTypes) {
        config.scanner = scanner;
        const result = await scanImage(config, { logger: mockLogger, sessionManager: mockSessionManager });
        
        expect(result.ok).toBe(true);
        // Verify the scanner was created with the correct type
        // (Implementation detail: scanner type is passed to createSecurityScanner)
      }
    });

    it('should support different severity thresholds', async () => {
      const thresholds: Array<'low' | 'medium' | 'high' | 'critical'> = ['low', 'medium', 'high', 'critical'];
      
      for (const threshold of thresholds) {
        config.severityThreshold = threshold;
        const result = await scanImage(config, { logger: mockLogger, sessionManager: mockSessionManager });
        
        expect(result.ok).toBe(true);
      }
    });
  });
});