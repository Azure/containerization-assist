/**
 * Application Entry Point - AppRuntime Implementation
 * Provides type-safe runtime with dependency injection support
 */

import type { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';

import { createLogger } from '@/lib/logger';
import { type Tool, type ToolName, ALL_TOOLS } from '@/tools';
import {
  createMCPServer,
  OUTPUTFORMAT,
  registerToolsWithServer,
  type MCPServer,
} from '@/mcp/mcp-server';
import { createOrchestrator } from './orchestrator';
import {
  CHAINHINTSMODE,
  type OrchestratorConfig,
  type ExecuteRequest,
  type ToolOrchestrator,
} from './orchestrator-types';
import type { Result } from '@/types';
import type {
  AppRuntime,
  AppRuntimeConfig,
  ToolInputMap,
  ToolResultMap,
  ExecutionMetadata,
} from '@/types/runtime';
import { createToolLoggerFile, getLogFilePath } from '@/lib/tool-logger';
import { checkDockerHealth, checkKubernetesHealth } from '@/infra/health/checks';
import { DEFAULT_CHAIN_HINTS } from './chain-hints';
import { loadKnowledgeBase } from '@/knowledge/loader';

/**
 * Apply tool aliases to create renamed versions of tools
 * Returns both the aliased tools and a reverse mapping (alias -> original)
 */
function applyToolAliases(
  tools: readonly Tool[],
  aliases?: Record<string, string>,
): { aliasedTools: Tool[]; aliasToOriginalMap: Record<string, string> } {
  if (!aliases) {
    return { aliasedTools: [...tools], aliasToOriginalMap: {} };
  }

  const aliasToOriginalMap: Record<string, string> = {};

  const aliasedTools = tools.map((tool) => {
    const alias = aliases[tool.name];
    if (!alias) return tool;

    // Store reverse mapping (alias -> original)
    aliasToOriginalMap[alias] = tool.name;

    // Create a new tool object with the alias name
    // Type assertion: aliases are treated as valid ToolNames for registration
    return { ...tool, name: alias as ToolName };
  });

  return { aliasedTools, aliasToOriginalMap };
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
  const { aliasedTools, aliasToOriginalMap } = applyToolAliases(tools, config.toolAliases);

  // Erase per-tool generics for runtime registration; validation still re-parses inputs per schema
  const registryTools: Tool[] = aliasedTools.map((tool) => tool as unknown as Tool);

  const toolsMap = new Map<string, Tool>();
  for (const tool of registryTools) {
    toolsMap.set(tool.name, tool);
  }

  const chainHintsMode = config.chainHintsMode || CHAINHINTSMODE.ENABLED;
  const outputFormat = config.outputFormat || OUTPUTFORMAT.NATURAL_LANGUAGE;
  const orchestratorConfig: OrchestratorConfig = {
    chainHintsMode,
    chainHints: DEFAULT_CHAIN_HINTS,
    aliasToOriginalMap,
  };

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
      ...(activeServer && { server: activeServer }),
    });
  }

  function ensureOrchestrator(): ToolOrchestrator {
    if (orchestratorClosed) {
      orchestrator = buildOrchestrator();
      orchestratorClosed = false;
    }
    return orchestrator;
  }

  const orchestratedExecute = (request: ExecuteRequest): Promise<Result<unknown>> => {
    logger.info({ toolName: request.toolName }, 'orchestratedExecute called in app/index.ts');
    const orch = ensureOrchestrator();
    logger.info('About to call orch.execute');
    return orch.execute(request);
  };

  return {
    /**
     * Configuration values from createApp
     */
    config: {
      chainHintsMode,
      outputFormat,
    },

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
          ...(metadata?.signal && { signal: metadata.signal }),
          ...(metadata?.progress !== undefined && { progress: metadata.progress }),
          ...(metadata?.sendNotification && { sendNotification: metadata.sendNotification }),
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

      // Load knowledge base before starting server
      // This will throw if any built-in packs fail to load
      try {
        loadKnowledgeBase();
      } catch (error) {
        logger.error({ error }, 'Failed to load knowledge base during server startup');
        throw new Error(
          `Server startup failed: ${error instanceof Error ? error.message : String(error)}`,
        );
      }

      ensureOrchestrator();

      const serverOptions: Parameters<typeof createMCPServer>[1] = {
        logger,
        transport: transport.transport,
        name: 'containerization-assist',
        version: '1.0.0',
        outputFormat,
        chainHintsMode,
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
        chainHintsMode,
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
        name: t.name,
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
