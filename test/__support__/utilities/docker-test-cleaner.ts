/**
 * Centralized Docker Test Resource Management
 * Single source of truth for all Docker test cleanup operations
 * 
 * SAFETY MEASURES:
 * - Only removes images with specific test prefixes and patterns
 * - Only removes containers created from our test images
 * - Only removes dangling images that appear test-related
 * - Build cache cleanup is disabled by default
 * - All operations are scoped to avoid interfering with other Docker projects
 */

import type { Logger } from 'pino';
import type { DockerClient } from '../../../src/infra/docker/client';

/**
 * Configuration for Docker test cleanup
 */
export interface DockerTestConfig {
  /** Whether to clean build cache (default: false) - disabled to avoid interfering with other projects */
  cleanBuildCache: boolean;
  /** Timeout for cleanup operations in ms (default: 30000) */
  cleanupTimeoutMs: number;
  /** Whether to verify cleanup success (default: false) */
  verifyCleanup: boolean;
}

// Centralized test image constant - SINGLE SOURCE OF TRUTH  
export const TEST_IMAGE_NAME = 'container-assist-mcp-test-image' as const;

/**
 * Check if an image name is a test image we manage
 */
function isTestImage(imageName: string): boolean {
  return imageName === TEST_IMAGE_NAME;
}

const DEFAULT_CONFIG: DockerTestConfig = {
  cleanBuildCache: false, // Disabled by default to avoid interfering with other projects
  cleanupTimeoutMs: 30000,
  verifyCleanup: false,
};

/**
 * Centralized Docker test resource manager
 */
export class DockerTestCleaner {
  private readonly config: DockerTestConfig;
  private readonly logger: Logger;
  private readonly dockerClient: DockerClient;
  private readonly trackedImages = new Set<string>();
  private readonly trackedContainers = new Set<string>();

  constructor(logger: Logger, dockerClient: DockerClient, config: Partial<DockerTestConfig> = {}) {
    this.logger = logger;
    this.dockerClient = dockerClient;
    this.config = {
      ...DEFAULT_CONFIG,
      ...config
    };
  }

  /**
   * Track an image for cleanup
   */
  trackImage(imageIdOrTag: string): void {
    if (imageIdOrTag && imageIdOrTag.trim()) {
      this.trackedImages.add(imageIdOrTag.trim());
      this.logger.debug(`Tracking image: ${imageIdOrTag}`);
    }
  }

  /**
   * Track a container for cleanup
   */
  trackContainer(containerId: string): void {
    if (containerId && containerId.trim()) {
      this.trackedContainers.add(containerId.trim());
      this.logger.debug(`Tracking container: ${containerId}`);
    }
  }

  /**
   * Clean dangling containers that might prevent image cleanup
   */
  private async cleanDanglingContainers(): Promise<void> {
    try {
      // Get all stopped containers
      const containersResult = await this.dockerClient.listContainers({
        all: true,
        filters: { status: ['exited'] }
      });
      
      if (!containersResult.ok) {
        this.logger.debug(`Failed to list containers: ${containersResult.error}`);
        return;
      }

      const containers = containersResult.value;
      
      for (const container of containers) {
        try {
          // Check if this container is using one of our test images  
          if (isTestImage(container.Image)) {
            const removeResult = await this.dockerClient.removeContainer(container.Id, true);
            if (removeResult.ok) {
              this.logger.debug(`Removed dangling test container: ${container.Id} (image: ${container.Image})`);
            }
          }
        } catch (error) {
          // Container might have been removed already, continue
          this.logger.debug(`Failed to inspect/remove container ${container.Id}: ${error}`);
        }
      }
    } catch (error) {
      this.logger.debug(`Failed to clean dangling containers: ${error}`);
    }
  }

  /**
   * Clean up test containers by pattern (for use in afterEach hooks)
   * ENHANCED - clean both tagged containers and build intermediates
   */
  async cleanupContainers(): Promise<void> {
    try {
      // Clean containers created from our test image
      const containersResult = await this.dockerClient.listContainers({
        all: true,
        filters: { ancestor: [TEST_IMAGE_NAME] }
      });
      
      if (containersResult.ok) {
        const containers = containersResult.value;
        
        if (containers.length > 0) {
          for (const container of containers) {
            const removeResult = await this.dockerClient.removeContainer(container.Id, true);
            if (removeResult.ok) {
              this.logger.debug(`Removed test container: ${container.Id} (from ${TEST_IMAGE_NAME})`);
            } else {
              this.logger.debug(`Failed to remove container ${container.Id}: ${removeResult.error}`);
            }
          }
        }
      }

      // Also clean any dangling containers from failed builds
      await this.cleanDanglingContainers();
    } catch (error) {
      this.logger.debug(`Container cleanup failed: ${error}`);
    }
  }  /**
   * Execute cleanup with timeout and proper error handling
   */
  async cleanup(): Promise<void> {
    const cleanupPromise = this.performCleanup();

    // Create timeout that doesn't prevent process exit
    let timeoutId: NodeJS.Timeout;
    const timeoutPromise = new Promise<never>((_, reject) => {
      timeoutId = setTimeout(() => reject(new Error('Cleanup timeout')), this.config.cleanupTimeoutMs);
      // Unref the timeout so it doesn't keep the event loop alive
      timeoutId.unref();
    });

    try {
      await Promise.race([cleanupPromise, timeoutPromise]);
      if (this.config.verifyCleanup) {
        await this.verifyCleanupSuccess();
      }
    } catch (error) {
      this.logger.error(`Cleanup failed: ${error}`);
      throw error;
    } finally {
      // Clear the timeout if cleanup completed before timeout
      clearTimeout(timeoutId);
      this.trackedImages.clear();
      this.trackedContainers.clear();
    }
  }

