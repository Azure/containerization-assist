/**
 * Functional Analysis Sampling Service - Drop-in replacement for class-based AnalysisSamplingService
 *
 * This provides the same API as the original AnalysisSamplingService but uses functional
 * implementation internally for better performance and maintainability.
 */

import type { Logger } from 'pino';
import { type Result } from '@types';
import type {
  AnalysisContext,
  AnalysisVariant,
  AnalysisScoringCriteria,
  AnalysisSamplingResult,
  AnalysisSamplingConfig,
} from './analysis-types';
import { createAnalysisSampling, type AnalysisSampler } from './functional-strategies';

/**
 * Configuration options for Analysis Sampling Service
 */
export interface AnalysisSamplingServiceConfig {
  defaultCriteria?: AnalysisScoringCriteria;
  defaultSamplingConfig?: AnalysisSamplingConfig;
  enableCaching?: boolean;
  maxVariants?: number;
}

/**
 * Analysis comparison result
 */
export interface AnalysisComparisonResult {
  variants: Array<{
    strategy: string;
    score: number;
    strengths: string[];
    weaknesses: string[];
    recommendation: 'recommended' | 'acceptable' | 'not-recommended';
  }>;
  summary: {
    bestStrategy: string;
    worstStrategy: string;
    averageScore: number;
    recommendedCount: number;
  };
}

/**
 * Functional replacement for AnalysisSamplingService class
 * Maintains identical API for backward compatibility
 */
export class AnalysisSamplingService {
  private analysisSampler: AnalysisSampler;

  constructor(
    private _logger: Logger,
    config: AnalysisSamplingServiceConfig = {},
  ) {
    // Initialize config but don't store it since it's not used elsewhere
    const configWithDefaults = {
      maxVariants: 5,
      enableCaching: false,
      ...config,
    };

    // Use logger to record config if needed for debugging
    this._logger.debug({ config: configWithDefaults }, 'AnalysisSamplingService initialized');

    this.analysisSampler = createAnalysisSampling(this._logger);
  }

  /**
   * Generate the best analysis for a repository using intelligent sampling
   */
  async generateBestAnalysis(
    context: AnalysisContext,
    criteria?: AnalysisScoringCriteria,
    samplingConfig?: AnalysisSamplingConfig,
  ): Promise<Result<AnalysisSamplingResult>> {
    return this.analysisSampler.generateBest(context, criteria, samplingConfig);
  }

  /**
   * Compare multiple analysis variants
   */
  async compareAnalysisVariants(
    variants: AnalysisVariant[],
    criteria?: AnalysisScoringCriteria,
  ): Promise<Result<AnalysisComparisonResult>> {
    return this.analysisSampler.compareVariants(variants, criteria);
  }

  /**
   * Validate analysis variant
   */
  async validateAnalysisVariant(
    variant: AnalysisVariant,
  ): Promise<Result<{ isValid: boolean; issues: string[] }>> {
    return this.analysisSampler.validateVariant(variant);
  }

  /**
   * Get list of available analysis strategies
   */
  getAvailableStrategies(): string[] {
    return this.analysisSampler.getAvailableStrategies();
  }
}
