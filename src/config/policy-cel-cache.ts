/**
 * CEL Policy Compilation Cache
 *
 * Caches compiled CEL programs by policy condition content hash.
 * Single-user CLI tool - no TTL needed, cache persists for process lifetime.
 *
 * @module config/policy-cel-cache
 */

import { createHash } from 'node:crypto';
import type { ParseResult } from '@marcbachmann/cel-js';

/**
 * Cached CEL program with metadata
 */
interface CachedProgram {
  /** Compiled CEL expression program */
  program: ParseResult;
  /** Original condition string (truncated for logging) */
  condition: string;
  /** Timestamp when compiled */
  compiledAt: Date;
}

/**
 * CEL Compilation Cache
 *
 * Provides caching for compiled CEL programs to avoid re-compilation
 * on repeated policy loads. Uses content hash as cache key to ensure
 * identical conditions share compiled programs.
 *
 * @example
 * ```typescript
 * import { celCompilationCache } from './policy-cel-cache';
 *
 * // Check cache before compiling
 * let program = celCompilationCache.get(condition);
 * if (!program) {
 *   program = parse(condition);
 *   celCompilationCache.set(condition, program);
 * }
 * ```
 */
class CelCompilationCache {
  private cache = new Map<string, CachedProgram>();

  /**
   * Generate cache key from rule condition
   *
   * Uses SHA-256 hash of the condition string for consistent,
   * collision-resistant keys.
   */
  private getCacheKey(condition: string): string {
    return createHash('sha256').update(condition).digest('hex');
  }

  /**
   * Get cached compiled program
   *
   * @param condition - The CEL condition string
   * @returns Compiled program if cached, null otherwise
   */
  get(condition: string): ParseResult | null {
    const key = this.getCacheKey(condition);
    const cached = this.cache.get(key);
    return cached?.program ?? null;
  }

  /**
   * Store compiled program in cache
   *
   * @param condition - The CEL condition string
   * @param program - The compiled CEL program
   */
  set(condition: string, program: ParseResult): void {
    const key = this.getCacheKey(condition);
    this.cache.set(key, {
      program,
      condition: condition.substring(0, 100), // Truncate for logging
      compiledAt: new Date(),
    });
  }

  /**
   * Check if condition is cached
   *
   * @param condition - The CEL condition string
   * @returns true if condition is in cache
   */
  has(condition: string): boolean {
    const key = this.getCacheKey(condition);
    return this.cache.has(key);
  }

  /**
   * Clear all cached programs
   *
   * Useful for testing or when policy files are known to have changed.
   */
  clear(): void {
    this.cache.clear();
  }

  /**
   * Get cache statistics
   *
   * Returns information about cached entries for debugging and monitoring.
   *
   * @returns Object with cache size and entry metadata
   */
  getStats(): {
    size: number;
    entries: Array<{ condition: string; compiledAt: Date }>;
  } {
    return {
      size: this.cache.size,
      entries: Array.from(this.cache.values()).map((entry) => ({
        condition: entry.condition,
        compiledAt: entry.compiledAt,
      })),
    };
  }
}

/**
 * Singleton cache instance
 *
 * Single instance shared across all CEL policy evaluators.
 * Cache persists for process lifetime - appropriate for CLI tool use case.
 */
export const celCompilationCache = new CelCompilationCache();
