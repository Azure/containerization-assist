/**
 * MCP Server Implementation
 * Register tools against the orchestrator executor and manage transports.
 */

import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import type { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import {
  McpError,
  ErrorCode,
  type ServerRequest,
  type ServerNotification,
} from '@modelcontextprotocol/sdk/types.js';
import type { RequestHandlerExtra } from '@modelcontextprotocol/sdk/shared/protocol.js';
import { extractErrorMessage } from '@/lib/errors';
import { createLogger, type Logger } from '@/lib/logger';
import type { MCPTool } from '@/types/tool';
import type { ExecuteRequest, ExecuteMetadata } from '@/app/orchestrator-types';
import type { Result, ErrorGuidance } from '@/types';

/**
 * Constants
 */
const RESOURCE_URI = {
  STATUS: 'containerization://status',
} as const;

const ERROR_FORMAT = {
  HINT_PREFIX: '💡',
  RESOLUTION_PREFIX: '🔧',
  DEFAULT_RESOLUTION: 'Check logs for more information',
} as const;

/**
 * Type definitions for metadata extraction
 */
interface MetaParams {
  requestId?: string;
  invocationId?: string;
  [key: string]: unknown;
}

/**
 * Type guard to check if a value is valid metadata params
 */
function isMetaParams(value: unknown): value is MetaParams {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

/**
 * Server options
 */
export interface ServerOptions {
  logger?: Logger;
  transport?: 'stdio';
  name?: string;
  version?: string;
  outputFormat?: OutputFormat;
}

/**
 * MCP Server interface
 */
export interface MCPServer {
  start(): Promise<void>;
  stop(): Promise<void>;
  getServer(): Server;
  getTools(): Array<{ name: string; description: string }>;
}

export const OUTPUTFORMAT = {
  MARKDOWN: 'markdown',
  JSON: 'json',
  TEXT: 'text',
} as const;
export type OutputFormat = (typeof OUTPUTFORMAT)[keyof typeof OUTPUTFORMAT];

export interface RegisterOptions<TTool extends MCPTool = MCPTool> {
  outputFormat: OutputFormat;
  server: McpServer;
  tools: readonly TTool[];
  logger: Logger;
  transport: string;
  execute: ToolExecutor;
}

type ToolExecutor = (request: ExecuteRequest) => Promise<Result<unknown>>;

/**
 * Format error message with guidance for better user experience
 * @param error - The error message
 * @param guidance - Optional error guidance with hints and resolution steps
 * @returns Formatted error message with guidance
 */
function formatErrorWithGuidance(error: string, guidance?: ErrorGuidance): string {
  if (!guidance) {
    return error || 'Tool execution failed';
  }

  const parts = [error];

  if (guidance.hint) {
    parts.push(`${ERROR_FORMAT.HINT_PREFIX} ${guidance.hint}`);
  }

  parts.push(
    `${ERROR_FORMAT.RESOLUTION_PREFIX} Resolution:`,
    guidance.resolution || ERROR_FORMAT.DEFAULT_RESOLUTION,
  );

  return parts.join('\n\n');
}

/**
 * Create an MCP server that delegates execution to the orchestrator
 * @param tools - Array of MCP tools to register with the server
 * @param options - Server configuration options
 * @param execute - Tool executor function that handles tool execution requests
 * @returns MCPServer interface for managing the server lifecycle
 */
export function createMCPServer<TTool extends MCPTool>(
  tools: Array<TTool>,
  options: ServerOptions = {},
  execute: ToolExecutor,
): MCPServer {
  const logger = options.logger || createLogger({ name: 'mcp-server' });
  const serverOptions = {
    name: options.name || 'containerization-assist',
    version: options.version || '1.0.0',
  };

  const server = new McpServer(serverOptions);
  const transportType = options.transport ?? 'stdio';
  const outputFormat = options.outputFormat ?? OUTPUTFORMAT.MARKDOWN;
  let transportInstance: StdioServerTransport | null = null;
  let isRunning = false;

  registerToolsWithServer({
    outputFormat,
    server,
    tools,
    logger,
    transport: transportType,
    execute,
  });

  server.resource(
    'status',
    RESOURCE_URI.STATUS,
    {
      title: 'Container Status',
      description: 'Current status of the containerization system',
    },
    async () => ({
      contents: [
        {
          uri: RESOURCE_URI.STATUS,
          mimeType: 'application/json',
          text: JSON.stringify(
            {
              running: isRunning,
              tools: tools.length,
              transport: transportType,
              timestamp: new Date().toISOString(),
            },
            null,
            2,
          ),
        },
      ],
    }),
  );

  return {
    async start(): Promise<void> {
      if (isRunning) {
        throw new Error('Server is already running');
      }

      if (transportType !== 'stdio') {
        throw new Error(`Only 'stdio' transport is supported. Requested: '${transportType}'`);
      }

      transportInstance = new StdioServerTransport();
      await server.connect(transportInstance);

      isRunning = true;
      logger.info(
        {
          transport: transportType,
          toolCount: tools.length,
        },
        'MCP server started',
      );
    },

    async stop(): Promise<void> {
      if (!isRunning) {
        return;
      }

      await server.close();
      transportInstance = null;
      isRunning = false;
      logger.info({ transport: transportType }, 'MCP server stopped');
    },

    getServer(): Server {
      return server.server;
    },

    getTools(): Array<{ name: string; description: string }> {
      return tools.map((t) => ({
        name: t.name,
        description: t.description,
      }));
    },
  };
}

/**
 * Register tools against an MCP server instance, delegating to the orchestrator executor.
 * Each tool is registered with its name, description, and input schema. Tool execution is
 * delegated to the orchestrator's execute function.
 * @param options - Registration options including server, tools, and executor
 */
export function registerToolsWithServer<TTool extends MCPTool>(
  options: RegisterOptions<TTool>,
): void {
  const { server, tools, logger, transport, execute, outputFormat } = options;

  for (const tool of tools) {
    server.tool(
      tool.name,
      tool.description,
      tool.inputSchema,
      async (rawParams: Record<string, unknown> | undefined, extra) => {
        const params = rawParams ?? {};
        logger.info({ tool: tool.name, transport }, 'Executing tool');

        try {
          const { sanitizedParams, metadata } = prepareExecutionPayload(
            tool.name,
            params,
            transport,
            extra,
          );

          const result = await execute({
            toolName: tool.name,
            params: sanitizedParams,
            metadata,
          });

          if (!result.ok) {
            // Format error with guidance if available
            const errorMessage = formatErrorWithGuidance(result.error, result.guidance);
            throw new McpError(ErrorCode.InternalError, errorMessage);
          }

          return {
            content: [
              {
                type: 'text' as const,
                text: formatOutput(result.value, outputFormat),
              },
            ],
          };
        } catch (error) {
          logger.error(
            { error: extractErrorMessage(error), tool: tool.name, transport },
            'Tool execution error',
          );
          throw error instanceof McpError
            ? error
            : new McpError(ErrorCode.InternalError, extractErrorMessage(error));
        }
      },
    );
  }
}

/**
 * Creates logger context from tool name, transport, and metadata
 * @param toolName - Name of the tool being executed
 * @param transport - Transport type (e.g., 'stdio')
 * @param meta - Optional metadata parameters
 * @returns Logger context object
 */
function createLoggerContext(
  toolName: string,
  transport: string,
  meta?: MetaParams,
): Record<string, unknown> {
  return {
    transport,
    tool: toolName,
    ...(meta?.requestId && typeof meta.requestId === 'string' && { requestId: meta.requestId }),
    ...(meta?.invocationId &&
      typeof meta.invocationId === 'string' && {
        invocationId: meta.invocationId,
      }),
  };
}

/**
 * Creates execution metadata from parameters and request context
 * @param toolName - Name of the tool being executed
 * @param params - Tool parameters
 * @param transport - Transport type
 * @param extra - Request handler extras from MCP SDK
 * @returns ExecuteMetadata object
 */
function createExecuteMetadata(
  toolName: string,
  params: Record<string, unknown>,
  transport: string,
  extra: RequestHandlerExtra<ServerRequest, ServerNotification>,
): ExecuteMetadata {
  const meta = extractMeta(params);

  return {
    progress: params,
    loggerContext: createLoggerContext(toolName, transport, meta),
    ...(extra.sendNotification && {
      sendNotification: extra.sendNotification as (notification: unknown) => Promise<void>,
    }),
  };
}

/**
 * Prepares execution payload by sanitizing params and creating metadata
 * @param toolName - Name of the tool being executed
 * @param params - Raw tool parameters
 * @param transport - Transport type
 * @param extra - Request handler extras from MCP SDK
 * @returns Object containing sanitized params and execution metadata
 */
function prepareExecutionPayload(
  toolName: string,
  params: Record<string, unknown>,
  transport: string,
  extra: RequestHandlerExtra<ServerRequest, ServerNotification>,
): {
  sanitizedParams: Record<string, unknown>;
  metadata: ExecuteMetadata;
} {
  return {
    sanitizedParams: sanitizeParams(params),
    metadata: createExecuteMetadata(toolName, params, transport, extra),
  };
}

/**
 * Extracts metadata from tool parameters
 * @param params - Raw tool parameters
 * @returns Metadata object or undefined if not present/invalid
 */
function extractMeta(params: Record<string, unknown>): MetaParams | undefined {
  const meta = params._meta;
  return isMetaParams(meta) ? meta : undefined;
}

/**
 * Removes internal metadata fields from parameters
 * @param params - Raw parameters with potential metadata
 * @returns Sanitized parameters without _meta field
 */
function sanitizeParams(params: Record<string, unknown>): Record<string, unknown> {
  const entries = Object.entries(params).filter(([key]) => key !== '_meta');
  return Object.fromEntries(entries);
}

/**
 * Format tool output based on requested format
 */
export function formatOutput(output: unknown, format: OutputFormat): string {
  switch (format) {
    case OUTPUTFORMAT.JSON:
      return JSON.stringify(output, null, 2);

    case OUTPUTFORMAT.MARKDOWN:
      // Use simple code block for object output
      if (typeof output === 'object' && output !== null) {
        return `\`\`\`json\n${JSON.stringify(output, null, 2)}\n\`\`\``;
      }
      return String(output);

    case OUTPUTFORMAT.TEXT:
    default:
      if (typeof output === 'object' && output !== null) {
        return JSON.stringify(output, null, 2);
      }
      return String(output);
  }
}
