/**
 * Consolidated AI Utilities - Simplified AI functionality with essential features preserved
 *
 * This module consolidates:
 * - Core AI generation from tool-ai-generation.ts
 * - Parameter suggestion from host-ai-assist.ts
 * - Essential sampling from tool-ai-helpers.ts
 * - Simple prompt building (replacing complex prompt-builder.ts)
 */

import type { Logger } from 'pino';
import { z } from 'zod';
import { Success, Failure, type Result } from '@types';
import type { ToolContext, SamplingRequest, SamplingResponse } from '@mcp/context';
import { extractErrorMessage, formatErrorMessage } from './error-utils';

// ============================================================================
// Core Types and Interfaces
// ============================================================================

export interface AIResponse {
  content: string;
  model?: string;
  usage?: {
    inputTokens?: number;
    outputTokens?: number;
    totalTokens?: number;
  };
}

export interface AIGenerateOptions {
  promptName: string;
  promptArgs: Record<string, unknown>;
  expectation?: 'dockerfile' | 'yaml' | 'json' | 'text';
  maxRetries?: number;
  maxTokens?: number;
}

export interface AIContext {
  sampling?: (request: SamplingRequest) => Promise<SamplingResponse>;
  getPrompt?: (name: string, args: Record<string, unknown>) => Promise<{ messages: any[] }>;
}

export interface ParameterSuggestionRequest {
  toolName: string;
  currentParams: Record<string, unknown>;
  missingParams: string[];
  schema?: Record<string, unknown>;
  sessionContext?: Record<string, unknown>;
}

export interface ParameterSuggestionResponse {
  suggestions: Record<string, unknown>;
  confidence: number;
  reasoning?: string;
}

// ============================================================================
// Default Parameter Generators (simplified from default-suggestions.ts)
// ============================================================================

const DEFAULT_SUGGESTIONS: Record<string, (params: Record<string, unknown>) => unknown> = {
  path: () => '.',
  imageId: (params) =>
    `${params.appName || params.name || 'app'}:${params.tag || params.version || 'latest'}`,
  imageName: (params) =>
    `${params.appName || params.name || 'app'}:${params.tag || params.version || 'latest'}`,
  namespace: (params) => params.namespace || 'default',
  replicas: () => 1,
  port: (params) => params.port || 8080,
  dockerfile: (params) => `${params.path || '.'}/Dockerfile`,
  contextPath: (params) => params.path || '.',
  buildArgs: () => ({}),
  environment: () => 'development',
  timeout: () => 300,
  memory: () => '512Mi',
  cpu: () => '500m',
  serviceType: () => 'ClusterIP',
  targetPort: (params) => params.port || 8080,
  protocol: () => 'TCP',
  healthCheckPath: () => '/health',
};

// ============================================================================
// Core AI Generation (from tool-ai-generation.ts)
// ============================================================================

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

function extractContent(response: SamplingResponse): string {
  return response.content
    .filter((item) => item.type === 'text')
    .map((item) => item.text)
    .join('\n');
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * Core AI generation with retry logic and validation
 */
export async function generateWithAI(
  logger: Logger,
  context: ToolContext,
  options: AIGenerateOptions,
): Promise<Result<AIResponse>> {
  const { promptName, promptArgs, expectation, maxRetries = 3, maxTokens = 4096 } = options;

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

      logger.debug({ request, attempt: attempts }, 'Sending AI sampling request');
      const response = await context.sampling.createMessage(request);
      const content = extractContent(response);
      const validationResult = validateResponse(content, expectation);

      if (!validationResult.ok) {
        lastError = validationResult.error;
        logger.warn({ error: lastError, attempt: attempts }, 'AI response validation failed');

        if (attempts < maxRetries) {
          await sleep(1000 * attempts);
          continue;
        }
        return Failure(`AI response validation failed: ${lastError}`);
      }

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
      logger.error({ error: lastError, attempt: attempts }, 'AI generation error');

      if (attempts < maxRetries) {
        await sleep(1000 * attempts);
        continue;
      }
    }
  }

  return Failure(`AI generation failed after ${attempts} attempts: ${lastError}`);
}

// ============================================================================
// Parameter Suggestion (essential from host-ai-assist.ts)
// ============================================================================

/**
 * Simple prompt building (replacing complex prompt-builder.ts)
 */
function buildParameterSuggestionPrompt(request: ParameterSuggestionRequest): string {
  const sections = [
    `Tool: ${request.toolName}`,
    `Current: ${JSON.stringify(request.currentParams, null, 2)}`,
    `Missing: ${request.missingParams.join(', ')}`,
  ];

  if (request.schema) {
    sections.push(`Schema: ${JSON.stringify(request.schema, null, 2)}`);
  }

  if (request.sessionContext) {
    sections.push(`Context: ${JSON.stringify(request.sessionContext, null, 2)}`);
  }

  sections.push('');
  sections.push('Return JSON object with suggested parameter values.');
  sections.push('Example: {"path": ".", "imageId": "app:latest"}');

  return sections.join('\n');
}

/**
 * Generate default parameter suggestions using simple patterns
 */
