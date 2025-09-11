/**
 * AI Helpers Module
 *
 * Centralized AI invocation helpers with fallback logic, retry mechanisms,
 * and standardized response validation for the MCP tool ecosystem.
 */

import type { Logger } from 'pino';
import type { ToolContext, SamplingRequest, SamplingResponse } from '@mcp/context/types';
import { Result, Success, Failure } from '../../types';
import type {
  SamplingOptions,
  SamplingResult,
  SamplingCandidate,
  ScoringWeights,
} from '@lib/sampling';
import { config } from '../../config';

/**
 * AI response with extracted content
 */
export interface AIResponse {
  content: string;
  model?: string;
  usage?: {
    inputTokens?: number;
    outputTokens?: number;
    totalTokens?: number;
  };
}

/**
 * Options for AI generation
 */
export interface AIGenerateOptions {
  /** Required prompt name from the registry */
  promptName: string;
  /** Arguments to pass to the prompt template */
  promptArgs: Record<string, unknown>;
  /** Expected response format for validation */
  expectation?: 'dockerfile' | 'yaml' | 'json' | 'text';
  /** Fallback behavior when AI fails */
  fallbackBehavior?: 'retry' | 'default' | 'error';
  /** Maximum retry attempts */
  maxRetries?: number;
  /** Retry delay in milliseconds (base for exponential backoff) */
  retryDelay?: number;
  /** Maximum tokens to generate */
  maxTokens?: number;
  /** Stop sequences for generation */
  stopSequences?: string[];
  /** Model preference hints */
  modelHints?: string[];
}

/**
 * Options for AI fallback handling
 */
export interface AIFallbackOptions {
  /** Logger for error reporting */
  logger: Logger;
  /** Maximum retry attempts before fallback */
  maxRetries?: number;
  /** Whether to log fallback usage */
  logFallback?: boolean;
}

/**
 * Validate response based on expected format
 */
function validateResponse(
  content: string,
  expectation?: AIGenerateOptions['expectation'],
): Result<string> {
  if (!content || content.trim().length === 0) {
    return Failure('AI response is empty');
  }

  const trimmed = content.trim();

  switch (expectation) {
    case 'dockerfile':
      /**
       * Invariant: Dockerfiles must begin with FROM instruction
       * Trade-off: Simple regex over full AST parsing for performance
       */
      if (!trimmed.match(/^FROM\s+/im)) {
        return Failure('Invalid Dockerfile: missing FROM instruction');
      }
      return Success(trimmed);

    case 'yaml':
      /**
       * Trade-off: Basic structural validation vs. full YAML parsing
       * Rationale: Full parsing too expensive for initial validation
       */
      if (!trimmed.match(/^[\w-]+:/m) && !trimmed.startsWith('---')) {
        return Failure('Invalid YAML: missing key-value pairs or document marker');
      }
      /** Invariant: YAML requires spaces for indentation, never tabs */
      if (trimmed.includes('\t')) {
        return Failure('Invalid YAML: contains tabs (use spaces for indentation)');
      }
      return Success(trimmed);

    case 'json':
      try {
        JSON.parse(trimmed);
        return Success(trimmed);
      } catch (error) {
        return Failure(`Invalid JSON: ${error instanceof Error ? error.message : 'parse error'}`);
      }

    case 'text':
    default:
      /** Postcondition: Empty content already rejected above */
      return Success(trimmed);
  }
}

/**
 * Sleep helper for retry delays
 */
function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * Extract text content from sampling response
 */
function extractContent(response: SamplingResponse): string {
  return response.content
    .filter((item) => item.type === 'text')
    .map((item) => item.text)
    .join('\n');
}

/**
 * Centralized AI generation with retry logic and validation
 *
 * @param logger - Logger instance for error reporting
 * @param context - Tool context with AI sampling capabilities
 * @param options - Generation options including prompt and expectations
 * @returns Result with AI response or error message
 */
