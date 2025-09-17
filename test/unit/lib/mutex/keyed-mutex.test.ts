import { createKeyedMutex, type KeyedMutexInstance } from '@/lib/mutex';

describe('KeyedMutex', () => {
  let mutex: KeyedMutexInstance;

  beforeEach(() => {
    mutex = createKeyedMutex({ defaultTimeout: 1000 });
  });

  afterEach(() => {
    mutex.releaseAll();
  });

  describe('basic locking', () => {
    test('should acquire and release lock', async () => {
      const release = await mutex.acquire('test-key');
      expect(mutex.isLocked('test-key')).toBe(true);

      release();
      expect(mutex.isLocked('test-key')).toBe(false);
    });

    test('should prevent concurrent access to same key', async () => {
      const results: number[] = [];
      const release1 = await mutex.acquire('key1');

      // Try to acquire same key - should wait
      const promise2 = mutex.acquire('key1').then((release) => {
        results.push(2);
        release();
      });

      // First lock holder does work
      results.push(1);
      release1();

      // Wait for second to complete
      await promise2;

      expect(results).toEqual([1, 2]);
    });

    test('should allow concurrent access to different keys', async () => {
      const results: string[] = [];

      const promise1 = mutex.withLock('key1', async () => {
        await new Promise((resolve) => setTimeout(resolve, 50));
        results.push('key1');
      });

      const promise2 = mutex.withLock('key2', async () => {
        await new Promise((resolve) => setTimeout(resolve, 50));
        results.push('key2');
      });

      await Promise.all([promise1, promise2]);

      // Both should complete roughly at the same time
      expect(results).toHaveLength(2);
      expect(results).toContain('key1');
      expect(results).toContain('key2');
    });
  });

  describe('timeout behavior', () => {
    test('should timeout if lock not acquired', async () => {
      const release1 = await mutex.acquire('timeout-key');

      // Try to acquire with short timeout
      await expect(mutex.acquire('timeout-key', 100)).rejects.toThrow(
        'Mutex timeout for key: timeout-key',
      );

      release1();
    });

    test('should handle sequential lock acquisitions', async () => {
      // First acquisition
      const release1 = await mutex.acquire('sequential-test');
      expect(mutex.isLocked('sequential-test')).toBe(true);

      // Release first
      release1();
      expect(mutex.isLocked('sequential-test')).toBe(false);

      // Second acquisition should work immediately
      const release2 = await mutex.acquire('sequential-test');
      expect(mutex.isLocked('sequential-test')).toBe(true);

      release2();
      expect(mutex.isLocked('sequential-test')).toBe(false);
    });
  });

  describe('withLock helper', () => {
    test('should execute function with lock', async () => {
      let executed = false;

      await mutex.withLock('with-lock-key', async () => {
        expect(mutex.isLocked('with-lock-key')).toBe(true);
        executed = true;
      });

      expect(executed).toBe(true);
      expect(mutex.isLocked('with-lock-key')).toBe(false);
    });

    test('should release lock even on error', async () => {
      await expect(
        mutex.withLock('error-key', async () => {
          expect(mutex.isLocked('error-key')).toBe(true);
          throw new Error('Test error');
        }),
      ).rejects.toThrow('Test error');

      expect(mutex.isLocked('error-key')).toBe(false);
    });

    test('should return function result', async () => {
      const result = await mutex.withLock('result-key', async () => {
        return 'test-result';
      });

      expect(result).toBe('test-result');
    });
  });

  describe('queue management', () => {
    test('should handle multiple waiters', async () => {
      const order: number[] = [];
      const release1 = await mutex.acquire('queue-key');

      // Queue up multiple waiters
      const promises = [2, 3, 4].map((n) =>
        mutex.acquire('queue-key').then((release) => {
          order.push(n);
          release();
        }),
      );

      // Check waiters count
      expect(mutex.getWaiters('queue-key')).toBe(3);

      // Release first lock
      order.push(1);
      release1();

      // Wait for all to complete
      await Promise.all(promises);

      // Should execute in order
      expect(order).toEqual([1, 2, 3, 4]);
    });

    test('should clean up empty locks', async () => {
      const release = await mutex.acquire('cleanup-key');
      expect(mutex.getStatus().has('cleanup-key')).toBe(true);

      release();
      expect(mutex.getStatus().has('cleanup-key')).toBe(false);
    });
  });

  describe('monitoring and metrics', () => {
    test('should track lock status', async () => {
      const release = await mutex.acquire('status-key');
      const status = mutex.getStatus().get('status-key');

      expect(status).toBeDefined();
      expect(status?.locked).toBe(true);
      expect(status?.waiters).toBe(0);
      expect(status?.heldFor).toBeGreaterThanOrEqual(0);

      release();
    });

    test('should provide metrics', async () => {
      const release1 = await mutex.acquire('metrics-key1');
      const release2 = await mutex.acquire('metrics-key2');

      // Queue a waiter
      const promise3 = mutex.acquire('metrics-key1');

      const metrics = mutex.getMetrics();
      expect(metrics.totalKeys).toBe(2);
      expect(metrics.lockedKeys).toBe(2);
      expect(metrics.totalWaiters).toBe(1);
      if (metrics.longestHeldMs !== undefined) {
        expect(metrics.longestHeldMs).toBeGreaterThanOrEqual(0);
      }

      release1();
      release2();
      const release3 = await promise3;
      release3();
    });
  });

  describe('error cases', () => {
    test('should prevent double release', async () => {
      const release = await mutex.acquire('double-release');
      release();

      expect(() => release()).toThrow('Lock release attempted by non-holder');
    });

    test('should handle concurrent acquire attempts', async () => {
      const promises = Array.from({ length: 10 }, (_, i) =>
        mutex.withLock('concurrent-key', async () => {
          await new Promise((resolve) => setTimeout(resolve, 10));
          return i;
        }),
      );

      const results = await Promise.all(promises);
      expect(results).toHaveLength(10);
      expect(new Set(results).size).toBe(10); // All unique
    });
  });

  describe('stress testing', () => {
    test('should handle high concurrency without deadlock', async () => {
      const operations = 100;
      const keys = ['key-a', 'key-b', 'key-c'];
      const completed = new Set<number>();

      const promises = Array.from({ length: operations }, (_, i) =>
        mutex.withLock(keys[i % keys.length], async () => {
          await new Promise((resolve) => setTimeout(resolve, Math.random() * 10));
          completed.add(i);
        }),
      );

      await Promise.all(promises);
      expect(completed.size).toBe(operations);
    });

    test('should not leak memory', async () => {
      for (let i = 0; i < 1000; i++) {
        const release = await mutex.acquire(`leak-test-${i}`);
        release();
      }

      // All locks should be cleaned up
      expect(mutex.getStatus().size).toBe(0);
    });
  });
});
