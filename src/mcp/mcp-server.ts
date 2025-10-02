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
import { extractErrorMessage } from '@/lib/error-utils';
import { createLogger, type Logger } from '@/lib/logger';
import { extractSchemaShape } from '@/lib/zod-utils';
import type { Tool } from '@/types/tool';
import type { ExecuteRequest, ExecuteMetadata } from '@/app/orchestrator-types';
import type { Result, ErrorGuidance } from '@/types';

/**
 * Server options
 */
export interface ServerOptions {
  logger?: Logger;
  transport?: 'stdio';
  name?: string;
  version?: string;
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

export interface RegisterOptions<TTool extends Tool = Tool> {
  server: McpServer;
  tools: readonly TTool[];
  logger: Logger;
  transport: string;
  execute: ToolExecutor;
}

type ToolExecutor = (request: ExecuteRequest) => Promise<Result<unknown>>;

/**
 * Format error message with guidance for better user experience
 */
function formatErrorWithGuidance(error: string, guidance?: ErrorGuidance): string {
  if (!guidance) {
    return error || 'Tool execution failed';
  }

  return `${error}

💡 ${guidance.hint || ''}

🔧 Resolution:
${guidance.resolution || 'Check logs for more information'}`;
}

/**
 * Format tool result into MCP content blocks.
 *
 * Strategy:
 * - If result has a `summary` field: emit separate text (summary) + text (JSON data) blocks
 * - If result is primitive (string/number/boolean): emit single text block
 * - Otherwise: emit single text block with formatted JSON (backward compatible)
 *
 * This allows tools to provide human-readable summaries alongside structured data,
 * while maintaining compatibility with clients expecting JSON responses.
 */
function formatToolResult(value: unknown): Array<{ type: 'text'; text: string }> {
  // Handle null/undefined
  if (value === null || value === undefined) {
    return [{ type: 'text', text: String(value) }];
  }

  // Handle primitives (string, number, boolean)
  if (typeof value !== 'object') {
    return [{ type: 'text', text: String(value) }];
  }

  // Handle objects with summary field - emit both summary and data blocks
  if (
    typeof value === 'object' &&
    value !== null &&
    !Array.isArray(value) &&
    'summary' in value &&
    typeof (value as Record<string, unknown>).summary === 'string'
  ) {
    const obj = value as Record<string, unknown>;
    const summary = obj.summary as string;

    // Extract data (everything except summary)
    const { summary: _, ...data } = obj;

    // Only emit two blocks if there's meaningful data beyond the summary
    if (Object.keys(data).length > 0) {
      return [
        { type: 'text', text: summary },
        { type: 'text', text: `\n📊 Data:\n${JSON.stringify(data, null, 2)}` },
      ];
    }

    // If only summary exists, just return it
    return [{ type: 'text', text: summary }];
  }

  // Default: stringify the entire result (backward compatible)
  try {
    return [{ type: 'text', text: JSON.stringify(value, null, 2) }];
  } catch {
    // Fallback for circular references or other serialization errors
    return [{ type: 'text', text: String(value) }];
  }
}

/**
 * Create an MCP server that delegates execution to the orchestrator
 */
export function createMCPServer<TTool extends Tool>(
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
  let transportInstance: StdioServerTransport | null = null;
  let isRunning = false;

  registerToolsWithServer({
    server,
    tools,
    logger,
    transport: transportType,
    execute,
  });

  server.resource(
    'status',
    'containerization://status',
    {
      title: 'Container Status',
      description: 'Current status of the containerization system',
    },
    async () => ({
      contents: [
        {
          uri: 'containerization://status',
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
 */
export function registerToolsWithServer<TTool extends Tool>(options: RegisterOptions<TTool>): void {
  const { server, tools, logger, transport, execute } = options;

  for (const tool of tools) {
    const schemaShape = extractSchemaShape(tool.schema);

    server.tool(
      tool.name,
      tool.description,
      schemaShape,
      async (rawParams: Record<string, unknown> | undefined, extra) => {
        const params = rawParams ?? {};
        logger.info({ tool: tool.name, transport }, 'Executing tool');

        try {
          const { sanitizedParams, sessionId, metadata } = prepareExecutionPayload(
            tool.name,
            params,
            transport,
            extra,
          );

          const result = await execute({
            toolName: tool.name,
            params: sanitizedParams,
            ...(sessionId && { sessionId }),
            metadata,
          });

          if (!result.ok) {
            // Format error with guidance if available
            const errorMessage = formatErrorWithGuidance(result.error, result.guidance);
            throw new McpError(ErrorCode.InternalError, errorMessage);
          }

          return {
            content: formatToolResult(result.value),
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

function prepareExecutionPayload(
  toolName: string,
  params: Record<string, unknown>,
  transport: string,
  extra: RequestHandlerExtra<ServerRequest, ServerNotification>,
): {
  sanitizedParams: Record<string, unknown>;
  sessionId?: string;
  metadata: ExecuteMetadata;
} {
  const meta = extractMeta(params);

  // Extract sessionId from extra (transport-level) or _meta (params-level)
  const sessionId = extra.sessionId || extractSessionId(meta);

  // Wrap sendNotification to accept unknown and cast to ServerNotification
  const wrappedSendNotification = extra.sendNotification
    ? async (notification: unknown) => {
        await extra.sendNotification(notification as ServerNotification);
      }
    : undefined;

  // Extract AI generation limits from _meta
  const maxTokens = extractMaxTokens(meta);
  const stopSequences = extractStopSequences(meta);

  const metadata: ExecuteMetadata = {
    progress: params,
    loggerContext: {
      transport,
      requestId: extra.requestId,
      ...(meta?.invocationId && typeof meta.invocationId === 'string'
        ? { invocationId: meta.invocationId }
        : {}),
      tool: toolName,
    },
    // Transport-provided abort signal for cancellation support
    signal: extra.signal,
    // AI generation constraints
    ...(maxTokens !== undefined && { maxTokens }),
    ...(stopSequences !== undefined && { stopSequences }),
    ...(wrappedSendNotification && { sendNotification: wrappedSendNotification }),
  };

  return {
    sanitizedParams: sanitizeParams(params),
    ...(sessionId && { sessionId }),
    metadata,
  };
}

function extractMeta(params: Record<string, unknown>): Record<string, unknown> | undefined {
  const meta = params._meta;
  if (meta && typeof meta === 'object' && !Array.isArray(meta)) {
    return meta as Record<string, unknown>;
  }
  return undefined;
}

function extractSessionId(meta: Record<string, unknown> | undefined): string | undefined {
  if (!meta) return undefined;
  const sessionId = meta.sessionId;
  return typeof sessionId === 'string' ? sessionId : undefined;
}

function extractMaxTokens(meta: Record<string, unknown> | undefined): number | undefined {
  if (!meta) return undefined;
  const maxTokens = meta.maxTokens;
  return typeof maxTokens === 'number' ? maxTokens : undefined;
}

function extractStopSequences(meta: Record<string, unknown> | undefined): string[] | undefined {
  if (!meta) return undefined;
  const stopSequences = meta.stopSequences;
  if (Array.isArray(stopSequences) && stopSequences.every((s) => typeof s === 'string')) {
    return stopSequences;
  }
  return undefined;
}

function sanitizeParams(params: Record<string, unknown>): Record<string, unknown> {
  const entries = Object.entries(params).filter(([key]) => key !== '_meta');
  return Object.fromEntries(entries);
}
