/**
 * AI Helpers Module
 *
 * Re-exports from focused modules and contains sampling functionality.
 */

// Re-export core AI generation functionality
export {
  withAIFallback,
  structureError,
  aiError,
  aiGenerate,
  type AIResponse,
  type AIGenerateOptions,
  type AIFallbackOptions,
} from './tool-ai-generation';

// Re-export content analysis utilities
export {
  detectMultistageDocker,
  countDockerLayers,
  extractBaseImage,
  detectSecrets,
  validateYamlSyntax,
  extractK8sResources,
  type ResourceSpec,
} from './tool-content-analysis';

// Re-export scoring utilities
export { normalizeScore, weightedAverage } from '@lib/string-validators';

// Sampling functionality
import type { Logger } from 'pino';
import type { ToolContext } from '@mcp/context';
import { Result, Success, Failure } from '@types';
import { type AIResponse, type AIGenerateOptions, aiGenerate } from './tool-ai-generation';
import type { SamplingOptions, SamplingResult, SamplingCandidate } from '@lib/sampling';
import {
  scoreConfigCandidates,
  getConfigStrategies,
  quickConfigScore,
} from '@lib/integrated-scoring';
import {
  enhancePromptWithKnowledge,
  type PromptEnhancementContext,
} from '@lib/ai-knowledge-enhancer';
import { extractErrorMessage } from '@lib/error-utils';

/**
 * Generate multiple content candidates with strategy variation
 *
 * Handles multi-candidate generation with different strategies.
 * Supports early stopping when high-quality candidates are found.
 *
 * @param options - AI generation and sampling options
 * @param context - Tool context with sampling and prompt access
 * @param logger - Logger instance for debug/error output
 * @returns Promise resolving to Result with candidates array and early stop flag
 */
async function generateCandidates(
  options: AIGenerateOptions & SamplingOptions,
  context: ToolContext,
  logger: Logger,
): Promise<Result<{ candidates: string[]; stoppedEarly: boolean }>> {
  const maxCandidates = Math.min(options.maxCandidates || 3, 10);
  const earlyStopThreshold = options.earlyStopThreshold || 90;

  logger.debug({ maxCandidates, earlyStopThreshold }, 'Starting candidate generation');

  // Generate all candidates in parallel
  const candidatePromises = Array.from({ length: maxCandidates }, async (_, i) => {
    const strategy = await getSamplingStrategy(i, options.expectation, context, logger);
    const candidateOptions = {
      ...options,
      promptArgs: {
        ...options.promptArgs,
        strategy,
        variant: i + 1,
      },
    };
    return aiGenerate(logger, context, candidateOptions);
  });

  // Execute all in parallel and handle results
  const results = await Promise.allSettled(candidatePromises);

  const candidates: string[] = [];
  let stoppedEarly = false;

  for (let i = 0; i < results.length; i++) {
    const result = results[i];
    if (!result) continue;

    if (result.status === 'fulfilled' && result.value.ok) {
      candidates.push(result.value.value.content);

      // Quick score check for early stopping (only after we have at least 2 candidates)
      if (candidates.length >= 2 && options.earlyStopThreshold) {
        const quickScore = await quickScoreCandidate(
          result.value.value.content,
          options.expectation,
        );
        if (quickScore >= earlyStopThreshold) {
          logger.debug(
            { score: quickScore, threshold: earlyStopThreshold },
            'Early stop triggered',
          );
          stoppedEarly = true;
          // Don't add more candidates even if they completed
          break;
        }
      }
    } else if (result.status === 'rejected') {
      logger.warn({ attempt: i + 1, error: String(result.reason) }, 'Candidate generation failed');
    } else if (result.status === 'fulfilled' && !result.value.ok) {
      logger.warn({ attempt: i + 1, error: result.value.error }, 'Candidate generation failed');
    }
  }

  if (candidates.length === 0) {
    return Failure('No candidates generated successfully');
  }

  return Success({ candidates, stoppedEarly });
}

