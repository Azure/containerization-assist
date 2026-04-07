/**
 * Unit Tests: Scan Image Tool - Success and Error Scenarios
 * Tests the scan-image tool behavior and edge cases
 */

import { jest } from '@jest/globals';

// Result Type Helpers for Testing
function createSuccessResult<T>(value: T) {
  return {
    ok: true as const,
    value,
  };
}

function createFailureResult(error: string, guidance?: { hint?: string; resolution?: string }) {
  return {
    ok: false as const,
    error,
    ...(guidance && { guidance }),
  };
}

function createMockLogger() {
  return {
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
    trace: jest.fn(),
    fatal: jest.fn(),
    child: jest.fn().mockReturnThis(),
  } as any;
}

// Mock security scanner
const mockSecurityScanner = {
  scanImage: jest.fn() as jest.Mock,
} as any;

const mockTimer = {
  end: jest.fn(),
  error: jest.fn(),
};

const mockGetKnowledgeForCategory = jest.fn() as any;

// Mock knowledge system
jest.mock('../../../src/knowledge/index', () => ({
  getKnowledgeForCategory: mockGetKnowledgeForCategory,
}));

// Mock infra modules
jest.mock('../../../src/infra/security/scanner', () => ({
  createSecurityScanner: jest.fn(() => mockSecurityScanner),
}));

// Mock Docker context resolution
const mockResolveDockerContext = jest.fn() as any;

jest.mock('../../../src/infra/docker/context', () => ({
  resolveDockerContext: (...args: unknown[]) => mockResolveDockerContext(...args),
}));

jest.mock('../../../src/lib/logger', () => ({
  createTimer: jest.fn(() => mockTimer),
  createLogger: jest.fn(() => createMockLogger()),
}));

jest.mock('../../../src/lib/tool-helpers', () => ({
  getToolLogger: jest.fn(() => createMockLogger()),
  createToolTimer: jest.fn(() => mockTimer),
}));

// Import these after mocks are set up
import { scanImage } from '../../../src/tools/scan-image/tool';
import type { ScanImageParams } from '../../../src/tools/scan-image/schema';
import type { ToolContext } from '../../../src/mcp/context';

// Create mock ToolContext
function createMockToolContext(): ToolContext {
  return {
    logger: createMockLogger(),
  };
}

