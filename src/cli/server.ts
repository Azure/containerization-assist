/**
 * Containerization Assist MCP Server - SDK-Native Entry Point
 * Uses Application Kernel for tool execution
 */

import { createMCPServer, type IMCPServer } from '@/mcp/server';
import { createKernel, type RegisteredTool } from '@/app/kernel';
import { getAllInternalTools } from '@/exports/tools';
import { createLogger } from '@/lib/logger';
import process from 'node:process';

async function main(): Promise<void> {
  // Set MCP mode to ensure logs go to stderr, not stdout (prevents JSON-RPC corruption)
  process.env.MCP_MODE = 'true';

  const logger = createLogger({
    name: 'mcp-server',
    level: process.env.LOG_LEVEL || 'info',
  });

  let server: IMCPServer | undefined;

  try {
    // Session manager not needed - kernel creates its own

    // Load and register tools
    const tools = getAllInternalTools();
    const registeredTools = new Map<string, RegisteredTool>();

    for (const tool of tools) {
      registeredTools.set(tool.name, {
        name: tool.name,
        description: tool.description || '',
        handler: async (params: unknown) => {
          // Tools execute directly with their own context
          const toolLogger = logger.child({ tool: tool.name });
          return await tool.execute(
            params as Record<string, unknown>,
            toolLogger,
            undefined as any,
          );
        },
        schema: tool.zodSchema as any,
      });
    }

    // Create kernel
    const kernel = await createKernel(
      {
        sessionStore: 'memory',
        sessionTTL: 3600000,
        maxRetries: 2,
        retryDelay: 1000,
        policyPath: process.env.POLICY_PATH || 'config/policy.yaml',
        policyEnvironment: process.env.NODE_ENV || 'production',
        telemetryEnabled: true,
      },
      registeredTools,
    );

    logger.info('Starting SDK-Native MCP Server with Application Kernel');

    // Create and start the SDK-native server with kernel
    server = createMCPServer(kernel, {
      logger,
      name: 'containerization-assist',
      version: '1.0.0',
    });
    await server.start();

    logger.info('MCP Server started successfully');

    // Handle graceful shutdown
    const shutdown = async (): Promise<void> => {
      logger.info('Shutting down server...');
      try {
        if (server) {
          await server.stop();
        }
        logger.info('Server shutdown complete');
        process.exit(0);
      } catch (error) {
        logger.error({ error }, 'Error during shutdown');
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

    // Keep the process alive
    process.stdin.resume();
  } catch (error) {
    logger.error({ error }, 'Failed to start server');
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
