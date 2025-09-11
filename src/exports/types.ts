/**
 * Type definitions for external MCP tool consumption
 */

import { z } from 'zod';

/**
 * MCP tool metadata structure
 */
export interface MCPToolMetadata {
  title: string;
  description: string;
  inputSchema: z.ZodType<unknown> | Record<string, unknown>;
}

/**
 * MCP tool result structure
 */
export interface MCPToolResult {
  content: Array<{
    type: string;
    text?: string;
  }>;
}

/**
 * MCP tool definition for external consumption
 */
export interface MCPTool {
  name: string;
  metadata: MCPToolMetadata;
  handler: (params: unknown) => Promise<MCPToolResult>;
}

/**
 * MCP server interface supporting multiple registration styles
 */
export interface MCPServer {
  // High-level API (McpServer style)
  registerTool?(
    name: string,
    metadata: {
      title: string;
      description: string;
      inputSchema: Record<string, unknown>;
    },
    handler: (params: unknown) => Promise<MCPToolResult>,
  ): void;

  // Low-level API (Server style from @modelcontextprotocol/sdk)
  addTool?(
    definition: {
      name: string;
      description: string;
      inputSchema: unknown;
    },
    handler: (params: unknown) => Promise<MCPToolResult>,
  ): void;

  // Alternative registration method
  setTool?(
    name: string,
    tool: {
      description: string;
      inputSchema?: unknown;
      handler: (params: unknown) => Promise<MCPToolResult>;
    },
  ): void;
}
