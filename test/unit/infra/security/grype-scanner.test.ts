/**
 * Grype Scanner Tests
 *
 * Tests for Grype CLI integration and vulnerability parsing
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
import { scanImageWithGrype, checkGrypeAvailability } from '@/infra/security/grype-scanner';

describe('Grype Scanner', () => {
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
    jest.clearAllMocks();
  });

  describe('checkGrypeAvailability', () => {
    it('should return success with version when Grype is available', async () => {
      // Mock successful version check
      mockExecFileAsync.mockResolvedValue({
        stdout: 'Application:        grype\nVersion:            0.74.0\nBuildDate:          2024-01-01\n',
        stderr: '',
      });

      const result = await checkGrypeAvailability(mockLogger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe('0.74.0');
      }
      expect(mockExecFileAsync).toHaveBeenCalledWith('grype', ['version'], { timeout: 5000 });
    });

    it('should return failure when Grype is not installed', async () => {
      // Mock command not found error
      const error: any = new Error('command not found: grype');
      error.code = 127;
      mockExecFileAsync.mockRejectedValue(error);

      const result = await checkGrypeAvailability(mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Grype not installed');
        expect(result.guidance).toBeDefined();
        expect(result.guidance?.hint).toContain('Grype CLI is required');
        expect(result.guidance?.resolution).toContain('https://github.com/anchore/grype');
      }
    });

    it('should handle version check with no version match', async () => {
      mockExecFileAsync.mockResolvedValue({
        stdout: 'Some other output\n',
        stderr: '',
      });

      const result = await checkGrypeAvailability(mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('version could not be parsed');
      }
    });

    it('should handle version check timeout', async () => {
      const error: any = new Error('Timeout');
      error.code = 'ETIMEDOUT';
      mockExecFileAsync.mockRejectedValue(error);

      const result = await checkGrypeAvailability(mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('version check timed out');
        expect(mockLogger.error).toHaveBeenCalledWith(
          { error: expect.any(Object) },
          'Grype version check timed out',
        );
      }
    });
  });

  describe('scanImageWithGrype', () => {
    it('should scan image successfully and parse vulnerabilities', async () => {
      const mockGrypeOutput = {
        matches: [
          {
            vulnerability: {
              id: 'CVE-2023-1234',
              severity: 'High',
              description: 'OpenSSL vulnerability',
              fix: {
                versions: ['1.1.2'],
                state: 'fixed',
              },
            },
            artifact: {
              name: 'openssl',
              version: '1.1.1',
              type: 'apk',
            },
          },
          {
            vulnerability: {
              id: 'CVE-2023-5678',
              severity: 'Critical',
              description: 'Curl buffer overflow',
              fix: {
                versions: ['7.81.0'],
                state: 'fixed',
              },
            },
            artifact: {
              name: 'curl',
              version: '7.80.0',
              type: 'apk',
            },
          },
        ],
        source: {
          type: 'image',
          target: {
            userInput: 'test-image:latest',
          },
        },
      };

      // Mock version check
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'Version:            0.74.0\n',
        stderr: '',
      });
      // Mock scan
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockGrypeOutput),
        stderr: '',
      });

      const result = await scanImageWithGrype('test-image:latest', mockLogger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.imageId).toBe('test-image:latest');
        expect(result.value.totalVulnerabilities).toBe(2);
        expect(result.value.criticalCount).toBe(1);
        expect(result.value.highCount).toBe(1);
        expect(result.value.mediumCount).toBe(0);
        expect(result.value.lowCount).toBe(0);
        expect(result.value.vulnerabilities).toHaveLength(2);

        // Check first vulnerability
        const criticalVuln = result.value.vulnerabilities.find(
          (v: { severity: string }) => v.severity === 'CRITICAL',
        );
        expect(criticalVuln).toBeDefined();
        expect(criticalVuln?.id).toBe('CVE-2023-5678');
        expect(criticalVuln?.package).toBe('curl');
        expect(criticalVuln?.version).toBe('7.80.0');
        expect(criticalVuln?.fixedVersion).toBe('7.81.0');
      }
    });

    it('should handle empty scan results', async () => {
      const mockGrypeOutput = {
        matches: [],
        source: {
          type: 'image',
          target: {
            userInput: 'safe-image:latest',
          },
        },
      };

      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'Version:            0.74.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockGrypeOutput),
        stderr: '',
      });

      const result = await scanImageWithGrype('safe-image:latest', mockLogger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.totalVulnerabilities).toBe(0);
        expect(result.value.vulnerabilities).toHaveLength(0);
      }
    });

    it('should map all severity levels correctly', async () => {
      const mockGrypeOutput = {
        matches: [
          {
            vulnerability: {
              id: 'CVE-CRIT',
              severity: 'Critical',
              description: 'Critical issue',
            },
            artifact: { name: 'pkg1', version: '1.0' },
          },
          {
            vulnerability: {
              id: 'CVE-HIGH',
              severity: 'High',
              description: 'High issue',
            },
            artifact: { name: 'pkg2', version: '1.0' },
          },
          {
            vulnerability: {
              id: 'CVE-MED',
              severity: 'Medium',
              description: 'Medium issue',
            },
            artifact: { name: 'pkg3', version: '1.0' },
          },
          {
            vulnerability: {
              id: 'CVE-LOW',
              severity: 'Low',
              description: 'Low issue',
            },
            artifact: { name: 'pkg4', version: '1.0' },
          },
          {
            vulnerability: {
              id: 'CVE-NEGL',
              severity: 'Negligible',
              description: 'Negligible issue',
            },
            artifact: { name: 'pkg5', version: '1.0' },
          },
          {
            vulnerability: {
              id: 'CVE-UNK',
              severity: 'Unknown',
              description: 'Unknown severity',
            },
            artifact: { name: 'pkg6', version: '1.0' },
          },
        ],
      };

      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'Version:            0.74.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockGrypeOutput),
        stderr: '',
      });

      const result = await scanImageWithGrype('test-image:latest', mockLogger);

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

    it('should return failure when Grype is not available', async () => {
      const error: any = new Error('command not found');
      error.code = 127;
      mockExecFileAsync.mockRejectedValue(error);

      const result = await scanImageWithGrype('test-image:latest', mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Grype not installed');
        expect(result.guidance).toBeDefined();
      }
    });

    it('should handle Grype scan execution errors', async () => {
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'Version:            0.74.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockRejectedValueOnce(new Error('Image not found'));

      const result = await scanImageWithGrype('nonexistent:latest', mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Grype scan failed');
        expect(result.guidance).toBeDefined();
        expect(result.guidance?.hint).toContain('error while scanning');
      }
    });

    it('should handle invalid JSON output from Grype', async () => {
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'Version:            0.74.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'Not valid JSON{{{',
        stderr: '',
      });

      const result = await scanImageWithGrype('test-image:latest', mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Failed to parse Grype output');
        expect(result.guidance).toBeDefined();
        expect(result.guidance?.hint).toContain('invalid JSON');
      }
    });

    it('should log warnings from stderr', async () => {
      const mockGrypeOutput = {
        matches: [],
      };

      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'Version:            0.74.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockGrypeOutput),
        stderr: 'Warning: Database may be outdated',
      });

      const result = await scanImageWithGrype('test-image:latest', mockLogger);

      expect(result.ok).toBe(true);
      expect(mockLogger.debug).toHaveBeenCalledWith(
        expect.objectContaining({ stderr: expect.any(String) }),
        'Grype stderr output',
      );
    });

    it('should handle vulnerabilities with missing optional fields', async () => {
      const mockGrypeOutput = {
        matches: [
          {
            vulnerability: {
              id: 'CVE-2023-9999',
              severity: 'Medium',
              // Missing: description, fix
            },
            artifact: {
              name: 'test-pkg',
              version: '1.0',
            },
          },
        ],
      };

      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'Version:            0.74.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockGrypeOutput),
        stderr: '',
      });

      const result = await scanImageWithGrype('test-image:latest', mockLogger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.vulnerabilities).toHaveLength(1);
        const vuln = result.value.vulnerabilities[0];
        expect(vuln.id).toBe('CVE-2023-9999');
        expect(vuln.fixedVersion).toBeUndefined();
        expect(vuln.description).toBe('No description available');
      }
    });

    it('should handle fix with not-fixed state', async () => {
      const mockGrypeOutput = {
        matches: [
          {
            vulnerability: {
              id: 'CVE-2023-9999',
              severity: 'Medium',
              description: 'Test vuln',
              fix: {
                versions: [],
                state: 'not-fixed',
              },
            },
            artifact: {
              name: 'test-pkg',
              version: '1.0',
            },
          },
        ],
      };

      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'Version:            0.74.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockGrypeOutput),
        stderr: '',
      });

      const result = await scanImageWithGrype('test-image:latest', mockLogger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.vulnerabilities).toHaveLength(1);
        const vuln = result.value.vulnerabilities[0];
        expect(vuln.fixedVersion).toBeUndefined();
      }
    });

    it('should include correct args in logs', async () => {
      const mockGrypeOutput = {
        matches: [],
      };

      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'Version:            0.74.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockGrypeOutput),
        stderr: '',
      });

      await scanImageWithGrype('test-image:latest', mockLogger);

      expect(mockLogger.debug).toHaveBeenCalledWith(
        expect.objectContaining({
          args: expect.arrayContaining(['test-image:latest', '-o', 'json']),
        }),
        'Executing Grype command',
      );
    });
  });

  describe('Scanner Integration', () => {
    it('should create scanner instance and perform scan', async () => {
      const { createSecurityScanner } = await import('@/infra/security/scanner');

      const mockGrypeOutput = {
        matches: [],
      };

      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'Version:            0.74.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockGrypeOutput),
        stderr: '',
      });

      const scanner = createSecurityScanner(mockLogger, 'grype');
      const result = await scanner.scanImage('test:latest');

      expect(result.ok).toBe(true);
    });

    it('should ping scanner successfully when Grype is available', async () => {
      const { createSecurityScanner } = await import('@/infra/security/scanner');

      mockExecFileAsync.mockResolvedValue({
        stdout: 'Version:            0.74.0\n',
        stderr: '',
      });

      const scanner = createSecurityScanner(mockLogger, 'grype');
      const result = await scanner.ping();

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe(true);
      }
    });

    it('should fail ping when Grype is not available', async () => {
      const { createSecurityScanner } = await import('@/infra/security/scanner');

      const error: any = new Error('command not found');
      error.code = 127;
      mockExecFileAsync.mockRejectedValue(error);

      const scanner = createSecurityScanner(mockLogger, 'grype');
      const result = await scanner.ping();

      expect(result.ok).toBe(false);
    });
  });

  describe('ImageId Validation', () => {
    it('should reject imageId with shell metacharacters', async () => {
      mockExecFileAsync.mockResolvedValue({
        stdout: 'Version:            0.74.0\n',
        stderr: '',
      });

      const result = await scanImageWithGrype('test-image; rm -rf /', mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Invalid imageId format');
        expect(result.guidance?.hint).toContain('ImageId must contain only alphanumeric');
      }
    });

    it('should reject imageId with quotes', async () => {
      mockExecFileAsync.mockResolvedValue({
        stdout: 'Version:            0.74.0\n',
        stderr: '',
      });

      const result = await scanImageWithGrype('test"image', mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Invalid imageId format');
      }
    });

    it('should accept valid imageId with registry, namespace, and tag', async () => {
      const mockGrypeOutput = {
        matches: [],
      };

      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'Version:            0.74.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockGrypeOutput),
        stderr: '',
      });

      const result = await scanImageWithGrype(
        'registry.example.com/namespace/image:v1.0.0',
        mockLogger,
      );

      expect(result.ok).toBe(true);
    });

    it('should accept valid imageId with digest', async () => {
      const mockGrypeOutput = {
        matches: [],
      };

      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'Version:            0.74.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockGrypeOutput),
        stderr: '',
      });

      const result = await scanImageWithGrype(
        'myimage@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890',
        mockLogger,
      );

      expect(result.ok).toBe(true);
    });
  });
});
