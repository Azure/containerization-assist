/**
 * Host AI Assistant - Functional interface for parameter suggestion from host AI
 */

import type { Logger } from 'pino';
import { Success, Failure, type Result } from '../../types';
import { z } from 'zod';
import type { ToolContext, SamplingRequest } from '../context';
import { createParameterSuggestionPrompt } from './prompt-builder';
import { createSuggestionRegistry, type SuggestionGenerator } from './default-suggestions';

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

const DEFAULT_CONFIG: Required<Omit<AIAssistantConfig, 'customSuggestions'>> = {
  enabled: true,
  defaultConfidence: 0.8,
  maxTokens: 2048,
};

/**
 * Create AI assistant dependencies
 */
export const createAIAssistantDeps = (
  logger: Logger,
  config: AIAssistantConfig = {},
): AIAssistantDeps => {
  const mergedConfig = { ...DEFAULT_CONFIG, ...config };
  const suggestionRegistry = createSuggestionRegistry(config.customSuggestions, logger);

  return {
    logger: logger.child({ component: 'host-ai-assist' }),
    config: mergedConfig,
    suggestionRegistry,
  };
};

/**
 * Request parameter suggestions from host AI
 */
export const suggestParameters = async (
  deps: AIAssistantDeps,
  request: AIParamRequest,
  context?: ToolContext,
): Promise<Result<AIParamResponse>> => {
  if (!deps.config.enabled) {
    return Failure('AI assistance is disabled');
  }

  try {
    deps.logger.debug(
      { toolName: request.toolName, missingParams: request.missingParams },
      'Requesting AI parameter suggestions',
    );

    // Generate intelligent parameter suggestions based on context
    const suggestions = await generateParameterSuggestions(deps, request, context);

    const response: AIParamResponse = {
      suggestions,
      confidence: deps.config.defaultConfidence || 0.8,
      reasoning: 'Generated default values based on context and common patterns',
    };

    deps.logger.debug(
      { toolName: request.toolName, suggestions },
      'AI parameter suggestions generated',
    );

    return Success(response);
  } catch (error) {
    deps.logger.error({ error }, 'Failed to generate AI suggestions');
    return Failure(`AI suggestion failed: ${error}`);
  }
};

/**
 * Validate suggested parameters against schema
 */
export const validateSuggestions = (
  suggestions: Record<string, unknown>,
  schema: z.ZodType<unknown>,
): Result<Record<string, unknown>> => {
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
};

/**
 * Check if AI assistance is available
 */
export const isAIAssistanceAvailable = (deps: AIAssistantDeps): boolean => {
  return deps.config.enabled || false;
};

/**
 * Generate parameter suggestions using context and AI
 */
const generateParameterSuggestions = async (
  deps: AIAssistantDeps,
  request: AIParamRequest,
  context?: ToolContext,
): Promise<Record<string, unknown>> => {
  // Start with pattern-based defaults
  const suggestions = generateDefaultSuggestions(deps, request);

  // Build prompt for any remaining missing parameters
  const stillMissing = request.missingParams.filter((param) => !(param in suggestions));

  if (stillMissing.length > 0) {
    const prompt = buildPrompt({
      ...request,
      missingParams: stillMissing,
    });

    // Use MCP sampling if context is available
    if (context?.sampling) {
      try {
        const samplingRequest: SamplingRequest = {
          messages: [
            {
              role: 'user',
              content: [{ type: 'text', text: prompt }],
            },
          ],
          maxTokens: deps.config.maxTokens || 2048,
          includeContext: 'thisServer',
        };

        const response = await context.sampling.createMessage(samplingRequest);

        if (response.content?.[0]?.text) {
          try {
            // Parse AI response as JSON for parameter values
            const aiSuggestions = JSON.parse(response.content[0].text);
            Object.assign(suggestions, aiSuggestions);
            deps.logger.debug(
              { aiSuggestions, stillMissing },
              'AI generated parameter suggestions',
            );
          } catch (parseError) {
            deps.logger.warn(
              { parseError, response: response.content[0].text },
              'Failed to parse AI suggestions as JSON',
            );
          }
        }
      } catch (error) {
        deps.logger.warn(
          { error },
          'Failed to get AI suggestions, falling back to context extraction',
        );
      }
    }

    // Fallback: Extract values from session context
    if (request.sessionContext) {
      for (const param of stillMissing) {
        if (!(param in suggestions)) {
          const contextValue = extractFromContext(param, request.sessionContext);
          if (contextValue !== undefined) {
            suggestions[param] = contextValue;
          }
        }
      }
    }
  }

  return suggestions;
};

/**
 * Extract parameter value from session context
 */
const extractFromContext = (param: string, context: Record<string, unknown>): unknown => {
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
};

/**
 * Build prompt for AI parameter suggestion
 */
const buildPrompt = (request: AIParamRequest): string => {
  return createParameterSuggestionPrompt(request);
};

/**
 * Generate default suggestions based on context
 */
const generateDefaultSuggestions = (
  deps: AIAssistantDeps,
  request: AIParamRequest,
): Record<string, unknown> => {
  return deps.suggestionRegistry.generateAll(
    request.missingParams,
    request.currentParams,
    request.sessionContext,
  );
};

/**
 * Merge AI suggestions with user params (user params take precedence)
 */
export const mergeWithSuggestions = (
  userParams: Record<string, unknown>,
  suggestions: Record<string, unknown>,
): Record<string, unknown> => {
  // User params always override suggestions
  return {
    ...suggestions,
    ...userParams,
  };
};

/**
 * Create host AI assistant factory for backward compatibility
 */
export const createHostAIAssistant = (logger: Logger, config?: AIAssistantConfig) => {
  const deps = createAIAssistantDeps(logger, config);

  return {
    suggestParameters: (request: AIParamRequest, context?: ToolContext) =>
      suggestParameters(deps, request, context),
    validateSuggestions: (suggestions: Record<string, unknown>, schema: z.ZodType<unknown>) =>
      validateSuggestions(suggestions, schema),
    isAvailable: () => isAIAssistanceAvailable(deps),
  };
};

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
