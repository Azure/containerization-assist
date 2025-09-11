import { Result, Success, Failure } from '../../src/core/types.js';
import type { Resource, ResourceManager } from '../../src/mcp/resources/types.js';
import { parseUri, matchesUriPattern } from '../../src/mcp/resources/uri-schemes.js';

/**
 * Mock ResourceManager for testing
 * Simulates real behavior without external dependencies
 */
export class MockResourceManager implements ResourceManager {
  private resources = new Map<string, Resource>();
  private readonly config: {
    maxSize: number;
    defaultTtl: number;
    simulateLatency: boolean;
    failureRate: number;
  };

  constructor(config?: Partial<typeof MockResourceManager.prototype.config>) {
    this.config = {
      maxSize: 5 * 1024 * 1024, // 5MB
      defaultTtl: 3600000, // 1 hour
      simulateLatency: false,
      failureRate: 0, // 0% failure rate by default
      ...config,
    };
  }

  async publish(uri: string, content: unknown, ttl?: number): Promise<Result<string>> {
    await this.simulateDelay();

    if (this.shouldSimulateFailure()) {
      return Failure(`Mock failure for publish operation on ${uri}`);
    }

    try {
      // Validate URI
      const parseResult = parseUri(uri);
      if (!parseResult.ok) {
        return Failure(`Invalid URI: ${parseResult.error}`);
      }

      // Check size
      const contentSize = this.getContentSize(content);
      if (contentSize > this.config.maxSize) {
        return Failure(`Resource too large: ${contentSize} bytes (max: ${this.config.maxSize})`);
      }

      // Create resource
      const now = new Date();
      const effectiveTtl = ttl ?? this.config.defaultTtl;

      const resource: Resource = {
        uri,
        content,
        mimeType: this.determineMimeType(content),
        createdAt: now,
        expiresAt: effectiveTtl > 0 ? new Date(now.getTime() + effectiveTtl) : undefined,
        metadata: {
          size: contentSize,
          scheme: parseResult.value.scheme,
          mock: true,
        },
      };

      this.resources.set(uri, resource);

      console.log(`[MockResourceManager] Published resource: ${uri} (${contentSize} bytes)`);
      return Success(uri);
    } catch (error) {
      return Failure(`Mock publish failed: ${error.message}`);
    }
  }

  async read(uri: string): Promise<Result<Resource | null>> {
    await this.simulateDelay();

    if (this.shouldSimulateFailure()) {
      return Failure(`Mock failure for read operation on ${uri}`);
    }

    try {
      const resource = this.resources.get(uri);

      if (!resource) {
        console.log(`[MockResourceManager] Resource not found: ${uri}`);
        return Success(null);
      }

      // Check expiration
      if (resource.expiresAt && new Date() > resource.expiresAt) {
        this.resources.delete(uri);
        console.log(`[MockResourceManager] Resource expired: ${uri}`);
        return Success(null);
      }

      console.log(`[MockResourceManager] Resource read: ${uri}`);
      return Success(resource);
    } catch (error) {
      return Failure(`Mock read failed: ${error.message}`);
    }
  }

  async invalidate(pattern: string): Promise<Result<void>> {
    await this.simulateDelay();

    if (this.shouldSimulateFailure()) {
      return Failure(`Mock failure for invalidate operation with pattern ${pattern}`);
    }

    try {
      let invalidatedCount = 0;

      for (const [uri] of this.resources.entries()) {
        if (matchesUriPattern(uri, pattern)) {
          this.resources.delete(uri);
          invalidatedCount++;
        }
      }

      console.log(`[MockResourceManager] Invalidated ${invalidatedCount} resources with pattern: ${pattern}`);
      return Success(undefined);
    } catch (error) {
      return Failure(`Mock invalidate failed: ${error.message}`);
    }
  }

  async list(pattern: string): Promise<Result<string[]>> {
    await this.simulateDelay();

    if (this.shouldSimulateFailure()) {
      return Failure(`Mock failure for list operation with pattern ${pattern}`);
    }

    try {
      const matchingUris: string[] = [];

      for (const [uri] of this.resources.entries()) {
        if (matchesUriPattern(uri, pattern)) {
          matchingUris.push(uri);
        }
      }

      console.log(`[MockResourceManager] Listed ${matchingUris.length} resources matching: ${pattern}`);
      return Success(matchingUris);
    } catch (error) {
      return Failure(`Mock list failed: ${error.message}`);
    }
  }

