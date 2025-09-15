/**
 * Main export file for external tool consumption
 * Provides tools, helpers, and types for integration with MCP servers
 */

// MCP Server functionality
export * from './mcp/server.js';

// Tool registration helpers
export { registerTool, registerAllTools, toJsonSchema } from './exports/helpers.js';

// Main server factory for programmatic usage
export { createContainerAssistServer } from './exports/containerization-assist-server.js';
export type { IContainerAssistServer } from './exports/containerization-assist-server.js';

// Tool names for type-safe registration
export { TOOL_NAMES } from './exports/tools.js';
export type { ToolName } from './exports/tools.js';

// Core types needed for external usage
export type { MCPTool, MCPToolMetadata, MCPToolResult, MCPServer } from './exports/types';
export type { Tool, Result, Success, Failure } from './types';
