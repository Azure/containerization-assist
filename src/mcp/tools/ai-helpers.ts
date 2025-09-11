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

  // Check for multi-stage builds (higher weight for optimization)
  const fromCount = (content.match(/^FROM\s+/gm) || []).length;
  if (fromCount > 1) {
    score += 15;
    // Extra points for proper multi-stage naming
    if (content.match(/FROM\s+\S+\s+AS\s+/gi)) score += 5;
  }

  // Check for layer optimization with && chaining
  const runLines = content.match(/^RUN\s+.*/gm) || [];
  const chainedRuns = runLines.filter((line) => line.includes('&&')).length;
  if (chainedRuns > 0) {
    score += Math.min(10, chainedRuns * 3);
  }

  // Check for proper dependency copying pattern (cache optimization)
  const copyPackageIndex = content.indexOf('COPY package');
  const copyAllIndex = content.indexOf('COPY .');
  const runInstallIndex = content.indexOf('RUN npm install') || content.indexOf('RUN yarn');

  if (copyPackageIndex > -1 && copyAllIndex > -1) {
    if (copyPackageIndex < runInstallIndex && runInstallIndex < copyAllIndex) {
      score += 10; // Perfect pattern
    } else if (copyPackageIndex < copyAllIndex) {
      score += 5; // Partial optimization
    }
  }

  // Check for build arguments (flexibility)
  const argCount = (content.match(/^ARG\s+/gm) || []).length;
  if (argCount > 0) score += Math.min(5, argCount * 2);

  // Check for proper workdir
  if (content.match(/^WORKDIR\s+/m)) score += 10;

  // Check for .dockerignore usage hint (comments about ignored files)
  if (content.includes('# .dockerignore') || content.includes('node_modules')) score += 5;

  return Math.min(score, 100);
}

function scoreDockerfileSize(content: string): number {
  let score = 50;

  // Alpine or distroless images (major size reduction)
  if (content.match(/FROM.*alpine/i)) score += 20;
  else if (content.match(/FROM.*distroless/i)) score += 25;
  else if (content.match(/FROM.*slim/i)) score += 15;

  // Cleanup operations in same layer
  const cleanupPatterns = [
    /rm\s+-rf\s+\/var\/lib\/apt\/lists/,
    /apt-get\s+clean/,
    /yum\s+clean\s+all/,
    /apk\s+--no-cache/,
    /pip\s+install\s+--no-cache-dir/,
    /npm\s+cache\s+clean/,
  ];
  const cleanupScore = cleanupPatterns.filter((pattern) => content.match(pattern)).length;
  score += Math.min(15, cleanupScore * 5);

  // Layer consolidation
  const runCommands = (content.match(/^RUN\s+/gm) || []).length;
  if (runCommands <= 3) score += 10;
  else if (runCommands <= 5) score += 5;

  // Multi-stage build with proper copy from final stage
  if (content.includes('COPY --from=')) score += 10;

  // Specific size optimization flags
  if (content.includes('--no-install-recommends')) score += 5;

  return Math.min(score, 100);
}

function scoreDockerfileSecurity(content: string): number {
  let score = 30; // Start lower for security

  // Non-root user configuration
  const userMatch = content.match(/^USER\s+(\S+)/m);
  if (userMatch && userMatch[1] !== 'root') {
    score += 25;
    // Extra points for creating user properly
    if (content.includes('RUN useradd') || content.includes('RUN adduser')) score += 5;
  }

  // No hardcoded secrets or sensitive data
  const secretPatterns = [
    /PASSWORD\s*=\s*["'][^"']+["']/i,
    /SECRET\s*=\s*["'][^"']+["']/i,
    /API_KEY\s*=\s*["'][^"']+["']/i,
    /TOKEN\s*=\s*["'][^"']+["']/i,
  ];
  if (!secretPatterns.some((pattern) => content.match(pattern))) score += 15;

  // Proper file copying patterns (avoid copying everything early)
  const copyAll = content.match(/^COPY\s+\.\s+/m);
  const copyAllIndex = copyAll ? content.indexOf(copyAll[0]) : -1;
  const runIndex = content.indexOf('RUN ');
  if (copyAllIndex === -1 || (runIndex > -1 && copyAllIndex > runIndex)) score += 10;

  // Health checks for container monitoring
  if (content.match(/^HEALTHCHECK\s+/m)) score += 10;

  // Versioned base images (not using latest)
  const fromLines = content.match(/^FROM\s+(\S+)/gm) || [];
  const versionedImages = fromLines.filter(
    (line) => !line.includes(':latest') && line.includes(':'),
  ).length;
  if (versionedImages === fromLines.length && fromLines.length > 0) score += 10;

  // Security-focused base images
  if (content.includes('FROM scratch')) score += 5;

  // Capability dropping
  if (content.includes('--cap-drop')) score += 5;

  return Math.min(score, 100);
}

