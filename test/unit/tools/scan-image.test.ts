/**
 * Unit Tests: Scan Image Tool - Error Scenarios
 * Tests the scan-image tool error handling and edge cases
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
  scanImage: jest.fn(),
};

const mockTimer = {
  end: jest.fn(),
  error: jest.fn(),
};

// Mock knowledge system
jest.mock('../../../src/knowledge/index', () => ({
  getKnowledgeForCategory: jest.fn().mockResolvedValue([
    {
      entry: {
        recommendation: 'Upgrade to patched version',
        severity: 'HIGH',
        example: 'npm update package-name',
      },
    },
  ]),
}));

// Mock infra modules
jest.mock('../../../src/infra/security/scanner', () => ({
  createSecurityScanner: jest.fn(() => mockSecurityScanner),
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
import type { ToolContext } from '@/mcp/context';

// Create mock ToolContext
function createMockToolContext(): ToolContext {
  return {
    logger: createMockLogger(),
  };
}

describe('scanImage - Error Scenarios', () => {
  let config: ScanImageParams;

  beforeEach(() => {
    config = {
      imageId: 'test-app:latest',
      scanner: 'trivy',
      severity: 'high',
    };

    jest.clearAllMocks();

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

      const result = await scanImage(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.vulnerabilities.high).toBe(1);
        expect(result.value.remediationGuidance).toBeDefined();
        expect(result.value.passed).toBe(false); // Should fail with high severity
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

    it('should fail when imageId is missing', async () => {
      const invalidConfig = {
        scanner: 'trivy',
        severity: 'high',
      } as any;

      const result = await scanImage(invalidConfig, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('No image specified');
      }
    });

    it('should fail when imageId is empty string', async () => {
      const invalidConfig = {
        imageId: '',
        scanner: 'trivy',
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
});
