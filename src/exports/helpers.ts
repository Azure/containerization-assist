/**
 * Helper utilities for external MCP tool integration
 */

import { zodToJsonSchema } from 'zod-to-json-schema';
import type { MCPTool, MCPServer } from './types.js';
import { tools } from './tools.js';

/**
 * Register a single tool with any MCP server implementation.
 * Automatically detects the server's registration API and adapts accordingly.
 *
 * @param server - MCP server instance (supports multiple API styles)
 * @param tool - The tool to register
 * @param customName - Optional custom name for the tool (defaults to tool.name)
 * @throws {Error} If the server type is not supported
 *
 * @example
 * ```typescript
 * import { registerTool, tools } from '@thgamble/containerization-assist-mcp';
 *
 * registerTool(server, tools.analyzeRepo, 'custom_analyze');
 * ```
 */
export function registerTool(server: MCPServer | any, tool: MCPTool, customName?: string): void {
  const name = customName || tool.name;

  // Convert Zod schema to JSON Schema if needed
  const isZodSchema = (schema: any): boolean => {
    return schema && typeof schema === 'object' && typeof schema.parse === 'function';
  };

  const inputSchema = isZodSchema(tool.metadata.inputSchema)
    ? zodToJsonSchema(tool.metadata.inputSchema as any)
    : tool.metadata.inputSchema;

  // Try different registration methods based on server API
  if (typeof server.registerTool === 'function') {
    // High-level API (McpServer style)
    server.registerTool(
      name,
      {
        title: tool.metadata.title,
        description: tool.metadata.description,
        inputSchema,
      },
      tool.handler,
    );
  } else if (typeof server.addTool === 'function') {
    // Low-level API (Server style)
    server.addTool(
      {
        name,
        description: tool.metadata.description,
        inputSchema,
      },
      tool.handler,
    );
  } else if (typeof server.setTool === 'function') {
    // Alternative API
    server.setTool(name, {
      description: tool.metadata.description,
      inputSchema,
      handler: tool.handler,
    });
  } else if (server.tools && typeof server.tools.set === 'function') {
    // Direct Map-based registry
    server.tools.set(name, tool);
  } else {
    throw new Error(
      'Unsupported server type. Server must have registerTool, addTool, setTool method, or tools Map',
    );
  }
}

/**
 * Register all available Container Assist tools with an MCP server.
 *
 * @param server - MCP server instance to register tools with
 * @param nameMapping - Optional mapping of original tool names to custom names
 *
 * @example
 * ```typescript
 * import { registerAllTools } from '@thgamble/containerization-assist-mcp';
 *
 * // Register with default names
 * registerAllTools(server);
 *
 * // Register with custom names
 * registerAllTools(server, {
 *   'analyze_repo': 'repository_analyzer',
 *   'build_image': 'docker_build'
 * });
 * ```
 */
export function registerAllTools(
  server: MCPServer | any,
  nameMapping?: Record<string, string>,
): void {
  Object.entries(tools).forEach(([key, tool]) => {
    const customName = nameMapping?.[key];
    registerTool(server, tool, customName);
  });
}

/**
 * Convert Zod schema to JSON Schema format for MCP compatibility.
 *
 * @param zodSchema - Zod schema object to convert
 * @returns JSON Schema representation of the Zod schema
 *
 * @example
 * ```typescript
 * import { z } from 'zod';
 * import { convertZodToJsonSchema } from '@thgamble/containerization-assist-mcp';
 *
 * const schema = z.object({
 *   name: z.string(),
 *   age: z.number().optional()
 * });
 *
 * const jsonSchema = convertZodToJsonSchema(schema);
 * ```
 */
export function convertZodToJsonSchema(zodSchema: any): any {
  return zodToJsonSchema(zodSchema);
}

/**
 * Create a new unique session identifier for workflow tracking.
 *
 * @returns A unique session ID string
 *
 * @example
 * ```typescript
 * import { createSession } from '@thgamble/containerization-assist-mcp';
 *
 * const sessionId = createSession();
 * // Returns: 'session-1234567890-abc123def'
 * ```
 */
export function createSession(): string {
  const timestamp = Date.now();
  const random = Math.random().toString(36).substring(2, 11);
  return `session-${timestamp}-${random}`;
}

// Re-export getAllTools for convenience
export { getAllTools } from './tools.js';
