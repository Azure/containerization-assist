/**
 * Containerization Assist MCP Server - Direct Entry Point
 * Uses the simplified app architecture
 */

import { createApp } from '@/app';
import { createLogger } from '@/lib/logger';
import process from 'node:process';

async function main(): Promise<void> {
  // Set MCP mode to ensure logs go to stderr, not stdout (prevents JSON-RPC corruption)
  process.env.MCP_MODE = 'true';

  const logger = createLogger({
    name: 'mcp-server',
    level: process.env.LOG_LEVEL || 'info',
  });

  let app: ReturnType<typeof createApp> | undefined;

  try {
    logger.info('Starting Containerization Assist MCP Server');

    // Create the application
    app = createApp({
      logger,
      policyPath: process.env.POLICY_PATH || 'config/policy.yaml',
      policyEnvironment: process.env.NODE_ENV || 'production',
    });

    // Start the server with stdio transport
    await app.startServer({
      transport: 'stdio',
    });

    logger.info('MCP Server started successfully with stdio transport');

    // Handle graceful shutdown
    const shutdown = async (signal: string): Promise<void> => {
      logger.info({ signal }, 'Shutting down server');

      try {
        if (app) {
          await app.stop();
        }
        logger.info('Server stopped successfully');
        process.exit(0);
      } catch (error) {
        logger.error({ error }, 'Error during shutdown');
        process.exit(1);
      }
    };

    // Register signal handlers
    process.on('SIGTERM', () => shutdown('SIGTERM'));
    process.on('SIGINT', () => shutdown('SIGINT'));
  } catch (error) {
    logger.fatal({ error }, 'Failed to start server');
    process.exit(1);
  }
}

// Handle uncaught errors
process.on('uncaughtException', (error) => {
  console.error('Uncaught exception:', error);
  process.exit(1);
});

process.on('unhandledRejection', (reason) => {
  console.error('Unhandled rejection:', reason);
  process.exit(1);
});

// Run the server
void main();
