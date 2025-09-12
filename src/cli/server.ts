/**
 * Containerization Assist MCP Server - SDK-Native Entry Point
 * Uses direct SDK patterns with Zod schemas
 */

import { MCPServer } from '../mcp/server';
import {
  createDependencies,
  initializeDependencies,
  shutdownDependencies,
  type Dependencies,
} from '../container';
import process from 'node:process';

async function main(): Promise<void> {
  // Set MCP mode to ensure logs go to stderr, not stdout (prevents JSON-RPC corruption)
  process.env.MCP_MODE = 'true';

  let deps: Dependencies | undefined;
  let server: MCPServer | undefined;

  try {
    // Create dependencies
    deps = createDependencies();
    await initializeDependencies(deps);

    deps.logger.info('Starting SDK-Native MCP Server with DI container');

    // Create and start the SDK-native server with injected dependencies
    server = new MCPServer(deps);
    await server.start();

    deps.logger.info('MCP Server started successfully');

    // Handle graceful shutdown
    const shutdown = async (): Promise<void> => {
      if (deps) {
        deps.logger.info('Shutting down server...');
      }
      try {
        if (server) {
          await server.stop();
        }
        if (deps) {
          await shutdownDependencies(deps);
          deps.logger.info('Server shutdown complete');
        }
        process.exit(0);
      } catch (error) {
        if (deps) {
          deps.logger.error({ error }, 'Error during shutdown');
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

    // Keep the process alive
    process.stdin.resume();
  } catch (error) {
    const errorLogger = deps?.logger ?? console;
    if (typeof errorLogger.error === 'function') {
      errorLogger.error({ error }, 'Failed to start server');
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
