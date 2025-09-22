/**
 * Idiomatic TypeScript API for Container Assist MCP tools
 * Clean, functional interface with minimal abstractions
 */

import type { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import * as crypto from 'crypto';
import type { Logger } from 'pino';

import { createSessionManager, type SessionManager } from '@/lib/session.js';
import { createLogger } from '@/lib/logger.js';
import { createKernel, type Kernel, type RegisteredTool } from '@/app/kernel.js';
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
 * Generate a session ID based on tool parameters, using path-specific IDs when applicable
 */
function getSessionIdForParams(params: Record<string, unknown>, defaultSessionId: string): string {
  // If user explicitly provided a sessionId, use it
  if (typeof params.sessionId === 'string') {
    return params.sessionId;
  }

  // For tools that work on specific paths, create path-based session IDs
  if (typeof params.path === 'string') {
    const pathHash = Buffer.from(params.path)
      .toString('base64')
      .replace(/[/+=]/g, '')
      .substring(0, 8);
    return `${defaultSessionId}-path-${pathHash}`;
  }

  // For other tools, use the default instance session ID
  return defaultSessionId;
}

/**
 * Create a Container Assist instance with idiomatic TypeScript patterns
 */
export async function createContainerAssist(
  options: {
    logger?: Logger;
    sessionId?: string;
  } = {},
): Promise<ContainerAssist> {
  const logger = options.logger || createLogger({ name: 'containerization-assist' });
  const sessionManager = createSessionManager(logger);
  const tools = loadAllTools();
  const kernel = await createKernelForTools(tools, logger);

  // Generate a consistent session ID for this instance to maintain state across tool calls
  const defaultSessionId =
    options.sessionId ||
    `container-assist-${Date.now()}-${crypto.randomBytes(6).toString('base64').replace(/[/+=]/g, '').substring(0, 9)}`;

  return {
    bindToServer: (server: McpServer) =>
      bindAllTools(server, tools, kernel, logger, sessionManager, defaultSessionId),
    registerTools: (server: McpServer, config?: ToolConfig) =>
      registerSelectedTools(
        server,
        tools,
        kernel,
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
    registeredTools.set(name, {
      name,
      description: tool.description || '',
      handler: async (
        params: unknown,
        _kernelContext: import('@/app/types').ToolContext,
      ): Promise<Result<unknown>> => {
        // Tools will use the logger directly, no MCP context needed for kernel
        const toolLogger = logger.child({ tool: name });
        // Pass null context since tools should work without it
        return await tool.execute(params as Record<string, unknown>, toolLogger, undefined as any);
      },
      schema: tool.zodSchema as any,
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
  sessionManager: SessionManager,
  defaultSessionId: string,
): void {
  registerSelectedTools(server, tools, kernel, undefined, logger, sessionManager, defaultSessionId);
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
  sessionManager: SessionManager,
  defaultSessionId: string,
): void {
  const toolsToRegister = config.tools
    ? Array.from(tools.entries()).filter(([name]) => config.tools?.includes(name as ToolName))
    : Array.from(tools.entries());

  for (const [originalName, tool] of toolsToRegister) {
    const customName = config.nameMapping?.[originalName as ToolName] || originalName;

    registerSingleTool(server, tool, customName, kernel, logger, sessionManager, defaultSessionId);
  }
}

/**
 * Register a single tool with the MCP server
 */
function registerSingleTool(
  server: McpServerLike,
  tool: Tool,
  name: string,
  kernel: Kernel,
  logger: Logger,
  _sessionManager: SessionManager,
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

        // Extract session info from params - use path-based session for path-specific tools
        const paramsObj = (args || {}) as Record<string, unknown>;
        const sessionId = getSessionIdForParams(paramsObj, defaultSessionId);

        // Use kernel for execution
        const result = await kernel.execute({
          toolName: tool.name,
          params: paramsObj,
          sessionId,
          force: paramsObj.force === true,
        });

        // Log execution info
        toolLogger.info({ tool: tool.name, sessionId }, 'Kernel executed tool');

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
