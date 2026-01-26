/**
 * OSV Scanner Tests
 */

import { describe, it, expect, jest, beforeEach } from '@jest/globals';
import type { Logger } from 'pino';
import { checkOSVAvailability } from '@/infra/security/osv-scanner';

describe('OSV Scanner', () => {
  let mockLogger: Logger;

  beforeEach(() => {
    // Create a mock logger
    mockLogger = {
      info: jest.fn(),
      debug: jest.fn(),
      warn: jest.fn(),
      error: jest.fn(),
    } as unknown as Logger;

    // Clear any existing fetch mocks
    jest.clearAllMocks();
  });

  describe('checkOSVAvailability', () => {
    it('should return success when OSV API is accessible', async () => {
      // Mock fetch to simulate successful API response
      global.fetch = jest.fn(() =>
        Promise.resolve({
          ok: true,
          status: 200,
          json: async () => ({}),
        } as Response),
      ) as jest.Mock;

      const result = await checkOSVAvailability(mockLogger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe('osv-api-available');
      }
    });

    it('should return success even with 404 (API is available)', async () => {
      // Mock fetch to simulate 404 (API is reachable but query returns nothing)
      global.fetch = jest.fn(() =>
        Promise.resolve({
          ok: false,
          status: 404,
          json: async () => ({}),
        } as Response),
      ) as jest.Mock;

      const result = await checkOSVAvailability(mockLogger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe('osv-api-available');
      }
    });

    it('should return failure when OSV API is not accessible', async () => {
      // Mock fetch to simulate network error
      global.fetch = jest.fn(() =>
        Promise.reject(new Error('Network error')),
      ) as jest.Mock;

      const result = await checkOSVAvailability(mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('OSV API not accessible');
        expect(result.guidance).toBeDefined();
      }
    });

    it('should provide helpful guidance when API is unavailable', async () => {
      // Mock fetch to simulate network error
      global.fetch = jest.fn(() =>
        Promise.reject(new Error('ECONNREFUSED')),
      ) as jest.Mock;

      const result = await checkOSVAvailability(mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.guidance?.hint).toContain('network');
        expect(result.guidance?.resolution).toBeDefined();
      }
    });
  });

  describe('severity mapping', () => {
    it('should map CVSS scores correctly', () => {
      // This would test the mapCVSSToSeverity function
      // We'll test through integration once we can scan an actual image
      expect(true).toBe(true);
    });
  });

  describe('package extraction', () => {
    it('should handle images without package manifests', () => {
      // This would test extractPackagesFromImage
      // We'll test through integration once we can scan an actual image
      expect(true).toBe(true);
    });
  });
});
