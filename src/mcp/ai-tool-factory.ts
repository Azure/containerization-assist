/**
 * Prompt-Backed Tool Factory
 *
 * Factory pattern for creating AI-driven tools that use the prompt engine
 * for structured message generation with role separation.
 */

import type { ToolContext } from '@/mcp/context';
import {
  type Result,
  type OutputContract,
  type BuildPromptParams,
  Success,
  Failure,
  type Topic,
} from '@/types/index';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { createLogger } from '@/lib/logger';
import type { Logger } from 'pino';
import { z } from 'zod';
import { ToolName } from '@/tools';

const logger = createLogger().child({ module: 'prompt-backed-tool' });

/**
 * Configuration for a prompt-backed tool.
 */
export interface PromptBackedToolConfig {
  /** Tool name for identification and logging */
  name: ToolName;
  /** Topic for knowledge selection */
  topic: Topic;
  /** Default environment if not specified */
  defaultEnvironment?: string;
  /** Output contract for structured responses */
  outputContract?: OutputContract;
  /** Default knowledge budget in characters */
  defaultKnowledgeBudget?: number;
  /** Maximum tokens for AI response */
  maxTokens?: number;
  /** Template function to generate base prompt */
  promptTemplate: (params: Record<string, unknown>) => string;
  /** Optional parameter validation schema */
  parameterSchema?: z.ZodSchema;
  /** Optional response parser */
  responseParser?: (response: string) => Result<unknown>;
}

/**
 * Execution options for prompt-backed tools.
 */
export interface PromptToolExecutionOptions {
  /** Override environment */
  environment?: string;
  /** Override knowledge budget */
  knowledgeBudget?: number;
  /** Additional context for prompt */
  context?: Record<string, unknown>;
  /** Force specific roles in messages */
  forceRoles?: {
    system?: boolean;
    developer?: boolean;
  };
}

/**
 * Result from prompt-backed tool execution.
 */
export interface PromptToolResult<T = unknown> {
  success: boolean;
  data?: T;
  error?: string;
  metadata?: {
    tokensUsed?: number;
    knowledgeCount?: number;
    policyCount?: number;
    environment?: string;
  };
}

/**
 * Validates parameters against schema if provided.
 */
function validateParameters(
  params: Record<string, unknown>,
  schema?: z.ZodSchema,
  logger?: Logger,
): Result<Record<string, unknown>> {
  if (!schema) {
    return Success(params);
  }

  try {
    const validated = schema.parse(params);
    return Success(validated);
  } catch (error) {
    if (error instanceof z.ZodError) {
      const errors = error.errors.map((e) => `${e.path.join('.')}: ${e.message}`).join(', ');
      logger?.warn({ errors, params }, 'Parameter validation failed');
      return Failure(`Invalid parameters: ${errors}`);
    }
    return Failure(`Parameter validation error: ${error}`);
  }
}

/**
 * Extracts and parses JSON from text, handling code blocks and common issues.
 *
 * @param text - Text potentially containing JSON
 * @returns Parsed JSON object or throws error
 */
export function extractJSON(text: string): unknown {
  // Try to find JSON in code blocks or raw
  const codeBlockMatch = text.match(/```(?:json)?\n?([\s\S]*?)\n?```/);
  const rawJsonMatch = text.match(/{[\s\S]*}/);

  const match = codeBlockMatch || rawJsonMatch;
  if (!match) {
    throw new Error('No JSON found in response');
  }

  const jsonStr = codeBlockMatch ? match[1] : match[0];
  if (!jsonStr) {
    throw new Error('No JSON content found');
  }

  try {
    return JSON.parse(jsonStr.trim());
  } catch (error) {
    // Remove trailing commas which are common in LLM responses
    const fixed = jsonStr.replace(/,(\s*[}\]])/g, '$1');
    try {
      return JSON.parse(fixed.trim());
    } catch {
      // If still fails, throw original error with context
      throw new Error(`JSON parsing failed: ${error}`);
    }
  }
}

