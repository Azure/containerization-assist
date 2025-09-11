import { cacheInstances } from '@lib/cache';
import { createLogger } from '@lib/logger';
import { createHash } from 'crypto';

const logger = createLogger().child({ module: 'ai-cache' });

/**
 * Cache TTL balances freshness vs performance for AI operations.
 *
 * Trade-offs:
 * - baseImages: Longer TTL as base image recommendations change infrequently
 * - fix: Shorter TTL as fixes are highly context-specific and repository state changes
 * - analysis: Medium TTL as repository structure is relatively stable
 */
export const AI_CACHE_TTL = {
  dockerfile: 10 * 60 * 1000, // 10 minutes
  k8s: 15 * 60 * 1000, // 15 minutes
  validation: 5 * 60 * 1000, // 5 minutes
  baseImages: 30 * 60 * 1000, // 30 minutes
  analysis: 20 * 60 * 1000, // 20 minutes
  fix: 5 * 60 * 1000, // 5 minutes
  default: 10 * 60 * 1000, // 10 minutes
};

export interface AICacheKey {
  operation: string;
  prompt: string;
  context?: Record<string, unknown>;
  parameters?: Record<string, unknown>;
}

export interface AICacheEntry {
  response: string;
  metadata?: {
    model?: string;
    tokensUsed?: number;
    timestamp: number;
    operation: string;
  };
}

export interface AICacheStats {
  hits: number;
  misses: number;
  size: number;
  memoryUsed: number;
  byOperation: Record<string, number>;
  estimatedMemoryMB: number;
}

/**
 * Generate a fingerprint for the AI request
 */
export function generateRequestFingerprint(key: AICacheKey): string {
  // Normalize the key by sorting object keys
  const normalized = {
    operation: key.operation,
    prompt: key.prompt.trim().toLowerCase(),
    context: key.context ? sortObjectKeys(key.context) : {},
    parameters: key.parameters ? sortObjectKeys(key.parameters) : {},
  };

  const fingerprint = createHash('sha256')
    .update(JSON.stringify(normalized))
    .digest('hex')
    .substring(0, 16);

  logger.debug(
    {
      operation: key.operation,
      fingerprint,
    },
    'Generated request fingerprint',
  );

  return fingerprint;
}

/**
 * Sort object keys recursively for consistent hashing
 */
function sortObjectKeys(obj: Record<string, unknown>): unknown {
  if (typeof obj !== 'object' || obj === null) {
    return obj;
  }

  if (Array.isArray(obj)) {
    return obj.map((item) =>
      typeof item === 'object' && item !== null
        ? sortObjectKeys(item as Record<string, unknown>)
        : item,
    );
  }

  return Object.keys(obj)
    .sort()
    .reduce(
      (sorted, key) => {
        const value = obj[key];
        sorted[key] =
          typeof value === 'object' && value !== null
            ? sortObjectKeys(value as Record<string, unknown>)
            : value;
        return sorted;
      },
      {} as Record<string, unknown>,
    );
}

/**
 * Get cached AI response if available
 */
export function getCachedResponse(key: AICacheKey): AICacheEntry | undefined {
  const fingerprint = generateRequestFingerprint(key);
  const cached = cacheInstances.aiResponses.get(fingerprint);

  if (cached) {
    try {
      const entry = JSON.parse(cached) as AICacheEntry;
      logger.info(
        {
          operation: key.operation,
          fingerprint,
          age: Date.now() - (entry.metadata?.timestamp || 0),
        },
        'AI cache hit',
      );
      return entry;
    } catch (error) {
      logger.error({ error, fingerprint }, 'Failed to parse cached AI response');
      cacheInstances.aiResponses.delete(fingerprint);
    }
  }

  logger.debug({ operation: key.operation, fingerprint }, 'AI cache miss');
  return undefined;
}

/**
 * Cache an AI response
 */
