/**
 * Functional Sampling Service - Drop-in replacement for class-based SamplingService
 *
 * This provides the same API as the original SamplingService but uses functional
 * implementation internally for better performance and maintainability.
 */

import type { Logger } from 'pino';
import { type Result } from '@types';
import type {
  SamplingConfig,
  SamplingOptions,
  SamplingResult,
  ScoringCriteria,
  ScoredVariant,
} from './types';
import { createDockerfileSampling, type DockerfileSampler } from './functional-strategies';

/**
 * Functional replacement for SamplingService class
 * Maintains identical API for backward compatibility
 */
export class SamplingService {
  private dockerfileSampler: DockerfileSampler;

  constructor(private _logger: Logger) {
    this.dockerfileSampler = createDockerfileSampling(this._logger);
  }

  /**
   * Generate multiple Dockerfile variants and select the best one
   * This is the main method used by the workflow
   */
  async generateBestDockerfile(
    config: { sessionId: string; repoPath: string },
    options: SamplingOptions,
    _logger?: Logger, // Kept for API compatibility but not used
  ): Promise<Result<{ content: string; score: number; metadata: Record<string, unknown> }>> {
    return this.dockerfileSampler.generateBest(config, options);
  }

  /**
   * Generate and score multiple variants (for detailed analysis)
   */
  async generateVariants(config: SamplingConfig): Promise<Result<SamplingResult>> {
    return this.dockerfileSampler.generateVariants(config);
  }

  /**
   * Compare multiple Dockerfile variants
   */
  async compareDockerfiles(
    dockerfiles: { id: string; content: string; strategy?: string }[],
    criteria?: ScoringCriteria,
  ): Promise<
    Result<{
      variants: ScoredVariant[];
      bestVariant: ScoredVariant;
      comparison: {
        summary: string;
        advantages: Record<string, string[]>;
        tradeoffs: Record<string, string[]>;
      };
    }>
  > {
    return this.dockerfileSampler.compareDockerfiles(dockerfiles, criteria);
  }

  /**
   * Validate a Dockerfile against best practices
   */
  async validateDockerfile(
    content: string,
    criteria?: ScoringCriteria,
  ): Promise<
    Result<{
      score: number;
      breakdown: Record<string, number>;
      issues: string[];
      recommendations: string[];
      isValid: boolean;
    }>
  > {
    return this.dockerfileSampler.validateDockerfile(content, criteria);
  }

  /**
   * Get available sampling strategies
   */
  getAvailableStrategies(): string[] {
    return this.dockerfileSampler.getAvailableStrategies();
  }

  /**
   * Get sampling resource statistics (placeholder for compatibility)
   */
  getSamplingResourceStats(): {
    totalResources: number;
    activeResources: number;
    cachedResources: number;
    memoryUsage: number;
  } {
    return {
      totalResources: 0,
      activeResources: 0,
      cachedResources: 0,
      memoryUsage: 0,
    };
  }
}