/**
 * Parses AI response based on configuration.
 */
function parseResponse(
  response: string,
  config: PromptBackedToolConfig,
  logger?: Logger,
): Result<unknown> {
  // If custom parser provided, use it
  if (config.responseParser) {
    return config.responseParser(response);
  }

  // If output contract specifies JSON, try to parse
  if (
    config.outputContract?.name.includes('json') ||
    config.outputContract?.name.includes('JSON')
  ) {
    try {
      const parsed = extractJSON(response);
      return Success(parsed);
    } catch (error) {
      logger?.warn({ error, response }, 'Failed to parse JSON response');
      return Failure(`Failed to parse JSON response: ${error}`);
    }
  }

  // Default: return raw response
  return Success(response);
}

/**
 * Creates a prompt-backed tool executor.
 *
 * @param config - Tool configuration
 * @returns Executor function for the tool
 */
export function createPromptBackedTool(config: PromptBackedToolConfig) {
  const toolLogger = logger.child({ tool: config.name });

  return async function execute(
    params: Record<string, unknown>,
    executorLogger: Logger,
    context?: ToolContext,
    options?: PromptToolExecutionOptions,
  ): Promise<Result<PromptToolResult>> {
    const execLogger = executorLogger || toolLogger;

    try {
      // Validate parameters if schema provided
      const validationResult = validateParameters(params, config.parameterSchema, execLogger);
      if (!validationResult.ok) {
        return Failure(validationResult.error);
      }
      const validatedParams = validationResult.value;

      // Determine environment
      const environment =
        options?.environment ||
        (validatedParams.environment as string) ||
        config.defaultEnvironment ||
        'production';

      // Generate base prompt from template
      const basePrompt = config.promptTemplate(validatedParams);

      // Build prompt parameters
      const promptParams: BuildPromptParams = {
        basePrompt,
        topic: config.topic,
        tool: config.name,
        environment,
        ...(config.outputContract && { contract: config.outputContract }),
        ...(options?.knowledgeBudget !== undefined && { knowledgeBudget: options.knowledgeBudget }),
        ...(config.defaultKnowledgeBudget !== undefined && {
          knowledgeBudget: config.defaultKnowledgeBudget,
        }),
      };

      // Build messages using prompt engine
      const messageOptions = {
        ...(options?.forceRoles?.system !== undefined && {
          forceSystemRole: options.forceRoles.system,
        }),
        ...(options?.forceRoles?.developer !== undefined && {
          forceDeveloperRole: options.forceRoles.developer,
        }),
      };
      const messages = await buildMessages(
        promptParams,
        Object.keys(messageOptions).length > 0 ? messageOptions : undefined,
      );

      // Check if AI context is available
      if (!context?.sampling?.createMessage) {
        return Failure('AI context not available - sampling.createMessage is required');
      }

      execLogger.info(
        {
          tool: config.name,
          environment,
          messageCount: messages.messages.length,
        },
        'Executing prompt-backed tool',
      );

      // Convert to MCP-compatible format and call AI
      const mcpMessages = toMCPMessages(messages);
      const aiResponse = await context.sampling.createMessage({
        ...mcpMessages,
        maxTokens: config.maxTokens || 8192,
      });

      // Extract response text
      const responseText = aiResponse.content
        .filter((block: { type: string }) => block.type === 'text')
        .map((block: { text: string }) => block.text)
        .join('\n');

      if (!responseText) {
        return Failure('Empty response from AI');
      }

      // Parse response based on configuration
      const parseResult = parseResponse(responseText, config, execLogger);
      if (!parseResult.ok) {
        return Failure(parseResult.error);
      }

      // Build result
      const result: PromptToolResult = {
        success: true,
        data: parseResult.value,
        metadata: {
          environment,
          // tokensUsed removed - SamplingResponse doesn't have usage field
        },
      };

      execLogger.info(
        {
          tool: config.name,
          success: true,
          tokensUsed: result.metadata?.tokensUsed,
        },
        'Prompt-backed tool execution complete',
      );

      return Success(result);
    } catch (error) {
      execLogger.error({ error, tool: config.name }, 'Prompt-backed tool execution failed');
      return Failure(`Tool execution failed: ${error}`);
    }
  };
}

