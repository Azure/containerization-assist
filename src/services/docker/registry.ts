/**
 * Docker Registry Client
 *
 * Fetches real image metadata from Docker registries
 */

import type { Logger } from 'pino';

export interface ImageMetadata {
  name: string;
  tag: string;
  digest?: string;
  size?: number;
  lastUpdated?: string;
  architecture?: string;
  os?: string;
}

/**
 * Docker Hub API response structure for tag metadata
 * @see https://docs.docker.com/docker-hub/api/latest/
 */
interface DockerHubTagResponse {
  digest?: string;
  full_size?: number;
  size?: number;
  last_updated?: string;
  tag_last_pushed?: string;
  images?: Array<{ architecture?: string; os?: string }>;
}

/**
 * Fetch image metadata from Docker Hub
 */
async function fetchDockerHubMetadata(
  imageName: string,
  tag: string,
  logger: Logger,
): Promise<ImageMetadata | null> {
  try {
    // Parse image name to handle official images vs user/org images
    const parts = imageName.split('/');
    const isOfficial = parts.length === 1;
    const namespace = isOfficial ? 'library' : parts[0];
    const repo = isOfficial ? imageName : parts[1];

    // Docker Hub API endpoint
    const url = `https://hub.docker.com/v2/repositories/${namespace}/${repo}/tags/${tag}`;

    const response = await fetch(url, {
      headers: {
        Accept: 'application/json',
      },
    });

    if (!response.ok) {
      logger.debug({ imageName, tag, status: response.status }, 'Failed to fetch from Docker Hub');
      return null;
    }

    const data = (await response.json()) as DockerHubTagResponse;

    const metadata: ImageMetadata = {
      name: imageName,
      tag,
    };

    if (data.digest) {
      metadata.digest = data.digest;
    }
    const size = data.full_size ?? data.size;
    if (size !== undefined) {
      metadata.size = size;
    }
    const lastUpdated = data.last_updated ?? data.tag_last_pushed;
    if (lastUpdated !== undefined) {
      metadata.lastUpdated = lastUpdated;
    }
    if (data.images?.[0]?.architecture) {
      metadata.architecture = data.images[0].architecture;
    }
    if (data.images?.[0]?.os) {
      metadata.os = data.images[0].os;
    }

    return metadata;
  } catch (error) {
    logger.debug({ error, imageName, tag }, 'Error fetching Docker Hub metadata');
    return null;
  }
}

/**
 * Get estimated image sizes based on common patterns
 */
function getEstimatedImageSize(imageName: string, tag: string): number {
  // Estimated sizes in bytes based on common patterns
  const estimates: Record<string, number> = {
    alpine: 5 * 1024 * 1024, // ~5MB
    scratch: 0, // 0MB (empty base)
    slim: 150 * 1024 * 1024, // ~150MB
    bullseye: 250 * 1024 * 1024, // ~250MB
    buster: 250 * 1024 * 1024, // ~250MB
    latest: 500 * 1024 * 1024, // ~500MB (assume full image)
  };

  // Check tag patterns
  for (const [pattern, size] of Object.entries(estimates)) {
    if (tag.includes(pattern)) {
      return size;
    }
  }

  // Language-specific estimates
  if (imageName.includes('node')) {
    if (tag.includes('alpine')) return 50 * 1024 * 1024; // ~50MB
    if (tag.includes('slim')) return 200 * 1024 * 1024; // ~200MB
    return 350 * 1024 * 1024; // ~350MB
  }

  if (imageName.includes('python')) {
    if (tag.includes('alpine')) return 60 * 1024 * 1024; // ~60MB
    if (tag.includes('slim')) return 150 * 1024 * 1024; // ~150MB
    return 400 * 1024 * 1024; // ~400MB
  }

  if (imageName.includes('golang')) {
    if (tag.includes('alpine')) return 350 * 1024 * 1024; // ~350MB
    return 800 * 1024 * 1024; // ~800MB
  }

  if (imageName.includes('openjdk') || imageName.includes('eclipse-temurin')) {
    if (tag.includes('alpine')) return 200 * 1024 * 1024; // ~200MB
    if (tag.includes('slim')) return 400 * 1024 * 1024; // ~400MB
    return 600 * 1024 * 1024; // ~600MB
  }

  // Default estimate
  return 300 * 1024 * 1024; // ~300MB
}

/**
 * Get image metadata with fallback to estimates
 */
export async function getImageMetadata(
  imageName: string,
  tag: string,
  logger: Logger,
): Promise<ImageMetadata> {
  // Try to fetch real metadata from Docker Hub
  const metadata = await fetchDockerHubMetadata(imageName, tag, logger);

  if (metadata) {
    logger.debug({ imageName, tag, size: metadata.size }, 'Fetched real image metadata');
    return metadata;
  }

  // Fallback to estimates
  const estimatedSize = getEstimatedImageSize(imageName, tag);
  logger.debug({ imageName, tag, estimatedSize }, 'Using estimated image metadata');

  return {
    name: imageName,
    tag,
    size: estimatedSize,
    lastUpdated: new Date().toISOString(),
  };
}
