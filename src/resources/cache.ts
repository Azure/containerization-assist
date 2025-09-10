import type { Logger } from 'pino';
import { Result, Success, Failure } from '@types';
import type { ResourceCache } from './types';

interface CacheEntry {
  value: unknown;
  expiresAt?: number;
  createdAt: number;
}

interface CacheStore {
  cache: Map<string, CacheEntry>;
  cleanupInterval?: NodeJS.Timeout;
  logger: Logger;
  defaultTtl: number;
}

/**
 * Create a cache store with configuration
 */
function createCacheStore(
  defaultTtl: number = 3600000, // 1 hour default
  logger: Logger,
): CacheStore {
  const store: CacheStore = {
    cache: new Map(),
    logger: logger.child({ component: 'MemoryResourceCache' }),
    defaultTtl,
  };

  // Start cleanup every 5 minutes
  store.cleanupInterval = setInterval(
    () => {
      cleanupExpired(store).catch((error) => {
        store.logger.error({ error }, 'Failed to cleanup expired cache entries');
      });
    },
    5 * 60 * 1000,
  );

  return store;
}

/**
 * Cleanup expired entries
 */
async function cleanupExpired(store: CacheStore): Promise<Result<number>> {
  try {
    const now = Date.now();
    let cleanedCount = 0;

    for (const [key, entry] of store.cache.entries()) {
      if (entry.expiresAt && now > entry.expiresAt) {
        store.cache.delete(key);
        cleanedCount++;
      }
    }

    if (cleanedCount > 0) {
      store.logger.debug({ cleanedCount }, 'Cleaned up expired cache entries');
    }

    return Success(cleanedCount);
  } catch (error) {
    store.logger.error({ error }, 'Failed to cleanup expired entries');
    return Failure(
      `Failed to cleanup expired entries: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
}

/**
 * Set a cache entry
 */
export async function set(
  store: CacheStore,
  key: string,
  value: unknown,
  ttl?: number,
): Promise<Result<void>> {
  try {
    const now = Date.now();
    const effectiveTtl = ttl ?? store.defaultTtl;

    const entry: CacheEntry = {
      value,
      createdAt: now,
    };

    if (effectiveTtl > 0) {
      entry.expiresAt = now + effectiveTtl;
    }

    store.cache.set(key, entry);

    store.logger.debug(
      {
        key,
        ttl: effectiveTtl,
        expiresAt: entry.expiresAt,
        size: store.cache.size,
      },
      'Cache entry set',
    );

    return Success(undefined);
  } catch (error) {
    store.logger.error({ error, key }, 'Failed to set cache entry');
    return Failure(
      `Failed to set cache entry: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
}

/**
 * Get a cache entry
 */
export async function get(store: CacheStore, key: string): Promise<Result<unknown>> {
  try {
    const entry = store.cache.get(key);

    if (!entry) {
      store.logger.debug({ key }, 'Cache miss');
      return Success(null);
    }

    // Check expiration
    if (entry.expiresAt && Date.now() > entry.expiresAt) {
      store.cache.delete(key);
      store.logger.debug({ key, expiresAt: entry.expiresAt }, 'Cache entry expired');
      return Success(null);
    }

    store.logger.debug({ key }, 'Cache hit');
    return Success(entry.value);
  } catch (error) {
    store.logger.error({ error, key }, 'Failed to get cache entry');
    return Failure(
      `Failed to get cache entry: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
}

/**
 * Delete a cache entry
 */
export async function deleteEntry(store: CacheStore, key: string): Promise<Result<boolean>> {
  try {
    const deleted = store.cache.delete(key);
    store.logger.debug({ key, deleted }, 'Cache entry deleted');
    return Success(deleted);
  } catch (error) {
    store.logger.error({ error, key }, 'Failed to delete cache entry');
    return Failure(
      `Failed to delete cache entry: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
}

/**
 * Clear all cache entries
 */
export async function clear(store: CacheStore): Promise<Result<void>> {
  try {
    const size = store.cache.size;
    store.cache.clear();
    store.logger.debug({ clearedCount: size }, 'Cache cleared');
    return Success(undefined);
  } catch (error) {
    store.logger.error({ error }, 'Failed to clear cache');
    return Failure(
      `Failed to clear cache: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
}

/**
 * Check if cache has a key
 */
export async function has(store: CacheStore, key: string): Promise<Result<boolean>> {
  try {
    const entry = store.cache.get(key);

    if (!entry) {
      return Success(false);
    }

    // Check expiration
    if (entry.expiresAt && Date.now() > entry.expiresAt) {
      store.cache.delete(key);
      return Success(false);
    }

    return Success(true);
  } catch (error) {
    store.logger.error({ error, key }, 'Failed to check cache entry existence');
    return Failure(
      `Failed to check cache entry existence: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
}

/**
 * Invalidate entries matching a pattern
 */
export async function invalidate(
  store: CacheStore,
  pattern: string | { tags?: string[]; keyPattern?: string },
): Promise<Result<number>> {
  try {
    let invalidatedCount = 0;
    const patternStr = typeof pattern === 'string' ? pattern : pattern.keyPattern;

    if (patternStr) {
      const regex = new RegExp(patternStr);
      for (const key of store.cache.keys()) {
        if (regex.test(key)) {
          store.cache.delete(key);
          invalidatedCount++;
        }
      }
    }

    store.logger.debug({ pattern, invalidatedCount }, 'Cache entries invalidated');
    return Success(invalidatedCount);
  } catch (error) {
    store.logger.error({ error, pattern }, 'Failed to invalidate cache entries');
    return Failure(
      `Failed to invalidate cache entries: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
}

/**
 * Get all keys matching a pattern
 */
export function keys(store: CacheStore, pattern?: string): string[] {
  const allKeys = Array.from(store.cache.keys());

  if (!pattern) {
    return allKeys;
  }

  // Escape all RegExp special characters except glob wildcards (* ? [ ])
  // This prevents injection of unintended RegExp patterns
  const escapedPattern = pattern.replace(/[.+^${}()|\\]/g, '\\$&');

  // Now safely replace glob wildcards with their RegExp equivalents
  const regex = new RegExp(
    escapedPattern
      .replace(/\*/g, '.*')
      .replace(/\?/g, '.')
      .replace(/\\\[/g, '[') // Unescape [ that was escaped above
      .replace(/\\\]/g, ']'), // Unescape ] that was escaped above
  );

  return allKeys.filter((key) => regex.test(key));
}

/**
 * Get cache statistics
 */
export function getStats(store: CacheStore): {
  size: number;
  hitRate: number;
  memoryUsage: number;
} {
  let totalSize = 0;
  for (const [key, entry] of store.cache.entries()) {
    totalSize += JSON.stringify({ key, value: entry.value }).length;
  }

  return {
    size: store.cache.size,
    hitRate: 0,
    memoryUsage: totalSize,
  };
}

/**
 * Destroy the cache and cleanup resources
 */
export function destroy(store: CacheStore): void {
  if (store.cleanupInterval) {
    clearInterval(store.cleanupInterval);
    delete store.cleanupInterval;
  }
  store.cache.clear();
  store.logger.debug('Cache destroyed');
}

/**
 * Extended cache interface with additional methods
 */
export interface ExtendedResourceCache extends ResourceCache {
  getStats(): { size: number; hitRate: number; memoryUsage: number };
  destroy(): void;
}

/**
 * Create a memory-based resource cache
 */
export function createMemoryResourceCache(
  defaultTtl: number = 3600000,
  logger: Logger,
): ExtendedResourceCache {
  const store = createCacheStore(defaultTtl, logger);

  return {
    set: (key: string, value: unknown, ttl?: number) => set(store, key, value, ttl),
    get: (key: string) => get(store, key),
    delete: (key: string) => deleteEntry(store, key),
    clear: () => clear(store),
    has: (key: string) => has(store, key),
    invalidate: (pattern: string | { tags?: string[]; keyPattern?: string }) =>
      invalidate(store, pattern),
    keys: (pattern?: string) => keys(store, pattern),
    getStats: () => getStats(store),
    destroy: () => destroy(store),
  };
}
