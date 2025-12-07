/**
 * Unit tests for CEL Compilation Cache
 */

import { celCompilationCache } from '@/config/policy-cel-cache';
import { parse } from '@marcbachmann/cel-js';

describe('CelCompilationCache', () => {
  beforeEach(() => {
    // Clear cache before each test
    celCompilationCache.clear();
  });

  describe('set and get', () => {
    it('should cache compiled programs', () => {
      const condition = 'input.content.contains("test")';

      expect(celCompilationCache.has(condition)).toBe(false);

      const program = parse(condition);
      celCompilationCache.set(condition, program);

      expect(celCompilationCache.has(condition)).toBe(true);
    });

    it('should retrieve cached programs', () => {
      const condition = 'input.content.contains("test")';
      const program = parse(condition);

      celCompilationCache.set(condition, program);
      const cached = celCompilationCache.get(condition);

      expect(cached).toBe(program);
    });

    it('should return null for uncached conditions', () => {
      const cached = celCompilationCache.get('uncached condition');
      expect(cached).toBeNull();
    });

    it('should cache multiple programs', () => {
      const condition1 = 'true';
      const condition2 = 'false';
      const condition3 = 'input.content.size() > 0';

      celCompilationCache.set(condition1, parse(condition1));
      celCompilationCache.set(condition2, parse(condition2));
      celCompilationCache.set(condition3, parse(condition3));

      expect(celCompilationCache.has(condition1)).toBe(true);
      expect(celCompilationCache.has(condition2)).toBe(true);
      expect(celCompilationCache.has(condition3)).toBe(true);
    });
  });

  describe('has', () => {
    it('should return false for uncached condition', () => {
      expect(celCompilationCache.has('not cached')).toBe(false);
    });

    it('should return true for cached condition', () => {
      const condition = 'true';
      celCompilationCache.set(condition, parse(condition));

      expect(celCompilationCache.has(condition)).toBe(true);
    });
  });

  describe('clear', () => {
    it('should clear all cached entries', () => {
      celCompilationCache.set('condition1', parse('true'));
      celCompilationCache.set('condition2', parse('false'));

      expect(celCompilationCache.getStats().size).toBe(2);

      celCompilationCache.clear();

      expect(celCompilationCache.getStats().size).toBe(0);
    });

    it('should be safe to call on empty cache', () => {
      celCompilationCache.clear();
      expect(celCompilationCache.getStats().size).toBe(0);
    });
  });

  describe('getStats', () => {
    it('should return empty stats for empty cache', () => {
      const stats = celCompilationCache.getStats();

      expect(stats.size).toBe(0);
      expect(stats.entries).toHaveLength(0);
    });

    it('should provide cache statistics', () => {
      const condition = 'true';
      celCompilationCache.set(condition, parse(condition));

      const stats = celCompilationCache.getStats();

      expect(stats.size).toBe(1);
      expect(stats.entries).toHaveLength(1);
      expect(stats.entries[0].condition).toBe('true');
      expect(stats.entries[0].compiledAt).toBeInstanceOf(Date);
    });

    it('should truncate long conditions in stats', () => {
      const longCondition = 'a'.repeat(200);
      // We can't parse a string of 'a's as CEL, so use a valid condition
      const validCondition = `input.content.contains("${'a'.repeat(150)}")`;
      celCompilationCache.set(validCondition, parse('true'));

      const stats = celCompilationCache.getStats();

      // Condition should be truncated to 100 characters
      expect(stats.entries[0].condition.length).toBeLessThanOrEqual(100);
    });
  });

  describe('cache key generation', () => {
    it('should use content hash for cache key', () => {
      // Same condition should hit cache
      const condition = 'input.content.contains("test")';

      celCompilationCache.set(condition, parse(condition));
      expect(celCompilationCache.has(condition)).toBe(true);

      // Identical condition string should also hit cache
      const identicalCondition = 'input.content.contains("test")';
      expect(celCompilationCache.has(identicalCondition)).toBe(true);
    });

    it('should differentiate similar but different conditions', () => {
      const condition1 = 'input.content.contains("test1")';
      const condition2 = 'input.content.contains("test2")';

      celCompilationCache.set(condition1, parse(condition1));

      expect(celCompilationCache.has(condition1)).toBe(true);
      expect(celCompilationCache.has(condition2)).toBe(false);
    });

    it('should handle empty string condition', () => {
      // Empty string is technically a valid condition that returns empty string
      // But for cache purposes, we just test the hashing works
      celCompilationCache.set('', parse('true')); // Use valid CEL for parse

      expect(celCompilationCache.has('')).toBe(true);
    });

    it('should handle special characters in conditions', () => {
      const condition = '!input.content.contains("USER") && input.content.size() > 0';

      celCompilationCache.set(condition, parse(condition));

      expect(celCompilationCache.has(condition)).toBe(true);
      expect(celCompilationCache.get(condition)).not.toBeNull();
    });

    it('should handle unicode in conditions', () => {
      const condition = 'input.content.contains("日本語")';

      celCompilationCache.set(condition, parse(condition));

      expect(celCompilationCache.has(condition)).toBe(true);
    });
  });

  describe('program execution', () => {
    it('should return programs that can be executed', () => {
      const condition = 'input.content.contains("test")';
      const program = parse(condition);

      celCompilationCache.set(condition, program);
      const cached = celCompilationCache.get(condition);

      expect(cached).not.toBeNull();

      // Execute the cached program
      const result = cached!({ input: { content: 'this is a test string' } });
      expect(result).toBe(true);
    });

    it('should cache programs that work for multiple inputs', () => {
      const condition = 'input.content.size() > 5';
      const program = parse(condition);

      celCompilationCache.set(condition, program);
      const cached = celCompilationCache.get(condition);

      expect(cached).not.toBeNull();

      // Execute with different inputs
      expect(cached!({ input: { content: 'short' } })).toBe(false);
      expect(cached!({ input: { content: 'longer string' } })).toBe(true);
    });
  });

  describe('overwriting entries', () => {
    it('should overwrite existing entry with same condition', () => {
      const condition = 'true';
      const program1 = parse(condition);
      const program2 = parse(condition);

      celCompilationCache.set(condition, program1);
      const first = celCompilationCache.get(condition);

      celCompilationCache.set(condition, program2);
      const second = celCompilationCache.get(condition);

      // Should have replaced the entry
      expect(second).toBe(program2);
      expect(celCompilationCache.getStats().size).toBe(1);
    });
  });
});
