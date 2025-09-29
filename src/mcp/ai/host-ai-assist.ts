/**
 * Host AI Assistant - Functional interface for parameter suggestion from host AI
 */

import type { Logger } from 'pino';
import { type Result, Success, Failure } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { z, type ZodType } from 'zod';
import { createSuggestionRegistry, type SuggestionGenerator } from './default-suggestions';
import { extractJSON } from '@/mcp/ai-tool-factory';
import { SCORING_CONFIG } from '@/config/scoring';

/**
 * AI parameter suggestion request
 */
export interface AIParamRequest {
  /** Tool name requesting assistance */
  toolName: string;
  /** Current partial parameters */
  currentParams: Record<string, unknown>;
  /** Required parameter names */
  requiredParams: string[];
  /** Missing parameter names */
  missingParams: string[];
  /** Tool schema for context */
  schema?: Record<string, unknown>;
  /** Session context for better suggestions */
  sessionContext?: Record<string, unknown>;
}

/**
 * AI parameter suggestion response
 */
export interface AIParamResponse {
  /** Suggested parameter values */
  suggestions: Record<string, unknown>;
  /** Confidence score (0-1) */
  confidence: number;
  /** Explanation for suggestions */
  reasoning?: string;
}

/**
 * AI Assistant Configuration
 */
export interface AIAssistantConfig {
  enabled?: boolean;
  defaultConfidence?: number;
  maxTokens?: number;
  customSuggestions?: Record<string, SuggestionGenerator>;
}

/**
 * Dependencies for AI assistant functions
 */
export interface AIAssistantDeps {
  logger: Logger;
  config: AIAssistantConfig;
  suggestionRegistry: ReturnType<typeof createSuggestionRegistry>;
}

/**
 * Host AI assistant interface
 */
export interface HostAIAssistant {
  /**
   * Request parameter suggestions from host AI
   */
  suggestParameters(
    request: AIParamRequest,
    context?: ToolContext,
  ): Promise<Result<AIParamResponse>>;

  /**
   * Validate suggested parameters against schema
   */
  validateSuggestions(
    suggestions: Record<string, unknown>,
    schema: z.ZodType<unknown>,
  ): Result<Record<string, unknown>>;

  /**
   * Check if AI assistance is available
   */
  isAvailable(): boolean;
}

/**
 * Simple function to suggest missing parameters using host AI
 */
export async function suggestMissingParams(
  toolName: string,
  missing: string[],
  hostCall: (prompt: string) => Promise<string>,
  context?: Record<string, unknown>,
): Promise<Result<Record<string, unknown>>> {
  if (missing.length === 0) {
    return Success({});
  }

  const prompt = `
    For the ${toolName} tool, provide values for these missing parameters:
    ${missing.join(', ')}

    ${context ? `Context: ${JSON.stringify(context, null, 2)}` : ''}

    Return a JSON object with the parameter names as keys and suggested values.
  `;

  try {
    const response = await hostCall(prompt);
    const suggestions = extractJSON(response) as Record<string, unknown>;
    return Success(suggestions);
  } catch (error) {
    return Failure(`Failed to get parameter suggestions: ${error}`);
  }
}

/**
 * Validate parameters against a Zod schema
 */
export function validateWithSchema(
  params: Record<string, unknown>,
  schema: ZodType<unknown>,
): Result<Record<string, unknown>> {
  const parseResult = schema.safeParse(params);

  if (parseResult.success) {
    return Success(parseResult.data as Record<string, unknown>);
  }

  const errors = parseResult.error.errors
    .map((e) => `${e.path.join('.')}: ${e.message}`)
    .join(', ');

  return Failure(`Validation failed: ${errors}`);
}

/**
 * Create a simple host AI assistant
 */
export function createHostAIAssistant(
  hostCall: (prompt: string) => Promise<string>,
  config: AIAssistantConfig = {},
  logger?: Logger,
): HostAIAssistant {
  const enabled = config.enabled ?? true;
  const defaultConfidence = config.defaultConfidence ?? SCORING_CONFIG.CONFIDENCE.DEFAULT;

  return {
    async suggestParameters(
      request: AIParamRequest,
      _context?: ToolContext,
    ): Promise<Result<AIParamResponse>> {
      if (!enabled) {
        return Failure('AI assistance is disabled');
      }

      logger?.info(
        { toolName: request.toolName, missing: request.missingParams },
        'Requesting parameter suggestions',
      );

      const result = await suggestMissingParams(request.toolName, request.missingParams, hostCall, {
        ...request.currentParams,
        ...request.sessionContext,
      });

      if (result.ok) {
        return Success({
          suggestions: result.value,
          confidence: defaultConfidence,
          reasoning: `Suggested values for ${request.missingParams.join(', ')}`,
        });
      }

      return Failure(result.error || 'Failed to get suggestions');
    },

    validateSuggestions(
      suggestions: Record<string, unknown>,
      schema: ZodType<unknown>,
    ): Result<Record<string, unknown>> {
      return validateWithSchema(suggestions, schema);
    },

    isAvailable(): boolean {
      return enabled;
    },
  };
}

/**
 * Build a prompt for parameter suggestion
 */
export function buildParameterPrompt(
  toolName: string,
  params: string[],
  context?: Record<string, unknown>,
): string {
  const contextStr = context ? `\n\nContext:\n${JSON.stringify(context, null, 2)}` : '';

  return `Please provide values for the following parameters for the ${toolName} tool:

Parameters needed:
${params.map((p) => `- ${p}`).join('\n')}
${contextStr}

Return your response as a JSON object with the parameter names as keys.`;
}

/**
 * Extract parameter descriptions from Zod schema
 */
export function extractSchemaInfo(schema: ZodType<unknown>): Record<string, string> {
  const descriptions: Record<string, string> = {};

  if (schema instanceof z.ZodObject) {
    const shape = schema.shape;
    for (const [key, value] of Object.entries(shape)) {
      if (value instanceof z.ZodType && value.description) {
        descriptions[key] = value.description;
      }
    }
  }

  return descriptions;
}
