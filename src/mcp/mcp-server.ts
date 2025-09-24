/**
 * MCP Server Implementation
 * Direct tool registration without duplicate loading
 */

import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import { McpError, ErrorCode } from '@modelcontextprotocol/sdk/types.js';
import { extractErrorMessage } from '@/lib/error-utils';
import { createLogger, type Logger } from '@/lib/logger';
import { createToolContext } from '@/mcp/context';
import { createSessionManager } from '@/lib/session';
import type { Tool } from '@/types';

/**
 * Server options
 */
export interface ServerOptions {
  logger?: Logger;
  transport?: 'stdio' | 'http';
  port?: number;
  host?: string;
  name?: string;
  version?: string;
}

/**
 * MCP Server interface
 */
export interface MCPServer {
  start(): Promise<void>;
  stop(): Promise<void>;
  getServer(): McpServer;
  getTools(): Array<{ name: string; description: string }>;
}

/**
 * Create an MCP server that uses tools from registry
 */
export function createMCPServer(tools: Tool[], options: ServerOptions = {}): MCPServer {
  const logger = options.logger || createLogger({ name: 'mcp-server' });
  const serverOptions = {
    name: options.name || 'containerization-assist',
    version: options.version || '1.0.0',
  };

  const server = new McpServer(serverOptions);
  const transport = new StdioServerTransport();
  let isRunning = false;

  // Create session manager for tools
  const sessionManager = createSessionManager(logger);

  // Create MCP context
  const mcpContext = createToolContext(server.server, logger, {
    sessionManager,
  });

  // Register all tools
  for (const tool of tools) {
    if (!tool.schema) {
      logger.warn({ tool: tool.name }, 'Tool missing schema, skipping');
      continue;
    }

    server.tool(
      tool.name,
      tool.description || `${tool.name} tool`,
      tool.schema,
      async (params: Record<string, unknown>) => {
        logger.info({ tool: tool.name }, 'Executing tool');

        try {
          const result = await tool.execute(params, logger.child({ tool: tool.name }), mcpContext);

          if (!result.ok) {
            throw new McpError(ErrorCode.InternalError, result.error || 'Tool execution failed');
          }

          return {
            content: [
              {
                type: 'text' as const,
                text: JSON.stringify(result.value, null, 2),
              },
            ],
          };
        } catch (error) {
          logger.error(
            { error: extractErrorMessage(error), tool: tool.name },
            'Tool execution error',
          );
          throw error instanceof McpError
            ? error
            : new McpError(ErrorCode.InternalError, extractErrorMessage(error));
        }
      },
    );
  }

  // Register a simple status resource
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

      await server.connect(transport);
      isRunning = true;
      logger.info('MCP server started');
    },

    async stop(): Promise<void> {
      if (!isRunning) {
        return;
      }

      await server.close();
      isRunning = false;
      logger.info('MCP server stopped');
    },

    getServer(): McpServer {
      return server;
    },

    getTools(): Array<{ name: string; description: string }> {
      return tools.map((t) => ({
        name: t.name,
        description: t.description || '',
      }));
    },
  };
}