export async function aiGenerate(
  logger: Logger,
  context: ToolContext,
  options: AIGenerateOptions,
): Promise<Result<AIResponse>> {
  const {
    promptName,
    promptArgs,
    expectation,
    fallbackBehavior = 'error',
    maxRetries = 3,
    retryDelay = 1000,
    maxTokens = 4096,
    stopSequences,
    modelHints,
  } = options;

  let lastError: string = '';
  let attempts = 0;

  while (attempts < maxRetries) {
    attempts++;

    try {
      logger.debug({ promptName, promptArgs }, 'Fetching prompt from registry');
      const prompt = await context.getPrompt(promptName, promptArgs);

      if (!prompt.messages || prompt.messages.length === 0) {
        throw new Error(`Prompt '${promptName}' returned no messages`);
      }

      const request: SamplingRequest = {
        messages: prompt.messages,
        includeContext: 'thisServer',
        maxTokens,
      };

      if (stopSequences) {
        request.stopSequences = stopSequences;
      }

      if (modelHints) {
        request.modelPreferences = {
          hints: modelHints.map((name) => ({ name })),
        };
      }

      logger.debug({ request, attempt: attempts }, 'Sending AI sampling request');

      const response = await context.sampling.createMessage(request);

      const content = extractContent(response);
      const validationResult = validateResponse(content, expectation);

      if (!validationResult.ok) {
        lastError = validationResult.error;
        logger.warn(
          {
            error: lastError,
            attempt: attempts,
            expectation,
            contentLength: content.length,
          },
          'AI response validation failed',
        );

        if (fallbackBehavior === 'retry' && attempts < maxRetries) {
          /**
           * Trade-off: Exponential backoff vs. linear retry
           * Rationale: Prevents overwhelming AI service while allowing recovery
           */
          const delay = retryDelay * Math.pow(2, attempts - 1);
          logger.debug({ delay }, 'Retrying after delay');
          await sleep(delay);
          continue;
        }

        return Failure(`AI response validation failed: ${lastError}`);
      }

      logger.debug(
        {
          model: response.metadata?.model,
          usage: response.metadata?.usage,
          attempt: attempts,
        },
        'AI generation successful',
      );

      const aiResponse: AIResponse = {
        content: validationResult.value,
      };

      if (response.metadata?.model) {
        aiResponse.model = response.metadata.model;
      }

      if (response.metadata?.usage) {
        aiResponse.usage = response.metadata.usage;
      }

      return Success(aiResponse);
    } catch (error) {
      lastError = error instanceof Error ? error.message : String(error);
      logger.error(
        {
          error: lastError,
          attempt: attempts,
          promptName,
          maxRetries,
        },
        'AI generation error',
      );

      if (fallbackBehavior === 'retry' && attempts < maxRetries) {
        const delay = retryDelay * Math.pow(2, attempts - 1);
        logger.debug({ delay }, 'Retrying after error');
        await sleep(delay);
        continue;
      }
    }
  }

  /**
   * Failure Mode: All retry attempts exhausted
   * Postcondition: Empty content signals caller to use own fallback
   */
  if (fallbackBehavior === 'default') {
    logger.warn({ lastError, attempts }, 'AI generation failed, using default response');
    return Success({ content: '' });
  }

  return Failure(`AI generation failed after ${attempts} attempts: ${lastError}`);
}

/**
 * Execute an operation with AI fallback support
 *
 * @param operation - Async operation that returns a Result
 * @param fallback - Function that provides fallback value
 * @param options - Fallback options including logger
 * @returns Result with operation value or fallback
 */
export async function withAIFallback<T>(
  operation: () => Promise<Result<T>>,
  fallback: () => T | Promise<T>,
  options: AIFallbackOptions,
): Promise<Result<T>> {
  const { logger, maxRetries = 1, logFallback = true } = options;

  let lastError: string = '';
  let attempts = 0;

  while (attempts < maxRetries) {
    attempts++;

    try {
      const result = await operation();

      if (result.ok) {
        return result;
      }

      lastError = result.error;
      logger.debug({ error: lastError, attempt: attempts }, 'Operation failed');

      if (attempts < maxRetries) {
        /**
         * Trade-off: Simple linear backoff for fallback operations
         * Rationale: Less aggressive than main operation retries
         */
        await sleep(1000 * attempts);
        continue;
      }
    } catch (error) {
      lastError = error instanceof Error ? error.message : String(error);
      logger.error({ error: lastError, attempt: attempts }, 'Operation threw error');

      if (attempts < maxRetries) {
        await sleep(1000 * attempts);
        continue;
      }
    }
  }

  if (logFallback) {
    logger.info({ lastError, attempts }, 'Using fallback after operation failure');
  }

  try {
    const fallbackValue = await fallback();
    return Success(fallbackValue);
  } catch (fallbackError) {
    const error = fallbackError instanceof Error ? fallbackError.message : String(fallbackError);
    logger.error({ error, originalError: lastError }, 'Fallback also failed');
    return Failure(`Both operation and fallback failed: ${lastError} | Fallback: ${error}`);
  }
}

/**
 * Structure an error response for consistent error reporting
 *
 * @param error - Error object or string
 * @param context - Additional context for the error
 * @returns Structured error message
 */
