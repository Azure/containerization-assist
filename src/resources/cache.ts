import type { Logger } from 'pino';
import { Result, Success, Failure } from '@types';
import type { ResourceCache } from './types';

interface CacheEntry {
  value: unknown;
  expiresAt?: number;
  createdAt: number;
}

export class MemoryResourceCache implements ResourceCache {
  private cache = new Map<string, CacheEntry>();
  private cleanupInterval?: NodeJS.Timeout;
  private readonly logger: Logger;

  constructor(
    private readonly defaultTtl: number = 3600000, // 1 hour default
    logger: Logger,
  ) {
    this.logger = logger.child({ component: 'MemoryResourceCache' });

    // Start cleanup every 5 minutes
    this.cleanupInterval = setInterval(
      () => {
        this.cleanupExpired().catch((error) => {
          this.logger.error({ error }, 'Failed to cleanup expired cache entries');
        });
      },
      5 * 60 * 1000,
    );
  }

  async set(key: string, value: unknown, ttl?: number): Promise<Result<void>> {
    try {
      const now = Date.now();
      const effectiveTtl = ttl ?? this.defaultTtl;

      const entry: CacheEntry = {
        value,
        createdAt: now,
      };

      if (effectiveTtl > 0) {
        entry.expiresAt = now + effectiveTtl;
      }

      this.cache.set(key, entry);

      this.logger.debug(
        {
          key,
          ttl: effectiveTtl,
          expiresAt: entry.expiresAt,
          size: this.cache.size,
        },
        'Cache entry set',
      );

      return Success(undefined);
    } catch (error) {
      this.logger.error({ error, key }, 'Failed to set cache entry');
      return Failure(
        `Failed to set cache entry: ${error instanceof Error ? error.message : String(error)}`,
      );
    }
  }

  async get(key: string): Promise<Result<unknown>> {
    try {
      const entry = this.cache.get(key);

      if (!entry) {
        this.logger.debug({ key }, 'Cache miss');
        return Success(null);
      }

      // Check expiration
      if (entry.expiresAt && Date.now() > entry.expiresAt) {
        this.cache.delete(key);
        this.logger.debug({ key, expiresAt: entry.expiresAt }, 'Cache entry expired');
        return Success(null);
      }

      this.logger.debug({ key }, 'Cache hit');
      return Success(entry.value);
    } catch (error) {
      this.logger.error({ error, key }, 'Failed to get cache entry');
      return Failure(
        `Failed to get cache entry: ${error instanceof Error ? error.message : String(error)}`,
      );
    }
  }

  async delete(key: string): Promise<Result<boolean>> {
    try {
      const deleted = this.cache.delete(key);
      this.logger.debug({ key, deleted }, 'Cache entry deleted');
      return Success(deleted);
    } catch (error) {
      this.logger.error({ error, key }, 'Failed to delete cache entry');
      return Failure(
        `Failed to delete cache entry: ${error instanceof Error ? error.message : String(error)}`,
      );
    }
  }

  async clear(): Promise<Result<void>> {
    try {
      const size = this.cache.size;
      this.cache.clear();
      this.logger.debug({ clearedCount: size }, 'Cache cleared');
      return Success(undefined);
    } catch (error) {
      this.logger.error({ error }, 'Failed to clear cache');
      return Failure(
        `Failed to clear cache: ${error instanceof Error ? error.message : String(error)}`,
      );
    }
  }

  async has(key: string): Promise<Result<boolean>> {
    try {
      const entry = this.cache.get(key);

      if (!entry) {
        return Success(false);
      }

      // Check expiration
      if (entry.expiresAt && Date.now() > entry.expiresAt) {
        this.cache.delete(key);
        return Success(false);
      }

      return Success(true);
    } catch (error) {
      this.logger.error({ error, key }, 'Failed to check cache entry existence');
      return Failure(
        `Failed to check cache entry existence: ${error instanceof Error ? error.message : String(error)}`,
      );
    }
  }

  /**
   * Invalidate entries matching a pattern
   */
  async invalidate(
    pattern: string | { tags?: string[]; keyPattern?: string },
  ): Promise<Result<number>> {
    try {
      let invalidatedCount = 0;
      const patternStr = typeof pattern === 'string' ? pattern : pattern.keyPattern;

      if (patternStr) {
        const regex = new RegExp(patternStr);
        for (const key of this.cache.keys()) {
          if (regex.test(key)) {
            this.cache.delete(key);
            invalidatedCount++;
          }
        }
      }

      this.logger.debug({ pattern, invalidatedCount }, 'Cache entries invalidated');
      return Success(invalidatedCount);
    } catch (error) {
      this.logger.error({ error, pattern }, 'Failed to invalidate cache entries');
      return Failure(
        `Failed to invalidate cache entries: ${error instanceof Error ? error.message : String(error)}`,
      );
    }
  }

  /**
   * Get all keys matching a pattern
   */
  keys(pattern?: string): string[] {
    const allKeys = Array.from(this.cache.keys());

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
  getStats(): {
    size: number;
    hitRate: number;
    memoryUsage: number;
  } {
    let totalSize = 0;
    for (const [key, entry] of this.cache.entries()) {
      totalSize += JSON.stringify({ key, value: entry.value }).length;
    }

    return {
      size: this.cache.size,
      hitRate: 0,
      memoryUsage: totalSize,
    };
  }

  /**
   * Cleanup expired entries
   */
  private async cleanupExpired(): Promise<Result<number>> {
    try {
      const now = Date.now();
      let cleanedCount = 0;

      for (const [key, entry] of this.cache.entries()) {
        if (entry.expiresAt && now > entry.expiresAt) {
          this.cache.delete(key);
          cleanedCount++;
        }
      }

      if (cleanedCount > 0) {
        this.logger.debug({ cleanedCount }, 'Cleaned up expired cache entries');
      }

      return Success(cleanedCount);
    } catch (error) {
      this.logger.error({ error }, 'Failed to cleanup expired entries');
      return Failure(
        `Failed to cleanup expired entries: ${error instanceof Error ? error.message : String(error)}`,
      );
    }
  }

  /**
   * Destroy the cache and cleanup resources
   */
  destroy(): void {
    if (this.cleanupInterval) {
      clearInterval(this.cleanupInterval);
      delete (this as any).cleanupInterval;
    }
    this.cache.clear();
    this.logger.debug('Cache destroyed');
  }
}