  async cleanup(): Promise<Result<void>> {
    await this.simulateDelay();

    if (this.shouldSimulateFailure()) {
      return Failure('Mock failure for cleanup operation');
    }

    try {
      const now = new Date();
      let cleanedCount = 0;

      for (const [uri, resource] of this.resources.entries()) {
        if (resource.expiresAt && now > resource.expiresAt) {
          this.resources.delete(uri);
          cleanedCount++;
        }
      }

      console.log(`[MockResourceManager] Cleaned up ${cleanedCount} expired resources`);
      return Success(undefined);
    } catch (error) {
      return Failure(`Mock cleanup failed: ${error.message}`);
    }
  }

  async getMetadata(uri: string): Promise<Result<Omit<Resource, 'content'> | null>> {
    await this.simulateDelay();

    if (this.shouldSimulateFailure()) {
      return Failure(`Mock failure for metadata operation on ${uri}`);
    }

    try {
      const resource = this.resources.get(uri);

      if (!resource) {
        return Success(null);
      }

      // Check expiration
      if (resource.expiresAt && new Date() > resource.expiresAt) {
        this.resources.delete(uri);
        return Success(null);
      }

      const { content: _content, ...metadata } = resource;
      return Success(metadata);
    } catch (error) {
      return Failure(`Mock getMetadata failed: ${error.message}`);
    }
  }

  /**
   * Mock-specific methods for testing
   */

  /**
   * Get current resource count (for testing)
   */
  getResourceCount(): number {
    return this.resources.size;
  }

  /**
   * Clear all resources (for testing)
   */
  clearAll(): void {
    this.resources.clear();
    console.log('[MockResourceManager] All resources cleared');
  }

  /**
   * Set failure rate for simulating errors
   */
  setFailureRate(rate: number): void {
    this.config.failureRate = Math.max(0, Math.min(1, rate));
    console.log(`[MockResourceManager] Failure rate set to ${this.config.failureRate * 100}%`);
  }

  /**
   * Enable/disable latency simulation
   */
  setLatencySimulation(enabled: boolean): void {
    this.config.simulateLatency = enabled;
    console.log(`[MockResourceManager] Latency simulation ${enabled ? 'enabled' : 'disabled'}`);
  }

  /**
   * Get mock statistics
   */
  getStats(): {
    resourceCount: number;
    totalSize: number;
    expiredCount: number;
  } {
    let totalSize = 0;
    let expiredCount = 0;
    const now = new Date();

    for (const resource of this.resources.values()) {
      totalSize += this.getContentSize(resource.content);
      if (resource.expiresAt && now > resource.expiresAt) {
        expiredCount++;
      }
    }

    return {
      resourceCount: this.resources.size,
      totalSize,
      expiredCount,
    };
  }

  private async simulateDelay(): Promise<void> {
    if (this.config.simulateLatency) {
      const delay = Math.random() * 50 + 10; // 10-60ms delay
      await new Promise(resolve => setTimeout(resolve, delay));
    }
  }

  private shouldSimulateFailure(): boolean {
    return Math.random() < this.config.failureRate;
  }

  private getContentSize(content: unknown): number {
    if (typeof content === 'string') {
      return Buffer.byteLength(content, 'utf8');
    }

    if (Buffer.isBuffer(content)) {
      return content.length;
    }

    return Buffer.byteLength(JSON.stringify(content), 'utf8');
  }

  private determineMimeType(content: unknown): string {
    if (typeof content === 'string') {
      try {
        JSON.parse(content);
        return 'application/json';
      } catch {
        return 'text/plain';
      }
    }

    if (Buffer.isBuffer(content)) {
      return 'application/octet-stream';
    }

    return 'application/json';
  }
}

/**
 * Factory function for creating mock resource manager instances
 */
export const createMockResourceManager = (config?: Parameters<typeof MockResourceManager.prototype.constructor>[0]): ResourceManager => {
  return new MockResourceManager(config);
};
