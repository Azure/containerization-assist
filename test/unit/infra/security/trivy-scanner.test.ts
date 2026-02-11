/**
 * Trivy Scanner Tests
 *
 * Tests for Trivy CLI integration and vulnerability parsing
 */

import { describe, it, expect, jest, beforeEach, afterEach } from '@jest/globals';
import type { Logger } from 'pino';

// Create module-level mocks that will be accessed by the mock factory
let mockExecAsync: jest.MockedFunction<
  (command: string, options?: unknown) => Promise<{ stdout: string; stderr: string }>
>;
let mockExecFileAsync: jest.MockedFunction<
  (
    file: string,
    args: string[],
    options?: unknown,
  ) => Promise<{ stdout: string; stderr: string }>
>;

// Store promisified function references
const promisifiedFunctions = new Map<Function, 'exec' | 'execFile'>();

// Mock node:child_process
jest.mock('node:child_process', () => {
  const mockExecFn = jest.fn();
  const mockExecFileFn = jest.fn();
  promisifiedFunctions.set(mockExecFn, 'exec');
  promisifiedFunctions.set(mockExecFileFn, 'execFile');
  return {
    exec: mockExecFn,
    execFile: mockExecFileFn,
  };
});

// Mock node:util to return our mocks
jest.mock('node:util', () => {
  const actual = jest.requireActual<typeof import('node:util')>('node:util');
  return {
    ...actual,
    promisify: (fn: Function) => {
      const fnType = promisifiedFunctions.get(fn);
      // Return wrapper that delegates to our module-level mocks
      return (...args: unknown[]) => {
        // Check if this is execFile or exec based on what was promisified
        if (fnType === 'execFile') {
          if (!mockExecFileAsync) {
            throw new Error('mockExecFileAsync not initialized');
          }
          return mockExecFileAsync(...(args as [string, string[], unknown?]));
        } else {
          if (!mockExecAsync) {
            throw new Error('mockExecAsync not initialized');
          }
          return mockExecAsync(...(args as [string, unknown?]));
        }
      };
    },
  };
});

// Now import after mocks are set up
import { scanImageWithTrivy, checkTrivyAvailability } from '@/infra/security/trivy-scanner';