/**
 * Score and rank candidates using content-specific scoring algorithms
 *
 * Applies comprehensive scoring algorithms (Dockerfile, K8s, generic)
 * to candidates and ranks them by quality score.
 *
 * @param candidates - Array of content strings to score
 * @param expectation - Content type for scoring algorithm selection
 * @param logger - Logger instance for debug/error output
 * @returns Promise resolving to Result with ranked SamplingCandidate array
 */
async function scoreAndRankCandidates(
  candidates: string[],
  expectation: AIGenerateOptions['expectation'],
  logger: Logger,
): Promise<Result<SamplingCandidate[]>> {
  try {
    // Score all candidates
    const scoredCandidates = await scoreCandidates(candidates, expectation, logger);

    // Sort by score (highest first)
    scoredCandidates.sort((a, b) => b.score - a.score);

    // Add ranks
    scoredCandidates.forEach((candidate, index) => {
      candidate.rank = index + 1;
    });

    return Success(scoredCandidates);
  } catch (error) {
    return Failure(`Failed to score candidates: ${extractErrorMessage(error)}`);
  }
}

/**
 * Assemble the final sampling result with metadata and winner selection
 *
 * Takes scored candidates and assembles the final SamplingResult with
 * winner, metadata, and optional additional candidates based on options.
 *
 * @param scoredCandidates - Array of scored and ranked candidates
 * @param stoppedEarly - Whether candidate generation stopped early
 * @param samplingDuration - Duration of sampling process in milliseconds
 * @param options - Sampling options controlling result format
 * @returns Result with assembled SamplingResult or failure
 */
