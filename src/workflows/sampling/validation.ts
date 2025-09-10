/**
 * Validation functions for Dockerfile sampling
 */

import { Success, Failure, type Result } from '@types';
import { DEFAULT_TIMEOUTS } from '@config/defaults';
import type { SamplingConfig, DockerfileVariant, ScoringCriteria } from './types';
import { DEFAULT_SCORING_CRITERIA } from './scorer';

/**
 * Validate sampling configuration
 */
export const validateSamplingConfig = (config: SamplingConfig): Result<void> => {
  if (!config.sessionId || config.sessionId.trim().length === 0) {
    return Failure('Session ID is required');
  }

  if (!config.repoPath || config.repoPath.trim().length === 0) {
    return Failure('Repository path is required');
  }

  if (config.variantCount && (config.variantCount < 1 || config.variantCount > 10)) {
    return Failure('Variant count must be between 1 and 10');
  }

  const minTimeout = 5000; // 5 seconds
  const maxTimeout = DEFAULT_TIMEOUTS.dockerBuild; // 5 minutes
  if (config.timeout && (config.timeout < minTimeout || config.timeout > maxTimeout)) {
    return Failure(
      `Timeout must be between ${minTimeout / 1000} seconds and ${maxTimeout / 1000 / 60} minutes`,
    );
  }

  return Success(undefined);
};

/**
 * Validate and normalize scoring criteria
 */
export const validateScoringCriteria = (
  criteria: Partial<ScoringCriteria>,
): Result<ScoringCriteria> => {
  const weights = {
    security: criteria.security ?? DEFAULT_SCORING_CRITERIA.security,
    performance: criteria.performance ?? DEFAULT_SCORING_CRITERIA.performance,
    size: criteria.size ?? DEFAULT_SCORING_CRITERIA.size,
    maintainability: criteria.maintainability ?? DEFAULT_SCORING_CRITERIA.maintainability,
  };

  // Check individual weights
  for (const [key, weight] of Object.entries(weights)) {
    if (weight < 0 || weight > 1) {
      return Failure(`${key} weight must be between 0 and 1`);
    }
  }

  // Check total weight
  const total = Object.values(weights).reduce((sum, weight) => sum + weight, 0);
  if (Math.abs(total - 1) > 0.01) {
    return Failure('Scoring criteria weights must sum to 1.0');
  }

  return Success(weights as ScoringCriteria);
};

/**
 * Validate a Dockerfile variant
 */
export const validateVariant = (variant: DockerfileVariant): Result<void> => {
  if (!variant.id || variant.id.trim().length === 0) {
    return Failure('Variant must have a valid ID');
  }

  // Validate content is not empty
  if (!variant.content || variant.content.trim().length === 0) {
    return Failure('Dockerfile content cannot be empty');
  }

  const lines = variant.content
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line);

  // Must have at least one FROM instruction
  if (!lines.some((line) => line.toLowerCase().startsWith('from '))) {
    return Failure('Dockerfile must contain at least one FROM instruction');
  }

  // Check for basic structure
  if (lines.length < 3) {
    return Failure(
      'Dockerfile appears too minimal - needs at least FROM, WORKDIR/COPY, and CMD/ENTRYPOINT',
    );
  }

  if (!variant.strategy || variant.strategy.trim().length === 0) {
    return Failure('Variant must specify the strategy used');
  }

  if (!variant.metadata) {
    return Failure('Variant must include metadata');
  }

  if (!variant.metadata.baseImage || variant.metadata.baseImage.trim().length === 0) {
    return Failure('Variant metadata must specify base image');
  }

  return Success(undefined);
};