function scoreDockerfileSpeed(content: string): number {
  let score = 50;

  // Dependency caching optimization (most important for speed)
  const hasPackageJson = content.includes('package.json') || content.includes('package-lock.json');
  const hasRequirements = content.includes('requirements.txt');
  const hasGoMod = content.includes('go.mod');

  if (hasPackageJson) {
    const copyPackageIndex = Math.min(
      content.indexOf('COPY package.json') > -1 ? content.indexOf('COPY package.json') : Infinity,
      content.indexOf('COPY package*.json') > -1 ? content.indexOf('COPY package*.json') : Infinity,
    );
    const copyAllIndex = content.indexOf('COPY . ');
    if (copyPackageIndex < copyAllIndex) score += 20;
  } else if (hasRequirements) {
    const copyReqIndex = content.indexOf('COPY requirements.txt');
    const copyAllIndex = content.indexOf('COPY . ');
    if (copyReqIndex > -1 && copyReqIndex < copyAllIndex) score += 20;
  } else if (hasGoMod) {
    const copyGoModIndex = content.indexOf('COPY go.mod');
    const copyAllIndex = content.indexOf('COPY . ');
    if (copyGoModIndex > -1 && copyGoModIndex < copyAllIndex) score += 20;
  }

  // Parallel operations and build optimizations
  const parallelPatterns = [
    '--parallel',
    '-j',
    'make -j',
    'npm ci --prefer-offline',
    '--frozen-lockfile',
    '--mount=type=cache',
  ];
  const parallelScore = parallelPatterns.filter((pattern) => content.includes(pattern)).length;
  score += Math.min(15, parallelScore * 5);

  // Minimal base images for faster pulls
  if (content.match(/FROM.*alpine/i)) score += 15;
  else if (content.match(/FROM.*slim/i)) score += 10;

  // BuildKit features for better caching
  if (content.includes('# syntax=docker/dockerfile:1')) score += 5;

  // Layer count optimization
  const layers = (content.match(/^(FROM|RUN|COPY|ADD)/gm) || []).length;
  if (layers <= 10) score += 5;
  else if (layers <= 15) score += 3;

  return Math.min(score, 100);
}

/**
 * YAML/K8s-specific scoring functions
 */
function scoreYamlValidation(content: string): number {
  let score = 40;

  // Check for required Kubernetes fields
  if (content.match(/^apiVersion:\s*\S+/m)) score += 15;
  if (content.match(/^kind:\s*\S+/m)) score += 15;
  if (content.match(/^metadata:/m)) {
    score += 10;
    // Extra points for proper metadata
    if (content.match(/^\s+name:\s*\S+/m)) score += 5;
    if (content.match(/^\s+namespace:\s*\S+/m)) score += 3;
  }
  if (content.match(/^spec:/m)) score += 10;

  // Valid YAML structure (proper indentation)
  const lines = content.split('\n');
  const hasConsistentIndent = lines.every((line) => {
    // Check for tabs (invalid in YAML)
    if (line.includes('\t')) return false;
    // Check for proper spacing (multiples of 2)
    const leadingSpaces = line.match(/^(\s*)/)?.[1]?.length || 0;
    return leadingSpaces % 2 === 0;
  });
  if (hasConsistentIndent) score += 7;

  return Math.min(score, 100);
}

