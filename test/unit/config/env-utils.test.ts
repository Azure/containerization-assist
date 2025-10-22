/**
 * Unit tests for environment variable parsing utilities
 */
import { parseIntEnv, parseStringEnv, parseBoolEnv, parseListEnv } from '@/config/env-utils';

describe('env-utils', () => {
  const originalEnv = process.env;

  beforeEach(() => {
    // Create a fresh copy of environment for each test
    process.env = { ...originalEnv };
  });

  afterAll(() => {
    // Restore original environment
    process.env = originalEnv;
  });

  describe('parseIntEnv', () => {
    it('should return parsed integer when valid', () => {
      process.env.TEST_INT = '42';
      expect(parseIntEnv('TEST_INT', 10)).toBe(42);
    });

    it('should return default when environment variable is not set', () => {
      delete process.env.TEST_INT;
      expect(parseIntEnv('TEST_INT', 10)).toBe(10);
    });

    it('should return default when value is not a valid number', () => {
      process.env.TEST_INT = 'not-a-number';
      expect(parseIntEnv('TEST_INT', 10)).toBe(10);
    });

    it('should return default when value is empty string', () => {
      process.env.TEST_INT = '';
      expect(parseIntEnv('TEST_INT', 10)).toBe(10);
    });

    it('should handle negative numbers', () => {
      process.env.TEST_INT = '-42';
      expect(parseIntEnv('TEST_INT', 10)).toBe(-42);
    });

    it('should handle zero', () => {
      process.env.TEST_INT = '0';
      expect(parseIntEnv('TEST_INT', 10)).toBe(0);
    });

    it('should truncate decimal values', () => {
      process.env.TEST_INT = '42.7';
      expect(parseIntEnv('TEST_INT', 10)).toBe(42);
    });
  });

  describe('parseStringEnv', () => {
    it('should return environment variable value when set', () => {
      process.env.TEST_STRING = 'hello';
      expect(parseStringEnv('TEST_STRING', 'default')).toBe('hello');
    });

    it('should return default when environment variable is not set', () => {
      delete process.env.TEST_STRING;
      expect(parseStringEnv('TEST_STRING', 'default')).toBe('default');
    });

    it('should return default when value is empty string', () => {
      process.env.TEST_STRING = '';
      expect(parseStringEnv('TEST_STRING', 'default')).toBe('default');
    });

    it('should preserve whitespace in non-empty values', () => {
      process.env.TEST_STRING = '  spaces  ';
      expect(parseStringEnv('TEST_STRING', 'default')).toBe('  spaces  ');
    });

    it('should handle numeric strings', () => {
      process.env.TEST_STRING = '123';
      expect(parseStringEnv('TEST_STRING', 'default')).toBe('123');
    });
  });

  describe('parseBoolEnv', () => {
    describe('truthy values', () => {
      it.each(['true', 'TRUE', 'True', '1', 'yes', 'YES', 'Yes'])(
        'should return true for "%s"',
        (value) => {
          process.env.TEST_BOOL = value;
          expect(parseBoolEnv('TEST_BOOL', false)).toBe(true);
        },
      );
    });

    describe('falsy values', () => {
      it.each(['false', 'FALSE', 'False', '0', 'no', 'NO', 'No'])(
        'should return false for "%s"',
        (value) => {
          process.env.TEST_BOOL = value;
          expect(parseBoolEnv('TEST_BOOL', true)).toBe(false);
        },
      );
    });

    it('should return default when environment variable is not set', () => {
      delete process.env.TEST_BOOL;
      expect(parseBoolEnv('TEST_BOOL', true)).toBe(true);
      expect(parseBoolEnv('TEST_BOOL', false)).toBe(false);
    });

    it('should return default for unrecognized values', () => {
      process.env.TEST_BOOL = 'maybe';
      expect(parseBoolEnv('TEST_BOOL', true)).toBe(true);
      expect(parseBoolEnv('TEST_BOOL', false)).toBe(false);
    });

    it('should return default for empty string', () => {
      process.env.TEST_BOOL = '';
      expect(parseBoolEnv('TEST_BOOL', true)).toBe(true);
    });
  });

  describe('parseListEnv', () => {
    it('should parse comma-separated values', () => {
      process.env.TEST_LIST = 'one,two,three';
      expect(parseListEnv('TEST_LIST')).toEqual(['one', 'two', 'three']);
    });

    it('should trim whitespace from items', () => {
      process.env.TEST_LIST = 'one , two , three';
      expect(parseListEnv('TEST_LIST')).toEqual(['one', 'two', 'three']);
    });

    it('should filter out empty strings', () => {
      process.env.TEST_LIST = 'one,,two,,,three';
      expect(parseListEnv('TEST_LIST')).toEqual(['one', 'two', 'three']);
    });

    it('should return empty array when environment variable is not set', () => {
      delete process.env.TEST_LIST;
      expect(parseListEnv('TEST_LIST')).toEqual([]);
    });

    it('should return empty array when value is empty string', () => {
      process.env.TEST_LIST = '';
      expect(parseListEnv('TEST_LIST')).toEqual([]);
    });

    it('should handle single value', () => {
      process.env.TEST_LIST = 'single';
      expect(parseListEnv('TEST_LIST')).toEqual(['single']);
    });

    it('should support custom delimiter', () => {
      process.env.TEST_LIST = 'one;two;three';
      expect(parseListEnv('TEST_LIST', ';')).toEqual(['one', 'two', 'three']);
    });

    it('should handle values with spaces when trimmed', () => {
      process.env.TEST_LIST = '  one  ,  two  ,  three  ';
      expect(parseListEnv('TEST_LIST')).toEqual(['one', 'two', 'three']);
    });

    it('should handle mixed empty and whitespace items', () => {
      process.env.TEST_LIST = 'one,  ,two,   ,three';
      expect(parseListEnv('TEST_LIST')).toEqual(['one', 'two', 'three']);
    });
  });

  describe('integration scenarios', () => {
    it('should handle typical server configuration', () => {
      process.env.PORT = '8080';
      process.env.LOG_LEVEL = 'debug';
      process.env.ENABLE_METRICS = 'true';

      expect(parseIntEnv('PORT', 3000)).toBe(8080);
      expect(parseStringEnv('LOG_LEVEL', 'info')).toBe('debug');
      expect(parseBoolEnv('ENABLE_METRICS', false)).toBe(true);
    });

    it('should handle missing configuration gracefully', () => {
      delete process.env.PORT;
      delete process.env.LOG_LEVEL;
      delete process.env.ENABLE_METRICS;

      expect(parseIntEnv('PORT', 3000)).toBe(3000);
      expect(parseStringEnv('LOG_LEVEL', 'info')).toBe('info');
      expect(parseBoolEnv('ENABLE_METRICS', false)).toBe(false);
    });

    it('should handle allowlist/denylist patterns', () => {
      process.env.IMAGE_ALLOWLIST = 'nginx:latest, alpine:3.14, node:16';
      process.env.IMAGE_DENYLIST = '';

      expect(parseListEnv('IMAGE_ALLOWLIST')).toEqual(['nginx:latest', 'alpine:3.14', 'node:16']);
      expect(parseListEnv('IMAGE_DENYLIST')).toEqual([]);
    });
  });
});
