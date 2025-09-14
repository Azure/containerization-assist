/**
 * Host AI Assistant - Interface for parameter suggestion from host AI
 */

import type { Logger } from 'pino';
import { Success, Failure, type Result } from '../../types';
import { z } from 'zod';
import type { ToolContext, SamplingRequest } from '../context';

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
 * AI Assistant Configuration
 */
export interface AIAssistantConfig {
  enabled?: boolean;
  defaultConfidence?: number;
  maxTokens?: number;
}

const DEFAULT_CONFIG: Required<AIAssistantConfig> = {
  enabled: true,
  defaultConfidence: 0.8,
  maxTokens: 2048,
};

/**
 * Default implementation using MCP's AI context
 */
export class DefaultHostAIAssistant implements HostAIAssistant {
  private logger: Logger;
  private config: Required<AIAssistantConfig>;

  constructor(logger: Logger, config: AIAssistantConfig = {}) {
    this.logger = logger.child({ component: 'host-ai-assist' });
    this.config = { ...DEFAULT_CONFIG, ...config };
  }

  async suggestParameters(
    request: AIParamRequest,
    context?: ToolContext,
  ): Promise<Result<AIParamResponse>> {
    if (!this.config.enabled) {
      return Failure('AI assistance is disabled');
    }

    try {
      this.logger.debug(
        { toolName: request.toolName, missingParams: request.missingParams },
        'Requesting AI parameter suggestions',
      );

      // Generate intelligent parameter suggestions based on context
      const suggestions = await this.generateParameterSuggestions(request, context);

      const response: AIParamResponse = {
        suggestions,
        confidence: this.config.defaultConfidence,
        reasoning: 'Generated default values based on context and common patterns',
      };

      this.logger.debug(
        { toolName: request.toolName, suggestions },
        'AI parameter suggestions generated',
      );

      return Success(response);
    } catch (error) {
      this.logger.error({ error }, 'Failed to generate AI suggestions');
      return Failure(`AI suggestion failed: ${error}`);
    }
  }

  validateSuggestions(
    suggestions: Record<string, unknown>,
    schema: z.ZodType<unknown>,
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

  isAvailable(): boolean {
    return this.config.enabled;
  }

  /**
   * Generate parameter suggestions using context and AI
   */
  private async generateParameterSuggestions(
    request: AIParamRequest,
    context?: ToolContext,
  ): Promise<Record<string, unknown>> {
    // Start with pattern-based defaults
    const suggestions = this.generateDefaultSuggestions(request);

    // Build prompt for any remaining missing parameters
    const stillMissing = request.missingParams.filter((param) => !(param in suggestions));

    if (stillMissing.length > 0) {
      const prompt = this.buildPrompt({
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
            maxTokens: this.config.maxTokens,
            includeContext: 'thisServer',
          };

          const response = await context.sampling.createMessage(samplingRequest);

          if (response.content?.[0]?.text) {
            try {
              // Parse AI response as JSON for parameter values
              const aiSuggestions = JSON.parse(response.content[0].text);
              Object.assign(suggestions, aiSuggestions);
              this.logger.debug(
                { aiSuggestions, stillMissing },
                'AI generated parameter suggestions',
              );
            } catch (parseError) {
              this.logger.warn(
                { parseError, response: response.content[0].text },
                'Failed to parse AI suggestions as JSON',
              );
            }
          }
        } catch (error) {
          this.logger.warn(
            { error },
            'Failed to get AI suggestions, falling back to context extraction',
          );
        }
      }

      // Fallback: Extract values from session context
      if (request.sessionContext) {
        for (const param of stillMissing) {
          if (!(param in suggestions)) {
            const contextValue = this.extractFromContext(param, request.sessionContext);
            if (contextValue !== undefined) {
              suggestions[param] = contextValue;
            }
          }
        }
      }
    }

    return suggestions;
  }

  /**
   * Extract parameter value from session context
   */
  private extractFromContext(param: string, context: Record<string, unknown>): unknown {
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
   * Build prompt for AI parameter suggestion
   */
  private buildPrompt(request: AIParamRequest): string {
    const lines = [
      `Tool: ${request.toolName}`,
      `Current parameters: ${JSON.stringify(request.currentParams, null, 2)}`,
      `Missing parameters: ${request.missingParams.join(', ')}`,
    ];

    if (request.schema) {
      lines.push(`Schema: ${JSON.stringify(request.schema, null, 2)}`);
    }

    if (request.sessionContext) {
      lines.push(`Context: ${JSON.stringify(request.sessionContext, null, 2)}`);
    }

    lines.push(
      '',
      'Please suggest values for the missing parameters based on the context and common patterns.',
      'Return ONLY a JSON object with the parameter names as keys and suggested values.',
      'Example: {"path": ".", "imageId": "myapp:latest"}',
    );

    return lines.join('\n');
  }

  /**
   * Generate default suggestions based on context
   */
  private generateDefaultSuggestions(request: AIParamRequest): Record<string, unknown> {
    const suggestions: Record<string, unknown> = {};

    for (const param of request.missingParams) {
      // Use existing params as hints
      if (param === 'path' && !request.currentParams.path) {
        suggestions.path = '.';
      } else if (param === 'imageId' && !request.currentParams.imageId) {
        // Try to derive from other params or use default
        const appName = request.currentParams.appName || 'app';
        suggestions.imageId = `${appName}:latest`;
      } else if (param === 'registry' && !request.currentParams.registry) {
        // Check if there's a registry in session context
        const registry = request.sessionContext?.registry;
        if (registry) {
          suggestions.registry = registry;
        }
      } else if (param === 'namespace' && !request.currentParams.namespace) {
        suggestions.namespace = 'default';
      } else if (param === 'replicas' && !request.currentParams.replicas) {
        suggestions.replicas = 1;
      }
      // Add more intelligent defaults based on parameter names
    }

    return suggestions;
  }
}

/**
 * Factory function to create AI assistant
 */
export function createHostAIAssistant(logger: Logger, config?: AIAssistantConfig): HostAIAssistant {
  return new DefaultHostAIAssistant(logger, config);
}

/**
 * Merge AI suggestions with user params (user params take precedence)
 */
export function mergeWithSuggestions(
  userParams: Record<string, unknown>,
  suggestions: Record<string, unknown>,
): Record<string, unknown> {
  // User params always override suggestions
  return {
    ...suggestions,
    ...userParams,
  };
}
