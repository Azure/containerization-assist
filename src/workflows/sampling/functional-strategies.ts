/**
 * Functional Sampling Strategies
 *
 * Simplified function-based approach to replace class-heavy sampling system.
 * This provides the same functionality with significantly less overhead.
 */

import type { Logger } from 'pino';
import { Success, Failure, type Result } from '@types';
import type {
  SamplingConfig,
  SamplingOptions,
  SamplingResult,
  DockerfileVariant,
  ScoredVariant,
  ScoringCriteria,
} from './types';
import type {
  ScoredAnalysisVariant,
  AnalysisContext,
  AnalysisVariant,
  AnalysisScoringCriteria,
  AnalysisSamplingResult,
  AnalysisSamplingConfig,
} from './analysis-types';

interface ComparisonVariant {
  strategy: string;
  score: number;
  strengths: string[];
  weaknesses: string[];
  recommendation: 'recommended' | 'acceptable' | 'not-recommended';
}
import { VariantGenerationPipeline } from './generation-pipeline';
import { createMCPAIOrchestrator } from '@workflows/intelligent-orchestration';
import { DEFAULT_SCORING_CRITERIA } from './scorer';
import { AnalysisGenerationPipeline, AnalysisValidator } from './analysis-generation-pipeline';
import { AnalysisVariantScorer } from './analysis-scorer';

// ============================================================================
// CORE FUNCTIONAL TYPES
// ============================================================================

