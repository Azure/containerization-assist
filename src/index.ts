/**
 * Main export file for external tool consumption
 * Provides tools, helpers, and types for integration with MCP servers
 */

// Re-export server for backwards compatibility
export * from './mcp/server.js';

// Note: Individual tool exports have been removed in favor of ContainerAssistServer.
// Use ContainerAssistServer for proper tool integration:
//
// import { ContainerAssistServer } from '@thgamble/containerization-assist-mcp';
// const caServer = new ContainerAssistServer();
// caServer.bindAll({ server: yourMCPServer });

// Export helper functions
export { registerTool, registerAllTools, toJsonSchema } from './exports/helpers.js';

// Export the new clean API
export { ContainerAssistServer } from './exports/containerization-assist-server.js';

// Export types for external use
export type { MCPTool, MCPToolMetadata, MCPToolResult, MCPServer } from './exports/types.js';

// Re-export core types
export type { Tool, Result, Success, Failure } from './types.js';
