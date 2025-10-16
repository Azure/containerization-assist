/**
 * Docker Registry Client
 *
 * Provides Docker registry operations including authentication,
 * health checks, image validation, and metadata fetching
 */

import type { Logger } from 'pino';
import Docker from 'dockerode';
import { Success, Failure, type Result } from '@/types';

// Configuration constants
const DOCKER_HUB_REQUEST_TIMEOUT_MS = 7000; // 7 seconds
const REGISTRY_REQUEST_TIMEOUT_MS = 10000; // 10 seconds

/**
 * Registry authentication configuration
 */
export interface RegistryConfig {
  /** Registry URL (e.g., 'https://registry.example.com' or 'docker.io') */
  url: string;
  /** Username for authentication */
  username?: string;
  /** Password for authentication */
  password?: string;
  /** Token for authentication (alternative to username/password) */
  token?: string;
}

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

    // Create AbortController for timeout
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), DOCKER_HUB_REQUEST_TIMEOUT_MS);

    try {
      const response = await fetch(url, {
        headers: {
          Accept: 'application/json',
        },
        signal: controller.signal,
      });

      if (!response.ok) {
        // Handle rate limiting specifically
        if (response.status === 429) {
          const retryAfter = response.headers.get('Retry-After');
          logger.warn(
            { imageName, tag, retryAfter },
            'Docker Hub rate limit exceeded. Please try again later.',
          );
        } else {
          logger.debug(
            { imageName, tag, status: response.status },
            'Failed to fetch from Docker Hub',
          );
        }
        return null;
      }

      const data = (await response.json()) as DockerHubTagResponse;

      // Clear the timeout since request succeeded
      clearTimeout(timeoutId);

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
    } finally {
      clearTimeout(timeoutId);
    }
  } catch (error) {
    // Handle timeout error specifically
    if (error instanceof Error && error.name === 'AbortError') {
      logger.warn(
        { imageName, tag, timeoutMs: DOCKER_HUB_REQUEST_TIMEOUT_MS },
        `Docker Hub request timed out after ${DOCKER_HUB_REQUEST_TIMEOUT_MS / 1000} seconds`,
      );
    } else {
      logger.debug({ error, imageName, tag }, 'Error fetching Docker Hub metadata');
    }
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

/**
 * Docker Registry Client for private registry operations
 *
 * Provides authentication, health checks, and image validation
 * for Docker registries including Docker Hub and private registries
 */
export class DockerRegistry {
  private docker: Docker;
  private logger: Logger;
  private authConfig?: { username: string; password: string; serveraddress: string };

  constructor(docker: Docker, logger: Logger) {
    this.docker = docker;
    this.logger = logger;
  }

  /**
   * Authenticate with a Docker registry
   *
   * Validates credentials with the registry and stores them for subsequent operations.
   * Supports both username/password and token-based authentication.
   *
   * @param config - Registry configuration with credentials
   * @returns Result indicating success or failure with guidance
   */
  async authenticate(config: RegistryConfig): Promise<Result<void>> {
    try {
      const serverAddress = this.normalizeRegistryUrl(config.url);

      // Build auth config for dockerode
      if (config.username && config.password) {
        this.authConfig = {
          username: config.username,
          password: config.password,
          serveraddress: serverAddress,
        };
      } else if (config.token) {
        // For token-based auth, use token as password with empty username
        this.authConfig = {
          username: '',
          password: config.token,
          serveraddress: serverAddress,
        };
      } else {
        return Failure('Authentication requires either username/password or token', {
          message: 'Missing credentials',
          hint: 'Provide either username and password, or an authentication token',
          resolution: 'Add credentials to the registry configuration',
        });
      }

      // Verify authentication by calling Docker auth endpoint
      try {
        await this.docker.checkAuth(this.authConfig);
        this.logger.info({ registry: serverAddress }, 'Registry authentication successful');
        return Success(undefined);
      } catch (error) {
        // Clear stored auth config on failure
        delete this.authConfig;

        const errorMessage = error instanceof Error ? error.message : String(error);
        return Failure(`Authentication failed: ${errorMessage}`, {
          message: 'Registry authentication failed',
          hint: 'Invalid credentials or registry is unavailable',
          resolution:
            'Verify your username/password or token, and ensure the registry is accessible',
          details: { registry: serverAddress, error: errorMessage },
        });
      }
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      return Failure(`Authentication error: ${errorMessage}`, {
        message: 'Failed to authenticate with registry',
        hint: 'An unexpected error occurred during authentication',
        resolution: 'Check registry URL format and network connectivity',
        details: { error: errorMessage },
      });
    }
  }

  /**
   * Check if a registry is accessible
   *
   * Performs a health check by attempting to connect to the registry's API.
   * For Docker Hub, pings the v2 API endpoint. For private registries,
   * checks the /v2/ endpoint which should return 200 or 401.
   *
   * @param registryUrl - Registry URL to check
   * @returns Result with boolean indicating if registry is accessible
   */
  async healthCheck(registryUrl: string): Promise<Result<boolean>> {
    try {
      const normalizedUrl = this.normalizeRegistryUrl(registryUrl);
      const apiUrl = this.getRegistryApiUrl(normalizedUrl);

      this.logger.debug({ registryUrl: apiUrl }, 'Performing registry health check');

      // Create AbortController for timeout
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), REGISTRY_REQUEST_TIMEOUT_MS);

      try {
        const response = await fetch(apiUrl, {
          method: 'GET',
          signal: controller.signal,
        });

        clearTimeout(timeoutId);

        // Docker Registry v2 API returns 200 for authenticated requests
        // or 401 for unauthenticated but accessible registries
        const isHealthy = response.status === 200 || response.status === 401;

        this.logger.debug(
          { registryUrl: apiUrl, status: response.status, healthy: isHealthy },
          'Registry health check complete',
        );

        return Success(isHealthy);
      } catch (error) {
        clearTimeout(timeoutId);

        if (error instanceof Error && error.name === 'AbortError') {
          this.logger.warn({ registryUrl: apiUrl }, 'Registry health check timed out');
          return Success(false);
        }

        this.logger.debug({ registryUrl: apiUrl, error }, 'Registry health check failed');
        return Success(false);
      }
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      return Failure(`Health check error: ${errorMessage}`, {
        message: 'Failed to perform registry health check',
        hint: 'Unable to connect to registry',
        resolution: 'Verify the registry URL and network connectivity',
        details: { error: errorMessage },
      });
    }
  }

  /**
   * Check if an image exists locally in the Docker daemon
   *
   * Checks if a specific image exists in the local Docker daemon cache.
   * This does not query remote registries - it only checks locally pulled images.
   * Use `docker pull` first if you need to verify remote image availability.
   *
   * @param imageName - Full image name (repository:tag or repository@digest)
   * @returns Result with boolean indicating if image exists locally
   */
  async imageExists(imageName: string): Promise<Result<boolean>> {
    try {
      // Parse image name into components
      const { repository, reference } = this.parseImageName(imageName);

      this.logger.debug({ repository, reference }, 'Checking if image exists locally');

      try {
        // Use Docker API to inspect the image locally
        // This only checks the local Docker daemon, not remote registries
        const image = this.docker.getImage(imageName);
        await image.inspect();

        this.logger.debug({ imageName }, 'Image exists');
        return Success(true);
      } catch (error) {
        // If image doesn't exist locally, Docker will throw a 404 error
        const errorMessage = error instanceof Error ? error.message : String(error);

        if (errorMessage.includes('404') || errorMessage.includes('no such image')) {
          this.logger.debug({ imageName }, 'Image does not exist');
          return Success(false);
        }

        // For other errors, return failure with guidance
        return Failure(`Failed to check image existence: ${errorMessage}`, {
          message: 'Unable to verify image existence',
          hint: 'Error occurred while checking registry',
          resolution: 'Ensure Docker daemon is running and registry is accessible',
          details: { imageName, error: errorMessage },
        });
      }
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      return Failure(`Image existence check error: ${errorMessage}`, {
        message: 'Failed to check if image exists',
        hint: 'Invalid image name format or registry error',
        resolution: 'Verify the image name format (repository:tag)',
        details: { error: errorMessage },
      });
    }
  }

  /**
   * List available tags for a repository
   *
   * Retrieves all tags for a given repository from the registry.
   * For Docker Hub, uses the public API. For private registries,
   * requires authentication.
   *
   * @param repository - Repository name (e.g., 'library/node' or 'myorg/myapp')
   * @returns Result with array of tag names
   */
  async listTags(repository: string): Promise<Result<string[]>> {
    try {
      this.logger.debug({ repository }, 'Listing repository tags');

      // For Docker Hub, use the public API
      // Check only the first path segment for a hostname pattern (dot or colon)
      const firstSegment = repository.split('/')[0];
      if (!/[.:]/.test(firstSegment)) {
        return this.listDockerHubTags(repository);
      }

      // For private registries, use Docker Registry API v2
      return this.listPrivateRegistryTags(repository);
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      return Failure(`Failed to list tags: ${errorMessage}`, {
        message: 'Unable to retrieve repository tags',
        hint: 'Registry API error or authentication required',
        resolution: 'Verify repository name and ensure you have access permissions',
        details: { repository, error: errorMessage },
      });
    }
  }

  /**
   * Get stored authentication config for registry operations
   * @internal
   */
  getAuthConfig(): { username: string; password: string; serveraddress: string } | undefined {
    return this.authConfig;
  }

  // ===== PRIVATE HELPER METHODS =====

  /**
   * Normalize registry URL to standard format
   */
  private normalizeRegistryUrl(url: string): string {
    // Remove protocol if present
    let normalized = url.replace(/^https?:\/\//, '');

    // Remove trailing slash
    normalized = normalized.replace(/\/$/, '');

    // Default to docker.io if no registry specified
    if (!normalized || normalized === 'docker.io') {
      return 'https://index.docker.io/v1/';
    }

    return normalized;
  }

  /**
   * Get registry API URL for health checks
   */
  private getRegistryApiUrl(registryUrl: string): string {
    // Safely check for Docker Hub by parsing the hostname
    const normalizedUrl = registryUrl.startsWith('http') ? registryUrl : `https://${registryUrl}`;

    try {
      const url = new URL(normalizedUrl);
      const hostname = url.hostname.toLowerCase();

      // For Docker Hub - check exact hostname match
      if (
        hostname === 'docker.io' ||
        hostname === 'index.docker.io' ||
        hostname === 'registry-1.docker.io'
      ) {
        return 'https://registry-1.docker.io/v2/';
      }
    } catch {
      // If URL parsing fails, fall through to generic handling
    }

    // For other registries, append /v2/ if not present
    const baseUrl = registryUrl.startsWith('http') ? registryUrl : `https://${registryUrl}`;
    return baseUrl.endsWith('/v2/') ? baseUrl : `${baseUrl}/v2/`;
  }

  /**
   * Parse image name into repository and reference (tag or digest)
   */
  private parseImageName(imageName: string): { repository: string; reference: string } {
    // Check for digest reference (contains @sha256:)
    if (imageName.includes('@')) {
      const parts = imageName.split('@');
      const repository = parts[0] || imageName;
      const digest = parts[1] || '';
      return { repository, reference: digest };
    }

    // Check for tag reference (contains :)
    const lastColonIndex = imageName.lastIndexOf(':');
    if (lastColonIndex > imageName.lastIndexOf('/')) {
      // Colon is for tag, not port
      const repository = imageName.substring(0, lastColonIndex);
      const tag = imageName.substring(lastColonIndex + 1);
      return { repository, reference: tag };
    }

    // No tag or digest specified, assume latest
    return { repository: imageName, reference: 'latest' };
  }

  /**
   * List tags from Docker Hub public API
   */
  private async listDockerHubTags(repository: string): Promise<Result<string[]>> {
    try {
      // Parse repository to handle official images vs user/org images
      const parts = repository.split('/');
      const isOfficial = parts.length === 1;
      const namespace = isOfficial ? 'library' : parts[0];
      const repo = isOfficial ? repository : parts[1];

      // Docker Hub API endpoint for tags
      const url = `https://hub.docker.com/v2/repositories/${namespace}/${repo}/tags?page_size=100`;

      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), DOCKER_HUB_REQUEST_TIMEOUT_MS);

      try {
        const response = await fetch(url, {
          headers: { Accept: 'application/json' },
          signal: controller.signal,
        });

        clearTimeout(timeoutId);

        if (!response.ok) {
          return Failure(`Docker Hub returned status ${response.status}`, {
            message: 'Failed to fetch tags from Docker Hub',
            hint: `HTTP ${response.status} error`,
            resolution: 'Verify the repository name and try again',
            details: { repository, status: response.status },
          });
        }

        const data = (await response.json()) as { results: Array<{ name: string }> };
        const tags = data.results.map((tag) => tag.name);

        this.logger.debug({ repository, tagCount: tags.length }, 'Listed Docker Hub tags');
        return Success(tags);
      } finally {
        clearTimeout(timeoutId);
      }
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      return Failure(`Failed to list Docker Hub tags: ${errorMessage}`, {
        message: 'Error fetching tags from Docker Hub',
        hint: 'Network error or repository not found',
        resolution: 'Check repository name and network connectivity',
        details: { repository, error: errorMessage },
      });
    }
  }

  /**
   * List tags from private registry using Docker Registry API v2
   */
  private async listPrivateRegistryTags(repository: string): Promise<Result<string[]>> {
    try {
      // Extract registry and repository path
      const parts = repository.split('/');
      const registry = parts[0];
      const repoPath = parts.slice(1).join('/');

      const url = `https://${registry}/v2/${repoPath}/tags/list`;

      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), REGISTRY_REQUEST_TIMEOUT_MS);

      try {
        const headers: Record<string, string> = { Accept: 'application/json' };

        // Add authentication if available
        if (this.authConfig) {
          const authString = Buffer.from(
            `${this.authConfig.username}:${this.authConfig.password}`,
          ).toString('base64');
          headers['Authorization'] = `Basic ${authString}`;
        }

        const response = await fetch(url, {
          headers,
          signal: controller.signal,
        });

        clearTimeout(timeoutId);

        if (!response.ok) {
          if (response.status === 401) {
            return Failure('Authentication required', {
              message: 'Registry requires authentication',
              hint: 'Unauthorized access to private registry',
              resolution: 'Call authenticate() with valid credentials before listing tags',
              details: { repository },
            });
          }

          return Failure(`Registry returned status ${response.status}`, {
            message: 'Failed to fetch tags from registry',
            hint: `HTTP ${response.status} error`,
            resolution: 'Verify the repository name and registry access',
            details: { repository, status: response.status },
          });
        }

        const data = (await response.json()) as { tags: string[] };
        const tags = data.tags || [];

        this.logger.debug({ repository, tagCount: tags.length }, 'Listed private registry tags');
        return Success(tags);
      } finally {
        clearTimeout(timeoutId);
      }
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      return Failure(`Failed to list private registry tags: ${errorMessage}`, {
        message: 'Error fetching tags from private registry',
        hint: 'Network error or invalid registry URL',
        resolution: 'Check registry URL and network connectivity',
        details: { repository, error: errorMessage },
      });
    }
  }
}

/**
 * Create a Docker Registry client
 *
 * @param docker - Dockerode instance
 * @param logger - Logger instance
 * @returns DockerRegistry client instance
 */
export function createDockerRegistry(docker: Docker, logger: Logger): DockerRegistry {
  return new DockerRegistry(docker, logger);
}
