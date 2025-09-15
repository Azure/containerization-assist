/**
 * ContainerAssistServer - Clean API for integrating Container Assist tools
 * Eliminates global state by using an instance-based approach
 */

import type { Server } from '@modelcontextprotocol/sdk/server/index.js';
import type { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import type { Tool } from '../types';
import type { MCPTool, MCPToolResult } from './types';
import type { Logger } from 'pino';

import { createSessionManager, type SessionManager } from '../lib/session';
import { createLogger } from '../lib/logger.js';
import { createToolContext, type ToolContext } from '../mcp/context.js';

// Import all tools
import { getAllInternalTools } from './tools.js';
import { extractErrorMessage } from '../lib/error-utils';

/**
 * Container Assist Server state interface
 */
export interface ContainerAssistServerState {
  sessionManager: SessionManager;
  logger: Logger;
  mcpServer?: Server;
  tools: Map<string, Tool>;
  adaptedTools: Map<string, MCPTool>;
}

/**
 * Container Assist Server interface for dependency injection compatibility
 */
export interface IContainerAssistServer {
  bindAll(config: { server: McpServer }): void;
  bindSampling(config: { server: McpServer }): void;
  registerTools(
    config: { server: McpServer },
    options?: {
      tools?: string[];
      nameMapping?: Record<string, string>;
    },
  ): void;
  getTool(name: string): MCPTool | undefined;
  getAllTools(): MCPTool[];
}

/**
 * Load all internal tools
 */
export const loadTools = (state: ContainerAssistServerState): void => {
  const internalTools = getAllInternalTools();
  for (const tool of internalTools) {
    state.tools.set(tool.name, tool);
  }
};

/**
 * Bind to an MCP server and register all tools
 * This is the main entry point for integration
 */
export const bindAll = (state: ContainerAssistServerState, config: { server: McpServer }): void => {
  bindSampling(state, config);
  registerTools(state, config);
};

/**
 * Configure AI sampling capability
 * This allows tools to use the MCP server's sampling features
 */
export const bindSampling = (
  state: ContainerAssistServerState,
  config: { server: McpServer },
): void => {
  // Extract the underlying Server instance from McpServer
  state.mcpServer = config.server.server;
  state.logger.info('AI sampling configured for Container Assist tools');
};

/**
 * Register tools with the MCP server
 * Can optionally specify which tools to register
 */
export const registerTools = (
  state: ContainerAssistServerState,
  config: { server: McpServer },
  options: {
    tools?: string[]; // Specific tools to register
    nameMapping?: Record<string, string>; // Custom names for tools
  } = {},
): void => {
  const mcpServer = config.server;
  const toolsToRegister = options.tools
    ? Array.from(state.tools.entries()).filter(([name]) => options.tools?.includes(name) ?? false)
    : Array.from(state.tools.entries());

  for (const [originalName, tool] of toolsToRegister) {
    const customName = options.nameMapping?.[originalName] || originalName;
    const mcpTool = adaptTool(state, tool);

    // Use McpServer's public tool() method
    // All our tools have Zod schemas in tool.zodSchema (which is the .shape from the Zod object)
    if (!tool.zodSchema) {
      state.logger.warn({ tool: customName }, 'Tool missing Zod schema, skipping registration');
      continue;
    }

    // Register with schema - handler receives (args, extra)
    mcpServer.tool(
      customName,
      mcpTool.metadata.description,
      tool.zodSchema, // This is a ZodRawShape from schema.shape
      async (args: unknown, _extra: unknown) => {
        // Call our handler and convert the result
        const result = await mcpTool.handler(args);
        // Convert our result format to CallToolResult format
        return {
          content: result.content.map((item) => ({
            type: 'text' as const,
            text: item.text || '',
          })),
        };
      },
    );

    // Store adapted tool
    state.adaptedTools.set(customName, mcpTool);

    state.logger.info(
      {
        originalName,
        registeredAs: customName,
      },
      'Tool registered',
    );
  }
};

/**
 * Get an adapted tool by name
 */
export const getTool = (state: ContainerAssistServerState, name: string): MCPTool | undefined => {
  return state.adaptedTools.get(name);
};

/**
 * Get all registered tools
 */
export const getAllTools = (state: ContainerAssistServerState): MCPTool[] => {
  return Array.from(state.adaptedTools.values());
};

/**
 * Create a tool context for execution
 */
export const createContext = (state: ContainerAssistServerState, params?: unknown): ToolContext => {
  const logger = state.logger.child({ context: 'tool-execution' });

  const progressReporter = async (
    message: string,
    progress?: number,
    total?: number,
  ): Promise<void> => {
    if (progress !== undefined && total !== undefined) {
      logger.info({ progress, total }, message);
    } else {
      logger.info(message);
    }
  };

  const context = createToolContext(state.mcpServer as Server, logger, {
    sessionManager: state.sessionManager,
    maxTokens: 2048,
    stopSequences: ['```', '\n\n```', '\n\n# ', '\n\n---'],
    progress: progressReporter,
  });

  // Handle session creation if needed
  if (params && typeof params === 'object' && 'sessionId' in params) {
    const sessionId = (params as { sessionId?: string }).sessionId;
    if (sessionId) {
      void ensureSession(state, sessionId);
    }
  }

  return context;
};

/**
 * Ensure a session exists
 */
export const ensureSession = async (
  state: ContainerAssistServerState,
  sessionId: string,
): Promise<void> => {
  try {
    const session = await state.sessionManager.get(sessionId);
    if (!session.ok || !session.value) {
      await state.sessionManager.create(sessionId);
    }
  } catch (err) {
    state.logger.warn({ sessionId, error: err }, 'Session management error');
  }
};

/**
 * Adapt an internal tool to MCPTool interface
 */
export const adaptTool = (state: ContainerAssistServerState, tool: Tool): MCPTool => {
  return {
    name: tool.name,
    metadata: {
      title: tool.name.replace(/_/g, ' ').replace(/\b\w/g, (l) => l.toUpperCase()),
      description: tool.description || `${tool.name} tool`,
      inputSchema: tool.schema || { type: 'object', properties: {} },
    },
    handler: async (params: unknown) => {
      try {
        const toolLogger = state.logger.child({ tool: tool.name });
        const toolContext = createContext(state, params);

        const result = await tool.execute(
          (params || {}) as Record<string, unknown>,
          toolLogger,
          toolContext,
        );
        return formatResult(result);
      } catch (error) {
        return {
          content: [
            {
              type: 'text',
              text: `Error executing ${tool.name}: ${extractErrorMessage(error)}`,
            },
          ],
        };
      }
    },
  };
};

/**
 * Format tool results consistently
 */
export const formatResult = (result: unknown): MCPToolResult => {
  // Handle Result<T> pattern
  if (result && typeof result === 'object' && 'ok' in result) {
    const resultObj = result as { ok: boolean; value?: unknown; error?: unknown };
    if (resultObj.ok) {
      const value = resultObj.value;

      // Tools now provide their own enrichment (chain hints, file indicators)
      // Just return the value as JSON
      return {
        content: [
          {
            type: 'text',
            text: JSON.stringify(value, null, 2),
          },
        ],
      };
    } else {
      return {
        content: [
          {
            type: 'text',
            text: `Error: ${resultObj.error}`,
          },
        ],
      };
    }
  }

  // Direct response
  return {
    content: [
      {
        type: 'text',
        text: typeof result === 'string' ? result : JSON.stringify(result, null, 2),
      },
    ],
  };
};

/**
 * Factory function to create a ContainerAssistServer implementation.
 *
 * Maintains all existing functionality while using functional patterns internally.
 */
export const createContainerAssistServer = (
  options: { logger?: Logger } = {},
): IContainerAssistServer => {
  const state: ContainerAssistServerState = {
    logger: options.logger || createLogger({ name: 'containerization-assist' }),
    sessionManager: createSessionManager(
      options.logger || createLogger({ name: 'containerization-assist' }),
    ),
    tools: new Map(),
    adaptedTools: new Map(),
  };

  // Load all internal tools
  loadTools(state);

  return {
    bindAll: (config: { server: McpServer }) => bindAll(state, config),
    bindSampling: (config: { server: McpServer }) => bindSampling(state, config),
    registerTools: (
      config: { server: McpServer },
      options?: {
        tools?: string[];
        nameMapping?: Record<string, string>;
      },
    ) => registerTools(state, config, options),
    getTool: (name: string) => getTool(state, name),
    getAllTools: () => getAllTools(state),
  };
};
