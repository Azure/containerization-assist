/**
 * Type-safe MCP tool registration helpers
 * Minimal wrapper around MCP SDK's native registration
 */

import type { z } from 'zod';
import { zodToJsonSchema } from 'zod-to-json-schema';
import type { MCPTool } from '../mcp/types';

// Minimal server surface types for MCP SDK compatibility
export interface ServerRegisterTool {
  tool(
    name: string,
    description: string,
    schema: unknown,
    handler: (params: unknown) => Promise<unknown>,
  ): void;
}

export interface ServerAddTool {
  addTool(
    name: string,
    description: string,
    schema: unknown,
    handler: (params: unknown) => Promise<unknown>,
  ): void;
}

export interface ServerMapTools {
  tools: Map<
    string,
    {
      description: string;
      inputSchema: unknown;
      handler: (params: unknown) => Promise<unknown>;
    }
  >;
}

// Type guards for different server interfaces
export function hasToolMethod(server: unknown): server is ServerRegisterTool {
  return typeof server === 'object' && server !== null && 'tool' in server;
}

export function hasAddToolMethod(server: unknown): server is ServerAddTool {
  return typeof server === 'object' && server !== null && 'addTool' in server;
}

export function hasToolsMap(server: unknown): server is ServerMapTools {
  return typeof server === 'object' && server !== null && 'tools' in server;
}

/**
 * Convert Zod schema to JSON Schema format
 */
export function toJsonSchema<T extends z.ZodTypeAny>(schema: T): unknown {
  const jsonSchema = zodToJsonSchema(schema, { $refStrategy: 'none' });
  // Remove $schema property as MCP doesn't need it
  if (typeof jsonSchema === 'object' && jsonSchema !== null && '$schema' in jsonSchema) {
    const { $schema: _, ...rest } = jsonSchema as Record<string, unknown>;
    return rest;
  }
  return jsonSchema;
}

/**
 * Register a single tool with type safety
 * Adapts to different MCP server implementations
 */
export function registerTool<Schema extends z.ZodTypeAny, Out>(
  server: unknown,
  tool: MCPTool<Schema, Out>,
  customName?: string,
): void {
  const name = customName || tool.name;
  const description = tool.description || `${name} tool`;
  const jsonSchema = toJsonSchema(tool.inputSchema);

  // Wrap handler to parse with zod and handle errors
  const wrappedHandler = async (params: unknown): Promise<unknown> => {
    const parsed = tool.inputSchema.safeParse(params);
    if (!parsed.success) {
      return {
        content: [
          {
            type: 'text',
            text: `Invalid parameters: ${parsed.error.message}`,
          },
        ],
        error: parsed.error.message,
      };
    }
    return tool.handler(parsed.data);
  };

  // Try different server interfaces
  if (hasToolMethod(server)) {
    server.tool(name, description, jsonSchema, wrappedHandler);
  } else if (hasAddToolMethod(server)) {
    server.addTool(name, description, jsonSchema, wrappedHandler);
  } else if (hasToolsMap(server)) {
    server.tools.set(name, {
      description,
      inputSchema: jsonSchema,
      handler: wrappedHandler,
    });
  } else {
    throw new Error('Server does not support any known tool registration method');
  }
}

/**
 * Register multiple tools at once
 */
export function registerAllTools<T extends Record<string, MCPTool<z.ZodTypeAny, unknown>>>(
  server: unknown,
  tools: T,
  nameMapping?: Partial<Record<keyof T, string>>,
): void {
  for (const [key, tool] of Object.entries(tools)) {
    const customName = nameMapping?.[key as keyof T];
    registerTool(server, tool, customName);
  }
}
