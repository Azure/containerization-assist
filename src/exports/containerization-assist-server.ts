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
 * ContainerAssistServer provides a clean API for integrating tools
 * into existing MCP servers without global state
 */
export class ContainerAssistServer {
  private sessionManager: SessionManager;
  private logger: Logger;
  private mcpServer?: Server;
  private tools: Map<string, Tool>;
  private adaptedTools: Map<string, MCPTool>;

  constructor(options: { logger?: Logger } = {}) {
    this.logger = options.logger || createLogger({ name: 'containerization-assist' });
    this.sessionManager = createSessionManager(this.logger);
    this.tools = new Map();
    this.adaptedTools = new Map();

    // Load all internal tools
    this.loadTools();
  }

  /**
   * Load all internal tools
   */
  private loadTools(): void {
    const internalTools = getAllInternalTools();
    for (const tool of internalTools) {
      this.tools.set(tool.name, tool);
    }
  }

  /**
   * Bind to an MCP server and register all tools
   * This is the main entry point for integration
   *
   * @example
   * ```typescript
   * const caServer = new ContainerAssistServer();
   * caServer.bindAll({ server: myMCPServer });
   * ```
   */
  bindAll(config: { server: McpServer }): void {
    this.bindSampling(config);
    this.registerTools(config);
  }

  /**
   * Configure AI sampling capability
   * This allows tools to use the MCP server's sampling features
   */
  bindSampling(config: { server: McpServer }): void {
    // Extract the underlying Server instance from McpServer
    this.mcpServer = config.server.server;
    this.logger.info('AI sampling configured for Container Assist tools');
  }

  /**
   * Register tools with the MCP server
   * Can optionally specify which tools to register
   */
  registerTools(
    config: { server: McpServer },
    options: {
      tools?: string[]; // Specific tools to register
      nameMapping?: Record<string, string>; // Custom names for tools
    } = {},
  ): void {
    const mcpServer = config.server;
    const toolsToRegister = options.tools
      ? Array.from(this.tools.entries()).filter(([name]) => options.tools?.includes(name) ?? false)
      : Array.from(this.tools.entries());

    for (const [originalName, tool] of toolsToRegister) {
      const customName = options.nameMapping?.[originalName] || originalName;
      const mcpTool = this.adaptTool(tool);

      // Use McpServer's public tool() method
      // All our tools have Zod schemas in tool.zodSchema (which is the .shape from the Zod object)
      if (!tool.zodSchema) {
        this.logger.warn({ tool: customName }, 'Tool missing Zod schema, skipping registration');
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
      this.adaptedTools.set(customName, mcpTool);

      this.logger.info(
        {
          originalName,
          registeredAs: customName,
        },
        'Tool registered',
      );
    }
  }

  /**
   * Get an adapted tool by name
   */
  getTool(name: string): MCPTool | undefined {
    return this.adaptedTools.get(name);
  }

  /**
   * Get all registered tools
   */
  getAllTools(): MCPTool[] {
    return Array.from(this.adaptedTools.values());
  }

  /**
   * Create a tool context for execution
   */
  private createContext(params?: unknown): ToolContext {
    const logger = this.logger.child({ context: 'tool-execution' });

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

    const context = createToolContext(this.mcpServer as Server, logger, {
      sessionManager: this.sessionManager,
      maxTokens: 2048,
      stopSequences: ['```', '\n\n```', '\n\n# ', '\n\n---'],
      progress: progressReporter,
    });

    // Handle session creation if needed
    if (params && typeof params === 'object' && 'sessionId' in params) {
      const sessionId = (params as { sessionId?: string }).sessionId;
      if (sessionId) {
        void this.ensureSession(sessionId);
      }
    }

    return context;
  }

  /**
   * Ensure a session exists
   */
  private async ensureSession(sessionId: string): Promise<void> {
    try {
      const session = await this.sessionManager.get(sessionId);
      if (!session) {
        await this.sessionManager.create(sessionId);
      }
    } catch (err) {
      this.logger.warn({ sessionId, error: err }, 'Session management error');
    }
  }

  /**
   * Adapt an internal tool to MCPTool interface
   */
  private adaptTool(tool: Tool): MCPTool {
    return {
      name: tool.name,
      metadata: {
        title: tool.name.replace(/_/g, ' ').replace(/\b\w/g, (l) => l.toUpperCase()),
        description: tool.description || `${tool.name} tool`,
        inputSchema: tool.schema || { type: 'object', properties: {} },
      },
      handler: async (params: unknown) => {
        try {
          const toolLogger = this.logger.child({ tool: tool.name });
          const toolContext = this.createContext(params);

          const result = await tool.execute(
            (params || {}) as Record<string, unknown>,
            toolLogger,
            toolContext,
          );
          return this.formatResult(result);
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
  }

  /**
   * Format tool results consistently
   */
  private formatResult(result: unknown): MCPToolResult {
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
  }
}