export function structureError(error: unknown, context?: Record<string, unknown>): string {
  const baseError = error instanceof Error ? `${error.name}: ${error.message}` : String(error);

  if (!context || Object.keys(context).length === 0) {
    return baseError;
  }

  const contextStr = Object.entries(context)
    .map(([key, value]) => `${key}=${JSON.stringify(value)}`)
    .join(', ');

  return `${baseError} [${contextStr}]`;
}

/**
 * Create a structured AI error result
 *
 * @param phase - Phase where error occurred (e.g., 'prompt', 'sampling', 'validation')
 * @param error - The error that occurred
 * @param context - Additional error context
 * @returns Failure result with structured error message
 */
export function aiError<T>(
  phase: 'prompt' | 'sampling' | 'validation' | 'processing',
  error: unknown,
  context?: Record<string, unknown>,
): Result<T> {
  const message = structureError(error, { ...context, phase });
  return Failure(`AI ${phase} error: ${message}`);
}

/**
 * Enhanced AI generation with multi-candidate sampling and scoring
 */
export async function aiGenerateWithSampling<T = AIResponse>(
  logger: Logger,
  context: ToolContext,
  options: AIGenerateOptions & SamplingOptions,
): Promise<Result<SamplingResult<T>>> {
  // If sampling disabled, use standard generation
  if (!options.enableSampling || options.maxCandidates === 1) {
    const result = await aiGenerate(logger, context, options);
    if (!result.ok) return Failure(result.error);

    const winner = {
      ...result.value,
      score: 100,
      rank: 1,
    } as unknown as T & { score: number; rank?: number };

    return Success({
      winner,
      samplingMetadata: {
        candidatesGenerated: 1,
        winnerScore: 100,
        stoppedEarly: false,
      },
    });
  }

  const startTime = Date.now();
  const maxCandidates = Math.min(options.maxCandidates || 3, 10);
  const earlyStopThreshold = options.earlyStopThreshold || 90;

  logger.debug({ maxCandidates, earlyStopThreshold }, 'Starting sampling generation');

  const candidates: string[] = [];
  let stoppedEarly = false;

  // Generate candidates with strategy variation
  for (let i = 0; i < maxCandidates; i++) {
    const candidateOptions = {
      ...options,
      promptArgs: {
        ...options.promptArgs,
        strategy: getSamplingStrategy(i, options.expectation),
        variant: i + 1,
      },
    };

    const result = await aiGenerate(logger, context, candidateOptions);
    if (!result.ok) {
      logger.warn({ attempt: i + 1, error: result.error }, 'Candidate generation failed');
      continue;
    }

    candidates.push(result.value.content);

    // Quick score check for early stopping
    if (i >= 1 && options.earlyStopThreshold) {
      const quickScore = await quickScoreCandidate(result.value.content, options.expectation);
      if (quickScore >= earlyStopThreshold) {
        logger.debug({ score: quickScore, threshold: earlyStopThreshold }, 'Early stop triggered');
        stoppedEarly = true;
        break;
      }
    }
  }

  if (candidates.length === 0) {
    return Failure('No candidates generated successfully');
  }

  // Score all candidates
  const scoredCandidates = await scoreCandidates(candidates, options.expectation, logger);

  // Sort by score (highest first)
  scoredCandidates.sort((a, b) => b.score - a.score);

  // Add ranks
  scoredCandidates.forEach((candidate, index) => {
    candidate.rank = index + 1;
  });

  const winner = scoredCandidates[0];
  if (!winner) {
    return Failure('No valid candidates after scoring');
  }

  const samplingDuration = Date.now() - startTime;

  logger.info(
    {
      candidatesGenerated: candidates.length,
      winnerScore: winner.score,
      stoppedEarly,
      samplingDuration,
    },
    'Sampling completed',
  );

  const winnerData = {
    content: winner.content,
    score: winner.score,
    rank: winner.rank,
    ...(options.includeScoreBreakdown && { scoreBreakdown: winner.scoreBreakdown }),
  } as unknown as T & { score: number; scoreBreakdown?: Record<string, number>; rank?: number };

  const result: SamplingResult<T> = {
    winner: winnerData,
    samplingMetadata: {
      stoppedEarly,
      candidatesGenerated: candidates.length,
      winnerScore: winner.score,
      samplingDuration,
    },
  };

  if (options.returnAllCandidates) {
    result.allCandidates = scoredCandidates;
  }

  return Success(result);
}

/**
 * Score candidates based on content type and quality metrics
 */
