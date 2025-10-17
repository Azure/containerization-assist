/**
 * MCP Context - Tool execution environment abstraction
 *
 * Invariant: All tools receive consistent context interface
 * Trade-off: Abstraction overhead for tool isolation and testability
 * Design: Factory pattern enables context mocking in tests
 */

import type { Server } from '@modelcontextprotocol/sdk/server/index.js';
import type { Logger } from 'pino';
import { extractErrorMessage } from '@/lib/error-utils';
import { extractProgressReporter } from './context-helpers.js';

// ===== TYPES =====

/**
 * MCP-compatible text message structure
 * Based on actual MCP protocol format with content arrays
 */
export interface TextMessage {
  /** Message role in the conversation (system not supported by MCP) */
  role: 'user' | 'assistant';
  /** Content array with text objects (MCP format) */
  content: Array<{ type: 'text'; text: string }>;
  /** Allow additional properties for MCP compatibility */
  [key: string]: unknown;
}

/**
 * Sampling request following MCP client/sampling specification
 * Used to request AI responses from the MCP host
 */
export interface SamplingRequest {
  /** Messages array for the conversation context */
  messages: TextMessage[];
  /** Context inclusion strategy for the request */
  includeContext?: 'thisServer' | 'allServers' | 'none';
  /** Model preferences for the request */
  modelPreferences?: {
    /** Hints about the type of response needed */
    hints?: Array<{ name: string }>;
    /** Cost optimization priority (0-1) */
    costPriority?: number;
    /** Speed optimization priority (0-1) */
    speedPriority?: number;
    /** Intelligence/quality priority (0-1) */
    intelligencePriority?: number;
  };
  /** Stop sequences to end generation */
  stopSequences?: string[];
  /** Maximum tokens to generate */
  maxTokens?: number;
}

/**
 * Sampling response from MCP client/sampling
 * Based on actual MCP protocol response format
 */
export interface SamplingResponse {
  /** Response role (always 'assistant' for AI responses) */
  role: 'assistant';
  /** Response content array */
  content: Array<{ type: 'text'; text: string }>;
  /** Additional metadata about the response */
  metadata?: {
    /** Model used for generation */
    model?: string;
    /** Token usage statistics */
    usage?: {
      /** Input tokens consumed */
      inputTokens?: number;
      /** Output tokens generated */
      outputTokens?: number;
      /** Total tokens used */
      totalTokens?: number;
    };
    /** Generation finish reason */
    finishReason?: 'stop' | 'length' | 'content_filter' | 'tool_calls';
  };
}

/**
 * Prompt with metadata structure
 * Returned by server/prompts handlers
 */
export interface PromptWithMessages {
  /** Human-readable description of the prompt */
  description: string;
  /** Message array ready for sampling */
  messages: TextMessage[];
}

/**
 * Progress reporting function
 * Forwards progress updates through MCP notifications
 */
export type ProgressReporter = (
  /** Progress message or step name */
  message: string,
  /** Current progress value */
  progress?: number,
  /** Total progress value */
  total?: number,
) => Promise<void>;

/**
 * Main context object passed to tools - Unified interface for all tool implementations
 */
export interface ToolContext {
  /**
   * AI sampling capabilities for generating responses using the MCP host's AI models
   */
  sampling: {
    /**
     * Create a message using the MCP host's AI capabilities
     * Replaces direct AI service usage with proper MCP protocol
     */
    createMessage(request: SamplingRequest): Promise<SamplingResponse>;
  };

  /**
   * Get a prompt with arguments from the prompt registry
   * Uses proper MCP server/prompts protocol
   */
  getPrompt(name: string, args?: Record<string, unknown>): Promise<PromptWithMessages>;

  /**
   * Optional abort signal for cancellation support
   * Tools should check this signal periodically for long-running operations
   */
  signal: AbortSignal | undefined;

  /**
   * Optional progress reporting function for user feedback
   * Should be called at regular intervals during long operations
   */
  progress: ProgressReporter | undefined;

  /**
   * Logger for debugging and error tracking - Required for all tools
   * Use this for structured logging instead of console.log
   */
  logger: Logger;
}

// ===== PROGRESS HANDLING =====

// Re-export types and utilities from helpers
export type { EnhancedProgressReporter } from './context-helpers.js';
export { extractProgressToken, createProgressReporter } from './context-helpers.js';

// ===== CONTEXT CREATION =====

/**
 * Options for creating a tool context
 */
export interface ContextOptions {
  /** Optional abort signal for cancellation */
  signal?: AbortSignal;
  /** Optional progress reporter or request with progress token */
  progress?: ProgressReporter | unknown;
  /** Maximum tokens for sampling (default: 2048) */
  maxTokens?: number;
  /** Stop sequences for sampling */
  stopSequences?: string[];
  /** MCP notification callback for progress updates */
  sendNotification?: (notification: unknown) => Promise<void>;
}

