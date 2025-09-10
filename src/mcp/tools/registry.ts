/**
 * Unified Tool Registry - Simple Map-based registry for tools
 */

import type { Logger } from 'pino';
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
  McpError,
  ErrorCode,
} from '@modelcontextprotocol/sdk/types.js';
import type { Tool } from '../../domain/types';
import type { PromptRegistry } from '../../core/prompts/registry';
import type { SessionManager } from '../../lib/session';
import { createToolContextWithProgress } from '../context/tool-context';
import type { Server } from '@modelcontextprotocol/sdk/server/index.js';

// Sampling tools are now internal services - not imported here

/**
 * Unified tool registry interface - simple Map-based registry
 */
export interface SDKToolRegistry {
  tools: Map<string, Tool>;
  registerTool(tool: Tool): void;
  getTool(name: string): Tool | undefined;
  getAllTools(): Tool[];
  getToolSchemas(): Array<{
    name: string;
    description: string;
    inputSchema?: Record<string, unknown>;
  }>;
  setupServerHandlers(server: McpServer): void;
}

/**
 * Create SDK-native tool registry using simple Map
 */
export const createSDKToolRegistry = (
  logger: Logger,
  _server: McpServer,
  _sessionManager: SessionManager,
  _options?: {
    promptRegistry?: PromptRegistry;
    mcpServer?: Server; // Add reference to MCP server for ToolContext creation
  },
): SDKToolRegistry => {
  // Create simple Map-based registry
  const tools = new Map<string, Tool>();

  const registry: SDKToolRegistry = {
    tools,

    registerTool(tool: Tool): void {
      tools.set(tool.name, tool);
      logger.debug({ tool: tool.name }, 'Tool registered');
    },

    getTool(name: string): Tool | undefined {
      return tools.get(name);
    },

    getAllTools(): Tool[] {
      return Array.from(tools.values());
    },

    getToolSchemas(): Array<{
      name: string;
      description: string;
      inputSchema?: Record<string, unknown>;
    }> {
      return Array.from(tools.values()).map((tool) => ({
        name: tool.name,
        description: tool.description || `${tool.name} tool`,
        inputSchema: tool.schema || { type: 'object', properties: {} },
      }));
    },

    setupServerHandlers(server: McpServer): void {
      // SDK-native tool listing handler
      server.server.setRequestHandler(ListToolsRequestSchema, async () => {
        logger.debug('SDK registry handling tools/list request');
        const toolSchemas = registry.getToolSchemas();
        logger.info({ count: toolSchemas.length }, 'SDK registry returning tool list');
        return { tools: toolSchemas };
      });

      // SDK-native tool execution handler
      server.server.setRequestHandler(CallToolRequestSchema, async (request) => {
        const { name, arguments: args } = request.params;
        logger.info({ tool: name }, 'SDK registry executing tool');

        const tool = tools.get(name);
        if (!tool) {
          logger.error({ tool: name }, 'Tool not found');
          throw new McpError(ErrorCode.MethodNotFound, `Tool not found: ${name}`);
        }

        try {
          // Create ToolContext - no more dual context system
          const toolContext = _options?.mcpServer
            ? createToolContextWithProgress(
                _options.mcpServer,
                request,
                logger.child({ component: 'ToolContext', tool: name }),
                undefined, // signal
                undefined, // config
                _options.promptRegistry,
              )
            : undefined;

          const result = await tool.execute(args ?? {}, logger.child({ tool: name }), toolContext);

          // Handle Result<T> pattern
          if (result && typeof result === 'object' && 'ok' in result) {
            if (result.ok) {
              return {
                content: [
                  {
                    type: 'text' as const,
                    text: JSON.stringify(result.value, null, 2),
                  },
                ],
              };
            } else {
              throw new McpError(ErrorCode.InternalError, result.error);
            }
          }

          // Handle other response formats
          return {
            content: [
              {
                type: 'text' as const,
                text: JSON.stringify(result, null, 2),
              },
            ],
          };
        } catch (error) {
          logger.error({ tool: name, error }, 'SDK registry tool execution failed');

          // Re-throw MCP errors as-is
          if (error instanceof McpError) {
            throw error;
          }

          // Convert other errors to MCP errors
          throw new McpError(
            ErrorCode.InternalError,
            error instanceof Error ? error.message : 'Unknown error occurred',
            { tool: name },
          );
        }
      });

      logger.info('SDK registry handlers configured');
    },
  };

  return registry;
};
