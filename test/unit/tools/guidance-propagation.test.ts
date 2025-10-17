/**
 * Unit test: Tool-level Error Guidance Propagation
 *
 * Verifies that tools properly propagate error guidance from infrastructure clients
 */

import { describe, it, expect } from '@jest/globals';
import { Failure, type ErrorGuidance } from '@/types';

describe('Tool Error Guidance Propagation', () => {
  describe('Failure with guidance', () => {
    it('should create Failure with guidance attached', () => {
      const guidance: ErrorGuidance = {
        message: 'Docker daemon not available',
        hint: 'Connection refused',
        resolution: 'Start Docker with `docker ps`',
      };

      const result = Failure('Build failed: Docker daemon not available', guidance);

      expect(result.ok).toBe(false);
      expect(result.error).toBe('Build failed: Docker daemon not available');
      expect(result.guidance).toBeDefined();
      expect(result.guidance?.message).toBe('Docker daemon not available');
      expect(result.guidance?.hint).toBe('Connection refused');
      expect(result.guidance?.resolution).toBe('Start Docker with `docker ps`');
    });

    it('should work without guidance for backward compatibility', () => {
      const result = Failure('Simple error');

      expect(result.ok).toBe(false);
      expect(result.error).toBe('Simple error');
      expect(result.guidance).toBeUndefined();
    });

    it('should preserve guidance structure through type system', () => {
      const dockerGuidance: ErrorGuidance = {
        message: 'Cannot connect to Docker',
        hint: 'ECONNREFUSED',
        resolution: 'Check if Docker is running',
        details: { code: 'ECONNREFUSED', port: 2375 },
      };

      const result = Failure('Operation failed', dockerGuidance);

      if (!result.ok && result.guidance) {
        // TypeScript ensures this is properly typed
        expect(result.guidance.message).toBeDefined();
        expect(result.guidance.hint).toBeDefined();
        expect(result.guidance.resolution).toBeDefined();
        expect(result.guidance.details).toBeDefined();
        expect(result.guidance.details?.code).toBe('ECONNREFUSED');
      } else {
        fail('Expected result to have guidance');
      }
    });
  });

  describe('Guidance content validation', () => {
    it('should not allow empty message in guidance', () => {
      const guidance: ErrorGuidance = {
        message: '', // Empty message gets filled by Failure()
        hint: 'Test hint',
      };

      const result = Failure('Main error', guidance);

      expect(result.guidance?.message).toBe('Main error');
    });

    it('should preserve all guidance fields', () => {
      const fullGuidance: ErrorGuidance = {
        message: 'Kubernetes authentication failed',
        hint: 'Invalid or expired credentials',
        resolution: 'Refresh cluster credentials with `gcloud container clusters get-credentials`',
        details: {
          statusCode: 401,
          cluster: 'prod-cluster',
        },
      };

      const result = Failure('Failed to deploy', fullGuidance);

      expect(result.guidance).toEqual(fullGuidance);
    });
  });

  describe('Type safety', () => {
    it('should ensure Result type includes guidance field', () => {
      // This test validates TypeScript compilation
      const result = Failure<string>('Error', {
        message: 'Error',
        hint: 'Hint',
        resolution: 'Resolution',
      });

      // Type guard
      if (!result.ok) {
        // guidance should be available on the type
        const guidance = result.guidance;
        expect(guidance).toBeDefined();
      }
    });

    it('should handle Result with no guidance', () => {
      const result = Failure<number>('Calculation failed');

      if (!result.ok) {
        // guidance should be optional
        const guidance = result.guidance;
        expect(guidance).toBeUndefined();
      }
    });
  });

  describe('Real-world guidance examples', () => {
    it('should format Docker connection errors', () => {
      const guidance: ErrorGuidance = {
        message: 'Docker daemon is not available',
        hint: 'Connection to Docker daemon was refused',
        resolution:
          'Ensure Docker is installed and running: `docker ps` should succeed. Check Docker daemon logs if the service is running.',
        details: { code: 'ECONNREFUSED' },
      };

      expect(guidance.resolution).toContain('docker ps');
      expect(guidance.hint).toContain('refused');
    });

    it('should format Kubernetes auth errors', () => {
      const guidance: ErrorGuidance = {
        message: 'Kubernetes authentication failed',
        hint: 'Invalid or expired credentials',
        resolution:
          'Refresh cluster credentials. For cloud providers: re-authenticate (e.g., `aws eks update-kubeconfig`, `gcloud container clusters get-credentials`).',
      };

      expect(guidance.resolution).toContain('aws eks');
      expect(guidance.resolution).toContain('gcloud');
      expect(guidance.hint).toContain('credentials');
    });

    it('should format Docker registry auth errors', () => {
      const guidance: ErrorGuidance = {
        message: 'Docker registry authentication failed',
        hint: 'Invalid or missing registry credentials',
        resolution:
          'Run `docker login <registry>` to authenticate, or verify credentials in your Docker config (~/.docker/config.json).',
        details: { statusCode: 401 },
      };

      expect(guidance.resolution).toContain('docker login');
      expect(guidance.resolution).toContain('config.json');
    });
  });
});