/**
 * Create a ToolContext with MCP capabilities
 *
 * @param server - MCP server instance for sampling
 * @param logger - Logger for debugging and error tracking
 * @param options - Optional configuration
 * @returns Configured ToolContext
 *
 * @example
 * ```typescript
 * const context = createToolContext(server, logger, {
 *   progress: request, // Will auto-extract progress token
 * });
 * ```
 */
export function createToolContext(
  server: Server,
  logger: Logger,
  options: ContextOptions = {},
): ToolContext {
  const progressReporter = extractProgressReporter(
    options.progress,
    server,
    logger,
    options.sendNotification,
  );

  return {
    sampling: {
      createMessage: async (req: SamplingRequest) => {
        return createSamplingResponse(server, req, logger, {
          ...(options.maxTokens !== undefined && { maxTokens: options.maxTokens }),
          ...(options.stopSequences !== undefined && { stopSequences: options.stopSequences }),
        });
      },
    },
    getPrompt: (name: string, args?: Record<string, unknown>) =>
      getPromptWithFallback(undefined, name, args),
    logger,
    signal: options.signal,
    progress: progressReporter,
  };
}

// Helper function for sampling requests
async function createSamplingResponse(
  server: Server,
  samplingRequest: SamplingRequest,
  logger: Logger,
  options: { maxTokens?: number; stopSequences?: string[] } = {},
): Promise<SamplingResponse> {
  const startTime = Date.now();

  try {
    logger.debug(
      {
        messageCount: samplingRequest.messages.length,
        maxTokens: samplingRequest.maxTokens || 2048,
        includeContext: samplingRequest.includeContext || 'thisServer',
      },
      'Making sampling request',
    );

    // Convert internal message format to SDK format
    const sdkMessages = samplingRequest.messages.map((msg) => ({
      role: msg.role,
      content: {
        type: 'text' as const,
        text: msg.content.map((c) => c.text).join('\n'),
      },
    }));

    // Make the MCP request with defaults
    const requestWithDefaults = {
      maxTokens: samplingRequest.maxTokens || options.maxTokens || 2048,
      stopSequences: samplingRequest.stopSequences || ['```', '\n\n```', '\n\n# ', '\n\n---'],
      includeContext: samplingRequest.includeContext || 'thisServer',
      messages: sdkMessages,
      modelPreferences: samplingRequest.modelPreferences,
    };

    logger.info(
      { method: 'sampling/createMessage', hasServer: !!server },
      'About to call createMessage',
    );

    const response = await server.createMessage(requestWithDefaults);

    logger.info(
      {
        hasResponse: !!response,
        hasContent: !!response?.content,
        contentType: response?.content?.type,
        textLength: (response?.content as { text?: string })?.text?.length || 0,
      },
      'Received sampling response',
    );

    // Validate response
    if (!response?.content || response.content.type !== 'text') {
      throw new Error('Empty or invalid response from sampling - no text content found');
    }

    const text = response.content.text.trim();
    if (!text) {
      throw new Error('Empty response from sampling after processing');
    }

    const duration = Date.now() - startTime;
    logger.info(
      {
        duration,
        responseLength: text.length,
        preview: text.substring(0, 200),
      },
      'Sampling request completed',
    );

    // Return standardized response
    const metadata: {
      model?: string;
      usage?: { inputTokens?: number; outputTokens?: number; totalTokens?: number };
      finishReason?: 'stop' | 'length' | 'content_filter' | 'tool_calls';
    } = {};

    // Add model if present
    const responseModel = (response as { model?: string })?.model;
    if (responseModel) {
      metadata.model = responseModel;
    }

    // Add usage if present and valid
    const responseUsage = (response as { usage?: unknown })?.usage;
    if (responseUsage && typeof responseUsage === 'object') {
      metadata.usage = responseUsage as {
        inputTokens?: number;
        outputTokens?: number;
        totalTokens?: number;
      };
    }

    // Add finish reason
    metadata.finishReason = ((response as { finishReason?: string })?.finishReason || 'stop') as
      | 'stop'
      | 'length'
      | 'content_filter'
      | 'tool_calls';

    return {
      role: 'assistant',
      content: [{ type: 'text', text }],
      metadata,
    };
  } catch (error) {
    const duration = Date.now() - startTime;
    logger.error(
      {
        duration,
        error: extractErrorMessage(error),
        maxTokens: samplingRequest.maxTokens,
        messageCount: samplingRequest.messages.length,
      },
      'Sampling request failed',
    );

    if (error instanceof Error) {
      error.message = `Sampling failed: ${error.message}`;
    }
    throw error;
  }
}

// Helper function for prompts with fallback
async function getPromptWithFallback(
  _promptReg: undefined,
  name: string,
  _args?: Record<string, unknown>,
): Promise<PromptWithMessages> {
  return {
    description: 'Prompt not available - no registry',
    messages: [
      {
        role: 'user' as const,
        content: [
          {
            type: 'text' as const,
            text: `Error: No prompt registry available for prompt '${name}'`,
          },
        ],
      },
    ],
  };
}