  /**
   * Perform the actual cleanup operations - SIMPLIFIED
   */
  private async performCleanup(): Promise<void> {
    // Clean tracked containers first (they might reference images)
    await this.cleanTrackedContainers();

    await this.cleanTrackedImages();
    
    // Optional: Clean build cache if requested
    if (this.config.cleanBuildCache) {
      await this.cleanBuildCache();
    }
  }

  /**
   * Clean tracked containers
   */
  private async cleanTrackedContainers(): Promise<void> {
    const containers = Array.from(this.trackedContainers);
    if (containers.length === 0) return;

    this.logger.debug(`Cleaning ${containers.length} tracked containers`);
    
    for (const container of containers) {
      const removeResult = await this.dockerClient.removeContainer(container, true);
      if (removeResult.ok) {
        this.logger.debug(`Removed container: ${container}`);
      } else {
        this.logger.debug(`Failed to remove container ${container}: ${removeResult.error}`);
      }
    }
  }

  /**
   * Clean tracked images
   */
  private async cleanTrackedImages(): Promise<void> {
    const images = Array.from(this.trackedImages);
    if (images.length === 0) return;

    this.logger.debug(`Cleaning ${images.length} tracked images`);
    
    for (const image of images) {
      const removeResult = await this.dockerClient.removeImage(image, true);
      if (removeResult.ok) {
        this.logger.debug(`Removed image: ${image}`);
        // Remove from tracked set after successful removal
        this.trackedImages.delete(image);
      } else {
        this.logger.debug(`Failed to remove image ${image}: ${removeResult.error}`);
        // Keep in tracked set if removal failed
      }
    }
  }



  /**
   * Clean build cache (DISABLED by default to avoid interfering with other projects)
   * Only clean if explicitly enabled and we can target our specific builds
   */
  private async cleanBuildCache(): Promise<void> {
    // Skip build cache cleanup to avoid interfering with other Docker projects
    // Build cache is shared system-wide and cleaning it could slow down other builds
    this.logger.debug('Skipping build cache cleanup to avoid interfering with other projects');
    
    // If we really need to clean build cache in the future, we could:
    // 1. Add a specific label to our builds and filter by that
    // 2. Only clean if explicitly enabled with a dangerous flag
    // 3. Implement size-based cleanup (only if cache is very large)
    return;
  }

  /**
   * Verify cleanup success (if enabled) - SIMPLIFIED
   */
  private async verifyCleanupSuccess(): Promise<void> {
    try {
      // Simple verification: check if our tracked images are gone
      if (this.trackedImages.size > 0) {
        this.logger.warn(`Cleanup verification failed: ${this.trackedImages.size} tracked images remain`);
        throw new Error(`Cleanup incomplete: ${Array.from(this.trackedImages).join(', ')}`);
      }

      this.logger.debug('Cleanup verification successful');
    } catch (error) {
      this.logger.error(`Cleanup verification failed: ${error}`);
      throw error;
    }
  }

  /**
   * Get current tracked image count (for monitoring)
   */
  getTrackedImageCount(): number {
    return this.trackedImages.size;
  }

  /**
   * Static method for global cleanup - removes any test images without tracking
   * Uses a Docker client instance for consistency
   */
  static async globalCleanup(dockerClient: DockerClient): Promise<void> {
    try {
      // Simply remove the test image if it exists - no tracking needed
      const removeResult = await dockerClient.removeImage(TEST_IMAGE_NAME, true);
      if (!removeResult.ok) {
        // Ignore errors - image might not exist
        console.debug(`Global cleanup: ${removeResult.error}`);
      }
    } catch (error) {
      // Don't throw errors in global cleanup to avoid failing test runs
      console.warn('Global cleanup warning:', error);
    }
  }
}
