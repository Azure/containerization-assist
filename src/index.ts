/**
 * Main export file for external tool consumption
 * Provides tools, helpers, and types for integration with MCP servers
 */

export * from './mcp/server.js';

export { registerTool, registerAllTools, toJsonSchema } from './exports/helpers.js';

export { ContainerAssistServer } from './exports/containerization-assist-server.js';

export type { MCPTool, MCPToolMetadata, MCPToolResult, MCPServer } from './exports/types.js';

export type { Tool, Result, Success, Failure } from './types.js';
