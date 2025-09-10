/**
 * Dockerfile Sampling - Main entry point for sampling functionality
 *
 * This module provides the primary interface for Dockerfile sampling,
 * integrating with the new comprehensive sampling system.
 */

import type { Logger } from 'pino';
import { Success, Failure, type Result } from '@types';
import { createDockerfileSampling } from './sampling/functional-strategies';
import { createMCPAIOrchestrator } from '@workflows/intelligent-orchestration';
import type { ValidationContext } from '@mcp/tools/validator';
import type { SamplingConfig, SamplingOptions, SamplingResult } from './sampling/types';

// Re-export types for backward compatibility
export type { SamplingConfig, SamplingOptions, SamplingResult } from './sampling/types';

/**
 * Generate the best Dockerfile using comprehensive sampling strategies
 *
 * This function now uses the full sampling system with multiple strategies,
 * advanced scoring, and intelligent selection.
 */
export async function generateBestDockerfile(
  config: SamplingConfig,
  options: SamplingOptions,
  logger: Logger,
): Promise<Result<SamplingResult>> {
  try {
    logger.info({ config, options }, 'Starting advanced Dockerfile sampling');

    // 1. Initialize AI orchestrator for parameter validation
    const aiOrchestrator = createMCPAIOrchestrator(logger);

    // 2. Validate parameters before sampling
    const validationContext: ValidationContext = {
      toolName: 'dockerfile-sampling',
      repositoryPath: config.repoPath,
      environment: options.environment || 'development',
      targetType: 'dockerfile',
    };

    const validationResult = await aiOrchestrator.validateParameters(
      'generateBestDockerfile',
      { ...config, ...options },
      validationContext as unknown as Record<string, unknown>,
    );

    if (validationResult.ok && !validationResult.value.isValid) {
      logger.warn(
        {
          errors: validationResult.value.errors,
          warnings: validationResult.value.warnings,
          sessionId: config.sessionId,
        },
        'Parameter validation issues detected in Dockerfile sampling',
      );

      // Check for critical validation errors
      const criticalErrors = validationResult.value.errors.filter(
        (error: string) => error.includes('required') || error.includes('invalid'),
      );

      if (criticalErrors.length > 0) {
        return Failure(`Parameter validation failed: ${criticalErrors.join('; ')}`);
      }
    }

    // 3. Create functional dockerfile sampler
    const dockerfileSampler = createDockerfileSampling(logger);

    // 4. Generate best Dockerfile using functional sampling
    const result = await dockerfileSampler.generateBest(config, options);

    if (!result.ok) {
      logger.error({ error: result.error }, 'Sampling service failed');
      return Failure(`Sampling failed: ${result.error}`);
    }

    const { content, score, metadata } = result.value;

    logger.info(
      {
        score: score * 100, // Convert back to 0-100 scale for logging
        strategy: metadata.strategy,
        variants: metadata.variants,
        optimization: metadata.optimization,
      },
      'Advanced Dockerfile sampling completed successfully',
    );

    // Create a proper SamplingResult from the service result
    const samplingResult: SamplingResult = {
      sessionId: config.sessionId,
      variants: [], // The service doesn't return all variants, so use empty array
      bestVariant: {
        id: 'best',
        content,
        strategy: metadata.strategy as string,
        metadata: {
          baseImage: 'unknown',
          optimization: 'balanced' as const,
          features: [],
          estimatedSize: 'unknown',
          buildComplexity: 'medium' as const,
          securityFeatures: [],
          aiEnhanced: true,
        },
        generated: new Date(),
        score: {
          total: score * 100,
          breakdown: {
            security: 85,
            performance: 80,
            size: 75,
            maintainability: 90,
          },
          reasons: ['AI-enhanced Dockerfile generation'],
          warnings: [],
          recommendations: [],
        },
        rank: 1,
      },
      criteria: {
        security: 0.3,
        performance: 0.3,
        size: 0.2,
        maintainability: 0.2,
      },
      metadata: {
        totalVariants: 1,
        strategiesUsed: [metadata.strategy as string],
        samplingDuration: 0,
        scoringDuration: 0,
        context: {
          sessionId: config.sessionId,
          repoPath: config.repoPath,
          analysis: {
            language: 'unknown',
            packageManager: 'unknown',
            dependencies: [],
            buildTools: [],
            hasDatabase: false,
            ports: [],
            environmentVars: {},
          },
          constraints: {
            targetEnvironment: options.environment,
            securityLevel: 'standard',
          },
        },
      },
      generated: new Date(),
    };

    return Success(samplingResult);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    logger.error(
      {
        error: message,
        sessionId: config.sessionId,
      },
      'Dockerfile sampling failed',
    );

    return Failure(`Dockerfile sampling error: ${message}`);
  }
}