async function scoreCandidates(
  candidates: string[],
  expectation: AIGenerateOptions['expectation'],
  _logger: Logger,
): Promise<SamplingCandidate[]> {
  const weights = getScoringWeights(expectation);

  return Promise.all(
    candidates.map(async (content, index) => {
      const scoreBreakdown = await calculateScoreBreakdown(content, expectation, weights);
      const overallScore = Object.entries(scoreBreakdown).reduce(
        (sum, [criterion, score]) => sum + (score * (weights[criterion] || 0)) / 100,
        0,
      );

      return {
        id: `candidate-${index + 1}`,
        content,
        score: Math.round(overallScore),
        scoreBreakdown,
      };
    }),
  );
}

/**
 * Get scoring weights from config based on content type
 */
function getScoringWeights(expectation?: string): ScoringWeights {
  if (expectation === 'dockerfile') {
    return config.sampling.weights.dockerfile;
  }
  if (expectation === 'yaml') {
    return config.sampling.weights.k8s;
  }

  // Default weights for general content
  return {
    quality: 40,
    security: 30,
    efficiency: 20,
    maintainability: 10,
  };
}

/**
 * Calculate detailed score breakdown for a candidate
 */
async function calculateScoreBreakdown(
  content: string,
  expectation?: string,
  _weights: ScoringWeights = {},
): Promise<Record<string, number>> {
  const breakdown: Record<string, number> = {};

  if (expectation === 'dockerfile') {
    breakdown.build = scoreDockerfileBuild(content);
    breakdown.size = scoreDockerfileSize(content);
    breakdown.security = scoreDockerfileSecurity(content);
    breakdown.speed = scoreDockerfileSpeed(content);
  } else if (expectation === 'yaml') {
    breakdown.validation = scoreYamlValidation(content);
    breakdown.security = scoreYamlSecurity(content);
    breakdown.resources = scoreYamlResources(content);
    breakdown.best_practices = scoreYamlBestPractices(content);
  } else {
    // Generic scoring
    breakdown.quality = scoreGenericQuality(content);
    breakdown.security = scoreGenericSecurity(content);
    breakdown.efficiency = scoreGenericEfficiency(content);
    breakdown.maintainability = scoreGenericMaintainability(content);
  }

  return breakdown;
}

/**
 * Quick scoring for early stop decisions
 */
async function quickScoreCandidate(content: string, expectation?: string): Promise<number> {
  // Fast heuristic scoring without detailed analysis
  let score = 50; // Base score

  if (expectation === 'dockerfile') {
    if (content.includes('FROM')) score += 20;
    if (content.includes('WORKDIR')) score += 10;
    if (content.includes('COPY') || content.includes('ADD')) score += 10;
    if (content.includes('USER')) score += 5; // Security
    if (content.includes('--no-cache') || content.includes('--frozen-lockfile')) score += 5;
  }

  if (expectation === 'yaml') {
    if (content.includes('apiVersion:')) score += 20;
    if (content.includes('kind:')) score += 10;
    if (content.includes('metadata:')) score += 10;
    if (content.includes('resources:')) score += 5;
  }

  return Math.min(score, 100);
}

/**
 * Get sampling strategy for candidate variation
 */
function getSamplingStrategy(index: number, expectation?: string): string {
  const strategies = {
    dockerfile: ['security', 'performance', 'size', 'balanced'],
    yaml: ['reliability', 'performance', 'security', 'balanced'],
    default: ['quality', 'efficiency', 'security', 'balanced'],
  };

  const strategySet = strategies[expectation as keyof typeof strategies] || strategies.default;
  return strategySet[index % strategySet.length] || 'balanced';
}

/**
 * Dockerfile-specific scoring functions
 */
function scoreDockerfileBuild(content: string): number {
  let score = 50;

  // Check for multi-stage builds
  if ((content.match(/FROM/g) || []).length > 1) score += 15;

  // Check for layer optimization
  if (content.includes('RUN') && content.includes('&&')) score += 10;

  // Check for proper dependency copying
  if (
    content.includes('COPY package') &&
    content.includes('RUN') &&
    content.indexOf('COPY package') < content.indexOf('COPY .')
  )
    score += 10;

  // Check for build arguments
  if (content.includes('ARG')) score += 5;

  // Check for proper workdir
  if (content.includes('WORKDIR')) score += 10;

  return Math.min(score, 100);
}

function scoreDockerfileSize(content: string): number {
  let score = 50;

  // Alpine or distroless images
  if (content.includes('alpine') || content.includes('distroless')) score += 20;

  // Cleanup operations
  if (content.includes('rm -rf') || content.includes('apt-get clean')) score += 15;

  // No-cache flags
  if (content.includes('--no-cache')) score += 10;

  // Single RUN commands (layer optimization)
  const runCommands = (content.match(/^RUN/gm) || []).length;
  if (runCommands <= 3) score += 5;

  return Math.min(score, 100);
}

