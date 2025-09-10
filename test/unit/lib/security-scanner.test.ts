import { describe, it, expect, jest, beforeEach, afterEach } from '@jest/globals';
import { SecurityScanner, ScanResult, ScanOptions, VulnerabilityFinding } from '../../../src/lib/security-scanner';
import { Result, Success, Failure } from '../../../src/domain/types';
import type { Logger } from 'pino';

// Mock the logger
const mockLogger: Logger = {
  info: jest.fn(),
  warn: jest.fn(),
  error: jest.fn(),
  debug: jest.fn(),
  trace: jest.fn(),
  fatal: jest.fn(),
  child: jest.fn(() => mockLogger)
} as any;

describe('SecurityScanner', () => {
  let scanner: SecurityScanner;
  let mockCommandExecutor: jest.Mocked<any>;

  beforeEach(() => {
    mockCommandExecutor = {
      execute: jest.fn(),
      executeStreaming: jest.fn()
    };

    scanner = new SecurityScanner(mockCommandExecutor, mockLogger);
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('scanImage', () => {
    it('should scan Docker image for vulnerabilities successfully', async () => {
      const mockScanOutput = `{
        "Results": [
          {
            "Target": "test-image:latest",
            "Vulnerabilities": [
              {
                "VulnerabilityID": "CVE-2023-1234",
                "Severity": "HIGH",
                "PkgName": "libssl",
                "Title": "SSL vulnerability",
                "Description": "A critical SSL vulnerability",
                "FixedVersion": "1.2.3"
              }
            ]
          }
        ]
      }`;

      mockCommandExecutor.execute.mockResolvedValue(
        Success({ stdout: mockScanOutput, stderr: '', exitCode: 0 })
      );

      const result = await scanner.scanImage('test-image:latest');

      expect(result.ok).toBe(true);
      expect(result.value.vulnerabilities).toHaveLength(1);
      expect(result.value.vulnerabilities[0].id).toBe('CVE-2023-1234');
      expect(result.value.vulnerabilities[0].severity).toBe('HIGH');
      expect(result.value.summary.high).toBe(1);
      expect(mockCommandExecutor.execute).toHaveBeenCalledWith(
        'trivy',
        ['image', '--format', 'json', 'test-image:latest'],
        expect.any(Object)
      );
    });

    it('should handle scan with no vulnerabilities', async () => {
      const mockScanOutput = `{
        "Results": [
          {
            "Target": "test-image:latest",
            "Vulnerabilities": null
          }
        ]
      }`;

      mockCommandExecutor.execute.mockResolvedValue(
        Success({ stdout: mockScanOutput, stderr: '', exitCode: 0 })
      );

      const result = await scanner.scanImage('test-image:latest');

      expect(result.ok).toBe(true);
      expect(result.value.vulnerabilities).toHaveLength(0);
      expect(result.value.summary.total).toBe(0);
      expect(result.value.passed).toBe(true);
    });

    it('should handle scanner command failure', async () => {
      mockCommandExecutor.execute.mockResolvedValue(
        Failure('Trivy command failed')
      );

      const result = await scanner.scanImage('test-image:latest');

      expect(result.ok).toBe(false);
      expect(result.error).toContain('Security scan failed');
    });

    it('should handle invalid JSON output', async () => {
      mockCommandExecutor.execute.mockResolvedValue(
        Success({ stdout: 'invalid json', stderr: '', exitCode: 0 })
      );

      const result = await scanner.scanImage('test-image:latest');

      expect(result.ok).toBe(false);
      expect(result.error).toContain('Failed to parse scan results');
    });

    it('should apply severity filters correctly', async () => {
      const mockScanOutput = `{
        "Results": [
          {
            "Target": "test-image:latest",
            "Vulnerabilities": [
              {
                "VulnerabilityID": "CVE-2023-1234",
                "Severity": "HIGH",
                "PkgName": "libssl"
              }
            ]
          }
        ]
      }`;

      mockCommandExecutor.execute.mockResolvedValue(
        Success({ stdout: mockScanOutput, stderr: '', exitCode: 0 })
      );

      const options: ScanOptions = {
        minSeverity: 'MEDIUM',
        skipUnfixed: false,
        timeout: 300000
      };

      const result = await scanner.scanImage('test-image:latest', options);

      expect(result.ok).toBe(true);
      expect(result.value.vulnerabilities).toHaveLength(1);
      expect(result.value.vulnerabilities[0].severity).toBe('HIGH');
    });

    it('should respect timeout option', async () => {
      mockCommandExecutor.execute.mockImplementation(() => 
        new Promise((resolve) => 
          setTimeout(() => resolve(Success({ stdout: '{}', stderr: '', exitCode: 0 })), 10000)
        )
      );

      const options: ScanOptions = {
        timeout: 1000 // 1 second timeout
      };

      const result = await scanner.scanImage('test-image:latest', options);

      expect(result.ok).toBe(false);
      expect(result.error).toContain('Operation timed out after 1000ms');
    });
  });

  describe('scanFilesystem', () => {
    it('should scan filesystem for vulnerabilities', async () => {
      const mockScanOutput = `{
        "Results": [
          {
            "Target": "/path/to/scan",
            "Vulnerabilities": [
              {
                "VulnerabilityID": "CVE-2023-9999",
                "Severity": "MEDIUM",
                "PkgName": "npm-package"
              }
            ]
          }
        ]
      }`;

      mockCommandExecutor.execute.mockResolvedValue(
        Success({ stdout: mockScanOutput, stderr: '', exitCode: 0 })
      );

      const result = await scanner.scanFilesystem('/path/to/scan');

      expect(result.ok).toBe(true);
      expect(result.value.vulnerabilities).toHaveLength(1);
      expect(mockCommandExecutor.execute).toHaveBeenCalledWith(
        'trivy',
        ['fs', '--format', 'json', '/path/to/scan'],
        expect.any(Object)
      );
    });
  });

  describe('scanSecrets', () => {
    it('should scan for secrets in code', async () => {
      const mockScanOutput = `{
        "Results": [
          {
            "Target": "/path/to/scan",
            "Secrets": [
              {
                "RuleID": "aws-access-key-id",
                "Severity": "HIGH", 
                "Title": "AWS Access Key ID",
                "StartLine": 10,
                "EndLine": 10,
                "Code": {
                  "Lines": [
                    {
                      "Number": 10,
                      "Content": "AWS_ACCESS_KEY_ID=AKIAI1234567890"
                    }
                  ]
                }
              }
            ]
          }
        ]
      }`;

      mockCommandExecutor.execute.mockResolvedValue(
        Success({ stdout: mockScanOutput, stderr: '', exitCode: 0 })
      );

      const result = await scanner.scanSecrets('/path/to/scan');

      expect(result.ok).toBe(true);
      expect(result.value.secrets).toHaveLength(1);
      expect(result.value.secrets[0].type).toBe('aws-access-key-id');
      expect(result.value.secrets[0].severity).toBe('HIGH');
      expect(result.value.secrets[0].line).toBe(10);
    });

    it('should handle scan with no secrets', async () => {
      const mockScanOutput = `{
        "Results": [
          {
            "Target": "/path/to/scan",
            "Secrets": null
          }
        ]
      }`;

      mockCommandExecutor.execute.mockResolvedValue(
        Success({ stdout: mockScanOutput, stderr: '', exitCode: 0 })
      );

      const result = await scanner.scanSecrets('/path/to/scan');

      expect(result.ok).toBe(true);
      expect(result.value.secrets).toHaveLength(0);
    });
  });

  describe('generateReport', () => {
    it('should generate comprehensive security report', async () => {
      const mockImageScan = `{
        "Results": [
          {
            "Target": "test:latest",
            "Vulnerabilities": [
              {
                "VulnerabilityID": "CVE-2023-1234",
                "Severity": "HIGH",
                "PkgName": "libssl"
              }
            ]
          }
        ]
      }`;

      const mockSecretScan = `{
        "Results": [
          {
            "Target": "/app",
            "Secrets": [
              {
                "RuleID": "api-key",
                "Severity": "MEDIUM",
                "StartLine": 5
              }
            ]
          }
        ]
      }`;

      mockCommandExecutor.execute
        .mockResolvedValueOnce(Success({ stdout: mockImageScan, stderr: '', exitCode: 0 }))
        .mockResolvedValueOnce(Success({ stdout: mockSecretScan, stderr: '', exitCode: 0 }));

      const result = await scanner.generateReport('test:latest', '/app');

      expect(result.ok).toBe(true);
      expect(result.value.vulnerabilityResults.vulnerabilities).toHaveLength(1);
      expect(result.value.secretResults.secrets).toHaveLength(1);
      expect(result.value.summary.totalIssues).toBe(2);
      expect(result.value.summary.riskScore).toBeGreaterThan(0);
    });

    it('should calculate risk score correctly', async () => {
      const mockImageScan = `{
        "Results": [
          {
            "Target": "test:latest", 
            "Vulnerabilities": [
              {
                "VulnerabilityID": "CVE-2023-1",
                "Severity": "CRITICAL",
                "PkgName": "lib1"
              },
              {
                "VulnerabilityID": "CVE-2023-2", 
                "Severity": "HIGH",
                "PkgName": "lib2"
              },
              {
                "VulnerabilityID": "CVE-2023-3",
                "Severity": "MEDIUM", 
                "PkgName": "lib3"
              }
            ]
          }
        ]
      }`;

      mockCommandExecutor.execute
        .mockResolvedValueOnce(Success({ stdout: mockImageScan, stderr: '', exitCode: 0 }))
        .mockResolvedValueOnce(Success({ stdout: '{"Results":[]}', stderr: '', exitCode: 0 }));

      const result = await scanner.generateReport('test:latest', '/app');

      expect(result.ok).toBe(true);
      // Risk score should be calculated based on severity weights:
      // CRITICAL(10) + HIGH(7) + MEDIUM(5) = 22
      expect(result.value.summary.riskScore).toBe(22);
    });
  });

  describe('getScannerVersion', () => {
    it('should return scanner version', async () => {
      mockCommandExecutor.execute.mockResolvedValue(
        Success({ stdout: 'Version: 0.45.0', stderr: '', exitCode: 0 })
      );

      const result = await scanner.getScannerVersion();

      expect(result.ok).toBe(true);
      expect(result.value).toContain('0.45.0');
      expect(mockCommandExecutor.execute).toHaveBeenCalledWith('trivy', ['--version']);
    });

    it('should handle version command failure', async () => {
      mockCommandExecutor.execute.mockResolvedValue(
        Failure('Command not found')
      );

      const result = await scanner.getScannerVersion();

      expect(result.ok).toBe(false);
    });
  });

  describe('updateDatabase', () => {
    it('should update vulnerability database', async () => {
      mockCommandExecutor.execute.mockResolvedValue(
        Success({ stdout: 'Database updated successfully', stderr: '', exitCode: 0 })
      );

      const result = await scanner.updateDatabase();

      expect(result.ok).toBe(true);
      expect(mockCommandExecutor.execute).toHaveBeenCalledWith(
        'trivy',
        ['image', '--download-db-only'],
        expect.objectContaining({ timeout: 300000 })
      );
    });

    it('should handle database update failure', async () => {
      mockCommandExecutor.execute.mockResolvedValue(
        Failure('Update failed')
      );

      const result = await scanner.updateDatabase();

      expect(result.ok).toBe(false);
      expect(result.error).toContain('Failed to update vulnerability database');
    });
  });

  describe('error handling', () => {
    it('should handle malformed vulnerability data', async () => {
      const mockScanOutput = `{
        "Results": [
          {
            "Target": "test:latest",
            "Vulnerabilities": [
              {
                "VulnerabilityID": "CVE-2023-1234"
              }
            ]
          }
        ]
      }`;

      mockCommandExecutor.execute.mockResolvedValue(
        Success({ stdout: mockScanOutput, stderr: '', exitCode: 0 })
      );

      const result = await scanner.scanImage('test:latest');

      expect(result.ok).toBe(true);
      expect(result.value.vulnerabilities).toHaveLength(1);
      expect(result.value.vulnerabilities[0].severity).toBe('UNKNOWN');
    });

    it('should log scan progress and results', async () => {
      mockCommandExecutor.execute.mockResolvedValue(
        Success({ stdout: '{"Results":[]}', stderr: '', exitCode: 0 })
      );

      await scanner.scanImage('test:latest');

      expect(mockLogger.info).toHaveBeenCalledWith(
        { imageId: 'test:latest', options: undefined },
        'Starting security scan'
      );
      expect(mockLogger.info).toHaveBeenCalledWith(
        expect.any(Object),
        'Security scan completed'
      );
    });

    it('should handle scanner stderr output', async () => {
      mockCommandExecutor.execute.mockResolvedValue(
        Success({ 
          stdout: '{"Results":[]}', 
          stderr: 'Warning: deprecated package detected', 
          exitCode: 0 
        })
      );

      const result = await scanner.scanImage('test:latest');

      expect(result.ok).toBe(true);
      expect(mockLogger.warn).toHaveBeenCalledWith(
        { stderr: 'Warning: deprecated package detected' },
        'Scanner warnings'
      );
    });
  });

  describe('configuration validation', () => {
    it('should validate scan options', async () => {
      const invalidOptions: ScanOptions = {
        minSeverity: 'INVALID' as any,
        timeout: -1000
      };

      const result = await scanner.scanImage('test:latest', invalidOptions);
      
      expect(result.ok).toBe(false);
      expect(result.error).toContain('Invalid severity level: INVALID');
    });

    it('should use default options when none provided', async () => {
      mockCommandExecutor.execute.mockResolvedValue(
        Success({ stdout: '{"Results":[]}', stderr: '', exitCode: 0 })
      );

      await scanner.scanImage('test:latest');

      expect(mockCommandExecutor.execute).toHaveBeenCalledWith(
        'trivy',
        expect.arrayContaining(['image']),
        expect.objectContaining({
          timeout: expect.any(Number)
        })
      );
    });
  });
});