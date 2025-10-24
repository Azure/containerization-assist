/**
 * MCP Server Integration with Telemetry
 *
 * This example shows how to integrate Container Assist tools with an MCP server
 * while maintaining full control over telemetry, error handling, and lifecycle hooks.
 *
 * Use this pattern when you need:
 * - Custom telemetry tracking for tool executions
 * - Error reporting and monitoring
 * - Custom logging and observability
 * - Fine-grained control over tool registration
 */

import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import {
  createApp,
  ALL_TOOLS,
  createToolHandler,
  registerTools,
  type ToolHandlerOptions,
} from 'containerization-assist';

/**
 * Example telemetry service
 * Replace this with your actual telemetry provider
 */
class TelemetryService {
  trackToolExecution(toolName: string, success: boolean, durationMs?: number) {
    console.log(`[TELEMETRY] Tool: ${toolName}, Success: ${success}, Duration: ${durationMs}ms`);
    // Send to your telemetry backend (e.g., Application Insights, DataDog, etc.)
  }

  trackError(toolName: string, error: unknown) {
    console.error(`[TELEMETRY] Tool ${toolName} failed:`, error);
    // Send to your error tracking service
  }
}

const telemetry = new TelemetryService();

/**
 * Option 1: Use createToolHandler for maximum control
 *
 * This gives you full control over each tool registration
 * and allows per-tool customization of telemetry hooks.
 */
function registerWithMaximumControl(server: McpServer) {
  const app = createApp({
    outputFormat: 'natural-language',
    chainHintsMode: 'enabled',
  });

  // Register each tool individually with custom telemetry
  for (const tool of ALL_TOOLS) {
    const startTime = Date.now();

    server.tool(
      tool.name,
      tool.description,
      tool.inputSchema,
      createToolHandler(app, tool.name, {
        transport: 'my-integration',

        // Track successful executions
        onSuccess: (result, toolName, params) => {
          const duration = Date.now() - startTime;
          telemetry.trackToolExecution(toolName, true, duration);
          console.log(`[SUCCESS] ${toolName} executed successfully in ${duration}ms`);
        },

        // Track errors
        onError: (error, toolName, params) => {
          const duration = Date.now() - startTime;
          telemetry.trackToolExecution(toolName, false, duration);
          telemetry.trackError(toolName, error);
          console.error(`[ERROR] ${toolName} failed after ${duration}ms:`, error);
        },
      }),
    );
  }

  console.log(`✅ Registered ${ALL_TOOLS.length} tools with custom telemetry`);
}

/**
 * Option 2: Use registerTools for convenience
 *
 * This is simpler when you want the same telemetry configuration
 * for all tools.
 */
function registerWithConvenience(server: McpServer) {
  const app = createApp({
    outputFormat: 'natural-language',
  });

  // Register all tools at once with shared telemetry configuration
  registerTools(server, app, ALL_TOOLS, {
    transport: 'my-integration',

    onSuccess: (result, toolName, params) => {
      telemetry.trackToolExecution(toolName, true);
    },

    onError: (error, toolName, params) => {
      telemetry.trackToolExecution(toolName, false);
      telemetry.trackError(toolName, error);
    },
  });

  console.log(`✅ Registered ${ALL_TOOLS.length} tools with shared telemetry`);
}

/**
 * Option 3: Per-tool telemetry with different configurations
 *
 * Use different telemetry settings for different categories of tools
 */
function registerWithPerToolConfig(server: McpServer) {
  const app = createApp();

  // Critical tools with detailed telemetry
  const criticalTools = ALL_TOOLS.filter((t) =>
    ['build-image', 'deploy', 'scan-image'].includes(t.name),
  );

  for (const tool of criticalTools) {
    server.tool(
      tool.name,
      tool.description,
      tool.inputSchema,
      createToolHandler(app, tool.name, {
        transport: 'critical-path',
        onSuccess: (result, toolName) => {
          telemetry.trackToolExecution(toolName, true);
          // Send detailed metrics for critical tools
          console.log(`[CRITICAL] ${toolName} completed:`, result);
        },
        onError: (error, toolName) => {
          telemetry.trackToolExecution(toolName, false);
          telemetry.trackError(toolName, error);
          // Alert on critical tool failures
          console.error(`[ALERT] Critical tool ${toolName} failed!`, error);
        },
      }),
    );
  }

  // Other tools with basic telemetry
  const otherTools = ALL_TOOLS.filter(
    (t) => !criticalTools.includes(t),
  );

  registerTools(server, app, otherTools, {
    onSuccess: (result, toolName) => telemetry.trackToolExecution(toolName, true),
    onError: (error, toolName) => telemetry.trackToolExecution(toolName, false),
  });

  console.log(
    `✅ Registered ${criticalTools.length} critical tools and ${otherTools.length} standard tools`,
  );
}

/**
 * Main server setup
 */
async function main() {
  const server = new McpServer({
    name: 'containerization-assist-with-telemetry',
    version: '1.0.0',
  });

  // Choose your registration strategy:

  // Option 1: Maximum control (recommended for complex telemetry)
  registerWithMaximumControl(server);

  // Option 2: Convenience (recommended for simple telemetry)
  // registerWithConvenience(server);

  // Option 3: Per-tool configuration (recommended for mixed requirements)
  // registerWithPerToolConfig(server);

  // Start the server
  const transport = new StdioServerTransport();
  await server.connect(transport);

  console.log('🚀 Container Assist MCP server started with telemetry integration');
}

// Handle graceful shutdown
process.on('SIGINT', () => {
  console.log('\n👋 Shutting down server...');
  process.exit(0);
});

// Start the server
main().catch((error) => {
  console.error('Failed to start server:', error);
  process.exit(1);
});
