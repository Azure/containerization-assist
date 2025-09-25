/**
 * Application Entry Point
 * Simple functional composition for all use cases
 */

import type { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import type { Logger } from 'pino';
import type { Result } from '@/types';
import type { Tool } from '@/types/tool';
import type { ZodTypeAny } from 'zod';

import { createLogger } from '@/lib/logger';
import { extractSchemaShape } from '@/lib/zod-utils';
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
  tools?: Array<Tool<ZodTypeAny, any>>;
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
  const registry = createToolRegistry([...tools] as Tool<ZodTypeAny, any>[]); // Convert readonly array to mutable

  const orchestratorConfig: OrchestratorConfig = {};
  if (config.sessionTTL !== undefined) orchestratorConfig.sessionTTL = config.sessionTTL;
  if (config.policyPath !== undefined) orchestratorConfig.policyPath = config.policyPath;
  if (config.policyEnvironment !== undefined)
    orchestratorConfig.policyEnvironment = config.policyEnvironment;
  if (config.maxRetries !== undefined) orchestratorConfig.maxRetries = config.maxRetries;
  if (config.retryDelay !== undefined) orchestratorConfig.retryDelay = config.retryDelay;

  let mcpServer: ReturnType<typeof createMCPServer> | null = null;
  let orchestrator: ReturnType<typeof createOrchestrator> | null = null;

  return {
    /**
     * Execute a tool directly
     */
    execute: async (toolName: string, params: unknown): Promise<Result<unknown>> => {
      if (!orchestrator) {
        // If orchestrator doesn't exist yet, we need to create an MCP server first
        if (!mcpServer) {
          mcpServer = createMCPServer(registry.list(), {
            logger,
            transport: 'stdio',
            name: 'containerization-assist',
            version: '1.0.0',
          });
          await mcpServer.start();
        }

        // Now create the orchestrator with the MCP server
        orchestrator = createOrchestrator({
          registry: registry.list().reduce((map, tool) => {
            map.set(tool.name, tool);
            return map;
          }, new Map<string, Tool<ZodTypeAny, any>>()),
          server: mcpServer.getServer(),
          logger,
          config: orchestratorConfig,
        });
      }
      return orchestrator.execute({ toolName, params });
    },

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

      // Create orchestrator now that we have an MCP server
      orchestrator = createOrchestrator({
        registry: registry.list().reduce((map, tool) => {
          map.set(tool.name, tool);
          return map;
        }, new Map<string, Tool<any, any>>()),
        server: mcpServer.getServer(),
        logger,
        config: orchestratorConfig,
      });

      return mcpServer;
    },

    /**
     * Bind to existing MCP server
     */
    bindToMCP: (server: McpServer) => {
      // Create orchestrator with the provided server if not already created
      if (!orchestrator) {
        // Extract the underlying SDK Server from McpServer
        const sdkServer = (server as any).server as Server;
        orchestrator = createOrchestrator({
          registry: registry.list().reduce((map, tool) => {
            map.set(tool.name, tool);
            return map;
          }, new Map<string, Tool<ZodTypeAny, any>>()),
          server: sdkServer,
          logger,
          config: orchestratorConfig,
        });
      }

      // Register each tool with the MCP server
      for (const tool of registry.list()) {
        // Get the schema shape for MCP protocol
        const schema = extractSchemaShape(tool.schema);
        const description = tool.description;

        if (!schema || Object.keys(schema).length === 0) {
          logger.warn({ tool: tool.name }, 'Tool missing schema shape, using empty schema');
        }

        server.tool(tool.name, description, schema, async (params: unknown) => {
          const result = await orchestrator!.execute({
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
        });
      }
    },

    /**
     * List all available tools
     */
    listTools: () =>
      registry.list().map((t) => ({
        name: t.name,
        description: t.description,
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
