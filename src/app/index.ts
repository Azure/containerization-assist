/**
 * Application Entry Point - AppRuntime Implementation
 * Provides type-safe runtime with dependency injection support
 */

import type { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';

import { createLogger } from '@/lib/logger';
import { getAllInternalTools } from '@/exports/tools';
import type { AllToolTypes, ToolName } from '@/tools';
import { createToolContext } from '@/mcp/context';
import { createMCPServer, registerToolsWithServer, type MCPServer } from '@/mcp/mcp-server';
import type { Tool } from '@/types/tool';
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

/**
 * Widen tool type from Tool<TSchema, TOut> to Tool for runtime registration
 *
 * **Type Safety Rationale:**
 * This helper performs intentional generic type erasure, which is safe because:
 *
 * 1. **Structural Compatibility**: Tool<A, B> and Tool<C, D> have identical runtime structure
 * 2. **Schema Preservation**: The actual Zod schema object is preserved at runtime
 * 3. **Runtime Validation**: Input validation happens via the schema property, not the type parameter
 * 4. **Function Compatibility**: The run function signature is preserved at runtime
 *
 * **Why a helper instead of inline cast:**
 * - Makes the intentional widening explicit and searchable
 * - Enforces that only Tool types can be widened (not arbitrary objects)
 * - Documents the safety rationale in one place
 * - Safer than `as any` or multiple chained casts
 *
 * **TypeScript Limitation:**
 * TypeScript's type system doesn't recognize that Tool<TSchema, TOut> is always
 * assignable to Tool (with default generics) due to generic invariance rules.
 * This is a known limitation when working with generic interfaces.
 */
function widenToolType(tool: AllToolTypes): Tool {
  // Type assertion via unknown is necessary due to TypeScript's generic variance rules
  // We constrain the input to AllToolTypes to ensure only valid tools are widened
  return tool as unknown as Tool;
}

/**
 * Apply tool aliases to create renamed versions of tools
 */
function applyToolAliases(
  tools: readonly AllToolTypes[],
  aliases?: Record<string, string>,
): AllToolTypes[] {
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
  const tools = config.tools || getAllInternalTools();
  const aliasedTools = applyToolAliases(tools, config.toolAliases);

  // Widen tool types for registration - see widenToolType() documentation for safety rationale
  const registryTools: Tool[] = aliasedTools.map(widenToolType);

  const toolsMap = new Map<string, Tool>();
  for (const tool of registryTools) {
    toolsMap.set(tool.name, tool);
  }

  const orchestratorConfig: OrchestratorConfig = {};
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
      contextFactory: ({ request, sessionFacade, logger: toolLogger, sessionManager }) => {
        const metadata = request.metadata;
        const server = activeServer;
        if (server) {
          const contextOptions = {
            sessionManager,
            session: sessionFacade,
            ...(metadata?.signal && { signal: metadata.signal }),
            ...(metadata?.progress !== undefined && { progress: metadata.progress }),
            ...(metadata?.maxTokens !== undefined && { maxTokens: metadata.maxTokens }),
            ...(metadata?.stopSequences && { stopSequences: metadata.stopSequences }),
          };
          return createToolContext(server, toolLogger, contextOptions);
        }

        return createHostlessToolContext(toolLogger, {
          sessionManager,
          sessionFacade,
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
  };
}
