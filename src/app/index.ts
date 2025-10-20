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
import { checkDockerHealth, checkKubernetesHealth } from '@/lib/health-checks';
import { DEFAULT_CHAIN_HINTS } from './chain-hints';

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
  const orchestratorConfig: OrchestratorConfig = {
    chainHintsMode,
    chainHints: DEFAULT_CHAIN_HINTS,
  };
  if (config.policyPath !== undefined) orchestratorConfig.policyPath = config.policyPath;

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
          };
          return createToolContext(toolLogger, contextOptions);
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
    healthCheck: async () => {
      const toolCount = toolsMap.size;

      // Check Docker and Kubernetes connectivity in parallel
      const [dockerStatus, k8sStatus] = await Promise.all([
        checkDockerHealth(logger),
        checkKubernetesHealth(logger),
      ]);

      const hasIssues = !dockerStatus.available || !k8sStatus.available;
      const status: 'healthy' | 'unhealthy' = hasIssues ? 'unhealthy' : 'healthy';

      return {
        status,
        tools: toolCount,
        message: hasIssues
          ? `${toolCount} tools loaded, but some dependencies are unavailable`
          : `${toolCount} tools loaded`,
        dependencies: {
          docker: dockerStatus,
          kubernetes: k8sStatus,
        },
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
