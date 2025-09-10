import { Result, Success, Failure } from '@types';
import type { ResourceCategory } from './types';

/**
 * Simple resource storage entry
 */
interface StoredResource {
  data: unknown;
  expiresAt: number;
  category?: ResourceCategory | undefined;
}

/**
 * Simple in-memory resource storage with TTL
 */
const resourceStore = new Map<string, StoredResource>();

/**
 * Store a resource with optional TTL
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
 * Get a resource by URI
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
 * List all resource URIs, optionally filtered by category
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
 * Clear expired resources and return count removed
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
 * Get basic storage statistics
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
 * Clear all resources (cleanup function)
 */
export async function cleanup(): Promise<Result<void>> {
  try {
    resourceStore.clear();
    return Success(undefined);
  } catch (error) {
    return Failure(`Failed to cleanup resources: ${error}`);
  }
}
