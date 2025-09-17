/**
 * Clean, idiomatic TypeScript API for Container Assist MCP tools
 */

// Primary functional API
export { createContainerAssist as default } from './exports/container-assist.js';
export { createContainerAssist, type ContainerAssist } from './exports/container-assist.js';

// Tool names and types for type-safe registration
export { TOOLS, type ToolName } from './exports/tools.js';

// Core types for external usage
export type { MCPTool, MCPToolResult } from './exports/types.js';
export type { Tool, Result, Success, Failure } from './types.js';