export interface DockerfileSampler {
  generateBest(
    config: { sessionId: string; repoPath: string },
    options: SamplingOptions,
  ): Promise<Result<{ content: string; score: number; metadata: Record<string, unknown> }>>;
  generateVariants(config: SamplingConfig): Promise<Result<SamplingResult>>;
  compareDockerfiles(
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
  >;
  validateDockerfile(
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
  >;
  getAvailableStrategies(): string[];
}

export interface AnalysisSampler {
  generateBest(
    context: AnalysisContext,
    criteria?: AnalysisScoringCriteria,
    samplingConfig?: AnalysisSamplingConfig,
  ): Promise<Result<AnalysisSamplingResult>>;
  compareVariants(
    variants: AnalysisVariant[],
    criteria?: AnalysisScoringCriteria,
  ): Promise<
    Result<{
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
    }>
  >;
  validateVariant(
    variant: AnalysisVariant,
  ): Promise<Result<{ isValid: boolean; issues: string[] }>>;
  getAvailableStrategies(): string[];
}

// ============================================================================
// STRATEGY FACTORY REGISTRY
// ============================================================================

// ============================================================================
// MAIN SAMPLING FUNCTION
// ============================================================================

// ============================================================================
// DOCKERFILE SAMPLING FUNCTIONS (TO BE IMPLEMENTED)
// ============================================================================

/**
 * Creates a dockerfile sampling function suite
 */
export function createDockerfileSampling(logger: Logger): DockerfileSampler {
  const pipeline = new VariantGenerationPipeline(logger);
  const aiOrchestrator = createMCPAIOrchestrator(logger, {});

  return {
    async generateBest(config, options) {
      try {
        const samplingConfig: SamplingConfig = {
          sessionId: config.sessionId,
          repoPath: config.repoPath,
          variantCount: 5,
          enableCaching: true,
          timeout: 60000,
          criteria: buildScoringCriteria(options),
        };

        if (options.optimization) {
          samplingConfig.constraints = { preferredOptimization: options.optimization };
        }

        const result = await pipeline.generateSampledDockerfiles(samplingConfig);

        if (!result.ok) {
          return Failure(`Pipeline generation failed: ${result.error}`);
        }

        const samplingResult = result.value;
        if (!samplingResult || samplingResult.variants.length === 0) {
          return Failure('No variants generated');
        }

        const bestVariant = samplingResult.variants[0];
        if (!bestVariant) {
          return Failure('No best variant found');
        }

        return Success({
          content: bestVariant.content,
          score: bestVariant.score.total / 100,
          metadata: {
            approach: 'functional-pipeline',
            environment: options.environment,
            variants: samplingResult.variants.length,
            strategy: bestVariant.strategy,
            generatedAt: new Date().toISOString(),
          },
        });
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        logger.error(
          { error: message, sessionId: config.sessionId },
          'Functional dockerfile sampling failed',
        );
        return Failure(`Sampling error: ${message}`);
      }
    },

    async generateVariants(config) {
      // Validation using AI orchestrator
      const validationContext = {
        toolName: 'dockerfile-sampling',
        repositoryPath: config.repoPath,
        environment: config.environment || 'development',
        targetType: 'dockerfile',
      };

      const validationResult = await aiOrchestrator.validateParameters(
        'dockerfile-sampling',
        config as unknown as Record<string, unknown>,
        validationContext as unknown as Record<string, unknown>,
      );

      if (validationResult.ok && !validationResult.value.isValid) {
        logger.warn(
          {
            sessionId: config.sessionId,
            errors: validationResult.value.errors,
            warnings: validationResult.value.warnings,
          },
          'Sampling configuration validation issues detected',
        );

        // Return validation errors if critical
        const criticalErrors = validationResult.value.errors.filter(
          (error: string) => error.includes('required') || error.includes('invalid'),
        );

        if (criticalErrors.length > 0) {
          return Failure(`Configuration validation failed: ${criticalErrors.join('; ')}`);
        }
      }

      // Generate variants using pipeline
      return await pipeline.generateSampledDockerfiles(config);
    },

    async compareDockerfiles(dockerfiles, criteria) {
      try {
        // Convert input to DockerfileVariant format
        const variants: DockerfileVariant[] = dockerfiles.map((df, index) => ({
          id: df.id || `comparison-${index}`,
          content: df.content,
          strategy: df.strategy || 'unknown',
          metadata: {
            baseImage: extractBaseImage(df.content),
            optimization: 'balanced',
            features: [],
            estimatedSize: 'unknown',
            buildComplexity: 'medium',
            securityFeatures: [],
          },
          generated: new Date(),
        }));

        // Score all variants
        const scoringCriteria = criteria || DEFAULT_SCORING_CRITERIA;
        const scoredResult = await pipeline.scoreVariants(variants, scoringCriteria);

        if (!scoredResult.ok) {
          return Failure(`Comparison scoring failed: ${scoredResult.error}`);
        }

        const scoredData = scoredResult.value;
        if (!scoredData || scoredData.length === 0) {
          return Failure('No scored variants available');
        }

        // Map scored data back to ScoredVariant objects
        const scoredVariants: ScoredVariant[] = [];
        for (const scored of scoredData) {
          const originalVariant = variants.find((v) => v.id === scored.id);
          if (!originalVariant) {
            return Failure(`Original variant with id ${scored.id} not found`);
          }
          scoredVariants.push({
            ...originalVariant,
            score: scored.score,
            rank: scoredData.indexOf(scored) + 1,
          });
        }

        const bestVariant = scoredVariants[0];
        if (!bestVariant) {
          return Failure('No best variant found in comparison');
        }

        // Generate comparison analysis
        const comparison = generateComparisonAnalysis(scoredVariants);

        logger.info(
          {
            variantsCompared: scoredVariants.length,
            bestVariant: bestVariant.id,
            bestScore: bestVariant.score.total,
          },
          'Functional dockerfile comparison completed',
        );

        return Success({
          variants: scoredVariants,
          bestVariant,
          comparison,
        });
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        logger.error({ error: message }, 'Functional dockerfile comparison failed');
        return Failure(`Comparison failed: ${message}`);
      }
    },

    async validateDockerfile(content, criteria) {
      try {
        const variant: DockerfileVariant = {
          id: `validation-${Date.now()}`,
          content,
          strategy: 'validation',
          metadata: {
            baseImage: extractBaseImage(content),
            optimization: 'balanced',
            features: [],
            estimatedSize: 'unknown',
            buildComplexity: 'medium',
            securityFeatures: [],
          },
          generated: new Date(),
        };

        const scoredResult = await pipeline.scoreVariants(
          [variant],
          criteria || DEFAULT_SCORING_CRITERIA,
        );

        if (!scoredResult.ok) {
          return Failure(`Validation scoring failed: ${scoredResult.error}`);
        }

        const scoredValue = scoredResult.value;
        if (!scoredValue || scoredValue.length === 0) {
          return Failure('No validation scores available');
        }

        const scored = scoredValue[0];
        if (!scored) {
          return Failure('No validation score data found');
        }

        const isValid = scored.score.total >= 60; // Minimum acceptable score

        return Success({
          score: scored.score.total,
          breakdown: scored.score.breakdown,
          issues: scored.score.warnings,
          recommendations: scored.score.recommendations,
          isValid,
        });
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        logger.error({ error: message }, 'Functional dockerfile validation failed');
        return Failure(`Validation failed: ${message}`);
      }
    },

    getAvailableStrategies() {
      return pipeline.getAvailableStrategies();
    },
  };
}

// ============================================================================
// ANALYSIS SAMPLING FUNCTIONS (TO BE IMPLEMENTED)
// ============================================================================

/**
 * Creates an analysis sampling function suite
 */
export function createAnalysisSampling(logger: Logger): AnalysisSampler {
  const pipeline = new AnalysisGenerationPipeline(logger);
  const validator = new AnalysisValidator(logger);
  const scorer = new AnalysisVariantScorer(logger);

  return {
    async generateBest(context, criteria, samplingConfig) {
      const effectiveCriteria = criteria || getDefaultAnalysisCriteria(context);
      const effectiveConfig = { maxVariants: 5, enableCaching: false, ...samplingConfig };

      logger.info(
        {
          repoPath: context.repoPath,
          language: context.language,
          criteria: Object.keys(effectiveCriteria),
        },
        'Generating best analysis using functional sampling',
      );

      try {
        const result = await pipeline.executePipeline(context, effectiveCriteria, effectiveConfig);

        if (result.ok) {
          logger.info(
            {
              strategy: result.value.bestVariant.strategy,
              score: result.value.bestVariant.score,
              executionTime: result.value.metadata.executionTime,
            },
            'Best analysis generated successfully',
          );
        }

        return result;
      } catch (error) {
        logger.error({ error, context }, 'Functional analysis generation failed');
        return Failure(error instanceof Error ? error.message : String(error));
      }
    },

    async compareVariants(variants, criteria) {
      if (variants.length === 0) {
        return Failure('No variants provided for comparison');
      }

      logger.info(
        {
          variantCount: variants.length,
          strategies: variants.map((v) => v.strategy),
        },
        'Comparing analysis variants using functional approach',
      );

      try {
        // Use default criteria if none provided
        const effectiveCriteria = criteria || {
          accuracy: { weight: 0.3, minScore: 0.6 },
          completeness: { weight: 0.25, minScore: 0.5 },
          relevance: { weight: 0.25, minScore: 0.5 },
          actionability: { weight: 0.2, minScore: 0.4 },
        };

        // Score all variants
        const scoringResult = await scorer.scoreAnalysisVariants(variants, effectiveCriteria);
        if (!scoringResult.ok) {
          return scoringResult;
        }

        const scoredVariants = scoringResult.value;

        // Analyze each variant
        const comparisonVariants: ComparisonVariant[] = scoredVariants.map(
          (variant: ScoredAnalysisVariant) => {
            const strengths: string[] = [];
            const weaknesses: string[] = [];

            // Analyze scores to determine strengths and weaknesses
            if (variant.score.breakdown.accuracy >= 80) strengths.push('High accuracy analysis');
            if (variant.score.breakdown.completeness >= 80)
              strengths.push('Comprehensive coverage');
            if (variant.score.breakdown.relevance >= 80) strengths.push('Highly relevant insights');
            if (variant.score.breakdown.actionability >= 80)
              strengths.push('Clear actionable recommendations');

            if (variant.score.breakdown.accuracy < 60) weaknesses.push('Lower accuracy');
            if (variant.score.breakdown.completeness < 60) weaknesses.push('Limited coverage');
            if (variant.score.breakdown.relevance < 60) weaknesses.push('Less relevant');
            if (variant.score.breakdown.actionability < 60)
              weaknesses.push('Vague recommendations');

            // Determine recommendation level
            let recommendation: 'recommended' | 'acceptable' | 'not-recommended';
            if (variant.score.total >= 80) {
              recommendation = 'recommended';
            } else if (variant.score.total >= 60) {
              recommendation = 'acceptable';
            } else {
              recommendation = 'not-recommended';
            }

            return {
              strategy: variant.strategy,
              score: variant.score.total,
              strengths,
              weaknesses,
              recommendation,
            };
          },
        );

        // Calculate summary statistics
        const scores = scoredVariants.map((v: ScoredAnalysisVariant) => v.score.total);
        const averageScore =
          scores.reduce((sum: number, score: number) => sum + score, 0) / scores.length;
        const recommendedCount = comparisonVariants.filter(
          (v) => v.recommendation === 'recommended',
        ).length;

        const sortedVariants = [...comparisonVariants].sort((a, b) => b.score - a.score);
        const bestStrategy = sortedVariants[0]?.strategy ?? 'none';
        const worstStrategy = sortedVariants[sortedVariants.length - 1]?.strategy ?? 'none';

        const result = {
          variants: comparisonVariants,
          summary: {
            bestStrategy,
            worstStrategy,
            averageScore,
            recommendedCount,
          },
        };

        logger.info(
          {
            bestStrategy,
            averageScore: averageScore.toFixed(3),
            recommendedCount,
            totalVariants: variants.length,
          },
          'Functional analysis variants compared successfully',
        );

        return Success(result);
      } catch (error) {
        logger.error({ error, variants: variants.length }, 'Functional analysis comparison failed');
        return Failure(error instanceof Error ? error.message : String(error));
      }
    },

    async validateVariant(variant) {
      logger.debug({ strategy: variant.strategy }, 'Validating analysis variant functionally');

      try {
        const validationResult = validator.validateVariant(variant);
        const issues: string[] = [];

        if (!validationResult.ok) {
          issues.push(validationResult.error);
        }

        // Additional semantic validation
        if (!variant.dependencies || variant.dependencies.length === 0) {
          issues.push('No dependencies analyzed - may indicate incomplete analysis');
        }

        if (!variant.recommendations || Object.keys(variant.recommendations).length === 0) {
          issues.push('No recommendations provided');
        }

        const isValid = issues.length === 0;

        logger.info(
          {
            strategy: variant.strategy,
            isValid,
            issueCount: issues.length,
          },
          'Functional analysis variant validation completed',
        );

        return Success({ isValid, issues });
      } catch (error) {
        logger.error(
          { error, strategy: variant.strategy },
          'Functional analysis validation failed',
        );
        return Failure(error instanceof Error ? error.message : String(error));
      }
    },

    getAvailableStrategies() {
      return ['comprehensive', 'security-focused', 'performance-focused'];
    },
  };
}

// ============================================================================
// HELPER FUNCTIONS (extracted from original SamplingService)
// ============================================================================

function buildScoringCriteria(options: SamplingOptions): Partial<ScoringCriteria> {
  if (options.customCriteria) {
    return options.customCriteria;
  }

  // Environment-based defaults
  const environmentWeights: Record<string, Partial<ScoringCriteria>> = {
    production: { security: 0.4, performance: 0.3, size: 0.2, maintainability: 0.1 },
    staging: { security: 0.3, performance: 0.3, size: 0.2, maintainability: 0.2 },
    development: { security: 0.1, performance: 0.2, size: 0.2, maintainability: 0.5 },
  };

  return environmentWeights[options.environment] || {};
}

function extractBaseImage(content: string): string {
  const lines = content.split('\n');
  const fromLine = lines.find((line) => line.trim().toLowerCase().startsWith('from '));

  if (fromLine) {
    const parts = fromLine.trim().split(/\s+/);
    if (parts.length >= 2 && parts[1]) {
      return parts[1];
    }
  }

  return 'unknown';
}

function generateComparisonAnalysis(scoredVariants: ScoredVariant[]): {
  summary: string;
  advantages: Record<string, string[]>;
  tradeoffs: Record<string, string[]>;
} {
  const best = scoredVariants[0];
  if (!best) {
    return {
      summary: 'No variants available',
      advantages: {},
      tradeoffs: {},
    };
  }

  const summary = `Best variant: ${best.id} (${best.strategy}) with score ${best.score.total}/100`;

  const advantages: Record<string, string[]> = {};
  const tradeoffs: Record<string, string[]> = {};

  scoredVariants.forEach((variant) => {
    advantages[variant.id] = variant.score.reasons;
    tradeoffs[variant.id] = variant.score.warnings;
  });

  return { summary, advantages, tradeoffs };
}

function getDefaultAnalysisCriteria(context: AnalysisContext): AnalysisScoringCriteria {
  // Default balanced criteria
  let criteria: AnalysisScoringCriteria = {
    accuracy: { weight: 0.3, minScore: 0.6 },
    completeness: { weight: 0.25, minScore: 0.5 },
    relevance: { weight: 0.25, minScore: 0.5 },
    actionability: { weight: 0.2, minScore: 0.4 },
  };

  // Adjust based on context
  if (context.securityFocus) {
    criteria = {
      accuracy: { weight: 0.4, minScore: 0.7 },
      completeness: { weight: 0.2, minScore: 0.6 },
      relevance: { weight: 0.3, minScore: 0.6 },
      actionability: { weight: 0.1, minScore: 0.5 },
    };
  } else if (context.performanceFocus) {
    criteria = {
      accuracy: { weight: 0.25, minScore: 0.6 },
      completeness: { weight: 0.3, minScore: 0.5 },
      relevance: { weight: 0.25, minScore: 0.6 },
      actionability: { weight: 0.2, minScore: 0.5 },
    };
  }

  return criteria;
}
