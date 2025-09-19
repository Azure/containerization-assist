/**
 * Unit Tests: Image Scanning Tool
 * Tests the scan image tool functionality with mock security scanner
 */

import { jest } from '@jest/globals';

// Mock child_process and util together
const mockExecAsync = jest.fn();
jest.mock('node:child_process', () => ({
  exec: jest.fn(),
}));
jest.mock('node:util', () => ({
  promisify: jest.fn(() => mockExecAsync),
}));

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

// Mock prompt-backed-tool to avoid AI calls
const mockPromptBackedToolExecute = jest.fn();
jest.mock('../../../src/mcp/tools/prompt-backed-tool', () => ({
  createPromptBackedTool: jest.fn((options) => ({
    ...options,
    execute: mockPromptBackedToolExecute,
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

import { tool } from '../../../src/tools/scan/tool';
import type { ScanImageParams } from '../../../src/tools/scan/schema';
import { createSessionManager } from '../../../src/lib/session';
import { createLogger } from '../../../src/lib/logger';
import { ensureSession } from '../../../src/mcp/tool-session-helpers';
import { exec } from 'node:child_process';
import { promisify } from 'node:util';

// Get the mocked instances after imports
const mockExec = exec as jest.MockedFunction<typeof exec>;
const mockPromisify = promisify as jest.MockedFunction<typeof promisify>;
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
      imageId: 'nginx:latest',
      scanner: 'trivy',
      severity: 'HIGH',
    };

    // Reset all mocks
    jest.clearAllMocks();
    mockSessionManager.update.mockResolvedValue(true);

    // Default mock for prompt-backed tool (can be overridden in specific tests)
    mockPromptBackedToolExecute.mockResolvedValue({
      ok: true,
      value: {
        summary: {
          totalVulnerabilities: 1,
          critical: 0,
          high: 1,
          medium: 0,
          low: 0,
          negligible: 0,
        },
        topVulnerabilities: [{
          id: 'CVE-2023-1234',
          severity: 'HIGH',
          package: 'nginx',
          version: '1.20.1',
          fixedVersion: '1.20.2',
          description: 'Test vulnerability',
          exploitability: 'medium',
        }],
        remediations: [],
        baseImageRecommendations: [],
        complianceStatus: {
          passes: false,
          blockers: ['HIGH vulnerability found'],
          warnings: [],
        },
        riskScore: {
          score: 7.5,
          level: 'high',
          factors: ['HIGH severity vulnerability'],
        },
        deploymentRecommendation: {
          canDeploy: false,
          conditions: ['Fix HIGH vulnerability'],
          requiredActions: ['Upgrade nginx to 1.20.2'],
        },
        nextSteps: ['Fix vulnerabilities before deployment'],
      },
    });

    // Mock execAsync to return successful trivy scan
    mockExecAsync.mockResolvedValue({
      stdout: JSON.stringify({
        Results: [{
          Vulnerabilities: [{
            VulnerabilityID: 'CVE-2023-1234',
            Severity: 'HIGH',
            PkgName: 'nginx',
            InstalledVersion: '1.20.1',
            FixedVersion: '1.20.2',
            Title: 'Test vulnerability',
            Description: 'Test vulnerability description',
          }]
        }]
      }),
      stderr: ''
    });
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
      const result = await tool.execute(config, { logger: mockLogger, sessionManager: mockSessionManager } as any);

      if (!result.ok) {
        console.log('Scan Tool Error:', result.error);
        throw new Error(`Expected result.ok to be true, but got error: ${result.error}`);
      }
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.success).toBe(true);
        expect(result.value.sessionId).toBe('test-session-123');
        expect(result.value.scanner).toBe('trivy');
        expect(result.value.vulnerabilities).toBeDefined();
        expect(result.value.assessment).toBeDefined();
        expect(result.value.scanTime).toBeGreaterThanOrEqual(0);
        expect(result.value.ok).toBe(true);
      }

      // Verify execAsync was called with trivy command
      expect(mockExecAsync).toHaveBeenCalledWith(
        expect.stringContaining('trivy image'),
        expect.any(Object)
      );

      // Verify session was updated via sessionManager with the new toolSlices structure
      expect(mockSessionManager.update).toHaveBeenCalledWith(
        'test-session-123',
        expect.objectContaining({
          'scan-image': expect.objectContaining({
            imageName: 'nginx:latest',
            vulnerabilities: expect.any(Object),
            assessment: expect.any(Object),
          }),
        })
      );
    });

    it('should pass scan with no vulnerabilities', async () => {
      // Mock execAsync to return empty vulnerability list
      mockExecAsync.mockResolvedValue({
        stdout: JSON.stringify({
          Results: [{
            Vulnerabilities: []
          }]
        }),
        stderr: ''
      });

      // Mock the prompt-backed tool to return no vulnerabilities
      mockPromptBackedToolExecute.mockResolvedValue({
        ok: true,
        value: {
          summary: {
            totalVulnerabilities: 0,
            critical: 0,
            high: 0,
            medium: 0,
            low: 0,
            negligible: 0,
          },
          topVulnerabilities: [],
          remediations: [],
          baseImageRecommendations: [],
          complianceStatus: {
            passes: true,
            blockers: [],
            warnings: [],
          },
          riskScore: {
            score: 0,
            level: 'low',
            factors: [],
          },
          deploymentRecommendation: {
            canDeploy: true,
            conditions: [],
            requiredActions: [],
          },
          nextSteps: [],
        },
      });

      const mockContext = {
        sessionManager: mockSessionManager,
        sampling: {
          createMessage: jest.fn().mockResolvedValue({
            role: 'assistant',
            content: [{
              type: 'text',
              text: JSON.stringify({
                summary: {
                  totalVulnerabilities: 0,
                  critical: 0,
                  high: 0,
                  medium: 0,
                  low: 0,
                  negligible: 0,
                },
                topVulnerabilities: [],
                remediations: [],
                baseImageRecommendations: [],
                complianceStatus: {
                  passes: true,
                  blockers: [],
                  warnings: [],
                },
                riskScore: {
                  score: 0,
                  level: 'low',
                  factors: [],
                },
                deploymentRecommendation: {
                  canDeploy: true,
                  conditions: [],
                  requiredActions: [],
                },
                nextSteps: [],
              })
            }]
          })
        }
      } as any;

      const result = await tool.execute(config, { logger: mockLogger }, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        // Summary is in assessment field from AI, not top-level
        expect(result.value.assessment?.summary?.totalVulnerabilities).toBe(0);
        expect(result.value.assessment?.summary?.critical).toBe(0);
        expect(result.value.assessment?.summary?.high).toBe(0);
      }
    });

    it('should respect severity threshold settings', async () => {
      config.severity = 'CRITICAL';
      
      // Only high vulnerability, threshold is critical
      const result = await tool.execute(config, { logger: mockLogger, sessionManager: mockSessionManager } as any);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.success).toBe(true); // Should pass since high < critical
      }
    });

    it('should use default scanner and threshold when not specified', async () => {
      const minimalConfig: ScanImageParams = {
        sessionId: 'test-session-123',
        imageId: 'nginx:latest',
      };

      const result = await tool.execute(minimalConfig, { logger: mockLogger, sessionManager: mockSessionManager } as any);

      expect(result.ok).toBe(true);
      expect(result.value.scanner).toBe('trivy'); // Default scanner
    });
  });

  describe('Error Handling', () => {
    it('should handle missing imageId errors', async () => {
      const configWithoutImage: ScanImageParams = {
        sessionId: 'test-session-123',
        // No imageId provided
      } as any;

      const result = await tool.execute(configWithoutImage, { logger: mockLogger, sessionManager: mockSessionManager } as any);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Image name is required');
      }
    });

    it('should work even without build result in session', async () => {
      const configWithImage: ScanImageParams = {
        sessionId: 'test-session-123',
        imageId: 'nginx:latest',
      };

      const result = await tool.execute(configWithImage, { logger: mockLogger, sessionManager: mockSessionManager } as any);

      expect(result.ok).toBe(true);
      // Tool works with just imageId parameter
    });

    it('should handle scanner failures', async () => {
      // Mock execAsync to simulate scanner failure
      mockExecAsync.mockRejectedValue(new Error('Scanner failed to analyze image'));

      const result = await tool.execute(config, { logger: mockLogger, sessionManager: mockSessionManager } as any);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Scanner failed to analyze image');
      }
    });

    it('should handle exceptions during scan process', async () => {
      // Mock execAsync to throw an error
      mockExecAsync.mockRejectedValue(new Error('Scanner crashed'));

      const result = await tool.execute(config, { logger: mockLogger, sessionManager: mockSessionManager } as any);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Scanner crashed');
      }
    });
  });

  describe('Vulnerability Counting', () => {
    it('should correctly count vulnerabilities by severity', async () => {
      // Mock execAsync to return Trivy JSON output with vulnerabilities
      mockExecAsync.mockResolvedValue({
        stdout: JSON.stringify({
          Results: [{
            Vulnerabilities: [
              { VulnerabilityID: 'CVE-1', Severity: 'CRITICAL', PkgName: 'pkg1', InstalledVersion: '1.0' },
              { VulnerabilityID: 'CVE-2', Severity: 'HIGH', PkgName: 'pkg2', InstalledVersion: '1.0' },
              { VulnerabilityID: 'CVE-3', Severity: 'HIGH', PkgName: 'pkg3', InstalledVersion: '1.0' },
              { VulnerabilityID: 'CVE-4', Severity: 'MEDIUM', PkgName: 'pkg4', InstalledVersion: '1.0' },
              { VulnerabilityID: 'CVE-5', Severity: 'LOW', PkgName: 'pkg5', InstalledVersion: '1.0' },
            ]
          }]
        }),
        stderr: ''
      });

      const result = await tool.execute(config, { logger: mockLogger, sessionManager: mockSessionManager } as any);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.vulnerabilities).toEqual({
          critical: 1,
          high: 2,
          medium: 1,
          low: 1,
          total: 5,
        });
      }
    });
  });

  describe('Scanner Configuration', () => {
    beforeEach(() => {
      // Mock execAsync to return empty vulnerability list
      mockExecAsync.mockResolvedValue({
        stdout: JSON.stringify({
          Results: [{
            Vulnerabilities: []
          }]
        }),
        stderr: ''
      });
    });

    it('should support different scanner types', async () => {
      // Test each scanner type (tool supports trivy and grype)
      const scannerTypes: Array<'trivy' | 'grype'> = ['trivy', 'grype'];

      for (const scanner of scannerTypes) {
        config.scanner = scanner;
        const result = await tool.execute(config, { logger: mockLogger, sessionManager: mockSessionManager } as any);

        expect(result.ok).toBe(true);
        expect(result.value.scanner).toBe(scanner);
      }
    });

    it('should support different severity thresholds', async () => {
      const thresholds: Array<'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL'> = ['LOW', 'MEDIUM', 'HIGH', 'CRITICAL'];
      
      for (const threshold of thresholds) {
        config.severity = threshold;
        const result = await tool.execute(config, { logger: mockLogger, sessionManager: mockSessionManager } as any);
        
        expect(result.ok).toBe(true);
      }
    });
  });
});
