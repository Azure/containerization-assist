/**
 * Stable JSON utilities for deterministic cache keys
 */
import { createHash } from 'crypto';

/**
 * Create a stable string representation of an object by sorting keys
 * This ensures that objects with the same properties but different key orders
 * produce the same string representation for consistent cache keys.
 */
export function stableStringify(obj: unknown): string {
  if (obj === null || obj === undefined) {
    return String(obj);
  }

  if (typeof obj !== 'object') {
    return JSON.stringify(obj);
  }

  if (Array.isArray(obj)) {
    return `[${obj.map(stableStringify).join(',')}]`;
  }

  // For objects, sort keys for deterministic output
  const sortedKeys = Object.keys(obj as Record<string, unknown>).sort();
  const pairs = sortedKeys.map((key) => {
    const value = (obj as Record<string, unknown>)[key];
    return `${JSON.stringify(key)}:${stableStringify(value)}`;
  });

  return `{${pairs.join(',')}}`;
}

/**
 * Create a short hash string from input for cache keys
 * Uses SHA-256 and returns first 16 characters for reasonable collision resistance
 * while keeping cache keys short and readable.
 */
export function hashString(str: string): string {
  return createHash('sha256').update(str).digest('hex').slice(0, 16);
}

/**
 * Create a stable cache key from an ID and parameters object
 * This combines the ID with a stable representation of the parameters
 * to ensure consistent cache keys regardless of parameter order.
 */
export function createCacheKey(id: string, params: unknown): string {
  const stableParams = stableStringify(params);
  const combined = `${id}|${stableParams}`;
  return hashString(combined);
}
