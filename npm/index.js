#!/usr/bin/env node

/**
 * Container Kit MCP Server - Main Entry Point
 * This file provides backward compatibility and MCP server startup
 */

import containerKit from './lib/index.js';

// If running as MCP server (default), start STDIO transport
if (process.argv.length === 2 || process.argv[2] === '--stdio') {
  console.error('Starting Containerization Assist MCP Server...');
  console.error('Use the containerization-assist-mcp binary directly for full functionality');
  process.exit(1);
}

// Export the library
export default containerKit;