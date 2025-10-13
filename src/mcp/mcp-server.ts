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
import type { MCPTool } from '@/types/tool';
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
 * Create an MCP server that delegates execution to the orchestrator
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
export function registerToolsWithServer<TTool extends MCPTool>(
  options: RegisterOptions<TTool>,
): void {
  const { server, tools, logger, transport, execute, outputFormat } = options;

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
  const sessionId = extractSessionId(meta);

  // Wrap sendNotification to accept unknown and cast to ServerNotification
  const wrappedSendNotification = extra.sendNotification
    ? async (notification: unknown) => {
        await extra.sendNotification(notification as ServerNotification);
      }
    : undefined;

  const metadata: ExecuteMetadata = {
    progress: params,
    loggerContext: {
      transport,
      ...(meta?.requestId && typeof meta.requestId === 'string'
        ? { requestId: meta.requestId }
        : {}),
      ...(meta?.invocationId && typeof meta.invocationId === 'string'
        ? { invocationId: meta.invocationId }
        : {}),
      tool: toolName,
    },
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

function sanitizeParams(params: Record<string, unknown>): Record<string, unknown> {
  const entries = Object.entries(params).filter(([key]) => key !== '_meta');
  return Object.fromEntries(entries);
}

export function formatOutput(output: unknown, format: OutputFormat): string {
  switch (format) {
    case OUTPUTFORMAT.JSON:
      return JSON.stringify(output, null, 2);
    case OUTPUTFORMAT.MARKDOWN:
      // convert a json object to plain text
      if (typeof output === 'object' && output !== null) {
        return objectToMarkdownRecursive(output as Record<string, unknown>);
      }
      return String(output);
    default:
      return String(output);
  }
}

function printSimpleArray(arr: unknown[]): string {
  let markdown = '';
  arr.forEach((item) => {
    markdown += `- ${item}\n`;
  });
  markdown += '\n';
  return markdown;
}

function printSimpleObject(obj: Record<string, unknown>): string {
  let markdown = '';
  for (const [subKey, subValue] of Object.entries(obj)) {
    markdown += `**${subKey}**: ${subValue}\n\n`;
  }
  return markdown;
}

export function objectToMarkdownRecursive(obj: Record<string, unknown>, headingLevel = 2): string {
  // Check if the entire object is simple (only contains primitive values)
  if (isSimpleObject(obj)) {
    return printSimpleObject(obj);
  }

  let markdown = '';
  const headingPrefix = '#'.repeat(headingLevel);

  for (const [key, value] of Object.entries(obj)) {
    const capitalizedKey = key.charAt(0).toUpperCase() + key.slice(1);
    if (value === null || value === undefined) {
      markdown += `${headingPrefix} ${capitalizedKey}\n\n${value}\n\n`;
    } else if (Array.isArray(value)) {
      markdown += `${headingPrefix} ${capitalizedKey}\n\n`;
      if (isSimpleArray(value)) {
        // Format simple arrays as markdown lists
        markdown += printSimpleArray(value);
      } else {
        // Format complex arrays with numbered headings
        value.forEach((item, index) => {
          if (typeof item === 'object' && item !== null) {
            markdown += `${headingPrefix}# ${index + 1}\n\n`;
            markdown += objectToMarkdownRecursive(
              item as Record<string, unknown>,
              headingLevel + 2,
            );
          } else {
            markdown += `${index + 1}. ${item}\n\n`;
          }
        });
      }
    } else if (typeof value === 'object' && value !== null) {
      const valueObj = value as Record<string, unknown>;
      if (isSimpleObject(valueObj)) {
        // Format simple objects as key-value pairs
        markdown += `${headingPrefix} ${capitalizedKey}\n\n`;
        markdown += printSimpleObject(valueObj);
      } else {
        // Format complex objects with recursive headings
        markdown += `${headingPrefix} ${capitalizedKey}\n\n`;
        markdown += objectToMarkdownRecursive(valueObj, headingLevel + 1);
      }
    } else {
      markdown += `${headingPrefix} ${capitalizedKey}\n\n${value}\n\n`;
    }
  }
  return markdown;
}

function isSimpleArray(arr: unknown[]): boolean {
  return arr.every(
    (item) =>
      typeof item === 'string' ||
      typeof item === 'number' ||
      typeof item === 'boolean' ||
      item === null ||
      item === undefined,
  );
}

function isSimpleObject(obj: Record<string, unknown>): boolean {
  return Object.values(obj).every(
    (value) =>
      typeof value === 'string' ||
      typeof value === 'number' ||
      typeof value === 'boolean' ||
      value === null ||
      value === undefined,
  );
}
