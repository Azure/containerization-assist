/**
 * Clean, idiomatic TypeScript API for Container Assist
 * Provides strongly-typed runtime with dependency injection support
 */

// Primary API - AppRuntime with type safety and dependency injection
/** @public */
export { createApp } from './app/index.js';
/** @public */
export type { TransportConfig } from './app/index.js';

// New AppRuntime interface with precise typing
/** @public */
export type {
  AppRuntime,
  AppRuntimeConfig,
  ToolInputMap,
  ToolResultMap,
  ExecutionMetadata,
  CreateAppRuntime,
} from './types/runtime.js';

// Tool names and types
/** @public */
export {
  TOOLS,
  getAllInternalTools,
  ALL_TOOLS,
  type ToolName,
  type AllToolTypes,
} from './exports/tools.js';

// MCP Server types for cross-version compatibility
/** @public */
export type { McpServerLike } from './mcp/mcp-server.js';

// Core types
/** @public */
export type { Tool, Result, Success, Failure } from './types/index.js';