function assembleResult<T = AIResponse>(
  scoredCandidates: SamplingCandidate[],
  stoppedEarly: boolean,
  samplingDuration: number,
  options: SamplingOptions,
): Result<SamplingResult<T>> {
  const winner = scoredCandidates[0];
  if (!winner) {
    return Failure('No valid candidates after scoring');
  }

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
      candidatesGenerated: scoredCandidates.length,
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
 * Extract knowledge enhancement context from prompt arguments
 */
function extractKnowledgeContext(
  promptName: string,
  promptArgs: Record<string, unknown>,
): PromptEnhancementContext {
  const context: PromptEnhancementContext = {
    operation: promptName,
  };

  // Extract language information
  if (typeof promptArgs.language === 'string') {
    context.language = promptArgs.language;
  }

  // Extract framework information
  if (typeof promptArgs.framework === 'string') {
    context.framework = promptArgs.framework;
  }

  // Extract environment information
  if (typeof promptArgs.environment === 'string') {
    context.environment = promptArgs.environment;
  } else if (typeof promptArgs.target === 'string') {
    context.environment = promptArgs.target;
  }

  // Extract base image information
  if (typeof promptArgs.baseImage === 'string') {
    context.baseImage = promptArgs.baseImage;
  }

  // Extract content for analysis
  if (typeof promptArgs.dockerfileContent === 'string') {
    context.dockerfileContent = promptArgs.dockerfileContent;
  } else if (typeof promptArgs.content === 'string' && promptName.includes('dockerfile')) {
    context.dockerfileContent = promptArgs.content;
  }

  if (typeof promptArgs.manifestContent === 'string') {
    context.k8sContent = promptArgs.manifestContent;
  } else if (typeof promptArgs.content === 'string' && promptName.includes('k8s')) {
    context.k8sContent = promptArgs.content;
  }

  // Extract additional tags
  if (Array.isArray(promptArgs.tags)) {
    context.tags = promptArgs.tags.filter((tag) => typeof tag === 'string') as string[];
  }

  return context;
}

/**
 * Enhanced AI generation with multi-candidate sampling and scoring
 * Now includes caching and unified single/multi-candidate flow
 */
export async function aiGenerateWithSampling<T = AIResponse>(
  logger: Logger,
  context: ToolContext,
  options: AIGenerateOptions & SamplingOptions,
): Promise<Result<SamplingResult<T>>> {
  // Enhance prompt with knowledge base recommendations
  let enhancedPromptArgs = options.promptArgs;
  try {
    const knowledgeContext = extractKnowledgeContext(options.promptName, options.promptArgs);
    enhancedPromptArgs = await enhancePromptWithKnowledge(options.promptArgs, knowledgeContext);

    if (enhancedPromptArgs !== options.promptArgs) {
      logger.debug(
        {
          operation: knowledgeContext.operation,
          hasEnhancements: !!enhancedPromptArgs.bestPractices || !!enhancedPromptArgs.examples,
        },
        'Enhanced prompt with knowledge recommendations',
      );
    }
  } catch (error) {
    logger.debug({ error }, 'Knowledge enhancement failed, continuing without');
    // Continue with original arguments on failure
  }

  // Single candidate mode
  if (!options.enableSampling || options.maxCandidates === 1) {
    const enhancedOptions = { ...options, promptArgs: enhancedPromptArgs };
    const result = await aiGenerate(logger, context, enhancedOptions);
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

  // Multi-candidate sampling mode
  const startTime = Date.now();

  // Step 1: Generate candidates with strategy variation (with enhanced args)
  const enhancedOptions = { ...options, promptArgs: enhancedPromptArgs };
  const candidateResult = await generateCandidates(enhancedOptions, context, logger);
  if (!candidateResult.ok) return Failure(candidateResult.error);

  const { candidates, stoppedEarly } = candidateResult.value;

  // Step 2: Score and rank all candidates
  const scoringResult = await scoreAndRankCandidates(candidates, options.expectation, logger);
  if (!scoringResult.ok) return Failure(scoringResult.error);

  const scoredCandidates = scoringResult.value;
  const samplingDuration = Date.now() - startTime;

  logger.info(
    {
      candidatesGenerated: candidates.length,
      winnerScore: scoredCandidates[0]?.score,
      stoppedEarly,
      samplingDuration,
    },
    'Sampling completed',
  );

  // Step 3: Assemble final result with metadata
  return assembleResult<T>(scoredCandidates, stoppedEarly, samplingDuration, options);
}
/**
 * Score candidates using the config-based scoring system
 */
async function scoreCandidates(
  candidates: string[],
  expectation: AIGenerateOptions['expectation'],
  logger: Logger,
): Promise<SamplingCandidate[]> {
  // Use the config-based scoring system
  const environment = process.env.NODE_ENV || 'development';
  const scoreResult = await scoreConfigCandidates(candidates, expectation, environment, logger);

  if (!scoreResult.ok) {
    // Config scoring should always work since it auto-initializes
    // If it fails, it's a critical error that should be addressed
    logger.error({ error: scoreResult.error }, 'Config-based scoring failed');
    throw new Error(`Scoring system failed: ${scoreResult.error}`);
  }

  return scoreResult.value;
}

/**
 * Quick scoring for early stop decisions
 */
async function quickScoreCandidate(content: string, expectation?: string): Promise<number> {
  // Use config-based quick scoring
  const environment = process.env.NODE_ENV || 'development';
  return quickConfigScore(content, expectation as AIGenerateOptions['expectation'], environment);
}

/**
 * Get sampling strategy for candidate variation
 */
async function getSamplingStrategy(
  index: number,
  expectation: string | undefined,
  context: ToolContext,
  logger: Logger,
): Promise<string> {
  // Map expectation to content type for strategy lookup
  let contentType = 'generic';
  if (expectation === 'dockerfile') {
    contentType = 'dockerfile';
  } else if (expectation === 'yaml') {
    contentType = 'kubernetes';
  }

  // Get strategies from config
  const strategyContext = {
    ...context,
    environment: process.env.NODE_ENV || 'development',
    contentType,
  };
  const strategiesResult = await getConfigStrategies(
    contentType,
    strategyContext as Parameters<typeof getConfigStrategies>[1],
    logger,
  );

  if (strategiesResult.ok && strategiesResult.value.length > 0) {
    const strategies = strategiesResult.value;
    return strategies[index % strategies.length] || 'balanced';
  }

  // Fallback to simple strategy
  return 'balanced';
}

// Export functions for testing
export { generateCandidates, scoreAndRankCandidates, assembleResult };
