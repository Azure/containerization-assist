import { cacheInstances, type CacheStats } from '@lib/cache';
import { createLogger } from '@lib/logger';
import { Result, Success, Failure } from '@types';

const logger = createLogger().child({ module: 'docker-metadata' });

export interface ImageMetadata {
  size: number;
  layers: number;
  architecture?: string;
  os?: string;
  created?: string;
  author?: string;
}

// Default fallback metadata when API is unavailable
const DEFAULT_METADATA: ImageMetadata = {
  size: 100_000_000, // 100MB estimate
  layers: 10, // Typical layer count
  architecture: 'amd64',
  os: 'linux',
};

// Common base image estimates for better fallbacks
const KNOWN_IMAGE_ESTIMATES: Record<string, Partial<ImageMetadata>> = {
  'node:': { size: 150_000_000, layers: 12 },
  'node:.*alpine': { size: 50_000_000, layers: 8 },
  'python:': { size: 200_000_000, layers: 14 },
  'python:.*slim': { size: 120_000_000, layers: 10 },
  'python:.*alpine': { size: 60_000_000, layers: 9 },
  'openjdk:': { size: 300_000_000, layers: 15 },
  'openjdk:.*alpine': { size: 100_000_000, layers: 10 },
  'golang:': { size: 250_000_000, layers: 13 },
  'golang:.*alpine': { size: 80_000_000, layers: 9 },
  'nginx:': { size: 140_000_000, layers: 8 },
  'nginx:.*alpine': { size: 40_000_000, layers: 6 },
  'redis:': { size: 100_000_000, layers: 7 },
  'redis:.*alpine': { size: 30_000_000, layers: 5 },
  'postgres:': { size: 350_000_000, layers: 12 },
  'postgres:.*alpine': { size: 150_000_000, layers: 10 },
  'mysql:': { size: 400_000_000, layers: 13 },
  'ubuntu:': { size: 80_000_000, layers: 5 },
  'alpine:': { size: 15_000_000, layers: 3 },
  'busybox:': { size: 5_000_000, layers: 2 },
};

/**
 * Get estimated metadata based on image name patterns
 */
function getEstimatedMetadata(imageName: string): ImageMetadata {
  // Try to match known patterns
  for (const [pattern, estimates] of Object.entries(KNOWN_IMAGE_ESTIMATES)) {
    const regex = new RegExp(pattern, 'i');
    if (regex.test(imageName)) {
      logger.debug({ imageName, pattern }, 'Using known image estimates');
      return {
        ...DEFAULT_METADATA,
        ...estimates,
      };
    }
  }

  // Default fallback
  logger.debug({ imageName }, 'Using default metadata estimates');
  return DEFAULT_METADATA;
}

/**
 * Fetch metadata from Docker Hub API with timeout
 */
async function fetchFromDockerHub(imageName: string): Promise<Result<ImageMetadata>> {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 5000); // 5 second timeout

  try {
    // Parse image name to get repository and tag
    const [repository, tag = 'latest'] = imageName.split(':');
    if (!repository) {
      return Failure('Invalid image name');
    }
    const [namespace, image] = repository.includes('/')
      ? repository.split('/', 2)
      : ['library', repository]; // Official images use 'library' namespace

    const url = `https://hub.docker.com/v2/repositories/${namespace}/${image}/tags/${tag}`;

    logger.debug({ url, imageName }, 'Fetching from Docker Hub');

    const response = await fetch(url, {
      signal: controller.signal,
      headers: {
        Accept: 'application/json',
      },
    });

    if (!response.ok) {
      logger.warn(
        {
          status: response.status,
          imageName,
        },
        'Docker Hub API returned non-OK status',
      );
      return Failure(`Docker Hub API error: ${response.status}`);
    }

    const data = (await response.json()) as any;

    // Extract metadata from response
    const metadata: ImageMetadata = {
      size: data?.full_size || data?.size || DEFAULT_METADATA.size,
      layers: data?.images?.[0]?.layers?.length || DEFAULT_METADATA.layers,
      architecture: data?.images?.[0]?.architecture || 'amd64',
      os: data?.images?.[0]?.os || 'linux',
      created: data?.last_updated || data?.images?.[0]?.created,
      author: data?.user || data?.namespace,
    };

    logger.info(
      {
        imageName,
        size: metadata.size,
        layers: metadata.layers,
      },
      'Successfully fetched Docker Hub metadata',
    );

    return Success(metadata);
  } catch (error) {
    if (error instanceof Error && error.name === 'AbortError') {
      logger.warn({ imageName }, 'Docker Hub API request timed out');
      return Failure('Docker Hub API timeout after 5 seconds');
    }

    const errorMessage = error instanceof Error ? error.message : String(error);
    logger.error(
      {
        imageName,
        error: errorMessage,
      },
      'Failed to fetch from Docker Hub',
    );

    return Failure(`Docker Hub API error: ${errorMessage}`);
  } finally {
    clearTimeout(timeout);
  }
}