describe('scanImage - Success and Error Scenarios', () => {
  let config: ScanImageParams;

  beforeEach(() => {
    config = {
      imageId: 'test-app:latest',
      scanner: 'trivy',
      scanType: 'vulnerability',
      enableAISuggestions: false,
      severity: 'high',
    };

    jest.clearAllMocks();

    mockGetKnowledgeForCategory.mockResolvedValue([
      {
        entry: {
          recommendation: 'Upgrade to patched version',
          severity: 'HIGH',
          example: 'npm update package-name',
        },
      },
    ]);

    // Default successful scan
    mockSecurityScanner.scanImage.mockResolvedValue(
      createSuccessResult({
        vulnerabilities: [],
        criticalCount: 0,
        highCount: 0,
        mediumCount: 0,
        lowCount: 0,
        negligibleCount: 0,
        unknownCount: 0,
        totalVulnerabilities: 0,
        scanDate: new Date(),
      }),
    );
  });

  describe('Successful Scans', () => {
    it('should successfully scan image with no vulnerabilities', async () => {
      const result = await scanImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.success).toBe(true);
        expect(result.value.passed).toBe(true);
        expect(result.value.vulnerabilities.total).toBe(0);
      }
    });

    it('should detect vulnerabilities and provide remediation guidance', async () => {
      const aiConfig = { ...config, enableAISuggestions: true };
      mockSecurityScanner.scanImage.mockResolvedValue(
        createSuccessResult({
          vulnerabilities: [
            {
              id: 'CVE-2023-1234',
              severity: 'HIGH' as const,
              package: 'openssl',
              version: '1.1.1',
              description: 'Security vulnerability',
              fixedVersion: '1.1.1k',
            },
          ],
          criticalCount: 0,
          highCount: 1,
          mediumCount: 0,
          lowCount: 0,
          negligibleCount: 0,
          unknownCount: 0,
          totalVulnerabilities: 1,
          scanDate: new Date(),
        }),
      );

      const result = await scanImage(aiConfig, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.vulnerabilities.high).toBe(1);
        expect(result.value.remediationGuidance).toBeDefined();
        expect(result.value.passed).toBe(false); // Should fail with high severity
      }
    });

    it('should skip remediation guidance when AI suggestions disabled but still provide recommendedActions', async () => {
      const noAiConfig = {
        ...config,
        enableAISuggestions: false,
      };

      mockSecurityScanner.scanImage.mockResolvedValue(
        createSuccessResult({
          vulnerabilities: [
            {
              id: 'CVE-2023-1234',
              severity: 'HIGH' as const,
              package: 'openssl',
              version: '1.1.1',
              description: 'Security vulnerability',
              fixedVersion: '1.1.1k',
            },
          ],
          criticalCount: 0,
          highCount: 1,
          mediumCount: 0,
          lowCount: 0,
          negligibleCount: 0,
          unknownCount: 0,
          totalVulnerabilities: 1,
          scanDate: new Date(),
        }),
      );

      const result = await scanImage(noAiConfig, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.remediationGuidance).toBeUndefined();
        // recommendedActions is pure computation (not AI), so it should still be present
        expect(result.value.recommendedActions).toBeDefined();
      }
    });

    it('should generate recommendedActions grouped by package/version/fix', async () => {
      mockSecurityScanner.scanImage.mockResolvedValue(
        createSuccessResult({
          vulnerabilities: [
            {
              id: 'CVE-2023-1',
              severity: 'CRITICAL' as const,
              package: 'openssl',
              version: '1.1.1',
              description: 'Critical vulnerability 1',
              fixedVersion: '1.1.1t',
            },
            {
              id: 'CVE-2023-2',
              severity: 'HIGH' as const,
              package: 'openssl',
              version: '1.1.1',
              description: 'High vulnerability 2',
              fixedVersion: '1.1.1t',
            },
            {
              id: 'CVE-2023-3',
              severity: 'MEDIUM' as const,
              package: 'curl',
              version: '7.68.0',
              description: 'Medium vulnerability',
              fixedVersion: '7.88.0',
            },
          ],
          criticalCount: 1,
          highCount: 1,
          mediumCount: 1,
          lowCount: 0,
          negligibleCount: 0,
          unknownCount: 0,
          totalVulnerabilities: 3,
          scanDate: new Date(),
        }),
      );

      const result = await scanImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.recommendedActions).toBeDefined();
        expect(result.value.recommendedActions).toHaveLength(2);

        const action1 = result.value.recommendedActions![0];
        expect(action1.action).toBe('Upgrade openssl');
        expect(action1.package).toBe('openssl');
        expect(action1.vulnerabilitiesFixed).toBe(2);
        expect(action1.severityCounts.critical).toBe(1);
        expect(action1.severityCounts.high).toBe(1);
        expect(action1.current).toBe('openssl: 1.1.1');
        expect(action1.recommended).toBe('openssl: 1.1.1t');
        expect(action1.vulnerabilityIds).toContain('CVE-2023-1');
        expect(action1.vulnerabilityIds).toContain('CVE-2023-2');

        const action2 = result.value.recommendedActions![1];
        expect(action2.action).toBe('Upgrade curl');
        expect(action2.package).toBe('curl');
        expect(action2.vulnerabilitiesFixed).toBe(1);
        expect(action2.severityCounts.medium).toBe(1);
      }
    });

    it('should not generate recommendedActions when no fixes available', async () => {
      mockSecurityScanner.scanImage.mockResolvedValue(
        createSuccessResult({
          vulnerabilities: [
            {
              id: 'CVE-2023-9999',
              severity: 'CRITICAL' as const,
              package: 'oldpkg',
              version: '1.0.0',
              description: 'No fix available',
            },
          ],
          criticalCount: 1,
          highCount: 0,
          mediumCount: 0,
          lowCount: 0,
          negligibleCount: 0,
          unknownCount: 0,
          totalVulnerabilities: 1,
          scanDate: new Date(),
        }),
      );

      const result = await scanImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.recommendedActions).toBeUndefined();
      }
    });
  });

  describe('Error Scenarios - Infrastructure', () => {
    it('should fail when Trivy scanner is not installed', async () => {
      mockSecurityScanner.scanImage.mockResolvedValue(
        createFailureResult('Trivy not found in PATH', {
          hint: 'Trivy security scanner is not installed',
          resolution:
            'Install Trivy: brew install aquasecurity/trivy/trivy or follow https://aquasecurity.github.io/trivy/latest/getting-started/installation/',
        }),
      );

      const result = await scanImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Trivy not found');
        expect(result.guidance).toBeDefined();
        expect(result.guidance?.hint).toContain('not installed');
        expect(result.guidance?.resolution).toContain('Install Trivy');
      }
    });

    it('should fail when scanner binary is not executable', async () => {
      mockSecurityScanner.scanImage.mockResolvedValue(
        createFailureResult('EACCES: permission denied', {
          hint: 'Scanner binary does not have execute permissions',
          resolution: 'Grant execute permissions: chmod +x /usr/local/bin/trivy',
        }),
      );

      const result = await scanImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('permission denied');
        expect(result.guidance).toBeDefined();
      }
    });

    it('should fail when vulnerability database cannot be downloaded', async () => {
      mockSecurityScanner.scanImage.mockResolvedValue(
        createFailureResult('Failed to download vulnerability database', {
          hint: 'Cannot update vulnerability database due to network issues',
          resolution: 'Check internet connection or use offline mode with --offline flag',
        }),
      );

      const result = await scanImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('vulnerability database');
        expect(result.guidance).toBeDefined();
      }
    });
  });

  describe('Error Scenarios - Image Issues', () => {
    it('should fail when image does not exist', async () => {
      mockSecurityScanner.scanImage.mockResolvedValue(
        createFailureResult('Image not found: nonexistent:latest', {
          hint: 'The specified image does not exist locally',
          resolution: 'Build or pull the image first: docker pull nonexistent:latest',
        }),
      );

      const result = await scanImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('not found');
        expect(result.guidance).toBeDefined();
        expect(result.guidance?.hint).toContain('does not exist');
      }
    });

    it('should fail when Docker daemon is not running', async () => {
      mockSecurityScanner.scanImage.mockResolvedValue(
        createFailureResult('Cannot connect to Docker daemon', {
          hint: 'Docker daemon must be running to scan images',
          resolution: 'Start Docker: sudo systemctl start docker or start Docker Desktop',
        }),
      );

      const result = await scanImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Docker daemon');
        expect(result.guidance).toBeDefined();
      }
    });

    it('should fail when image layer is corrupted', async () => {
      mockSecurityScanner.scanImage.mockResolvedValue(
        createFailureResult('Failed to extract image layer: checksum mismatch', {
          hint: 'Image layer is corrupted or incomplete',
          resolution: 'Re-pull the image: docker pull test-app:latest',
        }),
      );

      const result = await scanImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('checksum mismatch');
        expect(result.guidance).toBeDefined();
      }
    });
  });

  describe('Error Scenarios - Input Validation', () => {
    it('should fail with invalid parameters', async () => {
      const result = await scanImage(null as any, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Invalid parameters');
      }
    });

    it('should reject unsupported scan types', async () => {
      const invalidConfig = {
        imageId: 'test-app:latest',
        scanner: 'osv',
        scanType: 'config',
        enableAISuggestions: false,
      } as any;

      const result = await scanImage(invalidConfig, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Scan type');
      }
    });

    it('should fail when imageId is missing', async () => {
      const invalidConfig = {
        scanner: 'trivy',
        scanType: 'vulnerability',
        enableAISuggestions: false,
        severity: 'high',
      } as any;

      const result = await scanImage(invalidConfig, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('No image specified');
      }
    });

    it('should fail when imageId is empty string', async () => {
      const invalidConfig: ScanImageParams = {
        imageId: '',
        scanner: 'trivy',
        scanType: 'vulnerability',
        enableAISuggestions: false,
      };

      const result = await scanImage(invalidConfig, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('No image specified');
      }
    });
  });

  describe('Error Scenarios - Scanner Failures', () => {
    it('should handle scanner timeout', async () => {
      mockSecurityScanner.scanImage.mockResolvedValue(
        createFailureResult('Scanner timeout after 300 seconds', {
          hint: 'Image scan took too long to complete',
          resolution: 'Try scanning a smaller image or increase timeout value',
        }),
      );

      const result = await scanImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('timeout');
        expect(result.guidance).toBeDefined();
      }
    });

    it('should handle scanner crash', async () => {
      mockSecurityScanner.scanImage.mockRejectedValue(new Error('Scanner process crashed'));

      const result = await scanImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Scanner process crashed');
      }
    });

    it('should handle malformed scanner output', async () => {
      mockSecurityScanner.scanImage.mockResolvedValue(
        createFailureResult('Failed to parse scanner output', {
          hint: 'Scanner produced invalid output format',
          resolution: 'Update scanner to latest version or check scanner logs',
        }),
      );

      const result = await scanImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('parse scanner output');
        expect(result.guidance).toBeDefined();
      }
    });
  });

  describe('Severity Threshold Testing', () => {
    it('should pass when vulnerabilities are below threshold', async () => {
      mockSecurityScanner.scanImage.mockResolvedValue(
        createSuccessResult({
          vulnerabilities: [
            {
              id: 'CVE-2023-5678',
              severity: 'LOW' as const,
              package: 'test-pkg',
              version: '1.0.0',
            },
          ],
          criticalCount: 0,
          highCount: 0,
          mediumCount: 0,
          lowCount: 1,
          negligibleCount: 0,
          unknownCount: 0,
          totalVulnerabilities: 1,
          scanDate: new Date(),
        }),
      );

      config.severity = 'high'; // Only fail on high/critical

      const result = await scanImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.passed).toBe(true); // Should pass with only LOW severity
      }
    });

    it('should fail when vulnerabilities exceed threshold', async () => {
      mockSecurityScanner.scanImage.mockResolvedValue(
        createSuccessResult({
          vulnerabilities: [
            {
              id: 'CVE-2023-9999',
              severity: 'CRITICAL' as const,
              package: 'critical-pkg',
              version: '1.0.0',
            },
          ],
          criticalCount: 1,
          highCount: 0,
          mediumCount: 0,
          lowCount: 0,
          negligibleCount: 0,
          unknownCount: 0,
          totalVulnerabilities: 1,
          scanDate: new Date(),
        }),
      );

      config.severity = 'critical';

      const result = await scanImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.passed).toBe(false); // Should fail with CRITICAL
        expect(result.value.vulnerabilities.critical).toBe(1);
      }
    });
  });

  describe('Network and Registry Errors', () => {
    it('should handle registry authentication failure', async () => {
      mockSecurityScanner.scanImage.mockResolvedValue(
        createFailureResult('Failed to pull image: authentication required', {
          hint: 'Image requires authentication to access',
          resolution: 'Login to registry: docker login registry.example.com',
        }),
      );

      const result = await scanImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('authentication');
        expect(result.guidance).toBeDefined();
      }
    });

    it('should handle network timeout during database update', async () => {
      mockSecurityScanner.scanImage.mockResolvedValue(
        createFailureResult('Network timeout while updating vulnerability database', {
          hint: 'Unable to reach vulnerability database server',
          resolution: 'Check network connection and firewall settings',
        }),
      );

      const result = await scanImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Network timeout');
        expect(result.guidance).toBeDefined();
      }
    });
  });

  describe('Disk Space and Resource Errors', () => {
    it('should fail when insufficient disk space for scanning', async () => {
      mockSecurityScanner.scanImage.mockResolvedValue(
        createFailureResult('No space left on device', {
          hint: 'Insufficient disk space to extract and scan image',
          resolution: 'Free up disk space: docker system prune or delete unused files',
        }),
      );

      const result = await scanImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('No space left');
        expect(result.guidance).toBeDefined();
      }
    });

    it('should handle out of memory errors', async () => {
      mockSecurityScanner.scanImage.mockResolvedValue(
        createFailureResult('Out of memory during scan', {
          hint: 'Scanner ran out of memory while processing image',
          resolution: 'Increase available memory or scan a smaller image',
        }),
      );

      const result = await scanImage(config, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Out of memory');
        expect(result.guidance).toBeDefined();
      }
    });
  });

  describe('Docker Context Support', () => {
    describe('Specific Context Scanning', () => {
      it('should resolve context and scan with dockerHost', async () => {
        mockResolveDockerContext.mockResolvedValue(
          createSuccessResult('unix:///Users/user/.colima/default/docker.sock'),
        );

        const contextConfig: ScanImageParams = {
          ...config,
          context: 'colima',
        };

        const result = await scanImage(contextConfig, createMockToolContext());

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.success).toBe(true);
          expect(result.value.context).toBe('colima');
        }
        expect(mockResolveDockerContext).toHaveBeenCalledWith('colima', expect.anything());
      });

      it('should fail when context resolution fails', async () => {
        mockResolveDockerContext.mockResolvedValue(
          createFailureResult("Failed to resolve Docker context 'nonexistent'", {
            hint: 'The context name may be incorrect',
            resolution: 'Run "docker context ls" to see available contexts.',
          }),
        );

        const contextConfig: ScanImageParams = {
          ...config,
          context: 'nonexistent',
        };

        const result = await scanImage(contextConfig, createMockToolContext());

        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('nonexistent');
        }
      });

      it('should not resolve context when none specified', async () => {
        const result = await scanImage(config, createMockToolContext());

        expect(result.ok).toBe(true);
        expect(mockResolveDockerContext).not.toHaveBeenCalled();
        if (result.ok) {
          expect(result.value.context).toBeUndefined();
        }
      });
    });
  });
});
