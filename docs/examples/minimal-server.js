#!/usr/bin/env node

/**
 * Minimal MCP server example with Container Assist tools
 * This is the simplest possible working exampleI
 */

import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';

// Import Container Assist API
import { createApp } from '@thgamble/containerization-assist-mcp';

async function main() {
  console.error('Starting MCP server with Container Assist tools...');

  try {
    // Create the MCP server
    const server = new McpServer(
      {
        name: 'containerization-assist-example',
        version: '1.0.0',
      },
      {
        capabilities: {
          tools: {},
        },
      }
    );

    // Create Container Assist app runtime and bind to MCP server
    console.error('Setting up Container Assist tools...');
    const app = createApp();

    // Bind all tools to the MCP server
    app.bindToMCP(server);

    // Create stdio transport and connect
    const transport = new StdioServerTransport();
    await server.connect(transport);

    const tools = app.listTools();
    console.error('✅ MCP server started successfully with Container Assist tools');
    console.error(`Available tools (${tools.length}): ${tools.slice(0, 5).map(t => t.name).join(', ')}...`);

  } catch (error) {
    console.error('Failed to start MCP server:', error);
    process.exit(1);
  }
}

// Handle errors
process.on('unhandledRejection', (error) => {
  console.error('Unhandled rejection:', error);
  process.exit(1);
});

// Run the server
main().catch((error) => {
  console.error('Fatal error:', error);
  process.exit(1);
});