import { randomUUID } from 'crypto';

interface MutexState {
  locked: boolean;
  queue: Array<() => void>;
  acquiredAt?: number;
  holderId?: string;
}

export interface LockStatus {
  locked: boolean;
  waiters: number;
  heldFor?: number;
  holderId?: string;
}

interface KeyedMutexOptions {
  defaultTimeout: number;
  monitoringEnabled: boolean;
}

export interface KeyedMutexInstance {
  acquire(key: string, timeoutMs?: number): Promise<() => void>;
  withLock<T>(key: string, fn: () => Promise<T>, timeoutMs?: number): Promise<T>;
  getStatus(): Map<string, LockStatus>;
  isLocked(key: string): boolean;
  getWaiters(key: string): number;
  releaseAll(): void;
  getMetrics(): {
    totalKeys: number;
    lockedKeys: number;
    totalWaiters: number;
    longestHeldMs?: number;
  };
}

/**
 * Creates a keyed mutex for preventing concurrent access to resources.
 * Each key has its own lock queue, allowing concurrent access to different resources.
 */
export const createKeyedMutex = (options?: Partial<KeyedMutexOptions>): KeyedMutexInstance => {
  const locks = new Map<string, MutexState>();
  const config: KeyedMutexOptions = {
    defaultTimeout: 30000,
    monitoringEnabled: true,
    ...options,
  };

  const acquire = async (key: string, timeoutMs?: number): Promise<() => void> => {
    const timeout = timeoutMs ?? config.defaultTimeout;
    const holderId = randomUUID();

    if (!locks.has(key)) {
      locks.set(key, { locked: false, queue: [] });
    }

    const lock = locks.get(key);
    if (!lock) {
      throw new Error(`Lock not found for key: ${key}`);
    }
    const deadline = Date.now() + timeout;

    // Wait for lock to be available
    while (lock.locked) {
      if (Date.now() > deadline) {
        throw new Error(`Mutex timeout for key: ${key} (waited ${timeout}ms)`);
      }

      await new Promise<void>((resolve) => {
        const wrappedResolve = (): void => {
          const timeoutId = (resolve as any).timeoutId;
          if (timeoutId) {
            clearTimeout(timeoutId);
          }
          resolve();
        };

        lock.queue.push(wrappedResolve);

        // Set up timeout to auto-resolve and clean up from queue
        const remainingTime = deadline - Date.now();
        if (remainingTime <= 0) {
          // Already past deadline
          const idx = lock.queue.indexOf(wrappedResolve);
          if (idx >= 0) {
            lock.queue.splice(idx, 1);
          }
          resolve();
          return;
        }

        const timeoutId = setTimeout(() => {
          const idx = lock.queue.indexOf(wrappedResolve);
          if (idx >= 0) {
            lock.queue.splice(idx, 1);
            resolve();
          }
        }, remainingTime);

        // Store timeout ID for cleanup
        (resolve as any).timeoutId = timeoutId;
      });
    }

    // Acquire the lock
    lock.locked = true;
    lock.acquiredAt = Date.now();
    lock.holderId = holderId;

    // Return release function
    return (): void => {
      if (lock.holderId !== holderId) {
        throw new Error(`Lock release attempted by non-holder for key: ${key}`);
      }

      lock.locked = false;
      delete lock.acquiredAt;
      delete lock.holderId;

      // Notify next waiter
      const next = lock.queue.shift();
      if (next) {
        next();
      }

      // Clean up if no waiters and not locked
      if (lock.queue.length === 0 && !lock.locked) {
        locks.delete(key);
      }
    };
  };

  const withLock = async <T>(key: string, fn: () => Promise<T>, timeoutMs?: number): Promise<T> => {
    const release = await acquire(key, timeoutMs);
    try {
      return await fn();
    } finally {
      release();
    }
  };

  const getStatus = (): Map<string, LockStatus> => {
    const status = new Map<string, LockStatus>();

    for (const [key, lock] of locks) {
      const lockStatus: LockStatus = {
        locked: lock.locked,
        waiters: lock.queue.length,
      };

      if (lock.acquiredAt !== undefined) {
        lockStatus.heldFor = Date.now() - lock.acquiredAt;
      }

      if (lock.holderId !== undefined) {
        lockStatus.holderId = lock.holderId;
      }

      status.set(key, lockStatus);
    }

    return status;
  };

  const isLocked = (key: string): boolean => {
    return locks.get(key)?.locked ?? false;
  };

  const getWaiters = (key: string): number => {
    return locks.get(key)?.queue.length ?? 0;
  };

  const releaseAll = (): void => {
    for (const [, lock] of locks) {
      lock.locked = false;
      delete lock.acquiredAt;
      delete lock.holderId;

      // Notify all waiters
      for (const resolve of lock.queue) {
        resolve();
      }
      lock.queue = [];
    }
    locks.clear();
  };

  const getMetrics = (): {
    totalKeys: number;
    lockedKeys: number;
    totalWaiters: number;
    longestHeldMs?: number;
  } => {
    let lockedKeys = 0;
    let totalWaiters = 0;
    let longestHeldMs: number | undefined;

    for (const lock of locks.values()) {
      if (lock.locked) {
        lockedKeys++;
        if (lock.acquiredAt) {
          const heldMs = Date.now() - lock.acquiredAt;
          if (!longestHeldMs || heldMs > longestHeldMs) {
            longestHeldMs = heldMs;
          }
        }
      }
      totalWaiters += lock.queue.length;
    }

    const stats: {
      totalKeys: number;
      lockedKeys: number;
      totalWaiters: number;
      longestHeldMs?: number;
    } = {
      totalKeys: locks.size,
      lockedKeys,
      totalWaiters,
    };

    if (longestHeldMs !== undefined && longestHeldMs > 0) {
      stats.longestHeldMs = longestHeldMs;
    }

    return stats;
  };

  return {
    acquire,
    withLock,
    getStatus,
    isLocked,
    getWaiters,
    releaseAll,
    getMetrics,
  };
};
