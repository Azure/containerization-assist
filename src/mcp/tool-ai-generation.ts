/**
 * AI Generation Module
 *
 * Core AI invocation with retry logic and validation.
 * Focused on single-candidate generation without sampling complexity.
 */

import type { Logger } from 'pino';
import type { ToolContext, SamplingRequest, SamplingResponse } from '@mcp/context';
import { Result, Success, Failure } from '@types';
import { extractErrorMessage, formatErrorMessage } from '@lib/error-utils';

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
      if (!trimmed.match(/^FROM\s+/im)) {
        return Failure('Invalid Dockerfile: missing FROM instruction');
      }
      return Success(trimmed);

    case 'yaml':
      if (!trimmed.match(/^[\w-]+:/m) && !trimmed.startsWith('---')) {
        return Failure('Invalid YAML: missing key-value pairs or document marker');
      }
      if (trimmed.includes('\t')) {
        return Failure('Invalid YAML: contains tabs (use spaces for indentation)');
      }
      return Success(trimmed);

    case 'json':
      try {
        JSON.parse(trimmed);
        return Success(trimmed);
      } catch (error) {
        return Failure(formatErrorMessage('Invalid JSON', error));
      }

    case 'text':
    default:
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
        includeContext: 'allServers', // Allow AI to use all available tools, not just this MCP server
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
      lastError = extractErrorMessage(error);
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

  if (fallbackBehavior === 'default') {
    logger.warn({ lastError, attempts }, 'AI generation failed, using default response');
    return Success({ content: '' });
  }

  return Failure(`AI generation failed after ${attempts} attempts: ${lastError}`);
}

/**
 * Execute an operation with AI fallback support
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
        await sleep(1000 * attempts);
        continue;
      }
    } catch (error) {
      lastError = extractErrorMessage(error);
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
 */
export function structureError(error: unknown, context?: Record<string, unknown>): string {
  const baseError = extractErrorMessage(error);

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
 */
export function aiError<T>(
  phase: 'prompt' | 'sampling' | 'validation' | 'processing',
  error: unknown,
  context?: Record<string, unknown>,
): Result<T> {
  const message = structureError(error, { ...context, phase });
  return Failure(`AI ${phase} error: ${message}`);
}