describe('Trivy Scanner', () => {
  let mockLogger: Logger;

  beforeEach(() => {
    // Initialize the mock functions
    mockExecAsync = jest.fn<
      (command: string, options?: unknown) => Promise<{ stdout: string; stderr: string }>
    >();
    mockExecFileAsync = jest.fn<
      (
        file: string,
        args: string[],
        options?: unknown,
      ) => Promise<{ stdout: string; stderr: string }>
    >();

    // Create a mock logger
    mockLogger = {
      info: jest.fn(),
      debug: jest.fn(),
      warn: jest.fn(),
      error: jest.fn(),
    } as unknown as Logger;

    // Clear all mocks
    jest.clearAllMocks();
  });

  afterEach(() => {
    // Use clearAllMocks to preserve mock implementations (resetAllMocks removes them)
    jest.clearAllMocks();
  });

  describe('checkTrivyAvailability', () => {
    it('should return success with version when Trivy is available', async () => {
      // Mock successful version check
      mockExecAsync.mockResolvedValue({
        stdout: 'Version: 0.48.0\n',
        stderr: '',
      });

      const result = await checkTrivyAvailability(mockLogger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe('0.48.0');
      }
    });

    it('should return failure when Trivy is not installed', async () => {
      // Mock command not found error
      const error: any = new Error('command not found: trivy');
      error.code = 127;
      mockExecAsync.mockRejectedValue(error);

      const result = await checkTrivyAvailability(mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Trivy not installed');
        expect(result.guidance).toBeDefined();
        expect(result.guidance?.hint).toContain('Trivy CLI is required');
        expect(result.guidance?.resolution).toContain('https://aquasecurity.github.io/trivy');
      }
    });

    it('should handle version check with no version match', async () => {
      // Mock version output without proper format
      mockExecAsync.mockResolvedValue({
        stdout: 'Some other output\n',
        stderr: '',
      });

      const result = await checkTrivyAvailability(mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('version could not be parsed');
      }
    });
  });

  describe('scanImageWithTrivy', () => {
    it('should scan image successfully and parse vulnerabilities', async () => {
      const mockTrivyOutput = {
        SchemaVersion: 2,
        ArtifactName: 'test-image:latest',
        ArtifactType: 'container_image',
        Results: [
          {
            Target: 'test-image:latest (alpine 3.18.0)',
            Class: 'os-pkgs',
            Type: 'alpine',
            Vulnerabilities: [
              {
                VulnerabilityID: 'CVE-2023-1234',
                PkgName: 'openssl',
                InstalledVersion: '1.1.1',
                FixedVersion: '1.1.2',
                Severity: 'HIGH',
                Title: 'OpenSSL vulnerability',
                Description: 'A security issue in OpenSSL',
                References: ['https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2023-1234'],
              },
              {
                VulnerabilityID: 'CVE-2023-5678',
                PkgName: 'curl',
                InstalledVersion: '7.80.0',
                FixedVersion: '7.81.0',
                Severity: 'CRITICAL',
                Title: 'Curl buffer overflow',
                Description: 'Buffer overflow in curl',
              },
            ],
          },
        ],
      };

      // Mock version check (uses execAsync) and scan (uses execFileAsync)
      mockExecAsync.mockResolvedValueOnce({
        stdout: 'Version: 0.48.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockTrivyOutput),
        stderr: '',
      });

      const result = await scanImageWithTrivy('test-image:latest', mockLogger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.imageId).toBe('test-image:latest');
        expect(result.value.totalVulnerabilities).toBe(2);
        expect(result.value.criticalCount).toBe(1);
        expect(result.value.highCount).toBe(1);
        expect(result.value.mediumCount).toBe(0);
        expect(result.value.lowCount).toBe(0);
        expect(result.value.negligibleCount).toBe(0);
        expect(result.value.unknownCount).toBe(0);
        expect(result.value.vulnerabilities).toHaveLength(2);

        // Check first vulnerability
        const criticalVuln = result.value.vulnerabilities.find((v) => v.severity === 'CRITICAL');
        expect(criticalVuln).toBeDefined();
        expect(criticalVuln?.id).toBe('CVE-2023-5678');
        expect(criticalVuln?.package).toBe('curl');
        expect(criticalVuln?.version).toBe('7.80.0');
        expect(criticalVuln?.fixedVersion).toBe('7.81.0');
      }
    });

    it('should handle empty scan results', async () => {
      const mockTrivyOutput = {
        SchemaVersion: 2,
        ArtifactName: 'safe-image:latest',
        ArtifactType: 'container_image',
        Results: [],
      };

      mockExecAsync.mockResolvedValueOnce({
        stdout: 'Version: 0.48.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockTrivyOutput),
        stderr: '',
      });

      const result = await scanImageWithTrivy('safe-image:latest', mockLogger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.totalVulnerabilities).toBe(0);
        expect(result.value.vulnerabilities).toHaveLength(0);
      }
    });

    it('should map all severity levels correctly', async () => {
      const mockTrivyOutput = {
        SchemaVersion: 2,
        ArtifactName: 'test-image:latest',
        ArtifactType: 'container_image',
        Results: [
          {
            Target: 'test-image:latest',
            Class: 'os-pkgs',
            Type: 'alpine',
            Vulnerabilities: [
              {
                VulnerabilityID: 'CVE-CRIT',
                PkgName: 'pkg1',
                InstalledVersion: '1.0',
                Severity: 'CRITICAL',
                Description: 'Critical issue',
              },
              {
                VulnerabilityID: 'CVE-HIGH',
                PkgName: 'pkg2',
                InstalledVersion: '1.0',
                Severity: 'HIGH',
                Description: 'High issue',
              },
              {
                VulnerabilityID: 'CVE-MED',
                PkgName: 'pkg3',
                InstalledVersion: '1.0',
                Severity: 'MEDIUM',
                Description: 'Medium issue',
              },
              {
                VulnerabilityID: 'CVE-LOW',
                PkgName: 'pkg4',
                InstalledVersion: '1.0',
                Severity: 'LOW',
                Description: 'Low issue',
              },
              {
                VulnerabilityID: 'CVE-NEGL',
                PkgName: 'pkg5',
                InstalledVersion: '1.0',
                Severity: 'NEGLIGIBLE',
                Description: 'Negligible issue',
              },
              {
                VulnerabilityID: 'CVE-UNK',
                PkgName: 'pkg6',
                InstalledVersion: '1.0',
                Severity: 'UNKNOWN',
                Description: 'Unknown severity',
              },
            ],
          },
        ],
      };

      mockExecAsync.mockResolvedValueOnce({
        stdout: 'Version: 0.48.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockTrivyOutput),
        stderr: '',
      });

      const result = await scanImageWithTrivy('test-image:latest', mockLogger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.criticalCount).toBe(1);
        expect(result.value.highCount).toBe(1);
        expect(result.value.mediumCount).toBe(1);
        expect(result.value.lowCount).toBe(1);
        expect(result.value.negligibleCount).toBe(1);
        expect(result.value.unknownCount).toBe(1);
        expect(result.value.totalVulnerabilities).toBe(6);
      }
    });

    it('should return failure when Trivy is not available', async () => {
      const error: any = new Error('command not found');
      error.code = 127;
      mockExecAsync.mockRejectedValue(error);

      const result = await scanImageWithTrivy('test-image:latest', mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Trivy not installed');
        expect(result.guidance).toBeDefined();
      }
    });

    it('should handle Trivy scan execution errors', async () => {
      mockExecAsync.mockResolvedValueOnce({
        stdout: 'Version: 0.48.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockRejectedValueOnce(new Error('Image not found'));

      const result = await scanImageWithTrivy('nonexistent:latest', mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Trivy scan failed');
        expect(result.guidance).toBeDefined();
        expect(result.guidance?.hint).toContain('error while scanning');
      }
    });

    it('should handle invalid JSON output from Trivy', async () => {
      mockExecAsync.mockResolvedValueOnce({
        stdout: 'Version: 0.48.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'Not valid JSON{{{',
        stderr: '',
      });

      const result = await scanImageWithTrivy('test-image:latest', mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Failed to parse Trivy output');
        expect(result.guidance).toBeDefined();
        expect(result.guidance?.hint).toContain('invalid JSON');
      }
    });

    it('should log warnings from stderr', async () => {
      const mockTrivyOutput = {
        SchemaVersion: 2,
        ArtifactName: 'test-image:latest',
        ArtifactType: 'container_image',
        Results: [],
      };

      mockExecAsync.mockResolvedValueOnce({
        stdout: 'Version: 0.48.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockTrivyOutput),
        stderr: 'Warning: Database may be outdated',
      });

      const result = await scanImageWithTrivy('test-image:latest', mockLogger);

      expect(result.ok).toBe(true);
      expect(mockLogger.debug).toHaveBeenCalledWith(
        expect.objectContaining({ stderr: expect.any(String) }),
        'Trivy stderr output',
      );
    });

    it('should handle vulnerabilities with missing optional fields', async () => {
      const mockTrivyOutput = {
        SchemaVersion: 2,
        ArtifactName: 'test-image:latest',
        ArtifactType: 'container_image',
        Results: [
          {
            Target: 'test-image:latest',
            Class: 'os-pkgs',
            Type: 'alpine',
            Vulnerabilities: [
              {
                VulnerabilityID: 'CVE-2023-9999',
                PkgName: 'test-pkg',
                InstalledVersion: '1.0',
                Severity: 'MEDIUM',
                // Missing: FixedVersion, Title, Description, References
              },
            ],
          },
        ],
      };

      mockExecAsync.mockResolvedValueOnce({
        stdout: 'Version: 0.48.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockTrivyOutput),
        stderr: '',
      });

      const result = await scanImageWithTrivy('test-image:latest', mockLogger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.vulnerabilities).toHaveLength(1);
        const vuln = result.value.vulnerabilities[0];
        expect(vuln.id).toBe('CVE-2023-9999');
        expect(vuln.fixedVersion).toBeUndefined();
        expect(vuln.description).toBe('No description available');
      }
    });

    it('should include correct args in logs', async () => {
      const mockTrivyOutput = {
        SchemaVersion: 2,
        ArtifactName: 'test-image:latest',
        ArtifactType: 'container_image',
        Results: [],
      };

      mockExecAsync.mockResolvedValueOnce({
        stdout: 'Version: 0.48.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockTrivyOutput),
        stderr: '',
      });

      await scanImageWithTrivy('test-image:latest', mockLogger);

      expect(mockLogger.debug).toHaveBeenCalledWith(
        expect.objectContaining({ args: expect.arrayContaining(['image', 'test-image:latest']) }),
        'Executing Trivy command',
      );
    });
  });

  describe('Scanner Integration', () => {
    it('should create scanner instance and perform scan', async () => {
      const { createSecurityScanner } = await import('@/infra/security/scanner');

      const mockTrivyOutput = {
        SchemaVersion: 2,
        ArtifactName: 'test:latest',
        ArtifactType: 'container_image',
        Results: [],
      };

      mockExecAsync.mockResolvedValueOnce({
        stdout: 'Version: 0.48.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockTrivyOutput),
        stderr: '',
      });

      const scanner = createSecurityScanner(mockLogger, 'trivy');
      const result = await scanner.scanImage('test:latest');

      expect(result.ok).toBe(true);
    });

    it('should ping scanner successfully when Trivy is available', async () => {
      const { createSecurityScanner } = await import('@/infra/security/scanner');

      mockExecAsync.mockResolvedValue({
        stdout: 'Version: 0.48.0\n',
        stderr: '',
      });

      const scanner = createSecurityScanner(mockLogger, 'trivy');
      const result = await scanner.ping();

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe(true);
      }
    });

    it('should fail ping when Trivy is not available', async () => {
      const { createSecurityScanner } = await import('@/infra/security/scanner');

      const error: any = new Error('command not found');
      error.code = 127;
      mockExecAsync.mockRejectedValue(error);

      const scanner = createSecurityScanner(mockLogger, 'trivy');
      const result = await scanner.ping();

      expect(result.ok).toBe(false);
    });
  });

  describe('ImageId Validation', () => {
    it('should reject imageId with shell metacharacters', async () => {
      mockExecAsync.mockResolvedValue({
        stdout: 'Version: 0.48.0\n',
        stderr: '',
      });

      const result = await scanImageWithTrivy('test-image; rm -rf /', mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Invalid imageId format');
        expect(result.guidance?.hint).toContain('ImageId must contain only alphanumeric');
      }
    });

    it('should reject imageId with quotes', async () => {
      mockExecAsync.mockResolvedValue({
        stdout: 'Version: 0.48.0\n',
        stderr: '',
      });

      const result = await scanImageWithTrivy('test"image', mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Invalid imageId format');
      }
    });

    it('should accept valid imageId with registry, namespace, and tag', async () => {
      const mockTrivyOutput = {
        SchemaVersion: 2,
        ArtifactName: 'registry.example.com/namespace/image:v1.0.0',
        ArtifactType: 'container_image',
        Results: [],
      };

      mockExecAsync.mockResolvedValueOnce({
        stdout: 'Version: 0.48.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockTrivyOutput),
        stderr: '',
      });

      const result = await scanImageWithTrivy(
        'registry.example.com/namespace/image:v1.0.0',
        mockLogger,
      );

      expect(result.ok).toBe(true);
    });

    it('should accept valid imageId with digest', async () => {
      const mockTrivyOutput = {
        SchemaVersion: 2,
        ArtifactName:
          'myimage@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890',
        ArtifactType: 'container_image',
        Results: [],
      };

      mockExecAsync.mockResolvedValueOnce({
        stdout: 'Version: 0.48.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockTrivyOutput),
        stderr: '',
      });

      const result = await scanImageWithTrivy(
        'myimage@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890',
        mockLogger,
      );

      expect(result.ok).toBe(true);
    });
  });
});