function scoreDockerfileSecurity(content: string): number {
  let score = 30; // Start lower for security

  // Non-root user
  if (content.includes('USER') && !content.includes('USER root')) score += 25;

  // No secrets in layers
  if (!content.includes('PASSWORD') && !content.includes('SECRET')) score += 15;

  // Proper copying (no COPY . early)
  if (!content.match(/^COPY \. /m)) score += 10;

  // Health checks
  if (content.includes('HEALTHCHECK')) score += 10;

  // Proper base image (not latest)
  if (!content.includes(':latest')) score += 10;

  return Math.min(score, 100);
}

function scoreDockerfileSpeed(content: string): number {
  let score = 50;

  // Dependency caching
  if (
    content.includes('package.json') &&
    content.indexOf('COPY package') < content.indexOf('COPY .')
  )
    score += 20;

  // Parallel operations
  if (content.includes('--parallel') || content.includes('-j')) score += 10;

  // Minimal base images
  if (content.includes('alpine')) score += 15;

  // Layer count (fewer is better for speed)
  const layers = (content.match(/^(FROM|RUN|COPY|ADD)/gm) || []).length;
  if (layers <= 10) score += 5;

  return Math.min(score, 100);
}

/**
 * YAML/K8s-specific scoring functions
 */
function scoreYamlValidation(content: string): number {
  let score = 50;

  // Check for required fields
  if (content.includes('apiVersion:')) score += 15;
  if (content.includes('kind:')) score += 15;
  if (content.includes('metadata:')) score += 10;
  if (content.includes('spec:')) score += 10;

  return Math.min(score, 100);
}

function scoreYamlSecurity(content: string): number {
  let score = 40;

  // Security contexts
  if (content.includes('securityContext:')) score += 20;
  if (content.includes('runAsNonRoot: true')) score += 15;
  if (content.includes('readOnlyRootFilesystem: true')) score += 10;

  // Network policies
  if (content.includes('NetworkPolicy')) score += 15;

  return Math.min(score, 100);
}

function scoreYamlResources(content: string): number {
  let score = 50;

  // Resource limits
  if (content.includes('resources:')) score += 20;
  if (content.includes('limits:')) score += 15;
  if (content.includes('requests:')) score += 15;

  return Math.min(score, 100);
}

function scoreYamlBestPractices(content: string): number {
  let score = 50;

  // Labels and selectors
  if (content.includes('labels:')) score += 10;
  if (content.includes('selector:')) score += 10;

  // Probes
  if (content.includes('livenessProbe:')) score += 15;
  if (content.includes('readinessProbe:')) score += 15;

  return Math.min(score, 100);
}

/**
 * Generic scoring functions for other content types
 */
function scoreGenericQuality(content: string): number {
  let score = 50;

  // Check for structure
  const lines = content.split('\n').length;
  if (lines > 5) score += 20;
  if (lines > 10) score += 10;

  // Check for comments
  if (content.includes('#') || content.includes('//')) score += 10;

  // Check for proper formatting
  if (content.includes('\n') && !content.includes('\t\t\t')) score += 10;

  return Math.min(score, 100);
}

function scoreGenericSecurity(content: string): number {
  let score = 70;

  // Deduct for potential security issues
  if (content.includes('password') || content.includes('PASSWORD')) score -= 20;
  if (content.includes('secret') || content.includes('SECRET')) score -= 20;
  if (content.includes('token') || content.includes('TOKEN')) score -= 10;

  return Math.max(score, 0);
}

function scoreGenericEfficiency(content: string): number {
  // Simple heuristic for efficiency
  const lines = content.split('\n').length;
  const chars = content.length;

  // Prefer concise content
  const ratio = chars / Math.max(lines, 1);
  let score = 50;

  if (ratio < 100) score += 25; // Not too verbose
  if (ratio > 20) score += 25; // Not too terse

  return Math.min(score, 100);
}

function scoreGenericMaintainability(content: string): number {
  let score = 50;

  // Check for readable structure
  if (content.includes('\n')) score += 20;

  // Check for documentation
  if (content.includes('#') || content.includes('//') || content.includes('/*')) score += 20;

  // Check for consistent indentation
  const hasConsistentIndent = content
    .split('\n')
    .every((line) => !line.startsWith('\t') || !line.startsWith(' '));
  if (hasConsistentIndent) score += 10;

  return Math.min(score, 100);
}