/**
 * Get image metadata with caching and fallback
 */
export async function getImageMetadata(imageName: string): Promise<ImageMetadata> {
  // Normalize image name
  const normalizedName = imageName.toLowerCase().trim();

  // Check cache first
  const cached = cacheInstances.dockerMetadata.get(normalizedName);
  if (cached) {
    logger.debug({ imageName: normalizedName }, 'Cache hit for image metadata');
    return cached;
  }

  logger.debug({ imageName: normalizedName }, 'Cache miss, fetching metadata');

  // Try to fetch from Docker Hub
  const result = await fetchFromDockerHub(normalizedName);

  let metadata: ImageMetadata;

  if (result.ok) {
    metadata = result.value;
  } else {
    // Use fallback estimates
    logger.info(
      {
        imageName: normalizedName,
        reason: result.error,
      },
      'Using fallback metadata estimates',
    );
    metadata = getEstimatedMetadata(normalizedName);
  }

  // Cache the result (whether from API or fallback)
  cacheInstances.dockerMetadata.set(normalizedName, metadata);

  return metadata;
}

/**
 * Batch fetch metadata for multiple images
 */
export async function getMultipleImageMetadata(
  imageNames: string[],
): Promise<Map<string, ImageMetadata>> {
  const results = new Map<string, ImageMetadata>();

  // Process in parallel with concurrency limit
  const concurrencyLimit = 3;
  const chunks: string[][] = [];

  for (let i = 0; i < imageNames.length; i += concurrencyLimit) {
    chunks.push(imageNames.slice(i, i + concurrencyLimit));
  }

  for (const chunk of chunks) {
    const promises = chunk.map(async (imageName) => {
      const metadata = await getImageMetadata(imageName);
      results.set(imageName, metadata);
    });

    await Promise.all(promises);
  }

  return results;
}

/**
 * Preload common base images into cache
 */
export async function preloadCommonImages(): Promise<void> {
  const commonImages = [
    'node:20-alpine',
    'node:20',
    'python:3.11-slim',
    'python:3.11-alpine',
    'nginx:alpine',
    'alpine:latest',
  ];

  logger.info({}, 'Preloading common image metadata');

  for (const image of commonImages) {
    try {
      await getImageMetadata(image);
    } catch (error) {
      logger.warn({ image, error }, 'Failed to preload image metadata');
    }
  }

  logger.info(
    {
      cachedImages: cacheInstances.dockerMetadata.getStats().size,
    },
    'Preloading complete',
  );
}

/**
 * Clear the metadata cache
 */
export function clearMetadataCache(): void {
  cacheInstances.dockerMetadata.clear();
  logger.info({}, 'Docker metadata cache cleared');
}

/**
 * Get cache statistics
 */
export function getMetadataCacheStats(): CacheStats {
  return cacheInstances.dockerMetadata.getStats();
}
