/**
 * Application Entry Point
 * Simple functional composition for all use cases
 */

import type { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import type { Logger } from 'pino';
import type { Result, Tool } from '@/types';

import { createLogger } from '@/lib/logger';
import { getAllInternalTools } from '@/exports/tools';
import { createToolRegistry } from './tool-registry';
import { createOrchestrator } from './orchestrator';
import { createMCPServer } from '@/mcp/mcp-server';
import type { OrchestratorConfig } from './orchestrator-types';

/**
 * Transport configuration for MCP server
 */
export interface TransportConfig {
  transport: 'stdio' | 'http';
  port?: number;
  host?: string;
}

/**
 * Application configuration
 */
export interface AppConfig {
  tools?: Tool[];
  sessionTTL?: number;
  policyPath?: string;
  policyEnvironment?: string;
  logger?: Logger;
  maxRetries?: number;
  retryDelay?: number;
}

/**
 * Create the containerization assist application
 */
export function createApp(config: AppConfig = {}): {
  execute: (toolName: string, params: unknown) => Promise<Result<unknown>>;
  startServer: (transport: TransportConfig) => Promise<ReturnType<typeof createMCPServer>>;
  bindToMCP: (server: McpServer) => void;
  listTools: () => Array<{ name: string; description: string }>;
  healthCheck: () => { status: 'healthy'; tools: number; message: string };
  stop: () => Promise<void>;
} {
  const logger = config.logger || createLogger({ name: 'containerization-assist' });
  const tools = config.tools || getAllInternalTools();
  const registry = createToolRegistry([...tools]); // Convert readonly array to mutable

  const orchestratorConfig: OrchestratorConfig = {};
  if (config.sessionTTL !== undefined) orchestratorConfig.sessionTTL = config.sessionTTL;
  if (config.policyPath !== undefined) orchestratorConfig.policyPath = config.policyPath;
  if (config.policyEnvironment !== undefined)
    orchestratorConfig.policyEnvironment = config.policyEnvironment;
  if (config.maxRetries !== undefined) orchestratorConfig.maxRetries = config.maxRetries;
  if (config.retryDelay !== undefined) orchestratorConfig.retryDelay = config.retryDelay;

  const orchestrator = createOrchestrator({
    registry: registry.list().reduce((map, tool) => {
      map.set(tool.name, tool);
      return map;
    }, new Map<string, Tool>()),
    logger,
    config: orchestratorConfig,
  });

  let mcpServer: ReturnType<typeof createMCPServer> | null = null;

  return {
    /**
     * Execute a tool directly
     */
    execute: (toolName: string, params: unknown): Promise<Result<unknown>> =>
      orchestrator.execute({ toolName, params }),

    /**
     * Start MCP server with the specified transport
     */
    startServer: async (transport: TransportConfig) => {
      // Create MCP server with tools from registry
      const serverOptions: {
        logger: Logger;
        transport: 'stdio' | 'http';
        name: string;
        version: string;
        port?: number;
        host?: string;
      } = {
        logger,
        transport: transport.transport,
        name: 'containerization-assist',
        version: '1.0.0',
      };
      if (transport.port !== undefined) serverOptions.port = transport.port;
      if (transport.host !== undefined) serverOptions.host = transport.host;

      mcpServer = createMCPServer(registry.list(), serverOptions);

      await mcpServer.start();
      return mcpServer;
    },

    /**
     * Bind to existing MCP server
     */
    bindToMCP: (server: McpServer) => {
      // Register each tool with the MCP server
      for (const tool of registry.list()) {
        if (!tool.zodSchema || !tool.schema) {
          logger.warn({ tool: tool.name }, 'Tool missing schema, skipping MCP registration');
          continue;
        }

        server.tool(
          tool.name,
          tool.description || `${tool.name} tool`,
          tool.schema,
          async (params: unknown) => {
            const result = await orchestrator.execute({
              toolName: tool.name,
              params,
            });

            if (!result.ok) {
              throw new Error(result.error);
            }

            return {
              content: [
                {
                  type: 'text' as const,
                  text: JSON.stringify(result.value, null, 2),
                },
              ],
            };
          },
        );
      }
    },

    /**
     * List all available tools
     */
    listTools: () =>
      registry.list().map((t) => ({
        name: t.name,
        description: t.description || '',
      })),

    /**
     * Simple health check
     */
    healthCheck: () => {
      const toolCount = registry.list().length;
      return {
        status: 'healthy' as const,
        tools: toolCount,
        message: `${toolCount} tools loaded`,
      };
    },

    /**
     * Stop the server if running
     */
    stop: async () => {
      if (mcpServer) {
        await mcpServer.stop();
        mcpServer = null;
      }
    },
  };
}
