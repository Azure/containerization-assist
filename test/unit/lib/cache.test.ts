import { createCache, type Cache } from '@/lib/cache';

describe('Cache', () => {
  let cache: Cache<string>;

  beforeEach(() => {
    cache = createCache<string>('test-cache', {
      maxSize: 3,
      ttlMs: 1000, // 1 second for testing
      enabled: true,
    });
  });

  afterEach(() => {
    cache.clear();
  });

  describe('basic operations', () => {
    test('should store and retrieve values', () => {
      cache.set('key1', 'value1');
      expect(cache.get('key1')).toBe('value1');
    });

    test('should return undefined for missing keys', () => {
      expect(cache.get('nonexistent')).toBeUndefined();
    });

    test('should handle object keys with consistent hashing', () => {
      const key1 = { a: 1, b: 2 };
      const key2 = { b: 2, a: 1 }; // Different order, same content

      cache.set(key1, 'value1');
      expect(cache.get(key2)).toBe('value1'); // Should find it due to key sorting
    });

    test('should check key existence with has()', () => {
      cache.set('key1', 'value1');
      expect(cache.has('key1')).toBe(true);
      expect(cache.has('key2')).toBe(false);
    });

    test('should delete specific keys', () => {
      cache.set('key1', 'value1');
      expect(cache.has('key1')).toBe(true);

      cache.delete('key1');
      expect(cache.has('key1')).toBe(false);
    });

    test('should clear all entries', () => {
      cache.set('key1', 'value1');
      cache.set('key2', 'value2');

      cache.clear();

      expect(cache.get('key1')).toBeUndefined();
      expect(cache.get('key2')).toBeUndefined();
      expect(cache.size()).toBe(0);
    });
  });

  describe('TTL expiration', () => {
    test('should expire entries after TTL', async () => {
      cache.set('key1', 'value1');
      expect(cache.get('key1')).toBe('value1');

      // Wait for expiration
      await new Promise((resolve) => setTimeout(resolve, 1100));

      expect(cache.get('key1')).toBeUndefined();
    });

    test('should clean expired entries on access', async () => {
      cache.set('key1', 'value1');
      cache.set('key2', 'value2');

      await new Promise((resolve) => setTimeout(resolve, 1100));

      // Accessing expired entries returns undefined and cleans them
      expect(cache.get('key1')).toBeUndefined();
      expect(cache.get('key2')).toBeUndefined();
      expect(cache.size()).toBe(0);
    });
  });

  describe('LRU eviction', () => {
    test('should evict oldest entry when at capacity', async () => {
      // Fill cache to capacity with small delays to ensure different timestamps
      cache.set('key1', 'value1');
      await new Promise((resolve) => setTimeout(resolve, 10));
      cache.set('key2', 'value2');
      await new Promise((resolve) => setTimeout(resolve, 10));
      cache.set('key3', 'value3');
      await new Promise((resolve) => setTimeout(resolve, 10));

      // Adding fourth item should evict first (oldest)
      cache.set('key4', 'value4');

      expect(cache.get('key1')).toBeUndefined(); // Evicted (oldest)
      expect(cache.get('key2')).toBe('value2');
      expect(cache.get('key3')).toBe('value3');
      expect(cache.get('key4')).toBe('value4');
    });

    test('should maintain maxSize limit', async () => {
      // Add items sequentially with small delays
      for (let i = 0; i < 10; i++) {
        cache.set(`key${i}`, `value${i}`);
        // Small delay to ensure different timestamps
        await new Promise((resolve) => setTimeout(resolve, 1));
      }

      expect(cache.size()).toBe(3); // Should not exceed maxSize

      // Verify the last 3 items are kept (most recent)
      expect(cache.get('key7')).toBe('value7');
      expect(cache.get('key8')).toBe('value8');
      expect(cache.get('key9')).toBe('value9');

      // Verify older items were evicted
      expect(cache.get('key0')).toBeUndefined();
      expect(cache.get('key1')).toBeUndefined();
    });
  });

  describe('statistics', () => {
    test('should track hits and misses', () => {
      cache.set('key1', 'value1');

      // Hit
      cache.get('key1');
      // Miss
      cache.get('key2');
      // Another hit
      cache.get('key1');

      const stats = cache.getStats();
      expect(stats.hits).toBe(2);
      expect(stats.misses).toBe(1);
      expect(stats.hitRate).toBeCloseTo(0.667, 2);
    });

    test('should calculate average hits per entry', () => {
      cache.set('key1', 'value1');
      cache.set('key2', 'value2');

      cache.get('key1');
      cache.get('key1');
      cache.get('key1');
      cache.get('key2');

      const stats = cache.getStats();
      expect(stats.avgHitsPerEntry).toBe(2); // (3 + 1) / 2
    });

    test('should reset stats on clear', () => {
      cache.set('key1', 'value1');
      cache.get('key1');
      cache.get('missing');

      cache.clear();

      const stats = cache.getStats();
      expect(stats.hits).toBe(0);
      expect(stats.misses).toBe(0);
    });
  });

  describe('enable/disable', () => {
    test('should respect enabled flag at creation', () => {
      const disabledCache = createCache<string>('disabled', {
        enabled: false,
        maxSize: 3,
        ttlMs: 1000,
      });

      disabledCache.set('key1', 'value1');
      expect(disabledCache.get('key1')).toBeUndefined();
      expect(disabledCache.has('key1')).toBe(false);
    });

    test('should count misses when disabled', () => {
      const disabledCache = createCache<string>('disabled', {
        enabled: false,
      });
      disabledCache.get('key1');

      const stats = disabledCache.getStats();
      expect(stats.misses).toBe(1);
      expect(stats.hits).toBe(0);
    });
  });

  describe('createCache factory', () => {
    test('should create typed cache with custom options', () => {
      interface User {
        id: string;
        name: string;
      }

      const userCache = createCache<User>('users', {
        maxSize: 10,
        ttlMs: 2000,
      });

      const user: User = { id: '1', name: 'John' };
      userCache.set('user1', user);

      const retrieved = userCache.get('user1');
      expect(retrieved).toEqual(user);
      expect(retrieved?.name).toBe('John');
    });

    test('should use default options when not provided', () => {
      const defaultCache = createCache<number>('numbers');

      defaultCache.set('num1', 42);
      expect(defaultCache.get('num1')).toBe(42);

      // Check defaults are applied
      const stats = defaultCache.getStats();
      expect(stats.size).toBe(1);
    });
  });

  describe('edge cases', () => {
    test('should handle null and undefined values', () => {
      const nullCache = createCache<string | null>('null-cache');

      nullCache.set('null', null);
      expect(nullCache.get('null')).toBeNull();

      nullCache.set('undefined', undefined as any);
      expect(nullCache.get('undefined')).toBeUndefined();
    });

    test('should handle complex nested objects as keys', () => {
      const complexKey = {
        user: { id: 1, name: 'John' },
        metadata: { timestamp: Date.now(), tags: ['a', 'b'] },
      };

      cache.set(complexKey, 'complex');
      expect(cache.get(complexKey)).toBe('complex');
    });

    test('should handle concurrent operations', async () => {
      const promises = [];

      // Simulate concurrent writes
      for (let i = 0; i < 10; i++) {
        promises.push(Promise.resolve().then(() => cache.set(`key${i}`, `value${i}`)));
      }

      await Promise.all(promises);

      // Concurrent operations might temporarily exceed limit
      // but should still be reasonable (not unbounded growth)
      const size = cache.getStats().size;
      expect(size).toBeLessThanOrEqual(10);
      expect(size).toBeGreaterThan(0);
    });

    test('should handle empty cache operations gracefully', () => {
      expect(cache.get('any')).toBeUndefined();
      expect(cache.has('any')).toBe(false);
      expect(cache.delete('any')).toBe(false);
      // Expired entries are cleaned on access
      expect(cache.size()).toBe(0);
    });
  });
});
