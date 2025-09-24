/**
 * Direct MCP Server Implementation
 *
 * Uses the Application Kernel for all tool execution.
 * Minimal wrapper around the SDK with kernel integration.
 */

import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import { McpError, ErrorCode } from '@modelcontextprotocol/sdk/types.js';
import { z } from 'zod';
import { extractErrorMessage } from '@/lib/error-utils';
import type { Kernel } from '@/app/kernel';
import { createLogger, type Logger } from '@/lib/logger';
import { createToolContext } from '@/mcp/context';
import { getAllInternalTools } from '@/exports/tools';
import { createSessionManager } from '@/lib/session';

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
 * Type adapter for MCP SDK schema expectations
 */
function adaptSchemaForMCP(schemaShape: Record<string, unknown>): any {
  // The MCP SDK expects a specific format for schemas.
  // This adapter ensures compatibility between our Zod schemas and MCP's expectations.
  // Using 'any' here is intentional as the MCP SDK has specific runtime expectations
  // that don't align with TypeScript's type system.
  return schemaShape;
}

/**
 * Register all tools and resources directly with SDK
 */
const registerHandlers = async (state: MCPServerState): Promise<void> => {
  // Load tools directly - don't get from kernel
  const tools = getAllInternalTools();

  // Create session manager for tools to share data
  const sessionManager = createSessionManager(state.logger);

  // Create MCP context from the server with session manager
  const mcpContext = createToolContext(state.server.server, state.logger, {
    sessionManager,
  });

  // Register each tool
  for (const tool of tools) {
    const name = tool.name;
    // Tools have both 'schema' (shape) and 'zodSchema' (full schema)
    let schemaShape: Record<string, unknown> = {};

    if (tool.schema) {
      // Use the shape directly
      schemaShape = tool.schema;
    } else if (tool.zodSchema) {
      // Extract shape from zodSchema if needed
      const extractShape = (schema: z.ZodTypeAny): Record<string, unknown> | null => {
        if (schema instanceof z.ZodObject) {
          return schema.shape;
        } else if (schema instanceof z.ZodEffects) {
          // ZodEffects wraps another schema
          return extractShape(schema.innerType());
        } else if ('shape' in schema && typeof schema.shape === 'object') {
          // Fallback: if it has a shape property, use it
          return schema.shape as Record<string, unknown>;
        }
        return null;
      };

      const shape = extractShape(tool.zodSchema);
      if (shape) {
        schemaShape = shape;
      } else {
        // Log warning for non-standard schema types
        state.logger.warn(
          {
            tool: name,
            schemaType: (tool.zodSchema as z.ZodTypeAny).constructor.name || 'unknown',
          },
          'Tool has non-standard Zod schema type, using empty schema shape',
        );
      }
    }

    state.server.tool(
      name,
      tool.description || `${name} tool`,
      adaptSchemaForMCP(schemaShape),
      async (params: Record<string, unknown>) => {
        state.logger.info({ tool: name }, 'Executing tool directly');

        try {
          // Execute tool directly with MCP context
          const result = await tool.execute(params, state.logger.child({ tool: name }), mcpContext);

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
      const tools = getAllInternalTools();

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
                  tools: tools.length,
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

  state.logger.info(`Registered ${tools.length} tools directly`);
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

  const tools = getAllInternalTools();
  state.logger.info(
    {
      tools: tools.length,
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
  const tools = getAllInternalTools();
  return {
    running: state.isRunning,
    tools: tools.length,
    resources: 1,
    prompts: 0, // Prompts handled separately for now
  };
};

/**
 * Get available tools for CLI listing
 */
const getTools = (): Array<{ name: string; description: string }> => {
  const tools = getAllInternalTools();
  return tools.map((tool) => ({
    name: tool.name,
    description: tool.description || '',
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
    getTools: () => getTools(),
  };
};
