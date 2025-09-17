/**
 * Resource Storage Module
 *
 * Provides simple in-memory storage for temporary resources with TTL support.
 * This module offers a lightweight alternative to complex caching solutions
 * for managing build artifacts, scan results, and deployment statuses.
 *
 * Key features:
 * - Time-based expiration (TTL)
 * - Category-based filtering
 * - Automatic cleanup of expired resources
 * - Simple key-value interface
 *
 * @module resources/manager
 */

import { Result, Success, Failure } from '@/types';
import type { ResourceCategory } from './types';

/**
 * Internal storage structure for resources with metadata
 */
interface StoredResource {
  /** The actual resource data */
  data: unknown;
  /** Timestamp when the resource expires */
  expiresAt: number;
  /** Optional category for filtering */
  category?: ResourceCategory | undefined;
}

/**
 * In-memory storage for resources.
 * Module-level Map provides simple, efficient storage without class overhead.
 */
const resourceStore = new Map<string, StoredResource>();

/**
 * Store a resource with automatic expiration.
 *
 * @param uri - Unique identifier for the resource
 * @param content - The resource data to store
 * @param ttl - Time to live in milliseconds (default: 1 hour)
 * @param category - Optional category for filtering
 * @returns Success if stored, Failure with error message if failed
 *
 * @example
 * ```typescript
 * storeResource('docker://image-123', imageData, 3600000, 'build-artifact');
 * ```
 */
export function storeResource(
  uri: string,
  content: unknown,
  ttl = 3600000, // 1 hour default
  category?: ResourceCategory,
): Result<void> {
  try {
    resourceStore.set(uri, {
      data: content,
      expiresAt: Date.now() + ttl,
      category,
    });
    return Success(undefined);
  } catch (error) {
    return Failure(`Failed to store resource: ${error}`);
  }
}

/**
 * Retrieve a resource by its URI.
 * Automatically removes expired resources when accessed.
 *
 * @param uri - The resource identifier
 * @returns Success with resource data or null if not found/expired
 *
 * @example
 * ```typescript
 * const result = getResource('docker://image-123');
 * if (result.ok && result.value) {
 *   // Use the resource data
 * }
 * ```
 */
export function getResource(uri: string): Result<unknown | null> {
  try {
    const entry = resourceStore.get(uri);
    if (!entry) {
      return Success(null);
    }

    // Check if expired
    if (Date.now() > entry.expiresAt) {
      resourceStore.delete(uri);
      return Success(null);
    }

    return Success(entry.data);
  } catch (error) {
    return Failure(`Failed to get resource: ${error}`);
  }
}

/**
 * List all active resource URIs.
 * Expired resources are automatically cleaned during listing.
 *
 * @param category - Optional filter by resource category
 * @returns Success with array of URIs
 *
 * @example
 * ```typescript
 * const scanResults = listResources('scan-result');
 * if (scanResults.ok) {
 *   console.log(`Found ${scanResults.value.length} scan results`);
 * }
 * ```
 */
export function listResources(category?: ResourceCategory): Result<string[]> {
  try {
    const uris: string[] = [];
    const now = Date.now();

    for (const [uri, entry] of resourceStore.entries()) {
      // Skip expired resources
      if (now > entry.expiresAt) {
        resourceStore.delete(uri);
        continue;
      }

      // Filter by category if specified
      if (category && entry.category !== category) {
        continue;
      }

      uris.push(uri);
    }

    return Success(uris);
  } catch (error) {
    return Failure(`Failed to list resources: ${error}`);
  }
}

/**
 * Manually trigger cleanup of expired resources.
 * Note: Expired resources are also cleaned automatically during get/list operations.
 *
 * @returns Success with count of resources removed
 *
 * @example
 * ```typescript
 * const result = clearExpired();
 * if (result.ok) {
 *   console.log(`Cleaned up ${result.value} expired resources`);
 * }
 * ```
 */
export function clearExpired(): Result<number> {
  try {
    const now = Date.now();
    let removed = 0;

    for (const [uri, entry] of resourceStore.entries()) {
      if (now > entry.expiresAt) {
        resourceStore.delete(uri);
        removed++;
      }
    }

    return Success(removed);
  } catch (error) {
    return Failure(`Failed to clear expired resources: ${error}`);
  }
}

/**
 * Get storage statistics and memory usage estimates.
 * Useful for monitoring and debugging resource consumption.
 *
 * @returns Object with total count, category breakdown, and memory estimate
 *
 * @example
 * ```typescript
 * const stats = getStats();
 * console.log(`Total resources: ${stats.total}`);
 * console.log(`Memory usage: ${stats.memoryUsage} bytes`);
 * ```
 */
export function getStats(): {
  total: number;
  byCategory: Record<ResourceCategory, number>;
  memoryUsage: number;
} {
  const now = Date.now();
  const byCategory: Record<ResourceCategory, number> = {
    dockerfile: 0,
    'k8s-manifest': 0,
    'scan-result': 0,
    'build-artifact': 0,
    'deployment-status': 0,
    'session-data': 0,
    'sampling-result': 0,
    'sampling-variant': 0,
    'sampling-config': 0,
  };

  let total = 0;
  for (const [, entry] of resourceStore.entries()) {
    if (now <= entry.expiresAt) {
      total++;
      if (entry.category) {
        byCategory[entry.category]++;
      }
    }
  }

  return {
    total,
    byCategory,
    memoryUsage: resourceStore.size * 1024, // rough estimate
  };
}

/**
 * Clear all stored resources.
 * Typically called during shutdown or test cleanup.
 *
 * @returns Success when cleanup is complete
 *
 * @example
 * ```typescript
 * // In shutdown handler
 * await cleanup();
 * ```
 */
export async function cleanup(): Promise<Result<void>> {
  try {
    resourceStore.clear();
    return Success(undefined);
  } catch (error) {
    return Failure(`Failed to cleanup resources: ${error}`);
  }
}
