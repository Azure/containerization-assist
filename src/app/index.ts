/**
 * Application Entry Point - AppRuntime Implementation
 * Provides type-safe runtime with dependency injection support
 */

import type { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';

import { createLogger } from '@/lib/logger';
import { type Tool, type ToolName, ALL_TOOLS } from '@/tools';
import { createToolContext } from '@/mcp/context';
import {
  createMCPServer,
  OUTPUTFORMAT,
  registerToolsWithServer,
  type MCPServer,
} from '@/mcp/mcp-server';
import { createOrchestrator, createHostlessToolContext } from './orchestrator';
import type { OrchestratorConfig, ExecuteRequest, ToolOrchestrator } from './orchestrator-types';
import type { Result } from '@/types';
import type {
  AppRuntime,
  AppRuntimeConfig,
  ToolInputMap,
  ToolResultMap,
  ExecutionMetadata,
} from '@/types/runtime';
import { createToolLoggerFile, getLogFilePath } from '@/lib/tool-logger';

/**
 * Apply tool aliases to create renamed versions of tools
 */
function applyToolAliases(tools: readonly Tool[], aliases?: Record<string, string>): Tool[] {
  if (!aliases) return [...tools];

  return tools.map((tool) => {
    const alias = aliases[tool.name];
    if (!alias) return tool;

    // Create a new tool object with the alias name
    return { ...tool, name: alias };
  });
}

/**
 * Transport configuration for MCP server
 */
export interface TransportConfig {
  transport: 'stdio';
}

/**
 * Create the containerization assist application with AppRuntime interface
 */
export function createApp(config: AppRuntimeConfig = {}): AppRuntime {
  const logger = config.logger || createLogger({ name: 'containerization-assist' });

  // Initialize tool logging file at startup
  createToolLoggerFile(logger);

  const tools = config.tools || ALL_TOOLS;
  const aliasedTools = applyToolAliases(tools, config.toolAliases);

  // Erase per-tool generics for runtime registration; validation still re-parses inputs per schema
  const registryTools: Tool[] = aliasedTools.map((tool) => tool as unknown as Tool);

  const toolsMap = new Map<string, Tool>();
  for (const tool of registryTools) {
    toolsMap.set(tool.name, tool);
  }

  const chainHintsMode = config.chainHintsMode || 'enabled';
  const outputFormat = config.outputFormat || OUTPUTFORMAT.MARKDOWN;
  const orchestratorConfig: OrchestratorConfig = { chainHintsMode };
  if (config.policyPath !== undefined) orchestratorConfig.policyPath = config.policyPath;
  if (config.policyEnvironment !== undefined)
    orchestratorConfig.policyEnvironment = config.policyEnvironment;

  const toolList = Array.from(toolsMap.values());

  let activeServer: Server | null = null;
  let activeMcpServer: MCPServer | null = null;
  let orchestrator = buildOrchestrator();
  let orchestratorClosed = false;

  function buildOrchestrator(): ToolOrchestrator {
    return createOrchestrator({
      registry: toolsMap,
      logger,
      config: orchestratorConfig,
      contextFactory: ({ request, logger: toolLogger }) => {
        const metadata = request.metadata;
        const server = activeServer;
        if (server) {
          const contextOptions = {
            ...(metadata?.signal && { signal: metadata.signal }),
            ...(metadata?.progress !== undefined && { progress: metadata.progress }),
            ...(metadata?.maxTokens !== undefined && { maxTokens: metadata.maxTokens }),
            ...(metadata?.stopSequences && { stopSequences: metadata.stopSequences }),
          };
          return createToolContext(server, toolLogger, contextOptions);
        }

        return createHostlessToolContext(toolLogger, {
          ...(metadata && { metadata }),
        });
      },
    });
  }

  function ensureOrchestrator(): ToolOrchestrator {
    if (orchestratorClosed) {
      orchestrator = buildOrchestrator();
      orchestratorClosed = false;
    }
    return orchestrator;
  }

  const orchestratedExecute = (request: ExecuteRequest): Promise<Result<unknown>> =>
    ensureOrchestrator().execute(request);

  return {
    /**
     * Execute a tool with type-safe parameters and results
     */
    execute: async <T extends ToolName>(
      toolName: T,
      params: ToolInputMap[T],
      metadata?: ExecutionMetadata,
    ): Promise<Result<ToolResultMap[T]>> =>
      orchestratedExecute({
        toolName: toolName as string,
        params,
        ...(metadata?.sessionId && { sessionId: metadata.sessionId }),
        metadata: {
          loggerContext: {
            transport: metadata?.transport || 'programmatic',
            requestId: metadata?.requestId,
            ...metadata,
          },
        },
      }) as Promise<Result<ToolResultMap[T]>>,

    /**
     * Start MCP server with the specified transport
     */
    startServer: async (transport: TransportConfig) => {
      if (activeMcpServer) {
        throw new Error('MCP server is already running');
      }

      ensureOrchestrator();

      const serverOptions: Parameters<typeof createMCPServer>[1] = {
        logger,
        transport: transport.transport,
        name: 'containerization-assist',
        version: '1.0.0',
        outputFormat,
      };

      const mcpServer = createMCPServer(toolList, serverOptions, orchestratedExecute);
      activeServer = mcpServer.getServer();

      try {
        await mcpServer.start();
        activeMcpServer = mcpServer;
        return mcpServer;
      } catch (error) {
        activeServer = null;
        throw error;
      }
    },

    /**
     * Bind to existing MCP server
     */
    bindToMCP: (server: McpServer, transportLabel = 'external') => {
      ensureOrchestrator();

      // Extract the underlying SDK Server from McpServer
      const sdkServer = (server as unknown as { server: Server }).server;
      activeServer = sdkServer;

      registerToolsWithServer({
        outputFormat,
        server,
        tools: toolList,
        logger,
        transport: transportLabel,
        execute: orchestratedExecute,
      });
    },

    /**
     * List all available tools with their metadata
     */
    listTools: () =>
      toolList.map((t) => ({
        name: t.name as ToolName,
        description: t.description,
        ...(t.version && { version: t.version }),
        ...(t.category && { category: t.category }),
      })),

    /**
     * Perform health check
     */
    healthCheck: () => {
      const toolCount = toolsMap.size;
      return {
        status: 'healthy' as const,
        tools: toolCount,
        sessions: 0, // Session count not available in current implementation
        message: `${toolCount} tools loaded`,
      };
    },

    /**
     * Stop the server and orchestrator if running
     */
    stop: async () => {
      if (activeMcpServer) {
        await activeMcpServer.stop();
        activeMcpServer = null;
      }

      activeServer = null;

      if (!orchestratorClosed) {
        orchestrator.close();
        orchestratorClosed = true;
      }
    },

    /**
     * Get the current log file path (if tool logging is enabled)
     */
    getLogFilePath: () => {
      return getLogFilePath();
    },
  };
}