function scoreYamlSecurity(content: string): number {
  let score = 30;

  // Pod Security Standards
  if (content.includes('securityContext:')) {
    score += 15;

    // Specific security configurations
    if (content.match(/runAsNonRoot:\s*true/)) score += 10;
    if (content.match(/runAsUser:\s*[1-9]\d*/)) score += 5; // Non-zero UID
    if (content.match(/readOnlyRootFilesystem:\s*true/)) score += 10;
    if (content.match(/allowPrivilegeEscalation:\s*false/)) score += 8;
    if (content.match(/capabilities:\s*\n\s+drop:\s*\n\s+-\s*ALL/m)) score += 7;
  }

  // Network policies
  if (content.includes('kind: NetworkPolicy')) score += 10;

  // RBAC configurations
  if (content.includes('kind: Role') || content.includes('kind: ClusterRole')) score += 5;
  if (content.includes('serviceAccountName:')) score += 5;

  // Secret management
  if (content.includes('secretRef:') || content.includes('secretKeyRef:')) {
    score += 5;
    // Deduct if secrets are hardcoded
    if (content.match(/value:\s*["'].*password/i)) score -= 10;
  }

  // Pod Security Policy/Standards
  if (content.includes('podSecurityPolicy:') || content.includes('securityContext:')) score += 5;

  return Math.min(Math.max(score, 0), 100);
}

function scoreYamlResources(content: string): number {
  let score = 40;

  // Resource specifications
  if (content.includes('resources:')) {
    score += 15;

    // Memory and CPU limits
    if (content.match(/limits:\s*\n\s+memory:/m)) score += 10;
    if (content.match(/limits:\s*\n\s+cpu:/m)) score += 10;

    // Memory and CPU requests
    if (content.match(/requests:\s*\n\s+memory:/m)) score += 10;
    if (content.match(/requests:\s*\n\s+cpu:/m)) score += 10;
  }

  // Autoscaling configuration
  if (content.includes('kind: HorizontalPodAutoscaler')) score += 5;

  // PersistentVolumeClaim specifications
  if (content.includes('kind: PersistentVolumeClaim')) {
    score += 5;
    if (content.match(/storage:\s*\d+[GM]i/)) score += 5; // Proper storage size
  }

  // Quality of Service classes hint
  if (
    content.includes('qosClass:') ||
    (content.includes('limits:') && content.includes('requests:'))
  ) {
    score += 5;
  }

  return Math.min(score, 100);
}

function scoreYamlBestPractices(content: string): number {
  let score = 30;

  // Labels and annotations
  if (content.includes('labels:')) {
    score += 8;
    // Standard labels
    if (content.includes('app.kubernetes.io/')) score += 5;
    if (content.includes('version:') || content.includes('app.kubernetes.io/version:')) score += 3;
  }

  if (content.includes('annotations:')) score += 5;

  // Selectors for proper service discovery
  if (content.includes('selector:')) {
    score += 7;
    if (content.includes('matchLabels:')) score += 3;
  }

  // Health checks (critical for production)
  if (content.includes('livenessProbe:')) {
    score += 10;
    if (
      content.includes('httpGet:') ||
      content.includes('tcpSocket:') ||
      content.includes('exec:')
    ) {
      score += 3;
    }
  }
  if (content.includes('readinessProbe:')) {
    score += 10;
    if (content.includes('initialDelaySeconds:')) score += 2;
  }
  if (content.includes('startupProbe:')) score += 5;

  // Deployment strategies
  if (content.includes('strategy:')) {
    score += 5;
    if (content.includes('RollingUpdate')) score += 3;
  }

  // Pod disruption budgets
  if (content.includes('kind: PodDisruptionBudget')) score += 5;

  // Anti-affinity rules for HA
  if (content.includes('podAntiAffinity:')) score += 5;

  // Proper container naming
  if (content.match(/containers:\s*\n\s+-\s+name:\s*\S+/m)) score += 3;

  return Math.min(score, 100);
}

/**
 * Generic scoring functions for other content types
 */
function scoreGenericQuality(content: string): number {
  let score = 40;

  const lines = content.split('\n');
  const nonEmptyLines = lines.filter((line) => line.trim().length > 0);

  // Structure and completeness
  if (nonEmptyLines.length >= 5) score += 15;
  if (nonEmptyLines.length >= 10) score += 10;

  // Check for documentation/comments
  const hasComments = content.match(/(#|\/\/|\/\*|\*\/|<!--)/);
  if (hasComments) score += 10;

  // Check for proper formatting and structure
  const hasProperNewlines = content.includes('\n') && !content.includes('\r\n\r\n\r\n');
  if (hasProperNewlines) score += 5;

  // Check for consistent patterns (suggests well-structured content)
  const hasPatterns =
    (content.match(/^\s*[-*]\s+/gm) || []).length >= 3 || // List items
    (content.match(/^\s*\d+\.\s+/gm) || []).length >= 3 || // Numbered items
    (content.match(/^[A-Z][A-Z_]+=/gm) || []).length >= 3; // Environment variables
  if (hasPatterns) score += 10;

  // Check for proper escaping and quoting
  const hasProperQuoting = !content.includes('\\\\\\') && !content.includes('"""');
  if (hasProperQuoting) score += 5;

  // Semantic structure indicators
  if (content.includes('#!/') || content.includes('<?') || content.includes('---')) score += 5;

  return Math.min(score, 100);
}

function scoreGenericSecurity(content: string): number {
  let score = 80; // Start with good score

  // Pattern-based security checks with context
  const securityPatterns = [
    { pattern: /password\s*[:=]\s*["'][^"']+["']/gi, penalty: 25 }, // Hardcoded password
    { pattern: /api[_-]?key\s*[:=]\s*["'][^"']+["']/gi, penalty: 25 }, // Hardcoded API key
    { pattern: /secret\s*[:=]\s*["'][^"']+["']/gi, penalty: 20 }, // Hardcoded secret
    { pattern: /token\s*[:=]\s*["'][^"']+["']/gi, penalty: 20 }, // Hardcoded token
    { pattern: /private[_-]?key/gi, penalty: 15 }, // Private key reference
    { pattern: /BEGIN\s+(RSA|DSA|EC)\s+PRIVATE\s+KEY/gi, penalty: 30 }, // Actual private key
    { pattern: /aws_access_key_id/gi, penalty: 20 }, // AWS credentials
    { pattern: /mongodb:\/\/[^@]+@/gi, penalty: 20 }, // MongoDB with credentials
  ];

  for (const { pattern, penalty } of securityPatterns) {
    if (content.match(pattern)) {
      score -= penalty;
    }
  }

  // Positive security practices
  if (content.includes('${') || content.includes('$(')) score += 5; // Environment variable usage
  if (content.includes('from_secret:') || content.includes('valueFrom:')) score += 5; // Secret refs
  if (content.match(/chmod\s+[0-6]00/)) score += 5; // Restrictive permissions

  // Security misconfigurations
  if (content.includes('--insecure') || content.includes('--no-check-certificate')) score -= 10;
  if (content.includes('0.0.0.0:') || content.includes('*:')) score -= 5; // Broad binding

  return Math.min(Math.max(score, 0), 100);
}

function scoreGenericEfficiency(content: string): number {
  let score = 50;

  const lines = content.split('\n');
  const nonEmptyLines = lines.filter((line) => line.trim().length > 0);
  const totalChars = content.length;

  // Content density (not too sparse, not too dense)
  const avgCharsPerLine = totalChars / Math.max(nonEmptyLines.length, 1);
  if (avgCharsPerLine >= 20 && avgCharsPerLine <= 100) score += 20;
  else if (avgCharsPerLine >= 10 && avgCharsPerLine <= 150) score += 10;

  // No excessive repetition
  const uniqueLines = new Set(nonEmptyLines);
  const uniquenessRatio = uniqueLines.size / Math.max(nonEmptyLines.length, 1);
  if (uniquenessRatio > 0.8) score += 15;
  else if (uniquenessRatio > 0.6) score += 10;

  // Efficient patterns
  const efficientPatterns = [
    /\|\|/g, // OR operations for fallbacks
    /&&/g, // AND operations for chaining
    /\${.*:-.*}/g, // Default values
    />/g, // Redirections
    /2>&1/g, // Error handling
  ];

  const efficiencyMatches = efficientPatterns.reduce(
    (count, pattern) => count + (content.match(pattern) || []).length,
    0,
  );
  if (efficiencyMatches > 0) score += Math.min(15, efficiencyMatches * 3);

  return Math.min(score, 100);
}

function scoreGenericMaintainability(content: string): number {
  let score = 40;

  const lines = content.split('\n');

  // Readable structure with proper line breaks
  if (lines.length > 1) score += 15;

  // Documentation presence
  const docPatterns = [
    /#\s+\w+/g, // Shell/Python comments
    /\/\/\s+\w+/g, // C-style comments
    /\/\*[\s\S]*?\*\//g, // Block comments
    /<!--[\s\S]*?-->/g, // HTML/XML comments
    /@\w+/g, // Annotations/decorators
  ];

  const hasDocumentation = docPatterns.some((pattern) => content.match(pattern));
  if (hasDocumentation) score += 20;

  // Consistent indentation
  const indentTypes = new Set();
  lines.forEach((line) => {
    const leadingWhitespace = line.match(/^(\s+)/);
    if (leadingWhitespace?.[1]) {
      if (leadingWhitespace[1].includes('\t')) indentTypes.add('tab');
      if (leadingWhitespace[1].includes('  ')) indentTypes.add('space');
    }
  });
  if (indentTypes.size <= 1) score += 15; // Consistent indent type

  // Meaningful naming (variables, functions, etc.)
  const hasDescriptiveNames =
    content.match(/[a-z][a-zA-Z]{3,}_[a-z][a-zA-Z]+/g) || // snake_case
    content.match(/[a-z][a-zA-Z]{3,}[A-Z][a-zA-Z]+/g); // camelCase
  if (hasDescriptiveNames && hasDescriptiveNames.length > 0) score += 10;

  // Modular structure indicators
  if (
    content.includes('function ') ||
    content.includes('def ') ||
    content.includes('class ') ||
    content.includes('export ')
  ) {
    score += 5;
  }

  // Version indicators
  if (content.match(/v?\d+\.\d+\.\d+/) || content.includes('version:')) score += 5;

  return Math.min(score, 100);
}

/**
 * Helper utilities for content analysis
 */

/**
 * Detect if a Dockerfile uses multi-stage builds
 */
export function detectMultistageDocker(content: string): boolean {
  const fromMatches = content.match(/^FROM\s+/gm) || [];
  return fromMatches.length > 1;
}

/**
 * Count the number of layers in a Dockerfile
 */
export function countDockerLayers(content: string): number {
  const layerInstructions = [
    /^FROM\s+/gm,
    /^RUN\s+/gm,
    /^COPY\s+/gm,
    /^ADD\s+/gm,
    /^ENV\s+/gm,
    /^ARG\s+/gm,
    /^USER\s+/gm,
    /^WORKDIR\s+/gm,
  ];

  return layerInstructions.reduce((count, pattern) => {
    return count + (content.match(pattern) || []).length;
  }, 0);
}

/**
 * Extract the base image from a Dockerfile
 */
export function extractBaseImage(content: string): string | null {
  // Handle optional --platform flag: FROM --platform=linux/amd64 node:18
  const match = content.match(/^FROM\s+(?:--platform=[^\s]+\s+)?([^\s]+)/m);
  return match?.[1] ?? null;
}

/**
 * Detect potential secrets in content
 */
export function detectSecrets(content: string): string[] {
  const secrets: string[] = [];
  const secretPatterns = [
    { pattern: /password\s*[:=]\s*["'][^"']+["']/gi, type: 'password' },
    { pattern: /api[_-]?key\s*[:=]\s*["'][^"']+["']/gi, type: 'api_key' },
    { pattern: /secret\s*[:=]\s*["'][^"']+["']/gi, type: 'secret' },
    { pattern: /token\s*[:=]\s*["'][^"']+["']/gi, type: 'token' },
    { pattern: /BEGIN\s+(RSA|DSA|EC)\s+PRIVATE\s+KEY/gi, type: 'private_key' },
  ];

  for (const { pattern, type } of secretPatterns) {
    const matches = content.match(pattern);
    if (matches) {
      secrets.push(`${type}: ${matches.length} occurrence(s)`);
    }
  }

  return secrets;
}

/**
 * Validate YAML syntax (basic check)
 */
export function validateYamlSyntax(content: string): boolean {
  // Basic YAML validation
  if (content.includes('\t')) {
    return false; // YAML doesn't allow tabs
  }

  // Check for basic YAML structure
  if (!content.match(/^[\w-]+:/m) && !content.startsWith('---')) {
    return false;
  }

  // Check for consistent indentation
  const lines = content.split('\n');
  for (const line of lines) {
    if (line.trim() === '') continue;
    const indent = line.match(/^(\s*)/)?.[1]?.length || 0;
    if (indent % 2 !== 0) {
      return false; // YAML typically uses 2-space indentation
    }
  }

  return true;
}

/**
 * Extract Kubernetes resource specifications from YAML
 */
export interface ResourceSpec {
  kind?: string;
  apiVersion?: string;
  name?: string;
  namespace?: string;
  replicas?: number;
  resources?: {
    limits?: { cpu?: string; memory?: string };
    requests?: { cpu?: string; memory?: string };
  };
}

export function extractK8sResources(content: string): ResourceSpec[] {
  const resources: ResourceSpec[] = [];
  const documents = content.split(/^---$/m);

  for (const doc of documents) {
    if (!doc.trim()) continue;

    const spec: ResourceSpec = {};

    // Extract basic fields
    const kindMatch = doc.match(/^kind:\s*(.+)$/m);
    if (kindMatch?.[1]) spec.kind = kindMatch[1].trim();

    const apiVersionMatch = doc.match(/^apiVersion:\s*(.+)$/m);
    if (apiVersionMatch?.[1]) spec.apiVersion = apiVersionMatch[1].trim();

    const nameMatch = doc.match(/^\s+name:\s*(.+)$/m);
    if (nameMatch?.[1]) spec.name = nameMatch[1].trim();

    const namespaceMatch = doc.match(/^\s+namespace:\s*(.+)$/m);
    if (namespaceMatch?.[1]) spec.namespace = namespaceMatch[1].trim();

    const replicasMatch = doc.match(/^\s+replicas:\s*(\d+)$/m);
    if (replicasMatch?.[1]) spec.replicas = parseInt(replicasMatch[1], 10);

    // Extract resource specifications
    if (doc.includes('resources:')) {
      spec.resources = {};

      const cpuLimitMatch = doc.match(/limits:[\s\S]*?cpu:\s*["']?([^"'\n]+)["']?/);
      const memLimitMatch = doc.match(/limits:[\s\S]*?memory:\s*["']?([^"'\n]+)["']?/);
      const cpuRequestMatch = doc.match(/requests:[\s\S]*?cpu:\s*["']?([^"'\n]+)["']?/);
      const memRequestMatch = doc.match(/requests:[\s\S]*?memory:\s*["']?([^"'\n]+)["']?/);

      if (cpuLimitMatch || memLimitMatch) {
        spec.resources.limits = {};
        if (cpuLimitMatch?.[1]) spec.resources.limits.cpu = cpuLimitMatch[1].trim();
        if (memLimitMatch?.[1]) spec.resources.limits.memory = memLimitMatch[1].trim();
      }

      if (cpuRequestMatch || memRequestMatch) {
        spec.resources.requests = {};
        if (cpuRequestMatch?.[1]) spec.resources.requests.cpu = cpuRequestMatch[1].trim();
        if (memRequestMatch?.[1]) spec.resources.requests.memory = memRequestMatch[1].trim();
      }
    }

    if (Object.keys(spec).length > 0) {
      resources.push(spec);
    }
  }

  return resources;
}

/**
 * Normalize a score to ensure it's within valid range
 */
export function normalizeScore(score: number, max: number = 100): number {
  return Math.min(Math.max(score, 0), max);
}

/**
 * Calculate weighted average of scores
 */
export function weightedAverage(
  scores: Record<string, number>,
  weights: Record<string, number>,
): number {
  let totalWeight = 0;
  let weightedSum = 0;

  for (const [criterion, score] of Object.entries(scores)) {
    const weight = weights[criterion] || 0;
    totalWeight += weight;
    weightedSum += score * weight;
  }

  if (totalWeight === 0) {
    // If no weights, return simple average
    const values = Object.values(scores);
    return values.reduce((sum, val) => sum + val, 0) / Math.max(values.length, 1);
  }

  return weightedSum / totalWeight;
}
