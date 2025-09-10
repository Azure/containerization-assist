/**
 * ContainerAssistServer - Clean API for integrating Container Assist tools
 * Eliminates global state by using an instance-based approach
 */

import type { Server } from '@modelcontextprotocol/sdk/server/index.js';
import type { Tool } from '../types.js';
import type { MCPTool, MCPToolResult } from './types.js';
import type { ToolContext } from '../mcp/context/types.js';
import type { Logger } from 'pino';

import { createSessionManager, type SessionManager } from '../lib/session.js';
import { createLogger } from '../lib/logger.js';
import { StandardToolContext } from '../mcp/context/tool-context.js';

// Import all tools
import { getAllInternalTools } from './tools.js';

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
  bindAll(config: { server: Server }): void {
    this.bindSampling(config);
    this.registerTools(config);
  }

  /**
   * Configure AI sampling capability
   * This allows tools to use the MCP server's sampling features
   */
  bindSampling(config: { server: Server }): void {
    this.mcpServer = config.server;
    this.logger.info('AI sampling configured for Container Assist tools');
  }

  /**
   * Register tools with the MCP server
   * Can optionally specify which tools to register
   */
  registerTools(
    config: { server: Server },
    options: {
      tools?: string[]; // Specific tools to register
      nameMapping?: Record<string, string>; // Custom names for tools
    } = {},
  ): void {
    const server = config.server;
    const toolsToRegister = options.tools
      ? Array.from(this.tools.entries()).filter(([name]) => options.tools!.includes(name))
      : Array.from(this.tools.entries());

    for (const [originalName, tool] of toolsToRegister) {
      const customName = options.nameMapping?.[originalName] || originalName;
      const mcpTool = this.adaptTool(tool);

      // Register tool with the server
      if (typeof (server as any).registerTool === 'function') {
        // High-level API
        (server as any).registerTool(customName, mcpTool.metadata, mcpTool.handler);
      } else if (typeof (server as any).addTool === 'function') {
        // Low-level API
        (server as any).addTool(
          {
            name: customName,
            description: mcpTool.metadata.description,
            inputSchema: mcpTool.metadata.inputSchema,
          },
          mcpTool.handler,
        );
      } else {
        this.logger.warn(
          { tool: customName },
          'Server does not have registerTool or addTool method',
        );
      }

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
  private createContext(params?: { sessionId?: string }): ToolContext {
    const logger = this.logger.child({ context: 'tool-execution' });

    const context = new StandardToolContext(
      this.mcpServer as any,
      logger,
      undefined,
      undefined,
      undefined,
      {
        debug: false,
        defaultTimeout: 30000,
        defaultMaxTokens: 2048,
        defaultStopSequences: ['```', '\n\n```', '\n\n# ', '\n\n---'],
      },
      this.sessionManager,
    );

    // Simple progress reporter
    context.progress = async (message: string, progress?: number, total?: number) => {
      if (progress !== undefined && total !== undefined) {
        logger.info({ progress, total }, message);
      } else {
        logger.info(message);
      }
    };

    // Handle session creation if needed
    if (params?.sessionId) {
      void this.ensureSession(params.sessionId);
    }

    return context;
  }

  /**
   * Ensure a session exists
   */
  private async ensureSession(sessionId: string): Promise<void> {
    try {
      const session = await this.sessionManager.getSession(sessionId);
      if (!session.ok) {
        await this.sessionManager.createSession(sessionId);
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
      handler: async (params: any) => {
        try {
          const toolLogger = this.logger.child({ tool: tool.name });
          const toolContext = this.createContext(params);

          const result = await tool.execute(params || {}, toolLogger, toolContext);
          return this.formatResult(result);
        } catch (error) {
          return {
            content: [
              {
                type: 'text',
                text: `Error executing ${tool.name}: ${
                  error instanceof Error ? error.message : String(error)
                }`,
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
  private formatResult(result: any): MCPToolResult {
    // Handle Result<T> pattern
    if (result && typeof result === 'object' && 'ok' in result) {
      if (result.ok) {
        const value = result.value;

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
              text: `Error: ${result.error}`,
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