export function cacheResponse(
  key: AICacheKey,
  response: string,
  metadata?: Partial<AICacheEntry['metadata']>,
): void {
  const fingerprint = generateRequestFingerprint(key);

  const entry: AICacheEntry = {
    response,
    metadata: {
      ...metadata,
      timestamp: Date.now(),
      operation: key.operation,
    },
  };

  // Use operation-specific TTL if available
  const ttl = AI_CACHE_TTL[key.operation as keyof typeof AI_CACHE_TTL] || AI_CACHE_TTL.default;

  // Note: TTL is configured at cache creation time, not per-item
  // The ttl variable is used for logging purposes only
  cacheInstances.aiResponses.set(fingerprint, JSON.stringify(entry));

  logger.info(
    {
      operation: key.operation,
      fingerprint,
      ttl,
      responseLength: response.length,
    },
    'Cached AI response',
  );
}

/**
 * Check if a request should be cached based on operation type
 */
export function shouldCacheOperation(operation: string): boolean {
  // Operations that should always be cached
  const cacheableOperations = [
    'generate_dockerfile',
    'generate_k8s_manifests',
    'resolve_base_images',
    'analyze_repository',
    'validate_dockerfile',
    'validate_k8s',
  ];

  // Operations that should NOT be cached
  const nonCacheableOperations = [
    'build_image',
    'push_image',
    'deploy_application',
    'scan_image', // Scan results change over time
  ];

  if (cacheableOperations.includes(operation)) {
    return true;
  }

  if (nonCacheableOperations.includes(operation)) {
    return false;
  }

  // Default to caching for unknown operations
  logger.debug({ operation }, 'Unknown operation for caching decision, defaulting to cache');
  return true;
}

/**
 * Invalidate cache entries for a specific operation
 */
export function invalidateOperationCache(operation: string): number {
  // Since we can't inspect cache entries directly, we'll clear the entire cache
  // This is a simplified approach - in production, we might need a different strategy
  const stats = cacheInstances.aiResponses.getStats();
  const sizeBefore = stats.size;

  // Clear entire cache (limitation of current cache implementation)
  cacheInstances.aiResponses.clear();

  logger.info({ operation, count: sizeBefore }, 'Cleared cache for operation');
  return sizeBefore;
}

/**
 * Get cache statistics for AI responses
 */
export function getAICacheStats(): AICacheStats {
  const stats = cacheInstances.aiResponses.getStats();

  // Without inspect method, we can't get detailed operation counts
  // Return simplified stats
  const byOperation: Record<string, number> = {
    unknown: stats.size,
  };

  // Estimate memory usage based on cache size
  // Assume average entry size of ~2KB
  const estimatedBytesPerEntry = 2048;
  const memoryBytes = stats.size * estimatedBytesPerEntry;

  return {
    ...stats,
    byOperation,
    memoryUsed: memoryBytes,
    estimatedMemoryMB: memoryBytes / (1024 * 1024),
  };
}

/**
 * Warm up cache with common requests
 */
export async function warmupCache(
  makeRequest: (key: AICacheKey) => Promise<string>,
): Promise<void> {
  const warmupRequests: AICacheKey[] = [
    {
      operation: 'resolve_base_images',
      prompt: 'Find best Node.js base image',
      context: { language: 'javascript', framework: 'express' },
    },
    {
      operation: 'resolve_base_images',
      prompt: 'Find best Python base image',
      context: { language: 'python', framework: 'django' },
    },
    {
      operation: 'generate_dockerfile',
      prompt: 'Generate Dockerfile for Node.js app',
      context: { language: 'javascript', hasPackageJson: true },
    },
  ];

  logger.info({ requests: warmupRequests.length }, 'Starting cache warmup');

  for (const request of warmupRequests) {
    try {
      // Check if already cached
      const existing = getCachedResponse(request);
      if (!existing) {
        // Make the request and cache it
        const response = await makeRequest(request);
        cacheResponse(request, response);
      }
    } catch (error) {
      logger.warn(
        {
          operation: request.operation,
          error,
        },
        'Failed to warm up cache entry',
      );
    }
  }

  logger.info(
    {
      cacheSize: cacheInstances.aiResponses.getStats().size,
    },
    'Cache warmup complete',
  );
}

/**
 * Clear all AI response cache
 */
export function clearAICache(): void {
  cacheInstances.aiResponses.clear();
  logger.info({}, 'AI response cache cleared');
}
