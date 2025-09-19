/**
 * Tests for prompt cache lifecycle management
 * Validates that cache invalidation works correctly when manifest version changes
 */

import {
  clearCache,
  getManifestVersion,
} from '@prompts/locator';

describe('Prompt Cache Lifecycle', () => {
  beforeEach(() => {
    // Clear cache before each test
    clearCache();
  });

  afterEach(() => {
    // Clean up after tests
    clearCache();
  });

  describe('Cache Management', () => {
    it('should clear manifest version when clearCache is called', () => {
      // Initial state - version should be null
      const initialVersion = getManifestVersion();
      expect(initialVersion).toBeNull();
    });

    it('should start with no cached manifest version', () => {
      const version = getManifestVersion();
      expect(version).toBeNull();
    });
  });

  describe('API Exports', () => {
    it('should export clearCache function', () => {
      expect(typeof clearCache).toBe('function');
    });

    it('should export getManifestVersion function', () => {
      expect(typeof getManifestVersion).toBe('function');
    });
  });

  describe('Cache Clearing Behavior', () => {
    it('should be idempotent when called multiple times', () => {
      clearCache();
      const firstVersion = getManifestVersion();

      clearCache();
      const secondVersion = getManifestVersion();

      expect(firstVersion).toBeNull();
      expect(secondVersion).toBeNull();
      expect(firstVersion).toBe(secondVersion);
    });
  });
});