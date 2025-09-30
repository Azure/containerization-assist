/**
 * Functional Cache Implementation
 *
 * Simple, efficient caching with TTL support and eviction policies.
 */

import { createHash } from 'crypto';
import { createLogger, type Logger } from '@/lib/logger';

interface CacheEntry<T> {
  value: T;
  expires: number;
  hits: number;
  created: number;
  key: string;
}

export interface Cache<T> {
  get(key: unknown): T | undefined;
  set(key: unknown, value: T): string;
  has(key: unknown): boolean;
  delete(key: unknown): boolean;
  clear(): void;
  size(): number;
  getStats(): CacheStats;
}

export interface CacheStats {
  size: number;
  hits: number;
  misses: number;
  hitRate: number;
  totalRequests: number;
  avgHitsPerEntry: number;
}

/**
 * Generate SHA-256 hash of the key
 */
function hashKey(key: unknown): string {
  const keyString = typeof key === 'string' ? key : JSON.stringify(key, sortObjectKeys);

  return createHash('sha256').update(keyString).digest('hex').substring(0, 16); // Use first 16 chars for brevity
}

/**
 * Sort object keys for consistent hashing
 */
function sortObjectKeys(_key: string, value: unknown): unknown {
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    return Object.keys(value)
      .sort()
      .reduce(
        (sorted, k) => {
          sorted[k] = (value as Record<string, unknown>)[k];
          return sorted;
        },
        {} as Record<string, unknown>,
      );
  }
  return value;
}

/**
 * Evict oldest entries to maintain maxSize
 */
function evictIfNeeded<T>(
  store: Map<string, CacheEntry<T>>,
  maxSize: number,
  logger: Logger,
): void {
  while (store.size >= maxSize) {
    let oldestKey: string | null = null;
    let oldestTime = Infinity;

    for (const [key, entry] of store.entries()) {
      if (entry.created < oldestTime) {
        oldestTime = entry.created;
        oldestKey = key;
      }
    }

    if (oldestKey) {
      const entry = store.get(oldestKey);
      logger.debug(
        {
          key: oldestKey,
          originalKey: entry?.key,
          age: Date.now() - oldestTime,
        },
        'Evicting oldest cache entry',
      );
      store.delete(oldestKey);
    } else {
      break; // Safety: prevent infinite loop
    }
  }
}

/**
 * Create a new cache instance
 */
export function createCache<T>(
  name: string,
  options: {
    maxSize?: number;
    ttlMs?: number;
    enabled?: boolean;
  } = {},
  logger?: Logger,
): Cache<T> {
  const config = {
    maxSize: options.maxSize ?? 100,
    ttlMs: options.ttlMs ?? 5 * 60 * 1000, // 5 minutes default
    enabled: options.enabled ?? true,
  };

  const log = logger || createLogger().child({ module: 'cache', name });
  const store = new Map<string, CacheEntry<T>>();
  const stats = { hits: 0, misses: 0 };

  log.info({ options: config, name }, 'Cache initialized');

  return {
    set(key: unknown, value: T): string {
      if (!config.enabled) {
        return '';
      }

      const hash = hashKey(key);
      evictIfNeeded(store, config.maxSize, log);

      const entry: CacheEntry<T> = {
        value,
        expires: Date.now() + config.ttlMs,
        hits: 0,
        created: Date.now(),
        key: typeof key === 'string' ? key : JSON.stringify(key),
      };

      store.set(hash, entry);

      log.debug(
        {
          hash,
          key: entry.key,
          ttl: config.ttlMs,
          cacheSize: store.size,
        },
        'Cache set',
      );

      return hash;
    },

    get(key: unknown): T | undefined {
      if (!config.enabled) {
        stats.misses++;
        return undefined;
      }

      const hash = hashKey(key);
      const entry = store.get(hash);

      if (!entry) {
        stats.misses++;
        log.debug({ hash, key: typeof key === 'string' ? key : 'object' }, 'Cache miss');
        return undefined;
      }

      // Check if expired
      if (Date.now() > entry.expires) {
        store.delete(hash);
        stats.misses++;
        log.debug(
          {
            hash,
            key: entry.key,
            age: Date.now() - entry.created,
          },
          'Cache expired',
        );
        return undefined;
      }

      entry.hits++;
      stats.hits++;

      log.debug(
        {
          hash,
          key: entry.key,
          hits: entry.hits,
          age: Date.now() - entry.created,
        },
        'Cache hit',
      );

      return entry.value;
    },

    has(key: unknown): boolean {
      if (!config.enabled) return false;

      const hash = hashKey(key);
      const entry = store.get(hash);

      if (!entry) return false;
      if (Date.now() > entry.expires) {
        store.delete(hash);
        return false;
      }

      return true;
    },

    delete(key: unknown): boolean {
      const hash = hashKey(key);
      return store.delete(hash);
    },

    clear(): void {
      const size = store.size;
      store.clear();
      stats.hits = 0;
      stats.misses = 0;
      log.info({ entriesCleared: size }, 'Cache cleared');
    },

    size(): number {
      return store.size;
    },

    getStats(): CacheStats {
      const totalRequests = stats.hits + stats.misses;
      let totalHits = 0;

      for (const entry of store.values()) {
        totalHits += entry.hits;
      }

      return {
        size: store.size,
        hits: stats.hits,
        misses: stats.misses,
        hitRate: totalRequests > 0 ? stats.hits / totalRequests : 0,
        totalRequests,
        avgHitsPerEntry: store.size > 0 ? totalHits / store.size : 0,
      };
    },
  };
}

// Pre-configured cache instances for different use cases
export const cacheInstances = {
  dockerMetadata: createCache<{ size: number; layers: number }>('docker-metadata', {
    ttlMs: 30 * 60 * 1000, // 30 minutes for Docker metadata
    maxSize: 100,
  }),

  scanResults: createCache<unknown>('scan-results', {
    ttlMs: 60 * 60 * 1000, // 1 hour for scan results
    maxSize: 20,
  }),
};
