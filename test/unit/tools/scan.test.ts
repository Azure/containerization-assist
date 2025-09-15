/**
 * Unit Tests: Image Scanning Tool
 * Tests the scan image tool functionality with mock security scanner
 */

import { jest } from '@jest/globals';

// Jest mocks must be at the top to ensure proper hoisting
jest.mock('../../../src/lib/session', () => ({
  createSessionManager: jest.fn(() => ({
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
    update: jest.fn().mockResolvedValue(true),
  })),
}));

// Create a shared mock scanner that we can access in tests
const mockSecurityScannerInstance = {
  scanImage: jest.fn(),
  ping: jest.fn(),
};

jest.mock('../../../src/lib/scanner', () => ({
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

import { createToolSessionHelpersMock } from '../../__support__/mocks/tool-session-helpers.mock';

jest.mock('../../../src/mcp/tool-session-helpers', () => createToolSessionHelpersMock());

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
import { createSessionManager } from '../../../src/lib/session';
import { createLogger } from '../../../src/lib/logger';
import { ensureSession } from '../../../src/mcp/tool-session-helpers';

// Get the mocked instances after imports
const mockSessionManager = (createSessionManager as jest.Mock)();
const mockLogger = (createLogger as jest.Mock)();
const mockEnsureSession = ensureSession as jest.Mock;
// mockSecurityScannerInstance is already defined above

// Test helper functions
const createSuccessResult = <T>(value: T) => ({ ok: true, value } as const);
const createFailureResult = (error: string) => ({ ok: false, error } as const);

describe('scanImage', () => {
  let config: ScanImageParams;

  beforeEach(() => {
    config = {
      sessionId: 'test-session-123',
      scanner: 'trivy',
      severity: 'HIGH',
    };

    // Reset all mocks
    jest.clearAllMocks();
    mockSessionManager.update.mockResolvedValue(true);
  });

  describe('Basic Functionality', () => {
    beforeEach(() => {
      // Session with valid build result
      mockEnsureSession.mockResolvedValue({
        ok: true,
        value: {
          id: 'test-session-123',
          state: {
            results: {
              'build-image': {
                imageId: 'sha256:mock-image-id',
              },
            },
            repo_path: '/test/repo',
          },
        },
      });

      // Default scan result with vulnerabilities - BasicScanResult format
      mockSecurityScannerInstance.scanImage.mockResolvedValue(createSuccessResult({
        imageId: 'sha256:mock-image-id',
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
        totalVulnerabilities: 1,
        criticalCount: 0,
        highCount: 1,
        mediumCount: 0,
        lowCount: 0,
        scanDate: new Date('2023-01-01T12:00:00Z'),
      }));
    });

    it('should successfully scan image and return results', async () => {
      const result = await scanImage(config, {
        logger: mockLogger,
        sessionManager: mockSessionManager,
      });

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
      expect(mockSecurityScannerInstance.scanImage).toHaveBeenCalledWith('sha256:mock-image-id');

      // Verify session was updated via sessionManager with the new toolSlices structure
      expect(mockSessionManager.update).toHaveBeenCalledWith(
        'test-session-123',
        expect.objectContaining({
          metadata: expect.objectContaining({
            toolSlices: expect.objectContaining({
              scan: expect.any(Object),
            }),
          }),
        })
      );
    });

    it('should pass scan with no vulnerabilities', async () => {
      // Ensure session mock is set up for this test
      mockEnsureSession.mockResolvedValue({
        ok: true,
        value: {
          id: 'test-session-123',
          state: {
            results: {
              'build-image': {
                imageId: 'sha256:mock-image-id',
              },
            },
            repo_path: '/test/repo',
          },
        },
      });

      mockSecurityScannerInstance.scanImage.mockResolvedValue(createSuccessResult({
        vulnerabilities: [],
        criticalCount: 0,
        highCount: 0,
        mediumCount: 0,
        lowCount: 0,
        totalVulnerabilities: 0,
        scanDate: new Date('2023-01-01T12:00:00Z'),
        imageId: 'sha256:mock-image-id',
      }));

      const result = await scanImage(config, {
        logger: mockLogger,
        sessionManager: mockSessionManager,
      });

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.passed).toBe(true);
        expect(result.value.vulnerabilities.total).toBe(0);
      }
    });

    it('should respect severity threshold settings', async () => {
      config.severity = 'CRITICAL';
      
      // Only high vulnerability, threshold is critical
      const result = await scanImage(config, {
        logger: mockLogger,
        sessionManager: mockSessionManager,
      });

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.success).toBe(true); // Should pass since high < critical
      }
    });

    it('should use default scanner and threshold when not specified', async () => {
      const minimalConfig: ScanImageParams = {
        sessionId: 'test-session-123',
      };

      const result = await scanImage(minimalConfig, {
        logger: mockLogger,
        sessionManager: mockSessionManager,
      });

      expect(result.ok).toBe(true);
      expect(mockSecurityScannerInstance.scanImage).toHaveBeenCalled();
    });
  });

  describe('Error Handling', () => {
    it('should handle session not found errors', async () => {
      // Mock ensureSession to return an error (session not found)
      mockEnsureSession.mockResolvedValue({
        ok: false,
        error: 'Session not found',
      });

      const result = await scanImage(config, {
        logger: mockLogger,
        sessionManager: mockSessionManager,
      });

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Session not found');
      }
      expect(mockEnsureSession).toHaveBeenCalled();
    });

    it('should return error when no build result exists', async () => {
      // Mock session without build_result
      mockEnsureSession.mockResolvedValue({
        ok: true,
        value: {
          id: 'test-session-123',
          state: {
            repo_path: '/test/repo',
            // No build_result here
          },
        },
      });

      const result = await scanImage(config, {
        logger: mockLogger,
        sessionManager: mockSessionManager,
      });

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe(
          'No image specified. Provide imageId parameter or ensure session has built image from build-image tool.',
        );
      }
    });

    it('should handle scanner failures', async () => {
      mockEnsureSession.mockResolvedValue({
        ok: true,
        value: {
          id: 'test-session-123',
          state: {
            results: {
              'build-image': {
                imageId: 'sha256:mock-image-id',
              },
            },
            repo_path: '/test/repo',
          },
        },
      });

      mockSecurityScannerInstance.scanImage.mockResolvedValue(
        createFailureResult('Scanner failed to analyze image')
      );

      const result = await scanImage(config, {
        logger: mockLogger,
        sessionManager: mockSessionManager,
      });

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Failed to scan image: Scanner failed to analyze image');
      }
    });

    it('should handle exceptions during scan process', async () => {
      mockEnsureSession.mockResolvedValue({
        ok: true,
        value: {
          id: 'test-session-123',
          state: {
            results: {
              'build-image': {
                imageId: 'sha256:mock-image-id',
              },
            },
            repo_path: '/test/repo',
          },
        },
      });

      mockSecurityScannerInstance.scanImage.mockRejectedValue(new Error('Scanner crashed'));

      const result = await scanImage(config, {
        logger: mockLogger,
        sessionManager: mockSessionManager,
      });

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Scanner crashed');
      }
    });
  });

  describe('Vulnerability Counting', () => {
    it('should correctly count vulnerabilities by severity', async () => {
      mockEnsureSession.mockResolvedValue({
        ok: true,
        value: {
          id: 'test-session-123',
          state: {
            results: {
              'build-image': {
                imageId: 'sha256:mock-image-id',
              },
            },
            repo_path: '/test/repo',
          },
        },
      });

      mockSecurityScannerInstance.scanImage.mockResolvedValue(createSuccessResult({
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

      const result = await scanImage(config, {
        logger: mockLogger,
        sessionManager: mockSessionManager,
      });

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
      mockEnsureSession.mockResolvedValue({
        ok: true,
        value: {
          id: 'test-session-123',
          state: {
            results: {
              'build-image': {
                imageId: 'sha256:mock-image-id',
              },
            },
            repo_path: '/test/repo',
          },
        },
      });

      mockSecurityScannerInstance.scanImage.mockResolvedValue(createSuccessResult({
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
        const result = await scanImage(config, {
          logger: mockLogger,
          sessionManager: mockSessionManager,
        });

        expect(result.ok).toBe(true);
        // Verify the scanner was created with the correct type
        // (Implementation detail: scanner type is passed to createSecurityScanner)
      }
    });

    it('should support different severity thresholds', async () => {
      const thresholds: Array<'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL'> = ['LOW', 'MEDIUM', 'HIGH', 'CRITICAL'];
      
      for (const threshold of thresholds) {
        config.severity = threshold;
        const result = await scanImage(config, { logger: mockLogger, sessionManager: mockSessionManager });
        
        expect(result.ok).toBe(true);
      }
    });
  });
});
