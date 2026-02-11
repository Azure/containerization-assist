/**
 * Snyk Scanner Tests
 *
 * Tests for Snyk CLI integration and vulnerability parsing
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
import { scanImageWithSnyk, checkSnykAvailability } from '@/infra/security/snyk-scanner';

describe('Snyk Scanner', () => {
  let mockLogger: Logger;
  let originalEnv: NodeJS.ProcessEnv;

  beforeEach(() => {
    // Save original environment
    originalEnv = { ...process.env };

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
    // Restore environment
    process.env = originalEnv;
    jest.clearAllMocks();
  });

  describe('checkSnykAvailability', () => {
    it('should return success with version when Snyk is available', async () => {
      // Mock successful version check (Snyk uses different version command)
      mockExecFileAsync.mockResolvedValue({
        stdout: '1.1230.0\n',
        stderr: '',
      });

      const result = await checkSnykAvailability(mockLogger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe('1.1230.0');
      }
      expect(mockExecFileAsync).toHaveBeenCalledWith('snyk', ['--version'], { timeout: 5000 });
    });

    it('should return failure when Snyk is not installed', async () => {
      // Mock command not found error
      const error: any = new Error('command not found: snyk');
      error.code = 127;
      mockExecFileAsync.mockRejectedValue(error);

      const result = await checkSnykAvailability(mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Snyk not installed');
        expect(result.guidance).toBeDefined();
        expect(result.guidance?.hint).toContain('Snyk CLI is required');
        expect(result.guidance?.resolution).toContain('https://docs.snyk.io/snyk-cli');
      }
    });

    it('should handle version check with empty output', async () => {
      mockExecFileAsync.mockResolvedValue({
        stdout: '',
        stderr: '',
      });

      const result = await checkSnykAvailability(mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('version could not be parsed');
      }
    });

    it('should handle version check timeout', async () => {
      const error: any = new Error('Timeout');
      error.code = 'ETIMEDOUT';
      mockExecFileAsync.mockRejectedValue(error);

      const result = await checkSnykAvailability(mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('version check timed out');
        expect(mockLogger.error).toHaveBeenCalledWith(
          { error: expect.any(Object) },
          'Snyk version check timed out',
        );
      }
    });
  });

  describe('scanImageWithSnyk', () => {
    it('should scan image successfully and parse vulnerabilities', async () => {
      const mockSnykOutput = {
        ok: false,
        vulnerabilities: [
          {
            id: 'SNYK-ALPINE-OPENSSL-123456',
            title: 'OpenSSL vulnerability',
            severity: 'high',
            packageName: 'openssl',
            version: '1.1.1',
            nearestFixedInVersion: '1.1.2',
            references: [{ title: 'Snyk Advisory', url: 'https://snyk.io/vuln/123456' }],
          },
          {
            id: 'SNYK-ALPINE-CURL-789012',
            title: 'Curl buffer overflow',
            severity: 'critical',
            packageName: 'curl',
            version: '7.80.0',
            nearestFixedInVersion: '7.81.0',
          },
        ],
        dependencyCount: 42,
        uniqueCount: 2,
      };

      // Mock version check
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: '1.1230.0\n',
        stderr: '',
      });
      // Mock auth check
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'api-token-here',
        stderr: '',
      });
      // Mock scan
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockSnykOutput),
        stderr: '',
      });

      const result = await scanImageWithSnyk('test-image:latest', mockLogger);

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
        expect(criticalVuln?.id).toBe('SNYK-ALPINE-CURL-789012');
        expect(criticalVuln?.package).toBe('curl');
        expect(criticalVuln?.version).toBe('7.80.0');
        expect(criticalVuln?.fixedVersion).toBe('7.81.0');
      }
    });

    it('should use SNYK_TOKEN environment variable for authentication', async () => {
      process.env.SNYK_TOKEN = 'test-token-123';

      const mockSnykOutput = {
        ok: true,
        vulnerabilities: [],
        dependencyCount: 10,
        uniqueCount: 0,
      };

      mockExecFileAsync.mockResolvedValueOnce({
        stdout: '1.1230.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockSnykOutput),
        stderr: '',
      });

      const result = await scanImageWithSnyk('test-image:latest', mockLogger);

      expect(result.ok).toBe(true);
      expect(mockLogger.debug).toHaveBeenCalledWith('SNYK_TOKEN environment variable found');
    });

    it('should handle empty scan results', async () => {
      const mockSnykOutput = {
        ok: true,
        vulnerabilities: [],
        dependencyCount: 5,
        uniqueCount: 0,
      };

      mockExecFileAsync.mockResolvedValueOnce({
        stdout: '1.1230.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'api-token',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockSnykOutput),
        stderr: '',
      });

      const result = await scanImageWithSnyk('safe-image:latest', mockLogger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.totalVulnerabilities).toBe(0);
        expect(result.value.vulnerabilities).toHaveLength(0);
      }
    });

    it('should map all severity levels correctly', async () => {
      const mockSnykOutput = {
        ok: false,
        vulnerabilities: [
          {
            id: 'CVE-CRIT',
            title: 'Critical',
            packageName: 'pkg1',
            version: '1.0',
            severity: 'critical',
          },
          {
            id: 'CVE-HIGH',
            title: 'High',
            packageName: 'pkg2',
            version: '1.0',
            severity: 'high',
          },
          {
            id: 'CVE-MED',
            title: 'Medium',
            packageName: 'pkg3',
            version: '1.0',
            severity: 'medium',
          },
          {
            id: 'CVE-LOW',
            title: 'Low',
            packageName: 'pkg4',
            version: '1.0',
            severity: 'low',
          },
        ],
        dependencyCount: 20,
        uniqueCount: 4,
      };

      mockExecFileAsync.mockResolvedValueOnce({
        stdout: '1.1230.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'api-token',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockSnykOutput),
        stderr: '',
      });

      const result = await scanImageWithSnyk('test-image:latest', mockLogger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.criticalCount).toBe(1);
        expect(result.value.highCount).toBe(1);
        expect(result.value.mediumCount).toBe(1);
        expect(result.value.lowCount).toBe(1);
        expect(result.value.totalVulnerabilities).toBe(4);
      }
    });

    it('should return failure when Snyk is not available', async () => {
      const error: any = new Error('command not found');
      error.code = 127;
      mockExecFileAsync.mockRejectedValue(error);

      const result = await scanImageWithSnyk('test-image:latest', mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Snyk not installed');
        expect(result.guidance).toBeDefined();
      }
    });

    it('should return failure when Snyk authentication is required', async () => {
      // Version check succeeds
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: '1.1230.0\n',
        stderr: '',
      });
      // Auth check fails (no token, config check fails)
      const authError: any = new Error('Not authenticated');
      mockExecFileAsync.mockRejectedValueOnce(authError);

      const result = await scanImageWithSnyk('test-image:latest', mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Snyk authentication required');
        expect(result.guidance).toBeDefined();
        expect(result.guidance?.hint).toContain('API token');
        expect(result.guidance?.resolution).toContain('SNYK_TOKEN');
        expect(result.guidance?.resolution).toContain('snyk auth');
      }
    });

    it('should handle Snyk scan execution errors', async () => {
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: '1.1230.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'api-token',
        stderr: '',
      });
      mockExecFileAsync.mockRejectedValueOnce(new Error('Image not found'));

      const result = await scanImageWithSnyk('nonexistent:latest', mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Snyk scan failed');
        expect(result.guidance).toBeDefined();
        expect(result.guidance?.hint).toContain('error while scanning');
      }
    });

    it('should handle invalid JSON output from Snyk', async () => {
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: '1.1230.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'api-token',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'Not valid JSON{{{',
        stderr: '',
      });

      const result = await scanImageWithSnyk('test-image:latest', mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Failed to parse Snyk output');
        expect(result.guidance).toBeDefined();
        expect(result.guidance?.hint).toContain('invalid JSON');
      }
    });

    it('should handle error in Snyk output object', async () => {
      const mockSnykOutput = {
        ok: false,
        error: 'Failed to pull image: authentication required',
      };

      mockExecFileAsync.mockResolvedValueOnce({
        stdout: '1.1230.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'api-token',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockSnykOutput),
        stderr: '',
      });

      const result = await scanImageWithSnyk('test-image:latest', mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Snyk scan error');
        expect(result.error).toContain('authentication required');
      }
    });

    it('should log warnings from stderr', async () => {
      const mockSnykOutput = {
        ok: true,
        vulnerabilities: [],
        dependencyCount: 0,
        uniqueCount: 0,
      };

      mockExecFileAsync.mockResolvedValueOnce({
        stdout: '1.1230.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'api-token',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockSnykOutput),
        stderr: 'Warning: Database may be outdated',
      });

      const result = await scanImageWithSnyk('test-image:latest', mockLogger);

      expect(result.ok).toBe(true);
      expect(mockLogger.debug).toHaveBeenCalledWith(
        expect.objectContaining({ stderr: expect.any(String) }),
        'Snyk stderr output',
      );
    });

    it('should handle vulnerabilities with missing optional fields', async () => {
      const mockSnykOutput = {
        ok: false,
        vulnerabilities: [
          {
            id: 'SNYK-TEST-9999',
            packageName: 'test-pkg',
            version: '1.0',
            severity: 'medium',
            // Missing: nearestFixedInVersion, title, references
          },
        ],
        dependencyCount: 1,
        uniqueCount: 1,
      };

      mockExecFileAsync.mockResolvedValueOnce({
        stdout: '1.1230.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'api-token',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockSnykOutput),
        stderr: '',
      });

      const result = await scanImageWithSnyk('test-image:latest', mockLogger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.vulnerabilities).toHaveLength(1);
        const vuln = result.value.vulnerabilities[0];
        expect(vuln.id).toBe('SNYK-TEST-9999');
        expect(vuln.fixedVersion).toBeUndefined();
        expect(vuln.description).toBe('No description available');
      }
    });

    it('should include correct args in logs', async () => {
      const mockSnykOutput = {
        ok: true,
        vulnerabilities: [],
        dependencyCount: 0,
        uniqueCount: 0,
      };

      mockExecFileAsync.mockResolvedValueOnce({
        stdout: '1.1230.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'api-token',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockSnykOutput),
        stderr: '',
      });

      await scanImageWithSnyk('test-image:latest', mockLogger);

      expect(mockLogger.debug).toHaveBeenCalledWith(
        expect.objectContaining({
          args: expect.arrayContaining(['container', 'test', 'test-image:latest']),
        }),
        'Executing Snyk command',
      );
    });

    it('should handle authentication error in error response with stdout', async () => {
      const mockSnykError = {
        error: 'Authentication failed: Invalid token',
      };

      mockExecFileAsync.mockResolvedValueOnce({
        stdout: '1.1230.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'api-token',
        stderr: '',
      });

      const error: any = new Error('Command failed');
      error.stdout = JSON.stringify(mockSnykError);
      mockExecFileAsync.mockRejectedValueOnce(error);

      const result = await scanImageWithSnyk('test-image:latest', mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Snyk scan error');
        expect(result.error).toContain('Authentication failed');
        // Guidance should provide helpful resolution
        expect(result.guidance?.resolution).toBeDefined();
      }
    });
  });

  describe('Scanner Integration', () => {
    it('should create scanner instance and perform scan', async () => {
      const { createSecurityScanner } = await import('@/infra/security/scanner');

      const mockSnykOutput = {
        ok: true,
        vulnerabilities: [],
        dependencyCount: 0,
        uniqueCount: 0,
      };

      mockExecFileAsync.mockResolvedValueOnce({
        stdout: '1.1230.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'api-token',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockSnykOutput),
        stderr: '',
      });

      const scanner = createSecurityScanner(mockLogger, 'snyk');
      const result = await scanner.scanImage('test:latest');

      expect(result.ok).toBe(true);
    });

    it('should ping scanner successfully when Snyk is available', async () => {
      const { createSecurityScanner } = await import('@/infra/security/scanner');

      mockExecFileAsync.mockResolvedValue({
        stdout: '1.1230.0\n',
        stderr: '',
      });

      const scanner = createSecurityScanner(mockLogger, 'snyk');
      const result = await scanner.ping();

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe(true);
      }
    });

    it('should fail ping when Snyk is not available', async () => {
      const { createSecurityScanner } = await import('@/infra/security/scanner');

      const error: any = new Error('command not found');
      error.code = 127;
      mockExecFileAsync.mockRejectedValue(error);

      const scanner = createSecurityScanner(mockLogger, 'snyk');
      const result = await scanner.ping();

      expect(result.ok).toBe(false);
    });
  });

  describe('ImageId Validation', () => {
    it('should reject imageId with shell metacharacters', async () => {
      mockExecFileAsync.mockResolvedValue({
        stdout: '1.1230.0\n',
        stderr: '',
      });

      const result = await scanImageWithSnyk('test-image; rm -rf /', mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Invalid imageId format');
        expect(result.guidance?.hint).toContain('ImageId must contain only alphanumeric');
      }
    });

    it('should reject imageId with quotes', async () => {
      mockExecFileAsync.mockResolvedValue({
        stdout: '1.1230.0\n',
        stderr: '',
      });

      const result = await scanImageWithSnyk('test"image', mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Invalid imageId format');
      }
    });

    it('should accept valid imageId with registry, namespace, and tag', async () => {
      const mockSnykOutput = {
        ok: true,
        vulnerabilities: [],
        dependencyCount: 0,
        uniqueCount: 0,
      };

      mockExecFileAsync.mockResolvedValueOnce({
        stdout: '1.1230.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'api-token',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockSnykOutput),
        stderr: '',
      });

      const result = await scanImageWithSnyk(
        'registry.example.com/namespace/image:v1.0.0',
        mockLogger,
      );

      expect(result.ok).toBe(true);
    });

    it('should accept valid imageId with digest', async () => {
      const mockSnykOutput = {
        ok: true,
        vulnerabilities: [],
        dependencyCount: 0,
        uniqueCount: 0,
      };

      mockExecFileAsync.mockResolvedValueOnce({
        stdout: '1.1230.0\n',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: 'api-token',
        stderr: '',
      });
      mockExecFileAsync.mockResolvedValueOnce({
        stdout: JSON.stringify(mockSnykOutput),
        stderr: '',
      });

      const result = await scanImageWithSnyk(
        'myimage@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890',
        mockLogger,
      );

      expect(result.ok).toBe(true);
    });
  });
});
