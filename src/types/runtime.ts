/**
 * AppRuntime Types - Precise Typing for Application Runtime
 *
 * Provides strongly typed interfaces for the application runtime,
 * enabling type-safe tool execution and dependency injection hooks.
 */

import type { ZodTypeAny } from 'zod';
import type { Logger } from 'pino';
import type { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import type { Result } from './core';
import type { TransportConfig } from '@/app';
import type { MCPServer } from '@/mcp/mcp-server';
import type { Tool, ToolName } from '@/tools';

// Extract input/output types from tool registry
type ExtractToolInput<T extends { schema: ZodTypeAny }> = T['schema'] extends ZodTypeAny
  ? import('zod').infer<T['schema']>
  : never;
type ExtractToolOutput<T> = T extends { run: (...args: never[]) => Promise<Result<infer R>> }
  ? R
  : never;

// Map tool names to their input/output types
export type ToolInputMap = {
  [K in ToolName]: K extends Tool['name'] ? ExtractToolInput<Extract<Tool, { name: K }>> : never;
};

export type ToolResultMap = {
  [K in ToolName]: K extends Tool['name'] ? ExtractToolOutput<Extract<Tool, { name: K }>> : never;
};

/**
 * Tool execution context metadata
 */
export interface ExecutionMetadata {
  /** Session ID for the execution */
  sessionId?: string;

  /** Transport type (stdio, http, programmatic) */
  transport?: string;

  /** Request ID for tracing */
  requestId?: string;

  /** Additional metadata */
  [key: string]: unknown;
}

/**
 * Strongly typed AppRuntime interface with dependency injection support
 */
export interface AppRuntime {
  /**
   * Execute a tool with type-safe parameters and results
   */
  execute<T extends ToolName>(
    toolName: T,
    params: ToolInputMap[T],
    metadata?: ExecutionMetadata,
  ): Promise<Result<ToolResultMap[T]>>;

  /**
   * List all available tools with their metadata
   */
  listTools(): Array<{
    name: ToolName;
    description: string;
    version?: string;
    category?: string;
  }>;

  /**
   * Start MCP server with specified transport
   */
  startServer(transport: TransportConfig): Promise<MCPServer>;

  /**
   * Bind to existing MCP server instance
   */
  bindToMCP(server: McpServer, transportLabel?: string): void;

  /**
   * Perform health check
   */
  healthCheck(): {
    status: 'healthy' | 'unhealthy';
    tools: number;
    sessions?: number;
    message: string;
  };

  /**
   * Stop the runtime and clean up resources
   */
  stop(): Promise<void>;
}

/**
 * Runtime factory configuration
 */
export interface AppRuntimeConfig {
  /** Logger instance for runtime operations (set at creation time, not reconfigurable) */
  logger?: Logger;

  /** Custom tools to register */
  tools?: Array<Tool>;

  /** Tool name aliases */
  toolAliases?: Record<string, string>;

  /** Policy file path (static configuration) */
  policyPath?: string;

  /** Policy environment (development/production) */
  policyEnvironment?: string;

  /** Enable hints that suggest other tools to call next in tool responses */
  chainHintsMode?: 'enabled' | 'disabled';
}

/**
 * Factory function signature for creating AppRuntime instances
 */
export type CreateAppRuntime = (config?: AppRuntimeConfig) => AppRuntime;

/**
 * Utility type to extract tool names from the registry
 */
export type ValidToolName = ToolName;

/**
 * Utility type for tool execution results
 */
export type ToolExecutionResult<T extends ToolName> = Promise<Result<ToolResultMap[T]>>;
