/**
 * Sampling Types - Core interfaces for Dockerfile sampling system
 */

import type { Logger } from 'pino';
import type { Result } from '@types';

/**
 * Repository analysis context for sampling
 */
export interface DockerfileContext {
  sessionId: string;
  repoPath: string;
  analysis: {
    language: string;
    framework?: string;
    packageManager: string;
    dependencies: string[];
    buildTools: string[];
    testFramework?: string;
    hasDatabase: boolean;
    ports: number[];
    environmentVars: Record<string, string>;
  };
  constraints: {
    targetEnvironment: 'development' | 'staging' | 'production';
    maxImageSize?: string;
    securityLevel: 'basic' | 'standard' | 'strict';
    buildTimeLimit?: number;
  };
}

/**
 * Generated Dockerfile variant with metadata
 */
export interface DockerfileVariant {
  id: string;
  content: string;
  strategy: string;
  metadata: {
    baseImage: string;
    optimization: 'size' | 'security' | 'performance' | 'balanced';
    features: string[];
    estimatedSize: string;
    buildComplexity: 'low' | 'medium' | 'high';
    securityFeatures: string[];
    aiEnhanced?: boolean;
  };
  generated: Date;
}

/**
 * Scoring criteria for variant evaluation
 */
export interface ScoringCriteria {
  security: number; // Weight 0-1
  performance: number; // Weight 0-1
  size: number; // Weight 0-1
  maintainability: number; // Weight 0-1
}

/**
 * Detailed score breakdown
 */
export interface ScoreDetails {
  total: number; // Weighted total score 0-100
  breakdown: {
    security: number; // Individual score 0-100
    performance: number; // Individual score 0-100
    size: number; // Individual score 0-100
    maintainability: number; // Individual score 0-100
  };
  reasons: string[]; // Detailed scoring explanations
  warnings: string[]; // Potential issues found
  recommendations: string[]; // Improvement suggestions
}

/**
 * Variant with computed score
 */
export interface ScoredVariant extends DockerfileVariant {
  score: ScoreDetails;
  rank: number;
}

/**
 * Sampling strategy interface
 */
export interface SamplingStrategy {
  name: string;
  description: string;
  optimization: 'size' | 'security' | 'performance' | 'balanced';

  /**
   * Generate Dockerfile variant using this strategy
   */
  generateVariant(context: DockerfileContext, logger: Logger): Promise<Result<DockerfileVariant>>;

  /**
   * Score a variant based on this strategy's criteria
   */
  scoreVariant(
    variant: DockerfileVariant,
    criteria: ScoringCriteria,
    logger: Logger,
  ): Promise<Result<ScoreDetails>>;
}

/**
 * Selection constraints for choosing best variant
 */
export interface SelectionConstraints {
  mustHave?: string[]; // Required features
  mustNotHave?: string[]; // Forbidden features
  minScore?: number; // Minimum acceptable score
  preferredOptimization?: 'size' | 'security' | 'performance' | 'balanced';
}

/**
 * Complete sampling result
 */
export interface SamplingResult {
  sessionId: string;
  variants: ScoredVariant[];
  bestVariant: ScoredVariant;
  criteria: ScoringCriteria;
  metadata: {
    totalVariants: number;
    strategiesUsed: string[];
    samplingDuration: number;
    scoringDuration: number;
    context: DockerfileContext;
  };
  generated: Date;
}

/**
 * Sampling configuration options
 */
export interface SamplingConfig {
  sessionId: string;
  repoPath: string;
  variantCount?: number; // Default: 5
  strategies?: string[]; // Default: all available
  criteria?: Partial<ScoringCriteria>; // Default: balanced weights
  constraints?: SelectionConstraints;
  environment?: 'development' | 'staging' | 'production'; // Default: development
  enableCaching?: boolean; // Default: true
  timeout?: number; // Default: 60000ms
}

/**
 * Sampling options for workflow integration
 */
export interface SamplingOptions {
  environment: 'development' | 'staging' | 'production';
  optimization?: 'size' | 'security' | 'performance' | 'balanced';
  customCriteria?: Partial<ScoringCriteria>;
}
