/**
 * Idiomatic TypeScript API for Container Assist MCP tools
 * Clean, functional interface with minimal abstractions
 */

import type { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import type { Logger } from 'pino';

import { createSessionManager, type SessionManager } from '../lib/session.js';
import { createLogger } from '../lib/logger.js';
import { createToolContext, type ToolContext } from '../mcp/context.js';
import { createToolRouter, type IToolRouter, type RouterTool } from '../mcp/tool-router.js';
import { getAllInternalTools } from './tools.js';
import { extractErrorMessage } from '../lib/error-utils.js';
import type { Tool, Result } from '../types.js';
import type { ToolName } from './tool-names.js';

/**
 * Tool registration configuration
 */
export interface ToolConfig {
  /** Specific tools to register (defaults to all) */
  tools?: readonly ToolName[];
  /** Custom names for tools */
  nameMapping?: Partial<Record<ToolName, string>>;
}

/**
 * Container Assist API interface
 */
export interface ContainerAssist {
  /** Register all tools with an MCP server */
  bindToServer: (server: McpServer) => void;
  /** Register specific tools with configuration */
  registerTools: (server: McpServer, config?: ToolConfig) => void;
  /** Get list of available tool names */
  getAvailableTools: () => readonly string[];
  /** Get the session ID used for maintaining state across tool calls */
  getSessionId: () => string;
}

/**
 * MCP Server union type for compatibility
 */
type McpServerLike = McpServer;

/**
 * Create a Container Assist instance with idiomatic TypeScript patterns
 */
export function createContainerAssist(
  options: {
    logger?: Logger;
    sessionId?: string;
  } = {},
): ContainerAssist {
  const logger = options.logger || createLogger({ name: 'containerization-assist' });
  const sessionManager = createSessionManager(logger);
  const tools = loadAllTools();
  const router = createRouter(sessionManager, logger, tools);

  // Generate a consistent session ID for this instance to maintain state across tool calls
  const defaultSessionId =
    options.sessionId ||
    `container-assist-${Date.now()}-${Math.random().toString(36).substring(2, 9)}`;

  return {
    bindToServer: (server: McpServer) =>
      bindAllTools(server, tools, router, logger, sessionManager, defaultSessionId),
    registerTools: (server: McpServer, config?: ToolConfig) =>
      registerSelectedTools(
        server,
        tools,
        router,
        config,
        logger,
        sessionManager,
        defaultSessionId,
      ),
    getAvailableTools: () => Object.keys(tools) as readonly string[],
    getSessionId: () => defaultSessionId,
  } as const;
}

/**
 * Load all internal tools into a map
 */
function loadAllTools(): Map<string, Tool> {
  const toolsMap = new Map<string, Tool>();
  const internalTools = getAllInternalTools();

  for (const tool of internalTools) {
    toolsMap.set(tool.name, tool);
  }

  return toolsMap;
}

/**
 * Create tool router with proper configuration matching standalone server
 */
function createRouter(
  sessionManager: SessionManager,
  logger: Logger,
  tools: Map<string, Tool>,
): IToolRouter {
  const routerTools = new Map<string, RouterTool>();

  for (const [name, tool] of tools.entries()) {
    routerTools.set(name, {
      name,
      handler: async (
        params: Record<string, unknown>,
        context: ToolContext,
      ): Promise<Result<unknown>> => {
        // Use the tool's execute method directly - the router's executeToolImpl will handle session persistence
        const toolLogger = logger.child({ tool: name });
        return await tool.execute(params, toolLogger, context);
      },
      schema: tool.zodSchema ? (tool.zodSchema as any) : undefined,
    });
  }

  return createToolRouter({
    sessionManager,
    logger,
    tools: routerTools,
  });
}

/**
 * Register all tools with an MCP server
 */
function bindAllTools(
  server: McpServerLike,
  tools: Map<string, Tool>,
  router: IToolRouter,
  logger: Logger,
  sessionManager: SessionManager,
  defaultSessionId: string,
): void {
  registerSelectedTools(server, tools, router, undefined, logger, sessionManager, defaultSessionId);
}

/**
 * Register specific tools with configuration
 */
function registerSelectedTools(
  server: McpServerLike,
  tools: Map<string, Tool>,
  router: IToolRouter,
  config: ToolConfig = {},
  logger: Logger,
  sessionManager: SessionManager,
  defaultSessionId: string,
): void {
  const toolsToRegister = config.tools
    ? Array.from(tools.entries()).filter(([name]) => config.tools?.includes(name as ToolName))
    : Array.from(tools.entries());

  for (const [originalName, tool] of toolsToRegister) {
    const customName = config.nameMapping?.[originalName as ToolName] || originalName;

    registerSingleTool(server, tool, customName, router, logger, sessionManager, defaultSessionId);
  }
}

/**
 * Register a single tool with the MCP server
 */
function registerSingleTool(
  server: McpServerLike,
  tool: Tool,
  name: string,
  router: IToolRouter,
  logger: Logger,
  sessionManager: SessionManager,
  defaultSessionId: string,
): void {
  if (!tool.zodSchema) {
    logger.warn({ tool: name }, 'Tool missing Zod schema, skipping registration');
    return;
  }

  // Register with MCP server using clean handler
  server.tool(
    name,
    tool.description || `${name} tool`,
    tool.zodSchema,
    async (args: unknown, _extra: unknown) => {
      try {
        const toolLogger = logger.child({ tool: name });
        const context = createToolContext(server.server, toolLogger, {
          sessionManager,
          maxTokens: 2048,
          stopSequences: ['```', '\n\n```', '\n\n# ', '\n\n---'],
          progress: async (message: string, progress?: number, total?: number) => {
            if (progress !== undefined && total !== undefined) {
              toolLogger.info({ progress, total }, message);
            } else {
              toolLogger.info(message);
            }
          },
        });

        // Extract session info from params - use default session to maintain state
        const paramsObj = (args || {}) as Record<string, unknown>;
        const sessionId =
          typeof paramsObj.sessionId === 'string' ? paramsObj.sessionId : defaultSessionId;

        // Use router for execution with dependency resolution
        const routeResult = await router.route({
          toolName: tool.name,
          params: paramsObj,
          sessionId,
          context,
          force: paramsObj.force === true,
        });

        // Log execution info
        if (routeResult.executedTools.length > 0) {
          toolLogger.info(
            { executedTools: routeResult.executedTools },
            'Router executed tools in sequence',
          );
        }

        if (routeResult.workflowHint) {
          toolLogger.info(
            { hint: routeResult.workflowHint.message },
            'Workflow continuation available',
          );
        }

        return formatResult(routeResult.result);
      } catch (error) {
        return {
          content: [
            {
              type: 'text' as const,
              text: `Error executing ${name}: ${extractErrorMessage(error)}`,
            },
          ],
        };
      }
    },
  );

  logger.info({ tool: name }, 'Tool registered');
}

/**
 * Format tool results for MCP response
 */
function formatResult(result: unknown): { content: Array<{ type: 'text'; text: string }> } {
  // Handle Result<T> pattern
  if (result && typeof result === 'object' && 'ok' in result) {
    const resultObj = result as { ok: boolean; value?: unknown; error?: unknown };
    if (resultObj.ok) {
      return {
        content: [
          {
            type: 'text',
            text: JSON.stringify(resultObj.value, null, 2),
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
}
