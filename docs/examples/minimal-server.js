#!/usr/bin/env node

/**
 * Minimal MCP server example with Container Assist tools
 * This is the simplest possible working example
 */

import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';

// Import Container Assist tools
import { createApp } from 'containerization-assist-mcp';

async function main() {
  console.error('Starting MCP server with Container Assist tools...');

  try {
    // Create the MCP server
    const server = new McpServer({
      name: 'containerization-assist-example',
      version: '1.0.0',
    });

    // Create Container Assist app and bind tools
    console.error('Setting up Container Assist tools...');
    const app = createApp({
      toolAliases: {
        'analyze-repo': 'analyze_repo',
        'generate-dockerfile': 'generate_dockerfile',
        'build-image': 'build_image'
      }
    });

    // Bind tools to MCP server
    app.bindToMCP(server);

    // Create stdio transport
    const transport = new StdioServerTransport();

    // Connect server to transport
    await server.connect(transport);

    console.error('✅ MCP server started successfully with Container Assist tools');
    console.error(`Available tools: ${app.listTools().length} total`);

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