/**
 * Host AI Assistant - Functional interface for parameter suggestion from host AI
 */

import type { Logger } from 'pino';
import type { Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { z } from 'zod';
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
