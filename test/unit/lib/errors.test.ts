/**
 * Tests for error handling utilities
 */

import { extractErrorMessage, createErrorGuidance, ERROR_MESSAGES } from '@/lib/errors';

describe('errors', () => {
  describe('extractErrorMessage', () => {
    it('should extract message from Error object', () => {
      const error = new Error('Test error message');
      expect(extractErrorMessage(error)).toBe('Test error message');
    });

    it('should extract message from custom Error subclass', () => {
      class CustomError extends Error {
        constructor(message: string) {
          super(message);
          this.name = 'CustomError';
        }
      }
      const error = new CustomError('Custom error');
      expect(extractErrorMessage(error)).toBe('Custom error');
    });

    it('should handle string errors', () => {
      expect(extractErrorMessage('String error')).toBe('String error');
    });

    it('should handle number values', () => {
      expect(extractErrorMessage(404)).toBe('404');
    });

    it('should handle boolean values', () => {
      expect(extractErrorMessage(false)).toBe('false');
    });

    it('should handle null', () => {
      expect(extractErrorMessage(null)).toBe('null');
    });

    it('should handle undefined', () => {
      expect(extractErrorMessage(undefined)).toBe('undefined');
    });

    it('should handle objects by converting to string', () => {
      const obj = { code: 'ERROR', message: 'Something went wrong' };
      const result = extractErrorMessage(obj);
      expect(result).toContain('object');
    });

    it('should handle arrays', () => {
      const arr = ['error1', 'error2'];
      expect(extractErrorMessage(arr)).toBe('error1,error2');
    });
  });

  describe('createErrorGuidance', () => {
    it('should create guidance with only message', () => {
      const guidance = createErrorGuidance('Error occurred');

      expect(guidance.message).toBe('Error occurred');
      expect(guidance.hint).toBeUndefined();
      expect(guidance.resolution).toBeUndefined();
      expect(guidance.details).toBeUndefined();
    });

    it('should create guidance with message and hint', () => {
      const guidance = createErrorGuidance('Docker not found', 'Install Docker Desktop');

      expect(guidance.message).toBe('Docker not found');
      expect(guidance.hint).toBe('Install Docker Desktop');
      expect(guidance.resolution).toBeUndefined();
      expect(guidance.details).toBeUndefined();
    });

    it('should create guidance with message, hint, and resolution', () => {
      const guidance = createErrorGuidance(
        'Docker daemon not running',
        'The Docker daemon must be running',
        'Start Docker Desktop or run: sudo systemctl start docker',
      );

      expect(guidance.message).toBe('Docker daemon not running');
      expect(guidance.hint).toBe('The Docker daemon must be running');
      expect(guidance.resolution).toBe('Start Docker Desktop or run: sudo systemctl start docker');
      expect(guidance.details).toBeUndefined();
    });

    it('should create guidance with all parameters including details', () => {
      const details = { exitCode: 1, command: 'docker build' };
      const guidance = createErrorGuidance(
        'Build failed',
        'The Docker build process failed',
        'Check Dockerfile syntax',
        details,
      );

      expect(guidance.message).toBe('Build failed');
      expect(guidance.hint).toBe('The Docker build process failed');
      expect(guidance.resolution).toBe('Check Dockerfile syntax');
      expect(guidance.details).toEqual({ exitCode: 1, command: 'docker build' });
    });

    it('should handle empty strings', () => {
      const guidance = createErrorGuidance('', '', '');

      expect(guidance.message).toBe('');
      expect(guidance.hint).toBe('');
      expect(guidance.resolution).toBe('');
    });
  });

  describe('ERROR_MESSAGES', () => {
    describe('TOOL_NOT_FOUND', () => {
      it('should format tool not found message', () => {
        const message = ERROR_MESSAGES.TOOL_NOT_FOUND('build-image');
        expect(message).toBe('Tool not found: build-image');
      });
    });

    describe('VALIDATION_FAILED', () => {
      it('should format validation failed message', () => {
        const message = ERROR_MESSAGES.VALIDATION_FAILED('Missing required field: tag');
        expect(message).toBe('Validation failed: Missing required field: tag');
      });
    });

    describe('POLICY_BLOCKED', () => {
      it('should format policy blocked message with single rule', () => {
        const message = ERROR_MESSAGES.POLICY_BLOCKED(['no-root-user']);
        expect(message).toContain('Blocked by policy rules: no-root-user');
        expect(message).toContain('policy configuration');
      });

      it('should format policy blocked message with multiple rules', () => {
        const message = ERROR_MESSAGES.POLICY_BLOCKED(['no-root-user', 'require-health-check']);
        expect(message).toContain('Blocked by policy rules: no-root-user, require-health-check');
      });
    });

    describe('POLICY_VALIDATION_FAILED', () => {
      it('should format policy validation failed message', () => {
        const message = ERROR_MESSAGES.POLICY_VALIDATION_FAILED('Invalid YAML syntax');
        expect(message).toContain('Policy validation failed: Invalid YAML syntax');
        expect(message).toContain('policy file syntax');
      });
    });

    describe('POLICY_LOAD_FAILED', () => {
      it('should format policy load failed message', () => {
        const message = ERROR_MESSAGES.POLICY_LOAD_FAILED('File not found');
        expect(message).toContain('Failed to load policy: File not found');
        expect(message).toContain('Verify policy file exists');
      });
    });

    describe('DOCKER_OPERATION_FAILED', () => {
      it('should format Docker operation failed message', () => {
        const message = ERROR_MESSAGES.DOCKER_OPERATION_FAILED('build', 'syntax error in Dockerfile');
        expect(message).toBe('Docker build failed: syntax error in Dockerfile');
      });
    });

    describe('K8S_OPERATION_FAILED', () => {
      it('should format Kubernetes operation failed message', () => {
        const message = ERROR_MESSAGES.K8S_OPERATION_FAILED('apply', 'cluster unreachable');
        expect(message).toBe('Kubernetes apply failed: cluster unreachable');
      });
    });

    describe('K8S_APPLY_FAILED', () => {
      it('should format Kubernetes apply failed message', () => {
        const message = ERROR_MESSAGES.K8S_APPLY_FAILED(
          'Deployment',
          'my-app',
          'insufficient resources',
        );
        expect(message).toBe('Failed to apply Deployment/my-app: insufficient resources');
      });
    });

    describe('OPERATION_FAILED', () => {
      it('should format generic operation failed message', () => {
        const message = ERROR_MESSAGES.OPERATION_FAILED('file read', 'permission denied');
        expect(message).toBe('file read failed: permission denied');
      });
    });

    describe('RESOURCE_NOT_FOUND', () => {
      it('should format resource not found message', () => {
        const message = ERROR_MESSAGES.RESOURCE_NOT_FOUND('Image', 'nginx:latest');
        expect(message).toBe('Image not found: nginx:latest');
      });
    });
  });
});
