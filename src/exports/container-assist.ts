/**
 * Idiomatic TypeScript API for Container Assist MCP tools
 * Clean, functional interface with minimal abstractions
 */

import type { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import type { Server } from '@modelcontextprotocol/sdk/server/index.js';
import type { Logger } from 'pino';

import { createLogger } from '@/lib/logger.js';
import { createSessionManager } from '@/lib/session.js';
import { createKernel, type Kernel, type RegisteredTool } from '@/app/kernel.js';
import { createToolContext } from '@/mcp/context.js';
import { ToolName, getAllInternalTools } from './tools.js';
import { extractErrorMessage } from '@/lib/error-utils.js';
import type { Tool, Result } from '@/types';

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
export async function createContainerAssist(
  options: {
    logger?: Logger;
  } = {},
): Promise<ContainerAssist> {
  const logger = options.logger || createLogger({ name: 'containerization-assist' });
  const tools = loadAllTools();
  const kernel = await createKernelForTools(tools, logger);

  return {
    bindToServer: (server: McpServer) => bindAllTools(server, tools, kernel, logger),
    registerTools: (server: McpServer, config?: ToolConfig) =>
      registerSelectedTools(server, tools, kernel, config, logger),
    getAvailableTools: () => Object.keys(tools) as readonly string[],
    getSessionId: () => 'deprecated-use-tool-params',
  } as const;
}

/**
 * Load all internal tools into a map
 */
function loadAllTools(): Map<ToolName, Tool> {
  const toolsMap = new Map<ToolName, Tool>();
  const internalTools = getAllInternalTools();

  for (const tool of internalTools) {
    toolsMap.set(tool.name, tool);
  }

  return toolsMap;
}

/**
 * Create kernel for tools
 */
function createKernelForTools(tools: Map<ToolName, Tool>, logger: Logger): Promise<Kernel> {
  const registeredTools = new Map<string, RegisteredTool>();

  for (const [name, tool] of tools.entries()) {
    // Skip tools without schemas
    if (!tool.zodSchema) {
      logger.warn({ tool: name }, 'Tool missing Zod schema, skipping kernel registration');
      continue;
    }

    registeredTools.set(name, {
      name,
      description: tool.description || '',
      handler: async (
        params: unknown,
        _kernelContext: import('@/app/types').ToolContext,
      ): Promise<Result<unknown>> => {
        // Tools will use the logger directly, no MCP context needed for kernel
        const toolLogger = logger.child({ tool: name });
        // Pass undefined context - tools should handle optional context
        return await tool.execute(params as Record<string, unknown>, toolLogger, undefined);
      },
      schema: tool.zodSchema,
    });
  }

  return createKernel(
    {
      sessionStore: 'memory',
      maxRetries: 2,
      retryDelay: 1000,
    },
    registeredTools,
  );
}

/**
 * Register all tools with an MCP server
 */
function bindAllTools(
  server: McpServerLike,
  tools: Map<string, Tool>,
  kernel: Kernel,
  logger: Logger,
): void {
  // Create session manager for tools to share data
  const sessionManager = createSessionManager(logger);

  // Create MCP context from the provided server with session manager
  const mcpContext = createToolContext((server as McpServer & { server: Server }).server, logger, {
    sessionManager,
  });

  // Register all tools (no config filtering)
  const toolsToRegister = Array.from(tools.entries());
  for (const [name, tool] of toolsToRegister) {
    registerSingleToolWithContext(server, tool, name, kernel, logger, mcpContext);
  }
}

/**
 * Register specific tools with configuration
 */
function registerSelectedTools(
  server: McpServerLike,
  tools: Map<string, Tool>,
  kernel: Kernel,
  config: ToolConfig = {},
  logger: Logger,
): void {
  // Create session manager for tools to share data
  const sessionManager = createSessionManager(logger);

  // Create MCP context from the provided server with session manager
  const mcpContext = createToolContext((server as McpServer & { server: Server }).server, logger, {
    sessionManager,
  });

  // Filter tools based on config
  const toolsToRegister = config.tools
    ? Array.from(tools.entries()).filter(([name]) => config.tools?.includes(name as ToolName))
    : Array.from(tools.entries());

  // Register with optional name mapping
  for (const [originalName, tool] of toolsToRegister) {
    const customName = config.nameMapping?.[originalName as ToolName] || originalName;
    registerSingleToolWithContext(server, tool, customName, kernel, logger, mcpContext);
  }
}

/**
 * Register a single tool with the MCP server and context
 */
function registerSingleToolWithContext(
  server: McpServerLike,
  tool: Tool,
  name: string,
  _kernel: Kernel,
  logger: Logger,
  mcpContext: import('@/mcp/context').ToolContext,
): void {
  if (!tool.zodSchema || !tool.schema) {
    logger.warn({ tool: name }, 'Tool missing Zod schema or shape, skipping registration');
    return;
  }

  // Register with MCP server using clean handler
  // MCP SDK expects the raw shape, not the full schema
  server.tool(
    name,
    tool.description || `${name} tool`,
    tool.schema,
    async (args: unknown, _extra: unknown) => {
      try {
        const toolLogger = logger.child({ tool: name });
        const paramsObj = (args || {}) as Record<string, unknown>;

        // Execute tool directly with MCP context (bypass kernel for MCP calls)
        const result = await tool.execute(paramsObj, toolLogger, mcpContext);

        // Log execution info
        toolLogger.info({ tool: tool.name }, 'Tool executed directly');

        return formatResult(result);
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
