/**
 * Analysis Variant Generation Pipeline
 *
 * Orchestrates the complete analysis sampling workflow:
 * 1. Validate analysis context
 * 2. Generate analysis variants using different strategies
 * 3. Score and rank variants
 * 4. Select optimal analysis approach
 */

import type { Logger } from 'pino';
import { Success, Failure, type Result } from '@types';
import type {
  AnalysisContext,
  AnalysisVariant,
  AnalysisScoringCriteria,
  AnalysisSamplingResult,
  AnalysisSamplingConfig,
} from './analysis-types';
import { executeMultipleAnalysisStrategies } from './analysis-strategies';
import { AnalysisVariantScorer } from './analysis-scorer';

/**
 * Validates analysis context for completeness and consistency
 */
export class AnalysisValidator {
  constructor(private logger: Logger) {}

  /**
   * Validate analysis context
   */
  validateContext(context: AnalysisContext): Result<true> {
    const errors: string[] = [];

    // Required fields
    if (!context.repoPath?.trim()) {
      errors.push('Repository path is required');
    }

    if (!context.language?.trim()) {
      errors.push('Primary language must be specified');
    }

    // Validate dependencies structure
    if (context.dependencies) {
      const invalidDeps = context.dependencies.filter(
        (dep) => !dep.name?.trim() || !dep.type?.trim(),
      );
      if (invalidDeps.length > 0) {
        errors.push(
          `Invalid dependencies found: ${invalidDeps.length} entries missing name or type`,
        );
      }
    }

    // Validate ports
    if (context.ports) {
      const invalidPorts = context.ports.filter(
        (port) => !Number.isInteger(port) || port < 1 || port > 65535,
      );
      if (invalidPorts.length > 0) {
        errors.push(`Invalid ports found: ${invalidPorts.join(', ')}`);
      }
    }

    if (errors.length > 0) {
      this.logger.warn({ context, errors }, 'Analysis context validation failed');
      return Failure(`Context validation failed: ${errors.join(', ')}`);
    }

    this.logger.debug({ context }, 'Analysis context validated successfully');
    return Success(true);
  }

  /**
   * Validate analysis variant
   */
  validateVariant(variant: AnalysisVariant): Result<true> {
    const errors: string[] = [];

    if (!variant.strategy?.trim()) {
      errors.push('Variant strategy is required');
    }

    if (!variant.language?.trim()) {
      errors.push('Analysis must include primary language');
    }

    if (!variant.recommendations) {
      errors.push('Analysis must include recommendations');
    }

    if (variant.confidence < 0 || variant.confidence > 1) {
      errors.push('Confidence score must be between 0 and 1');
    }

    if (errors.length > 0) {
      this.logger.warn({ variant: variant.strategy, errors }, 'Analysis variant validation failed');
      return Failure(`Variant validation failed: ${errors.join(', ')}`);
    }

    return Success(true);
  }
}

/**
 * Analysis variant generation pipeline
 */
export class AnalysisGenerationPipeline {
  private validator: AnalysisValidator;
  private scorer: AnalysisVariantScorer;

  constructor(private logger: Logger) {
    this.validator = new AnalysisValidator(logger);
    this.scorer = new AnalysisVariantScorer(logger);
  }

  /**
   * Generate analysis variants using all available strategies
   */
  async generateVariants(
    context: AnalysisContext,
    config: AnalysisSamplingConfig = {},
  ): Promise<Result<AnalysisVariant[]>> {
    this.logger.info(
      { repoPath: context.repoPath, language: context.language },
      'Starting analysis variant generation',
    );

    try {
      // Validate context
      const contextValidation = this.validator.validateContext(context);
      if (!contextValidation.ok) {
        return contextValidation;
      }

      // Generate variants using different strategies
      const strategies = (config.strategies || ['comprehensive', 'security', 'performance']) as (
        | 'comprehensive'
        | 'security'
        | 'performance'
        | 'architecture'
        | 'deployment'
      )[];
      // Generate all variants using the functional API
      const variantsResult = await executeMultipleAnalysisStrategies(
        strategies,
        context,
        this.logger,
      );

      if (!variantsResult.ok) {
        return Failure(`Failed to generate analysis variants: ${variantsResult.error}`);
      }

      const variants: AnalysisVariant[] = [];

      // Validate each generated variant
      for (const variant of variantsResult.value) {
        const variantValidation = this.validator.validateVariant(variant);
        if (variantValidation.ok) {
          variants.push(variant);
        } else {
          this.logger.warn(
            { strategy: variant.strategy, error: variantValidation.error },
            'Skipping invalid variant',
          );
        }
      }

      if (variants.length === 0) {
        return Failure('No valid analysis variants could be generated');
      }

      this.logger.info(
        { variantCount: variants.length, strategies: variants.map((v) => v.strategy) },
        'Analysis variants generated successfully',
      );

      return Success(variants);
    } catch (error) {
      this.logger.error({ error, context }, 'Analysis variant generation failed');
      return Failure(error instanceof Error ? error.message : String(error));
    }
  }

  /**
   * Execute complete analysis sampling pipeline
   */
  async executePipeline(
    context: AnalysisContext,
    criteria: AnalysisScoringCriteria,
    config: AnalysisSamplingConfig = {},
  ): Promise<Result<AnalysisSamplingResult>> {
    const startTime = Date.now();

    try {
      this.logger.info(
        { repoPath: context.repoPath, language: context.language },
        'Starting analysis sampling pipeline',
      );

      // Step 1: Generate variants
      const variantsResult = await this.generateVariants(context, config);
      if (!variantsResult.ok) {
        return variantsResult;
      }

      const variants = variantsResult.value;

      // Step 2: Score variants
      const scoringResult = await this.scorer.scoreAnalysisVariants(variants, criteria);
      if (!scoringResult.ok) {
        return scoringResult;
      }

      const scoredVariants = scoringResult.value;

      // Step 3: Select best variant
      const bestVariant = this.scorer.selectBestAnalysisVariant(scoredVariants);
      if (!bestVariant) {
        return Failure('No suitable analysis variant found');
      }
      const executionTime = Date.now() - startTime;

      // Build sampling result
      const result: AnalysisSamplingResult = {
        bestVariant,
        variants: scoredVariants,
        metadata: {
          totalVariants: variants.length,
          executionTime,
          criteria,
          strategies: variants.map((v) => v.strategy),
          timestamp: new Date().toISOString(),
        },
      };

      this.logger.info(
        {
          bestStrategy: bestVariant.strategy,
          finalScore: bestVariant.score,
          executionTime,
          totalVariants: variants.length,
        },
        'Analysis sampling pipeline completed successfully',
      );

      return Success(result);
    } catch (error) {
      const executionTime = Date.now() - startTime;
      this.logger.error(
        { error, context, criteria, executionTime },
        'Analysis sampling pipeline failed',
      );
      return Failure(error instanceof Error ? error.message : String(error));
    }
  }
}