function generateDefaultSuggestions(
  missingParams: string[],
  currentParams: Record<string, unknown>,
  sessionContext?: Record<string, unknown>,
): Record<string, unknown> {
  const suggestions: Record<string, unknown> = {};

  for (const param of missingParams) {
    // Skip if already has value
    if (param in currentParams) continue;

    // Try default generator
    const generator = DEFAULT_SUGGESTIONS[param];
    if (generator) {
      try {
        suggestions[param] = generator(currentParams);
      } catch {
        // Ignore generator failures
      }
    }

    // Try to extract from session context
    if (!(param in suggestions) && sessionContext) {
      const contextValue = extractFromSessionContext(param, sessionContext);
      if (contextValue !== undefined) {
        suggestions[param] = contextValue;
      }
    }
  }

  return suggestions;
}

/**
 * Extract parameter value from session context
 */
function extractFromSessionContext(param: string, context: Record<string, unknown>): unknown {
  // Direct match
  if (param in context) {
    return context[param];
  }

  // Check nested results
  if (context.results && typeof context.results === 'object') {
    const results = context.results as Record<string, unknown>;
    for (const toolResult of Object.values(results)) {
      if (toolResult && typeof toolResult === 'object') {
        const resultObj = toolResult as Record<string, unknown>;
        if (param in resultObj) {
          return resultObj[param];
        }
      }
    }
  }

  return undefined;
}

/**
 * AI-powered parameter suggestion with fallback to defaults
 */
export async function suggestMissingParameters(
  logger: Logger,
  context: ToolContext,
  request: ParameterSuggestionRequest,
): Promise<Result<ParameterSuggestionResponse>> {
  try {
    logger.debug(
      { toolName: request.toolName, missingParams: request.missingParams },
      'Generating parameter suggestions',
    );

    // Start with pattern-based defaults
    const suggestions = generateDefaultSuggestions(
      request.missingParams,
      request.currentParams,
      request.sessionContext,
    );

    // Try AI for remaining missing parameters
    const stillMissing = request.missingParams.filter((param) => !(param in suggestions));

    if (stillMissing.length > 0 && context.sampling) {
      const prompt = buildParameterSuggestionPrompt({
        ...request,
        missingParams: stillMissing,
      });

      try {
        const samplingRequest: SamplingRequest = {
          messages: [
            {
              role: 'user',
              content: [{ type: 'text', text: prompt }],
            },
          ],
          maxTokens: 2048,
          includeContext: 'thisServer',
        };

        const response = await context.sampling.createMessage(samplingRequest);
        const content = extractContent(response);

        if (content) {
          try {
            const aiSuggestions = JSON.parse(content);
            Object.assign(suggestions, aiSuggestions);
            logger.debug({ aiSuggestions, stillMissing }, 'AI generated additional suggestions');
          } catch (parseError) {
            logger.warn({ parseError, content }, 'Failed to parse AI suggestions as JSON');
          }
        }
      } catch (error) {
        logger.warn({ error }, 'AI parameter suggestion failed, using defaults only');
      }
    }

    const response: ParameterSuggestionResponse = {
      suggestions,
      confidence: 0.8,
      reasoning: 'Generated using pattern-based defaults and AI assistance',
    };

    return Success(response);
  } catch (error) {
    logger.error({ error }, 'Parameter suggestion failed');
    return Failure(`Parameter suggestion failed: ${extractErrorMessage(error)}`);
  }
}

/**
 * Validate suggested parameters against schema
 */
export function validateSuggestions(
  suggestions: Record<string, unknown>,
  schema: z.ZodSchema,
): Result<Record<string, unknown>> {
  try {
    const validated = schema.parse(suggestions);
    return Success(validated as Record<string, unknown>);
  } catch (error) {
    if (error instanceof z.ZodError) {
      const issues = error.issues.map((i) => `${i.path.join('.')}: ${i.message}`).join(', ');
      return Failure(`Validation failed: ${issues}`);
    }
    return Failure(`Validation failed: ${error}`);
  }
}

/**
 * Merge user parameters with AI suggestions (user params take precedence)
 */
export function mergeWithSuggestions(
  userParams: Record<string, unknown>,
  suggestions: Record<string, unknown>,
): Record<string, unknown> {
  return {
    ...suggestions,
    ...userParams,
  };
}

// ============================================================================
// Legacy Support Functions
// ============================================================================

/**
 * Legacy wrapper for backward compatibility with withAIFallback
 */
export async function withAIFallback<T>(
  operation: () => Promise<Result<T>>,
  fallback: () => T | Promise<T>,
  options: { logger: Logger; maxRetries?: number },
): Promise<Result<T>> {
  const { logger, maxRetries = 1 } = options;

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
      if (attempts < maxRetries) {
        await sleep(1000 * attempts);
        continue;
      }
    } catch (error) {
      lastError = extractErrorMessage(error);
      if (attempts < maxRetries) {
        await sleep(1000 * attempts);
        continue;
      }
    }
  }

  // Use fallback
  try {
    const fallbackValue = await fallback();
    return Success(fallbackValue);
  } catch (fallbackError) {
    const error = fallbackError instanceof Error ? fallbackError.message : String(fallbackError);
    logger.error({ error, originalError: lastError }, 'Both operation and fallback failed');
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
