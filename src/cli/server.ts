/**
 * Containerization Assist MCP Server - SDK-Native Entry Point
 * Uses direct SDK patterns with Zod schemas
 */

import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import { config } from '@config/index';
import { createLogger } from '@lib/logger';
import { createSessionManager } from '@lib/session';
import { getToolRegistry } from '@mcp/tools/registry';
import { initializePrompts } from '@/prompts/prompt-registry';
import * as promptRegistry from '@/prompts/prompt-registry';
import * as resourceManager from '@/resources/manager';
import { createToolContext } from '@mcp/context';
import { createToolRouter } from '@mcp/tool-router';
import process from 'node:process';

async function createServer(server: McpServer, transport: StdioServerTransport): Promise<void> {
  await server.connect(transport);
}

async function main(): Promise<void> {
  // Set MCP mode to ensure logs go to stderr, not stdout (prevents JSON-RPC corruption)
  process.env.MCP_MODE = 'true';

  let logger: ReturnType<typeof createLogger> | undefined;
  let sessionManager: ReturnType<typeof createSessionManager> | undefined;
  let server: McpServer | undefined;

  try {
    // Create dependencies inline
    logger = createLogger({
      name: config.mcp.name,
      level: config.server.logLevel,
    });

    sessionManager = createSessionManager(logger, {
      ttl: config.session.ttl,
      maxSessions: config.session.maxSessions,
      cleanupIntervalMs: config.session.cleanupInterval,
    });

    // Initialize prompt registry (directory param ignored - uses embedded prompts)
    await initializePrompts('', logger);
    logger.info('Prompts initialized successfully');

    const tools = getToolRegistry();
    logger.info(`Tool registry loaded with ${tools.size} tools`);

    logger.info('Starting SDK-Native MCP Server');

    // Create the MCP SDK server instance
    server = new McpServer(
      {
        name: config.mcp.name,
        version: '1.4.2',
      },
      {
        capabilities: {
          tools: {},
        },
      },
    );

    // Create router for tool execution
    const router = createToolRouter({
      sessionManager,
      logger,
      tools,
    });

    // Register tools directly with MCP SDK server
    for (const [toolName, toolDef] of tools) {
      if (!toolDef.schema) {
        logger.warn({ tool: toolName }, 'Tool missing schema, skipping registration');
        continue;
      }

      server.tool(
        toolName,
        `${toolName} tool`,
        (toolDef.schema as any)?.shape || {},
        async (args: unknown) => {
          try {
            if (!logger || !server) {
              throw new Error('Logger or server not initialized');
            }
            const toolLogger = logger.child({ tool: toolName });
            const context = createToolContext(server.server, toolLogger, {
              sessionManager,
              promptRegistry,
              maxTokens: 2048,
              stopSequences: ['```', '\n\n```', '\n\n# ', '\n\n---'],
            });

            // Extract session info from params
            const paramsObj = (args || {}) as Record<string, unknown>;
            const sessionId = (paramsObj.sessionId as string) || `session-${Date.now()}`;

            // Use router for execution
            const result = await router.route({
              toolName,
              params: paramsObj,
              sessionId,
              context,
            });

            if (result.ok) {
              return {
                content: [
                  {
                    type: 'text' as const,
                    text: JSON.stringify(result.value, null, 2),
                  },
                ],
              };
            } else {
              throw new Error(result.error || `Tool "${toolName}" failed`);
            }
          } catch (error) {
            logger?.error({ error, tool: toolName }, 'Tool execution failed');
            throw error;
          }
        },
      );
    }

    logger.info('MCP Server tools registered successfully');

    // Wire up stdio transport for JSON-RPC
    const transport = new StdioServerTransport();
    await createServer(server, transport);

    logger.info('Connected to stdio transport');
    logger.info('MCP Server created successfully');

    // Keep process alive
    process.stdin.resume();

    // Handle graceful shutdown
    const shutdown = async (): Promise<void> => {
      if (logger) {
        logger.info('Shutting down server...');
      }
      try {
        // Close the MCP server
        if (server) {
          await server.close();
        }
        if (sessionManager && 'close' in sessionManager) {
          sessionManager.close();
        }
        if (resourceManager && 'cleanup' in resourceManager) {
          await resourceManager.cleanup();
        }
        if (logger) {
          logger.info('Server shutdown complete');
        }
        process.exit(0);
      } catch (error) {
        if (logger) {
          logger.error({ error }, 'Error during shutdown');
        } else {
          console.error('Error during shutdown:', error);
        }
        process.exit(1);
      }
    };

    process.on('SIGINT', () => {
      void shutdown();
    });
    process.on('SIGTERM', () => {
      void shutdown();
    });
    process.on('SIGQUIT', () => {
      void shutdown();
    });
  } catch (error) {
    if (logger) {
      logger.error({ error }, 'Failed to start server');
    } else {
      console.error('Failed to start server:', error);
    }
    process.exit(1);
  }
}

// Start the server
if (import.meta.url === `file://${process.argv[1]}`) {
  main().catch((error) => {
    console.error('Unhandled error:', error);
    process.exit(1);
  });
}
