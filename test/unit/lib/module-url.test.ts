import { getModuleUrl, isESMContext } from '@/lib/module-url';

describe('Module URL Utilities', () => {
  describe('getModuleUrl', () => {
    it('should return a string or undefined', () => {
      const url = getModuleUrl();
      expect(typeof url === 'string' || url === undefined).toBe(true);
    });

    it('should return file:// URL in ESM builds', () => {
      const url = getModuleUrl();
      // In Jest + ts-jest, this may vary based on configuration
      // Just verify it doesn't throw
      if (url) {
        expect(url.startsWith('file://')).toBe(true);
      }
    });

    it('should handle errors gracefully', () => {
      // Should not throw, even in unusual environments
      expect(() => getModuleUrl()).not.toThrow();
    });
  });

  describe('isESMContext', () => {
    it('should return a boolean', () => {
      const result = isESMContext();
      expect(typeof result).toBe('boolean');
    });

    it('should be consistent with getModuleUrl', () => {
      const url = getModuleUrl();
      const isESM = isESMContext();

      if (url !== undefined) {
        expect(isESM).toBe(true);
      } else {
        expect(isESM).toBe(false);
      }
    });
  });
});
