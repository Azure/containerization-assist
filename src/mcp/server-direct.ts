/**
 * Direct MCP Server Implementation
 *
 * Uses the Application Kernel for all tool execution.
 * Minimal wrapper around the SDK with kernel integration.
 */

import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import { McpError, ErrorCode } from '@modelcontextprotocol/sdk/types.js';
import { randomUUID } from 'node:crypto';
import { z } from 'zod';
import { extractErrorMessage } from '@/lib/error-utils';
import type { Kernel } from '@/app/kernel';
import { createLogger, type Logger } from '@/lib/logger';

/**
 * Server options for configuration
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
 * MCP Server state interface
 */
export interface MCPServerState {
  server: McpServer;
  transport: StdioServerTransport;
  isRunning: boolean;
  kernel: Kernel;
  logger: Logger;
  options: ServerOptions;
}

/**
 * MCP Server interface for dependency injection compatibility
 */
export interface IDirectMCPServer {
  start(): Promise<void>;
  stop(): Promise<void>;
  getServer(): unknown;
  getStatus(): {
    running: boolean;
    tools: number;
    resources: number;
    prompts: number;
  };
  getTools(): Array<{ name: string; description: string }>;
}

/**
 * Register all tools and resources directly with SDK
 */
const registerHandlers = async (state: MCPServerState): Promise<void> => {
  // Get tools from kernel
  const tools = state.kernel.tools();

  // Register each tool from kernel
  for (const [name, tool] of tools) {
    // Extract schema shape, with robust handling for different Zod schema types
    let schemaShape: Record<string, any> = {};

    if (tool.schema) {
      // Helper function to extract shape from schema
      const extractShape = (schema: any): Record<string, any> | null => {
        if (schema instanceof z.ZodObject) {
          return schema.shape;
        } else if (schema instanceof z.ZodEffects) {
          // ZodEffects wraps another schema
          return extractShape(schema.innerType());
        } else if ('shape' in schema && typeof schema.shape === 'object') {
          // Fallback: if it has a shape property, use it
          return schema.shape as Record<string, any>;
        }
        return null;
      };

      const shape = extractShape(tool.schema);
      if (shape) {
        schemaShape = shape;
      } else {
        // Log warning for non-standard schema types
        state.logger.warn(
          { tool: name, schemaType: tool.schema.constructor.name || 'unknown' },
          'Tool has non-standard Zod schema type, using empty schema shape',
        );
      }
    }

    state.server.tool(
      name,
      tool.description,
      schemaShape,
      async (params: Record<string, unknown>) => {
        state.logger.info({ tool: name }, 'Executing tool via kernel');

        try {
          // Generate session ID if not provided
          const sessionId = (params.sessionId as string) || randomUUID();

          // Execute through kernel
          const result = await state.kernel.execute({
            toolName: name,
            params,
            sessionId,
            force: params.force === true,
          });

          // Format result for MCP
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
          state.logger.error(
            { error: extractErrorMessage(error), tool: name },
            'Tool execution error',
          );
          throw error instanceof McpError
            ? error
            : new McpError(ErrorCode.InternalError, extractErrorMessage(error));
        }
      },
    );
  }

  // Register status resource directly
  state.server.resource(
    'status',
    'containerization://status',
    {
      title: 'Container Status',
      description: 'Current status of the containerization system',
    },
    async () => {
      const health = state.kernel.getHealth();
      const tools = state.kernel.tools();

      return {
        contents: [
          {
            uri: 'containerization://status',
            mimeType: 'application/json',
            text: JSON.stringify(
              {
                healthy: health.status === 'healthy',
                running: state.isRunning,
                kernel: health,
                stats: {
                  tools: tools.size,
                  resources: 1, // status resource
                  prompts: 0, // Update when prompts are added
                },
              },
              null,
              2,
            ),
          },
        ],
      };
    },
  );

  // Register prompts if needed
  await registerPrompts(state);

  state.logger.info(`Registered ${tools.size} tools via kernel`);
};

/**
 * Register prompts directly from registry
 */
const registerPrompts = async (state: MCPServerState): Promise<void> => {
  // For now, prompts are handled separately from the kernel's tool system
  // They can be added to kernel later if needed
  state.logger.info('Prompts registration handled separately from kernel');
};

/**
 * Start the server
 */
const startServer = async (state: MCPServerState): Promise<void> => {
  if (state.isRunning) {
    state.logger.warn('Server already running');
    return;
  }

  await registerHandlers(state);
  await state.server.connect(state.transport);
  state.isRunning = true;

  const tools = state.kernel.tools();
  state.logger.info(
    {
      tools: tools.size,
      prompts: 0,
      healthy: true,
    },
    'MCP server started',
  );
};

/**
 * Stop the server
 */
const stopServer = async (state: MCPServerState): Promise<void> => {
  if (!state.isRunning) {
    return;
  }

  await state.server.close();
  state.isRunning = false;
  state.logger.info('Server stopped');
};

/**
 * Get SDK server instance for sampling
 */
const getServer = (state: MCPServerState): unknown => {
  return state.server.server;
};

/**
 * Get server status
 */
const getStatus = (
  state: MCPServerState,
): {
  running: boolean;
  tools: number;
  resources: number;
  prompts: number;
} => {
  const tools = state.kernel.tools();
  return {
    running: state.isRunning,
    tools: tools.size,
    resources: 1,
    prompts: 0, // Prompts handled separately for now
  };
};

/**
 * Get available tools for CLI listing
 */
const getTools = (state: MCPServerState): Array<{ name: string; description: string }> => {
  const tools = state.kernel.tools();
  return Array.from(tools.entries()).map(([name, tool]) => ({
    name,
    description: tool.description,
  }));
};

/**
 * Factory function to create a DirectMCPServer implementation.
 *
 * Uses the Application Kernel for all tool execution.
 */
export const createDirectMCPServer = (
  kernel: Kernel,
  options?: ServerOptions,
): IDirectMCPServer => {
  const logger = options?.logger || createLogger({ name: 'mcp-server' });

  const state: MCPServerState = {
    server: new McpServer(
      {
        name: options?.name || 'containerization-assist',
        version: options?.version || '1.0.0',
      },
      {
        capabilities: {
          resources: { subscribe: false, listChanged: false },
          prompts: { listChanged: false },
          tools: { listChanged: false },
        },
      },
    ),
    transport: new StdioServerTransport(),
    isRunning: false,
    kernel,
    logger,
    options: options || {},
  };

  return {
    start: () => startServer(state),
    stop: () => stopServer(state),
    getServer: () => getServer(state),
    getStatus: () => getStatus(state),
    getTools: () => getTools(state),
  };
};
