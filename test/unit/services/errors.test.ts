import { describe, it, expect } from '@jest/globals';
import { extractDockerErrorGuidance } from '@/infra/docker/errors';

describe('extractDockerErrorGuidance', () => {
  describe('network error codes', () => {
    it('should handle ENOTFOUND errors', () => {
      const error = new Error('getaddrinfo ENOTFOUND registry.docker.io');
      (error as any).code = 'ENOTFOUND';

      const result = extractDockerErrorGuidance(error);

      expect(result.message).toContain('Cannot resolve Docker registry hostname');
      expect(result.hint).toBeDefined();
      expect(result.resolution).toBeDefined();
      expect(result.details?.code).toBe('ENOTFOUND');
    });

    it('should handle ECONNREFUSED errors', () => {
      const error = new Error('connect ECONNREFUSED 127.0.0.1:5000');
      (error as any).code = 'ECONNREFUSED';

      const result = extractDockerErrorGuidance(error);

      expect(result.message).toContain('Docker daemon is not available');
      expect(result.hint).toBeDefined();
      expect(result.resolution).toBeDefined();
      expect(result.details?.code).toBe('ECONNREFUSED');
    });

    it('should handle ETIMEDOUT errors', () => {
      const error = new Error('connect ETIMEDOUT 192.168.1.1:5000');
      (error as any).code = 'ETIMEDOUT';

      const result = extractDockerErrorGuidance(error);

      expect(result.message).toContain('Docker operation timed out');
      expect(result.hint).toBeDefined();
      expect(result.resolution).toBeDefined();
      expect(result.details?.code).toBe('ETIMEDOUT');
    });

    it('should handle ECONNRESET errors', () => {
      const error = new Error('read ECONNRESET');
      (error as any).code = 'ECONNRESET';

      const result = extractDockerErrorGuidance(error);

      expect(result.message).toContain('Connection reset');
      expect(result.message).toContain('forcibly closed');
      expect(result.hint).toBeDefined();
      expect(result.resolution).toBeDefined();
      expect(result.details?.code).toBe('ECONNRESET');
    });

    it('should handle EAI_AGAIN errors', () => {
      const error = new Error('getaddrinfo EAI_AGAIN registry.docker.io');
      (error as any).code = 'EAI_AGAIN';

      const result = extractDockerErrorGuidance(error);

      expect(result.message).toContain('DNS lookup failed');
      expect(result.message).toContain('Temporary failure');
      expect(result.hint).toBeDefined();
      expect(result.resolution).toBeDefined();
      expect(result.details?.code).toBe('EAI_AGAIN');
    });

    it('should handle EHOSTUNREACH errors', () => {
      const error = new Error('connect EHOSTUNREACH 10.0.0.1:5000');
      (error as any).code = 'EHOSTUNREACH';

      const result = extractDockerErrorGuidance(error);

      expect(result.message).toContain('Host unreachable');
      expect(result.message).toContain('Cannot reach the Docker registry host');
      expect(result.hint).toBeDefined();
      expect(result.resolution).toBeDefined();
      expect(result.details?.code).toBe('EHOSTUNREACH');
    });

    it('should handle ENETUNREACH errors', () => {
      const error = new Error('connect ENETUNREACH');
      (error as any).code = 'ENETUNREACH';

      const result = extractDockerErrorGuidance(error);

      expect(result.message).toContain('Network unreachable');
      expect(result.message).toContain('No route to the Docker registry network');
      expect(result.hint).toBeDefined();
      expect(result.resolution).toBeDefined();
      expect(result.details?.code).toBe('ENETUNREACH');
    });

    it('should handle EPIPE errors', () => {
      const error = new Error('write EPIPE');
      (error as any).code = 'EPIPE';

      const result = extractDockerErrorGuidance(error);

      expect(result.message).toContain('Broken pipe');
      expect(result.message).toContain('unexpectedly closed');
      expect(result.hint).toBeDefined();
      expect(result.resolution).toBeDefined();
      expect(result.details?.code).toBe('EPIPE');
    });
  });

  describe('HTTP status code errors', () => {
    it('should handle 401 authentication errors', () => {
      const error = new Error('unauthorized: authentication required');
      (error as any).statusCode = 401;

      const result = extractDockerErrorGuidance(error);

      expect(result.message).toContain('Docker registry authentication failed');
      expect(result.hint).toBeDefined();
      expect(result.resolution).toBeDefined();
      expect(result.details?.statusCode).toBe(401);
    });

    it('should handle 403 authorization errors', () => {
      const error = new Error('access denied');
      (error as any).statusCode = 403;

      const result = extractDockerErrorGuidance(error);

      expect(result.message).toContain('Authorization error');
      expect(result.message).toContain('Access denied');
      expect(result.hint).toBeDefined();
      expect(result.resolution).toBeDefined();
      expect(result.details?.statusCode).toBe(403);
    });

    it('should handle 404 not found errors', () => {
      const error = new Error('manifest unknown');
      (error as any).statusCode = 404;

      const result = extractDockerErrorGuidance(error);

      expect(result.message).toContain('Image or tag not found');
      expect(result.hint).toBeDefined();
      expect(result.resolution).toBeDefined();
      expect(result.details?.statusCode).toBe(404);
    });

    it('should handle 5xx server errors', () => {
      const error = new Error('internal server error');
      (error as any).statusCode = 500;

      const result = extractDockerErrorGuidance(error);

      expect(result.message).toBe('internal server error');
      expect(result.hint).toBeDefined();
      expect(result.resolution).toBeDefined();
      expect(result.details?.statusCode).toBe(500);
    });
  });

  describe('fallback handling', () => {
    it('should handle regular Error objects without dockerode properties', () => {
      const error = new Error('Something went wrong');

      const result = extractDockerErrorGuidance(error);

      expect(result.message).toBe('Something went wrong');
      expect(result.hint).toBeDefined();
      expect(result.resolution).toBeDefined();
      expect(result.details).toEqual({});
    });

    it('should handle non-Error objects', () => {
      const error = 'string error';

      const result = extractDockerErrorGuidance(error);

      expect(result.message).toBe('string error');
      expect(result.hint).toBeDefined();
      expect(result.resolution).toBeDefined();
      expect(result.details).toEqual({});
    });

    it('should handle null/undefined errors', () => {
      const result = extractDockerErrorGuidance(null);

      expect(result.message).toBe('null');
      expect(result.hint).toBeDefined();
      expect(result.resolution).toBeDefined();
      expect(result.details).toEqual({});
    });

    it('should handle errors with multiple properties', () => {
      const error = new Error('Complex error');
      (error as any).statusCode = 400;
      (error as any).json = { error: 'Bad request' };
      (error as any).reason = 'Invalid parameters';

      const result = extractDockerErrorGuidance(error);

      // Should use the reason as it's longer/more detailed than the message
      expect(result.message).toBe('Invalid parameters');
      expect(result.hint).toBeDefined();
      expect(result.resolution).toBeDefined();
      expect(result.details).toEqual({
        statusCode: 400,
        json: { error: 'Bad request' },
        reason: 'Invalid parameters'
      });
    });
  });
});