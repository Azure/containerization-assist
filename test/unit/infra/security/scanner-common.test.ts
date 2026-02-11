/**
 * Unit Tests for Scanner Common Utilities
 *
 * Tests for security-critical functionality including:
 * - Image ID validation (command injection prevention)
 * - Severity normalization
 * - Severity counting
 * - Version parsing
 * - Error message helpers
 * - Logging utilities
 */

import { describe, it, expect, beforeEach, jest } from '@jest/globals';
import type { Logger } from 'pino';
import {
  ScannerErrors,
  validateImageId,
  normalizeSeverity,
  SeverityCounter,
  parseVersion,
  logScanStart,
  logScanComplete,
  type StandardSeverity,
} from '@/infra/security/scanner-common';

describe('Scanner Common Utilities', () => {
  describe('validateImageId', () => {
    describe('Security - Command Injection Prevention', () => {
      it('should reject imageId with shell metacharacters - semicolon', () => {
        expect(validateImageId('image;rm -rf /')).toBe(false);
      });

      it('should reject imageId with shell metacharacters - pipe', () => {
        expect(validateImageId('image|ls')).toBe(false);
      });

      it('should reject imageId with shell metacharacters - ampersand', () => {
        expect(validateImageId('image&whoami')).toBe(false);
      });

      it('should reject imageId with shell metacharacters - backtick', () => {
        expect(validateImageId('image`cat /etc/passwd`')).toBe(false);
      });

      it('should reject imageId with shell metacharacters - dollar paren', () => {
        expect(validateImageId('image$(cat /etc/passwd)')).toBe(false);
      });

      it('should reject imageId with shell metacharacters - newline', () => {
        expect(validateImageId('image\nrm -rf /')).toBe(false);
      });

      it('should reject imageId with shell metacharacters - carriage return', () => {
        expect(validateImageId('image\rrm -rf /')).toBe(false);
      });

      it('should reject imageId with shell metacharacters - redirection', () => {
        expect(validateImageId('image>file')).toBe(false);
        expect(validateImageId('image<file')).toBe(false);
      });

      it('should reject imageId with shell metacharacters - parentheses', () => {
        expect(validateImageId('image(test)')).toBe(false);
      });

      it('should reject imageId with shell metacharacters - braces', () => {
        expect(validateImageId('image{test}')).toBe(false);
      });

      it('should reject imageId with shell metacharacters - square brackets', () => {
        expect(validateImageId('image[test]')).toBe(false);
      });

      it('should reject imageId with shell metacharacters - asterisk', () => {
        expect(validateImageId('image*')).toBe(false);
      });

      it('should reject imageId with shell metacharacters - question mark', () => {
        expect(validateImageId('image?')).toBe(false);
      });

      it('should reject imageId with shell metacharacters - exclamation', () => {
        expect(validateImageId('image!')).toBe(false);
      });

      it('should reject imageId with shell metacharacters - single quote', () => {
        expect(validateImageId("image'test'")).toBe(false);
      });

      it('should reject imageId with shell metacharacters - double quote', () => {
        expect(validateImageId('image"test"')).toBe(false);
      });

      it('should reject imageId with shell metacharacters - percent', () => {
        expect(validateImageId('image%test')).toBe(false);
      });

      it('should reject imageId with shell metacharacters - caret', () => {
        expect(validateImageId('image^test')).toBe(false);
      });

      it('should reject imageId with shell metacharacters - tilde', () => {
        expect(validateImageId('image~test')).toBe(false);
      });

      it('should reject imageId with spaces', () => {
        expect(validateImageId('image name')).toBe(false);
      });

      it('should reject imageId with tabs', () => {
        expect(validateImageId('image\tname')).toBe(false);
      });
    });

    describe('Valid Image ID Formats', () => {
      it('should accept simple image name', () => {
        expect(validateImageId('nginx')).toBe(true);
      });

      it('should accept image with tag', () => {
        expect(validateImageId('nginx:latest')).toBe(true);
      });

      it('should accept image with registry', () => {
        expect(validateImageId('docker.io/library/nginx')).toBe(true);
      });

      it('should accept image with registry and tag', () => {
        expect(validateImageId('docker.io/library/nginx:1.21')).toBe(true);
      });

      it('should accept image with port in registry', () => {
        expect(validateImageId('registry.example.com:5000/myimage')).toBe(true);
      });

      it('should accept image with digest', () => {
        expect(validateImageId('nginx@sha256:abcd1234')).toBe(true);
      });

      it('should accept image with underscores', () => {
        expect(validateImageId('my_custom_image')).toBe(true);
      });

      it('should accept image with hyphens', () => {
        expect(validateImageId('my-custom-image')).toBe(true);
      });

      it('should accept image with dots', () => {
        expect(validateImageId('my.custom.image')).toBe(true);
      });

      it('should accept complex valid image name', () => {
        expect(validateImageId('registry.hub.docker.com:443/org/repo_name-123:v1.2.3@sha256:abc123def456')).toBe(
          true,
        );
      });

      it('should accept image with multiple slashes', () => {
        expect(validateImageId('registry.io/namespace/project/image:tag')).toBe(true);
      });

      it('should accept image ID (SHA256)', () => {
        expect(validateImageId('sha256:1234567890abcdef')).toBe(true);
      });

      it('should accept numeric image names', () => {
        expect(validateImageId('12345')).toBe(true);
      });
    });

    describe('Edge Cases', () => {
      it('should reject empty string', () => {
        expect(validateImageId('')).toBe(false);
      });

      it('should accept single character', () => {
        expect(validateImageId('a')).toBe(true);
      });

      it('should accept very long valid image ID', () => {
        const longId = 'a'.repeat(500);
        expect(validateImageId(longId)).toBe(true);
      });

      it('should reject null bytes', () => {
        expect(validateImageId('image\0test')).toBe(false);
      });

      it('should reject unicode characters', () => {
        expect(validateImageId('image™')).toBe(false);
        expect(validateImageId('image💻')).toBe(false);
      });
    });
  });

  describe('normalizeSeverity', () => {
    describe('Standard Severities', () => {
      it('should normalize CRITICAL (uppercase)', () => {
        expect(normalizeSeverity('CRITICAL')).toBe('CRITICAL');
      });

      it('should normalize critical (lowercase)', () => {
        expect(normalizeSeverity('critical')).toBe('CRITICAL');
      });

      it('should normalize Critical (mixed case)', () => {
        expect(normalizeSeverity('Critical')).toBe('CRITICAL');
      });

      it('should normalize HIGH (uppercase)', () => {
        expect(normalizeSeverity('HIGH')).toBe('HIGH');
      });

      it('should normalize high (lowercase)', () => {
        expect(normalizeSeverity('high')).toBe('HIGH');
      });

      it('should normalize High (mixed case)', () => {
        expect(normalizeSeverity('High')).toBe('HIGH');
      });

      it('should normalize MEDIUM (uppercase)', () => {
        expect(normalizeSeverity('MEDIUM')).toBe('MEDIUM');
      });

      it('should normalize medium (lowercase)', () => {
        expect(normalizeSeverity('medium')).toBe('MEDIUM');
      });

      it('should normalize Medium (mixed case)', () => {
        expect(normalizeSeverity('Medium')).toBe('MEDIUM');
      });

      it('should normalize LOW (uppercase)', () => {
        expect(normalizeSeverity('LOW')).toBe('LOW');
      });

      it('should normalize low (lowercase)', () => {
        expect(normalizeSeverity('low')).toBe('LOW');
      });

      it('should normalize Low (mixed case)', () => {
        expect(normalizeSeverity('Low')).toBe('LOW');
      });

      it('should normalize NEGLIGIBLE (uppercase)', () => {
        expect(normalizeSeverity('NEGLIGIBLE')).toBe('NEGLIGIBLE');
      });

      it('should normalize negligible (lowercase)', () => {
        expect(normalizeSeverity('negligible')).toBe('NEGLIGIBLE');
      });

      it('should normalize Negligible (mixed case)', () => {
        expect(normalizeSeverity('Negligible')).toBe('NEGLIGIBLE');
      });

      it('should normalize UNKNOWN (uppercase)', () => {
        expect(normalizeSeverity('UNKNOWN')).toBe('UNKNOWN');
      });

      it('should normalize unknown (lowercase)', () => {
        expect(normalizeSeverity('unknown')).toBe('UNKNOWN');
      });

      it('should normalize Unknown (mixed case)', () => {
        expect(normalizeSeverity('Unknown')).toBe('UNKNOWN');
      });
    });

    describe('Invalid/Unknown Severities', () => {
      it('should return UNKNOWN for invalid severity', () => {
        expect(normalizeSeverity('INVALID')).toBe('UNKNOWN');
      });

      it('should return UNKNOWN for empty string', () => {
        expect(normalizeSeverity('')).toBe('UNKNOWN');
      });

      it('should return UNKNOWN for random text', () => {
        expect(normalizeSeverity('xyz')).toBe('UNKNOWN');
      });

      it('should return UNKNOWN for numbers', () => {
        expect(normalizeSeverity('123')).toBe('UNKNOWN');
      });

      it('should return UNKNOWN for special characters', () => {
        expect(normalizeSeverity('!@#$')).toBe('UNKNOWN');
      });
    });

    describe('Case Sensitivity', () => {
      it('should handle all uppercase variations', () => {
        expect(normalizeSeverity('CRITICAL')).toBe('CRITICAL');
        expect(normalizeSeverity('HIGH')).toBe('HIGH');
        expect(normalizeSeverity('MEDIUM')).toBe('MEDIUM');
        expect(normalizeSeverity('LOW')).toBe('LOW');
      });

      it('should handle all lowercase variations', () => {
        expect(normalizeSeverity('critical')).toBe('CRITICAL');
        expect(normalizeSeverity('high')).toBe('HIGH');
        expect(normalizeSeverity('medium')).toBe('MEDIUM');
        expect(normalizeSeverity('low')).toBe('LOW');
      });

      it('should handle mixed case variations', () => {
        expect(normalizeSeverity('CrItIcAl')).toBe('CRITICAL');
        expect(normalizeSeverity('HiGh')).toBe('HIGH');
        expect(normalizeSeverity('MeDiUm')).toBe('MEDIUM');
        expect(normalizeSeverity('LoW')).toBe('LOW');
      });
    });
  });

  describe('SeverityCounter', () => {
    let counter: SeverityCounter;

    beforeEach(() => {
      counter = new SeverityCounter();
    });

    describe('Initialization', () => {
      it('should initialize all counts to zero', () => {
        expect(counter.critical).toBe(0);
        expect(counter.high).toBe(0);
        expect(counter.medium).toBe(0);
        expect(counter.low).toBe(0);
        expect(counter.negligible).toBe(0);
        expect(counter.unknown).toBe(0);
        expect(counter.total).toBe(0);
      });
    });

    describe('increment', () => {
      it('should increment CRITICAL count', () => {
        counter.increment('CRITICAL');
        expect(counter.critical).toBe(1);
        expect(counter.total).toBe(1);
      });

      it('should increment HIGH count', () => {
        counter.increment('HIGH');
        expect(counter.high).toBe(1);
        expect(counter.total).toBe(1);
      });

      it('should increment MEDIUM count', () => {
        counter.increment('MEDIUM');
        expect(counter.medium).toBe(1);
        expect(counter.total).toBe(1);
      });

      it('should increment LOW count', () => {
        counter.increment('LOW');
        expect(counter.low).toBe(1);
        expect(counter.total).toBe(1);
      });

      it('should increment NEGLIGIBLE count', () => {
        counter.increment('NEGLIGIBLE');
        expect(counter.negligible).toBe(1);
        expect(counter.total).toBe(1);
      });

      it('should increment UNKNOWN count', () => {
        counter.increment('UNKNOWN');
        expect(counter.unknown).toBe(1);
        expect(counter.total).toBe(1);
      });

      it('should handle multiple increments', () => {
        counter.increment('CRITICAL');
        counter.increment('CRITICAL');
        counter.increment('HIGH');
        expect(counter.critical).toBe(2);
        expect(counter.high).toBe(1);
        expect(counter.total).toBe(3);
      });

      it('should handle incrementing all severities', () => {
        counter.increment('CRITICAL');
        counter.increment('HIGH');
        counter.increment('MEDIUM');
        counter.increment('LOW');
        counter.increment('NEGLIGIBLE');
        counter.increment('UNKNOWN');

        expect(counter.critical).toBe(1);
        expect(counter.high).toBe(1);
        expect(counter.medium).toBe(1);
        expect(counter.low).toBe(1);
        expect(counter.negligible).toBe(1);
        expect(counter.unknown).toBe(1);
        expect(counter.total).toBe(6);
      });
    });

    describe('total', () => {
      it('should return sum of all counts', () => {
        counter.increment('CRITICAL');
        counter.increment('CRITICAL');
        counter.increment('HIGH');
        counter.increment('MEDIUM');
        counter.increment('LOW');
        counter.increment('NEGLIGIBLE');
        counter.increment('UNKNOWN');

        expect(counter.total).toBe(7);
      });

      it('should return 0 when no vulnerabilities counted', () => {
        expect(counter.total).toBe(0);
      });
    });

    describe('getCounts', () => {
      it('should return counts object with all fields', () => {
        counter.increment('CRITICAL');
        counter.increment('HIGH');
        counter.increment('HIGH');
        counter.increment('MEDIUM');

        const counts = counter.getCounts();

        expect(counts).toEqual({
          criticalCount: 1,
          highCount: 2,
          mediumCount: 1,
          lowCount: 0,
          negligibleCount: 0,
          unknownCount: 0,
          totalVulnerabilities: 4,
        });
      });

      it('should return all zeros for empty counter', () => {
        const counts = counter.getCounts();

        expect(counts).toEqual({
          criticalCount: 0,
          highCount: 0,
          mediumCount: 0,
          lowCount: 0,
          negligibleCount: 0,
          unknownCount: 0,
          totalVulnerabilities: 0,
        });
      });

      it('should match total property', () => {
        counter.increment('CRITICAL');
        counter.increment('HIGH');
        counter.increment('MEDIUM');

        const counts = counter.getCounts();
        expect(counts.totalVulnerabilities).toBe(counter.total);
      });
    });

    describe('Multiple Instances', () => {
      it('should maintain independent counts across instances', () => {
        const counter1 = new SeverityCounter();
        const counter2 = new SeverityCounter();

        counter1.increment('CRITICAL');
        counter1.increment('HIGH');

        counter2.increment('MEDIUM');

        expect(counter1.critical).toBe(1);
        expect(counter1.high).toBe(1);
        expect(counter1.medium).toBe(0);
        expect(counter1.total).toBe(2);

        expect(counter2.critical).toBe(0);
        expect(counter2.high).toBe(0);
        expect(counter2.medium).toBe(1);
        expect(counter2.total).toBe(1);
      });
    });
  });

  describe('parseVersion', () => {
    describe('Successful Parsing', () => {
      it('should parse simple version number', () => {
        const output = 'version 1.2.3';
        const pattern = /version (\d+\.\d+\.\d+)/;
        expect(parseVersion(output, pattern)).toBe('1.2.3');
      });

      it('should parse version with text before and after', () => {
        const output = 'Tool v2.0.1 (build 12345)';
        const pattern = /v(\d+\.\d+\.\d+)/;
        expect(parseVersion(output, pattern)).toBe('2.0.1');
      });

      it('should trim whitespace from captured group', () => {
        const output = 'Version:   1.2.3   ';
        const pattern = /Version:\s+(\d+\.\d+\.\d+)/;
        expect(parseVersion(output, pattern)).toBe('1.2.3');
      });

      it('should handle multi-line output', () => {
        const output = 'Scanner Tool\nVersion: 3.0.0\nBuild: stable';
        const pattern = /Version: (\d+\.\d+\.\d+)/;
        expect(parseVersion(output, pattern)).toBe('3.0.0');
      });

      it('should parse version with build metadata', () => {
        const output = 'version 1.2.3-beta+build.123';
        const pattern = /version ([\d.]+-[\w.+]+)/;
        expect(parseVersion(output, pattern)).toBe('1.2.3-beta+build.123');
      });

      it('should parse first match when multiple matches exist', () => {
        const output = 'Tool 1.0.0 uses library 2.0.0';
        const pattern = /(\d+\.\d+\.\d+)/;
        expect(parseVersion(output, pattern)).toBe('1.0.0');
      });
    });

    describe('Failed Parsing', () => {
      it('should return undefined when pattern does not match', () => {
        const output = 'no version here';
        const pattern = /version (\d+\.\d+\.\d+)/;
        expect(parseVersion(output, pattern)).toBeUndefined();
      });

      it('should return undefined for empty output', () => {
        const output = '';
        const pattern = /version (\d+\.\d+\.\d+)/;
        expect(parseVersion(output, pattern)).toBeUndefined();
      });

      it('should return undefined when no capture group', () => {
        const output = 'version 1.2.3';
        const pattern = /version \d+\.\d+\.\d+/; // No capture group
        expect(parseVersion(output, pattern)).toBeUndefined();
      });

      it('should return undefined when capture group is empty', () => {
        const output = 'version ';
        const pattern = /version (.*)/;
        const result = parseVersion(output, pattern);
        // Empty string after trim becomes empty, but should still return the empty string
        expect(result).toBe('');
      });
    });

    describe('Edge Cases', () => {
      it('should handle special characters in version', () => {
        const output = 'version 1.2.3-alpha.1+20230101';
        const pattern = /version ([\d.a-z+-]+)/i;
        expect(parseVersion(output, pattern)).toBe('1.2.3-alpha.1+20230101');
      });

      it('should handle version at start of string', () => {
        const output = '1.2.3 is the version';
        const pattern = /^(\d+\.\d+\.\d+)/;
        expect(parseVersion(output, pattern)).toBe('1.2.3');
      });

      it('should handle version at end of string', () => {
        const output = 'version: 1.2.3';
        const pattern = /(\d+\.\d+\.\d+)$/;
        expect(parseVersion(output, pattern)).toBe('1.2.3');
      });
    });
  });

  describe('logScanStart', () => {
    let mockLogger: Logger;

    beforeEach(() => {
      mockLogger = {
        info: jest.fn<Logger['info']>(),
      } as unknown as Logger;
    });

    it('should log scan start with correct parameters', () => {
      logScanStart(mockLogger, 'Trivy', '0.50.0', 'nginx:latest');

      expect(mockLogger.info).toHaveBeenCalledWith(
        {
          scanner: 'Trivy',
          version: '0.50.0',
          imageId: 'nginx:latest',
        },
        'Starting Trivy scan',
      );
    });

    it('should include scanner name in message', () => {
      logScanStart(mockLogger, 'Snyk', '1.2.3', 'alpine:3.18');

      expect(mockLogger.info).toHaveBeenCalledWith(
        expect.any(Object),
        expect.stringContaining('Snyk'),
      );
    });

    it('should log metadata object with all fields', () => {
      logScanStart(mockLogger, 'Grype', '0.70.0', 'ubuntu:22.04');

      expect(mockLogger.info).toHaveBeenCalledWith(
        expect.objectContaining({
          scanner: 'Grype',
          version: '0.70.0',
          imageId: 'ubuntu:22.04',
        }),
        expect.any(String),
      );
    });
  });

  describe('logScanComplete', () => {
    let mockLogger: Logger;

    beforeEach(() => {
      mockLogger = {
        info: jest.fn<Logger['info']>(),
      } as unknown as Logger;
    });

    it('should log scan completion with correct parameters', () => {
      logScanComplete(mockLogger, 'Trivy', 'nginx:latest', 10, 2, 3);

      expect(mockLogger.info).toHaveBeenCalledWith(
        {
          scanner: 'Trivy',
          imageId: 'nginx:latest',
          totalVulnerabilities: 10,
          criticalCount: 2,
          highCount: 3,
        },
        'Trivy scan completed successfully',
      );
    });

    it('should include scanner name in message', () => {
      logScanComplete(mockLogger, 'Snyk', 'alpine:3.18', 5, 0, 1);

      expect(mockLogger.info).toHaveBeenCalledWith(
        expect.any(Object),
        expect.stringContaining('Snyk'),
      );
    });

    it('should log metadata object with all fields', () => {
      logScanComplete(mockLogger, 'Grype', 'ubuntu:22.04', 15, 3, 5);

      expect(mockLogger.info).toHaveBeenCalledWith(
        expect.objectContaining({
          scanner: 'Grype',
          imageId: 'ubuntu:22.04',
          totalVulnerabilities: 15,
          criticalCount: 3,
          highCount: 5,
        }),
        expect.any(String),
      );
    });

    it('should handle zero vulnerabilities', () => {
      logScanComplete(mockLogger, 'Trivy', 'nginx:latest', 0, 0, 0);

      expect(mockLogger.info).toHaveBeenCalledWith(
        expect.objectContaining({
          totalVulnerabilities: 0,
          criticalCount: 0,
          highCount: 0,
        }),
        expect.any(String),
      );
    });
  });

  describe('ScannerErrors', () => {
    describe('invalidImageId', () => {
      it('should create error with invalid imageId', () => {
        const result = ScannerErrors.invalidImageId('image;rm -rf /');

        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('Invalid imageId format');
          expect(result.guidance?.message).toBe('ImageId contains invalid characters');
          expect(result.guidance?.hint).toContain('alphanumeric');
          expect(result.guidance?.resolution).toContain('valid Docker image identifier');
          expect(result.guidance?.details).toEqual({ imageId: 'image;rm -rf /' });
        }
      });
    });

    describe('scannerNotInstalled', () => {
      it('should create error for missing scanner', () => {
        const result = ScannerErrors.scannerNotInstalled('Trivy', 'https://trivy.dev');

        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('Trivy not installed');
          expect(result.guidance?.message).toBe('Trivy CLI not found');
          expect(result.guidance?.resolution).toContain('https://trivy.dev');
        }
      });
    });

    describe('jsonParseError', () => {
      it('should create error for JSON parsing failure', () => {
        const result = ScannerErrors.jsonParseError('Snyk', 'Unexpected token', '{invalid json');

        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('Failed to parse Snyk output');
          expect(result.guidance?.message).toContain('parsing failed');
          expect(result.guidance?.details).toEqual({
            parseError: 'Unexpected token',
            outputPreview: '{invalid json',
          });
        }
      });
    });

    describe('emptyOutput', () => {
      it('should create error for empty scanner output', () => {
        const result = ScannerErrors.emptyOutput('Grype', 'nginx:latest');

        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('Grype returned empty output');
          expect(result.guidance?.resolution).toContain("docker image inspect 'nginx:latest'");
        }
      });
    });

    describe('scanExecutionError', () => {
      it('should create error for scan execution failure', () => {
        const result = ScannerErrors.scanExecutionError('Trivy', 'nginx:latest', 'Connection timeout');

        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('Trivy scan failed: Connection timeout');
          expect(result.guidance?.details).toEqual({ error: 'Connection timeout' });
          expect(result.guidance?.resolution).toContain("docker image ls | grep 'nginx:latest'");
        }
      });
    });

    describe('versionCheckTimeout', () => {
      it('should create error for version check timeout', () => {
        const result = ScannerErrors.versionCheckTimeout('Snyk', 'snyk --version');

        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('Snyk version check timed out');
          expect(result.guidance?.resolution).toContain('snyk --version');
        }
      });
    });

    describe('versionParseError', () => {
      it('should create error for version parsing failure', () => {
        const result = ScannerErrors.versionParseError('Grype', 'grype version');

        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('Grype version could not be parsed');
          expect(result.guidance?.resolution).toContain('grype version');
        }
      });
    });
  });
});
