/**
 * Error Guidance Tests
 *
 * Tests for structured error guidance in Docker and Kubernetes clients
 */

import { describe, it, expect } from '@jest/globals';
import { extractDockerErrorGuidance } from '@/infra/docker/errors';
import { extractK8sErrorGuidance } from '@/infra/kubernetes/errors';
import { Failure } from '@/types';

describe('Error Guidance System', () => {
  describe('Docker Error Guidance', () => {
    it('should provide guidance for connection refused errors', () => {
      const error = new Error('connect ECONNREFUSED');
      (error as any).code = 'ECONNREFUSED';

      const guidance = extractDockerErrorGuidance(error);

      expect(guidance.message).toBe('Docker daemon is not available');
      expect(guidance.hint).toBe('Connection to Docker daemon was refused');
      expect(guidance.resolution).toContain('docker ps');
      expect(guidance.details).toBeDefined();
    });

    it('should provide guidance for authentication errors', () => {
      const error = new Error('Authentication failed');
      (error as any).statusCode = 401;

      const guidance = extractDockerErrorGuidance(error);

      expect(guidance.message).toBe('Docker registry authentication failed');
      expect(guidance.hint).toBe('Invalid or missing registry credentials');
      expect(guidance.resolution).toContain('docker login');
    });

    it('should provide guidance for network timeout', () => {
      const error = new Error('Operation timed out');
      (error as any).code = 'ETIMEDOUT';

      const guidance = extractDockerErrorGuidance(error);

      expect(guidance.message).toBe('Docker operation timed out');
      expect(guidance.hint).toContain('too long');
      expect(guidance.resolution).toContain('network connectivity');
    });

    it('should provide guidance for image not found', () => {
      const error = new Error('Image not found');
      (error as any).statusCode = 404;

      const guidance = extractDockerErrorGuidance(error);

      expect(guidance.message).toBe('Image or tag not found');
      expect(guidance.hint).toContain('does not exist');
      expect(guidance.resolution).toContain('docker images');
    });

    it('should provide guidance for daemon socket errors', () => {
      const error = new Error('connect ENOENT /var/run/docker.sock');

      const guidance = extractDockerErrorGuidance(error);

      expect(guidance.message).toBe('Docker daemon is not running');
      expect(guidance.hint).toBe('Cannot connect to Docker socket');
      expect(guidance.resolution).toContain('systemctl start docker');
    });

    it('should handle unknown errors gracefully', () => {
      const error = new Error('Unknown error');

      const guidance = extractDockerErrorGuidance(error);

      expect(guidance.message).toBe('Unknown error');
      expect(guidance.hint).toBeDefined();
      expect(guidance.resolution).toBeDefined();
    });

    it('should handle non-Error objects', () => {
      const guidance = extractDockerErrorGuidance('String error');

      expect(guidance.message).toBe('String error');
      expect(guidance.hint).toBe('An unexpected error occurred');
      expect(guidance.resolution).toContain('Docker daemon');
    });
  });

  describe('Kubernetes Error Guidance', () => {
    it('should provide guidance for connection refused', () => {
      const error = new Error('connect ECONNREFUSED 127.0.0.1:6443');

      const guidance = extractK8sErrorGuidance(error);

      expect(guidance.message).toBe('Cannot connect to Kubernetes cluster');
      expect(guidance.hint).toContain('refused');
      expect(guidance.resolution).toContain('kubectl cluster-info');
    });

    it('should provide guidance for authentication errors', () => {
      const error = new Error('Unauthorized: Invalid credentials');

      const guidance = extractK8sErrorGuidance(error);

      expect(guidance.message).toBe('Kubernetes authentication failed');
      expect(guidance.hint).toContain('credentials');
      expect(guidance.resolution).toContain('re-authenticate');
    });

    it('should provide guidance for authorization errors', () => {
      const error = new Error('Forbidden: User cannot perform this action');

      const guidance = extractK8sErrorGuidance(error);

      expect(guidance.message).toBe('Kubernetes authorization failed');
      expect(guidance.hint).toContain('permissions');
      expect(guidance.resolution).toContain('kubectl auth can-i');
    });

    it('should provide guidance for missing kubeconfig', () => {
      const error = new Error('Unable to load kubeconfig file');

      const guidance = extractK8sErrorGuidance(error);

      expect(guidance.message).toBe('Kubernetes configuration not found');
      expect(guidance.hint).toContain('kubeconfig');
      expect(guidance.resolution).toContain('KUBECONFIG');
    });

    it('should provide guidance for namespace not found', () => {
      const error = new Error('namespace "default" does not exist');

      const guidance = extractK8sErrorGuidance(error);

      expect(guidance.message).toBe('Kubernetes namespace does not exist');
      expect(guidance.hint).toContain('not been created');
      expect(guidance.resolution).toContain('kubectl create namespace');
    });

    it('should provide guidance for resource conflicts', () => {
      const error = new Error('deployment.apps "myapp" already exists');

      const guidance = extractK8sErrorGuidance(error);

      expect(guidance.message).toBe('Kubernetes resource already exists');
      expect(guidance.hint).toContain('already exists');
      expect(guidance.resolution).toContain('kubectl apply');
    });

    it('should provide guidance for API version mismatch', () => {
      const error = new Error('no matches for kind "Deployment" in version "apps/v1beta1"');

      const guidance = extractK8sErrorGuidance(error);

      expect(guidance.message).toBe('Kubernetes API version not supported');
      expect(guidance.hint).toContain('not available');
      expect(guidance.resolution).toContain('kubectl version');
    });

    it('should include operation context when provided', () => {
      const error = new Error('Resource not found');

      const guidance = extractK8sErrorGuidance(error, 'deploy application');

      expect(guidance.message).toContain('deploy application');
    });
  });

  describe('Result Type with Guidance', () => {
    it('should create Failure with guidance', () => {
      const result = Failure('Operation failed', {
        message: 'Operation failed',
        hint: 'Something went wrong',
        resolution: 'Try again',
      });

      expect(result.ok).toBe(false);
      expect(result.error).toBe('Operation failed');
      expect(result.guidance).toBeDefined();
      expect(result.guidance?.hint).toBe('Something went wrong');
      expect(result.guidance?.resolution).toBe('Try again');
    });

    it('should create Failure without guidance (backward compatible)', () => {
      const result = Failure('Operation failed');

      expect(result.ok).toBe(false);
      expect(result.error).toBe('Operation failed');
      expect(result.guidance).toBeUndefined();
    });

    it('should auto-fill message if guidance provided but message empty', () => {
      const result = Failure('Primary error', {
        message: '',
        hint: 'Hint text',
      });

      expect(result.ok).toBe(false);
      expect(result.error).toBe('Primary error');
      expect(result.guidance?.message).toBe('Primary error');
    });
  });

  describe('Error Guidance Structure', () => {
    it('should have required message field', () => {
      const guidance = extractDockerErrorGuidance(new Error('test'));

      expect(guidance).toHaveProperty('message');
      expect(typeof guidance.message).toBe('string');
    });

    it('should have optional hint field', () => {
      const guidance = extractDockerErrorGuidance(new Error('test'));

      // hint can be undefined or string
      if (guidance.hint !== undefined) {
        expect(typeof guidance.hint).toBe('string');
      }
    });

    it('should have optional resolution field', () => {
      const guidance = extractDockerErrorGuidance(new Error('test'));

      // resolution can be undefined or string
      if (guidance.resolution !== undefined) {
        expect(typeof guidance.resolution).toBe('string');
      }
    });

    it('should not leak internal stack traces in resolution', () => {
      const error = new Error('Test error');
      error.stack = 'Error: Test\n    at Object.<anonymous> (/path/to/file.js:1:1)';

      const guidance = extractDockerErrorGuidance(error);

      // Resolution should not contain file paths or stack traces
      expect(guidance.resolution).not.toContain('/path/to/file');
      expect(guidance.resolution).not.toContain('at Object.<anonymous>');
    });

    it('should provide actionable resolutions with commands', () => {
      const error = new Error('connect ECONNREFUSED');
      (error as any).code = 'ECONNREFUSED';

      const guidance = extractDockerErrorGuidance(error);

      // Resolution should contain specific commands
      expect(guidance.resolution).toMatch(/`[^`]+`/); // Has backtick-quoted commands
    });

    it('should validate all guidance fields have appropriate lengths', () => {
      const error = new Error('test error');
      (error as any).code = 'ECONNREFUSED';

      const guidance = extractDockerErrorGuidance(error);

      // Message should be concise (under 200 chars ideally)
      expect(guidance.message.length).toBeLessThan(300);

      if (guidance.hint) {
        // Hint should be a short explanation
        expect(guidance.hint.length).toBeLessThan(200);
      }

      if (guidance.resolution) {
        // Resolution can be longer as it contains commands
        expect(guidance.resolution.length).toBeGreaterThan(0);
        expect(guidance.resolution.length).toBeLessThan(1000);
      }
    });

    it('should ensure resolutions are operator-friendly', () => {
      const testCases = [
        { code: 'ECONNREFUSED', shouldContain: 'docker' },
        { statusCode: 401, shouldContain: 'login' },
        { statusCode: 404, shouldContain: 'images' },
      ];

      for (const testCase of testCases) {
        const error = new Error('test');
        Object.assign(error, testCase);

        const guidance = extractDockerErrorGuidance(error);

        expect(guidance.resolution?.toLowerCase()).toContain(testCase.shouldContain);
      }
    });
  });
});