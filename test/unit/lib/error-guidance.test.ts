/**
 * Tests for error guidance pattern matching system
 */

import {
  createErrorGuidanceBuilder,
  messagePattern,
  customPattern,
  type ErrorPattern,
} from '@/lib/error-guidance';
import type { ErrorGuidance } from '@/types';

describe('error-guidance', () => {
  describe('messagePattern', () => {
    it('should match error message substring (case-insensitive)', () => {
      const pattern = messagePattern('ECONNREFUSED', {
        message: 'Connection refused',
        hint: 'Cannot connect to service',
        resolution: 'Ensure the service is running',
      });

      const error = new Error('ECONNREFUSED: Connection refused');
      expect(pattern.match(error)).toBe(true);
      const guidance = pattern.guidance(error);
      expect(guidance.message).toBe('Connection refused');
      expect(guidance.hint).toBe('Cannot connect to service');
      expect(guidance.resolution).toBe('Ensure the service is running');
    });

    it('should match case-insensitively', () => {
      const pattern = messagePattern('docker', {
        message: 'Docker error',
        hint: 'Docker-related issue',
      });

      expect(pattern.match(new Error('DOCKER daemon not running'))).toBe(true);
      expect(pattern.match(new Error('Docker Desktop required'))).toBe(true);
      expect(pattern.match(new Error('docker build failed'))).toBe(true);
    });

    it('should not match when substring is not present', () => {
      const pattern = messagePattern('kubernetes', {
        message: 'K8s error',
      });

      expect(pattern.match(new Error('Docker error'))).toBe(false);
      expect(pattern.match(new Error('Network failure'))).toBe(false);
    });

    it('should match string errors', () => {
      const pattern = messagePattern('not found', {
        message: 'Resource not found',
      });

      expect(pattern.match('File not found')).toBe(true);
      expect(pattern.match('Image not found')).toBe(true);
    });

    it('should handle non-Error objects', () => {
      const pattern = messagePattern('error', {
        message: 'Generic error',
      });

      expect(pattern.match({ toString: () => 'error occurred' })).toBe(true);
      expect(pattern.match(404)).toBe(false);
    });
  });

  describe('customPattern', () => {
    it('should use custom match function', () => {
      const pattern = customPattern(
        (error) => error instanceof TypeError,
        {
          message: 'Type error occurred',
          hint: 'Check variable types',
        },
      );

      expect(pattern.match(new TypeError('Cannot read property'))).toBe(true);
      expect(pattern.match(new Error('Generic error'))).toBe(false);
      expect(pattern.match(new RangeError('Out of range'))).toBe(false);
    });

    it('should support guidance function', () => {
      const pattern = customPattern(
        (error) => error instanceof Error && error.message.includes('timeout'),
        (error) => ({
          message: error instanceof Error ? error.message : 'Timeout error',
          hint: 'Operation timed out',
          resolution: 'Increase timeout value or check network',
        }),
      );

      const error = new Error('Request timeout after 30s');
      expect(pattern.match(error)).toBe(true);
      const guidance = pattern.guidance(error);
      expect(guidance.message).toBe('Request timeout after 30s');
      expect(guidance.hint).toBe('Operation timed out');
    });

    it('should support static guidance object', () => {
      const pattern = customPattern(
        (error) => typeof error === 'string' && error.startsWith('ERR_'),
        {
          message: 'Error code detected',
          hint: 'Check error code documentation',
        },
      );

      expect(pattern.match('ERR_INVALID_ARG')).toBe(true);
      const guidance = pattern.guidance('ERR_INVALID_ARG');
      expect(guidance.message).toBe('Error code detected');
    });

    it('should handle complex match logic', () => {
      const pattern = customPattern(
        (error) => {
          if (!(error instanceof Error)) return false;
          const message = error.message.toLowerCase();
          return message.includes('docker') && message.includes('permission');
        },
        {
          message: 'Docker permission denied',
          hint: 'Docker requires elevated permissions',
          resolution: 'Add user to docker group or run with sudo',
        },
      );

      expect(pattern.match(new Error('Docker permission denied'))).toBe(true);
      expect(pattern.match(new Error('Permission denied'))).toBe(false);
      expect(pattern.match(new Error('Docker error'))).toBe(false);
    });
  });

  describe('createErrorGuidanceBuilder', () => {
    it('should match first pattern in order', () => {
      const patterns: ErrorPattern[] = [
        messagePattern('docker', {
          message: 'Docker error',
          hint: 'Docker-specific issue',
        }),
        messagePattern('daemon', {
          message: 'Daemon error',
          hint: 'Daemon-specific issue',
        }),
      ];

      const builder = createErrorGuidanceBuilder(patterns);
      const error = new Error('Docker daemon not running');
      const guidance = builder(error);

      // Should match first pattern (docker), not second (daemon)
      expect(guidance.message).toBe('Docker error');
      expect(guidance.hint).toBe('Docker-specific issue');
    });

    it('should try patterns in order until match found', () => {
      const patterns: ErrorPattern[] = [
        messagePattern('kubernetes', {
          message: 'K8s error',
        }),
        messagePattern('docker', {
          message: 'Docker error',
        }),
        messagePattern('error', {
          message: 'Generic error',
        }),
      ];

      const builder = createErrorGuidanceBuilder(patterns);

      // Should match second pattern
      const dockerError = builder(new Error('Docker build failed'));
      expect(dockerError.message).toBe('Docker error');

      // Should match third pattern
      const genericError = builder(new Error('Unknown error occurred'));
      expect(genericError.message).toBe('Generic error');
    });

    it('should use default guidance when no pattern matches', () => {
      const patterns: ErrorPattern[] = [
        messagePattern('docker', {
          message: 'Docker error',
        }),
      ];

      const defaultGuidance = (error: unknown): ErrorGuidance => ({
        message: `Custom default: ${error}`,
        hint: 'No pattern matched',
      });

      const builder = createErrorGuidanceBuilder(patterns, defaultGuidance);
      const guidance = builder(new Error('Network failure'));

      expect(guidance.message).toContain('Custom default');
      expect(guidance.hint).toBe('No pattern matched');
    });

    it('should use generic fallback when no pattern matches and no default provided', () => {
      const patterns: ErrorPattern[] = [
        messagePattern('docker', {
          message: 'Docker error',
        }),
      ];

      const builder = createErrorGuidanceBuilder(patterns);
      const guidance = builder(new Error('Unhandled error'));

      expect(guidance.message).toBe('Unhandled error');
      expect(guidance.hint).toBe('An unexpected error occurred');
      expect(guidance.resolution).toBe('Check the error message and logs for more details');
    });

    it('should handle empty pattern list', () => {
      const builder = createErrorGuidanceBuilder([]);
      const guidance = builder(new Error('Some error'));

      expect(guidance.message).toBe('Some error');
      expect(guidance.hint).toBe('An unexpected error occurred');
    });

    it('should support complex pattern combinations', () => {
      const patterns: ErrorPattern[] = [
        customPattern(
          (error) => error instanceof TypeError,
          {
            message: 'Type error',
            hint: 'Invalid type usage',
          },
        ),
        messagePattern('ECONNREFUSED', {
          message: 'Connection refused',
          hint: 'Service unavailable',
        }),
        messagePattern('ENOENT', {
          message: 'File not found',
          hint: 'Check file path',
        }),
      ];

      const builder = createErrorGuidanceBuilder(patterns);

      const typeError = builder(new TypeError('Cannot read property'));
      expect(typeError.message).toBe('Type error');

      const connError = builder(new Error('ECONNREFUSED'));
      expect(connError.message).toBe('Connection refused');

      const fileError = builder(new Error('ENOENT: file not found'));
      expect(fileError.message).toBe('File not found');
    });

    it('should extract message from non-Error types in fallback', () => {
      const builder = createErrorGuidanceBuilder([]);

      const stringGuidance = builder('String error');
      expect(stringGuidance.message).toBe('String error');

      const numberGuidance = builder(404);
      expect(numberGuidance.message).toBe('404');

      const objectGuidance = builder({ error: 'object error' });
      expect(objectGuidance.message).toBeDefined();
    });

    it('should allow patterns to return dynamic guidance based on error', () => {
      const patterns: ErrorPattern[] = [
        customPattern(
          (error) => error instanceof Error && /timeout/.test(error.message),
          (error) => {
            const match = error instanceof Error && error.message.match(/(\d+)s/);
            const seconds = match ? match[1] : 'unknown';
            return {
              message: `Operation timed out after ${seconds} seconds`,
              hint: 'Consider increasing timeout',
              resolution: `Increase timeout beyond ${seconds}s or optimize operation`,
            };
          },
        ),
      ];

      const builder = createErrorGuidanceBuilder(patterns);
      const guidance = builder(new Error('Request timeout after 30s'));

      expect(guidance.message).toBe('Operation timed out after 30 seconds');
      expect(guidance.resolution).toContain('beyond 30s');
    });
  });
});