/**
 * Creates a prompt-backed tool with automatic MCP tool registration format.
 *
 * @param config - Tool configuration
 * @returns MCP-compatible tool definition
 */
export function createMcpPromptTool(
  config: PromptBackedToolConfig & {
    description: string;
    inputSchema: Record<string, unknown>;
  },
): {
  name: string;
  description: string;
  schema: Record<string, unknown>;
  execute: (
    params: Record<string, unknown>,
    logger: Logger,
    context?: ToolContext,
  ) => Promise<Result<unknown>>;
} {
  const executor = createPromptBackedTool(config);

  return {
    name: config.name,
    description: config.description,
    schema: config.inputSchema,
    execute: async (
      params: Record<string, unknown>,
      logger: Logger,
      context?: ToolContext,
    ): Promise<Result<unknown>> => {
      const result = await executor(params, logger, context);
      if (result.ok) {
        return Success(result.value.data);
      }
      return Failure(result.error);
    },
  };
}

/**
 * Helper to create output contracts for common response types.
 */
export const OutputContracts = {
  /** JSON object response */
  json: (name: string, description?: string): OutputContract => ({
    name: `${name}_json`,
    description: description || `Respond with valid JSON for ${name}`,
  }),

  /** Dockerfile response */
  dockerfile: (): OutputContract => ({
    name: 'dockerfile_v1',
    description: 'Respond with a valid Dockerfile and optional explanation',
  }),

  /** Kubernetes manifests response */
  kubernetes: (): OutputContract => ({
    name: 'kubernetes_manifests_v1',
    description: 'Respond with valid Kubernetes YAML manifests',
  }),

  /** Helm chart response */
  helm: (): OutputContract => ({
    name: 'helm_chart_v1',
    description: 'Respond with Helm chart files in YAML format',
  }),

  /** Analysis response */
  analysis: (): OutputContract => ({
    name: 'analysis_v1',
    description: 'Respond with structured analysis including findings and recommendations',
  }),
};

/**
 * Factory for creating a suite of related prompt-backed tools.
 */
export class PromptToolFactory {
  private tools: Map<string, ReturnType<typeof createPromptBackedTool>> = new Map();
  private defaultConfig: Partial<PromptBackedToolConfig>;

  constructor(defaultConfig?: Partial<PromptBackedToolConfig>) {
    this.defaultConfig = defaultConfig || {};
  }

  /**
   * Register a new prompt-backed tool.
   */
  register(config: PromptBackedToolConfig): void {
    const fullConfig = { ...this.defaultConfig, ...config };
    const tool = createPromptBackedTool(fullConfig);
    this.tools.set(config.name, tool);

    logger.info({ tool: config.name }, 'Registered prompt-backed tool');
  }

  /**
   * Get a registered tool executor.
   */
  getTool(name: string): ReturnType<typeof createPromptBackedTool> | undefined {
    return this.tools.get(name);
  }

  /**
   * Get all registered tool names.
   */
  getToolNames(): string[] {
    return Array.from(this.tools.keys());
  }

  /**
   * Execute a tool by name.
   */
  async execute(
    name: string,
    params: Record<string, unknown>,
    logger: Logger,
    context?: ToolContext,
    options?: PromptToolExecutionOptions,
  ): Promise<Result<PromptToolResult>> {
    const tool = this.tools.get(name);
    if (!tool) {
      return Failure(`Tool not found: ${name}`);
    }

    return tool(params, logger, context, options);
  }
}
