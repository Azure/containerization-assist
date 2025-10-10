import type { z } from 'zod';
import type { Result } from './core';
import type { ToolContext } from '@/mcp/context';
import type { ToolCategory } from './categories';
import type { ToolMetadata } from './tool-metadata';

/**
 * Unified tool interface for all MCP tools
 */
export interface MCPTool<TSchema extends z.ZodTypeAny = z.ZodTypeAny, TOut = unknown> {
  /** Unique tool identifier */
  name: string;

  /** Human-readable description */
  description: string;

  /** Tool category for organization and grouping */
  category?: ToolCategory;

  /** Optional semantic version */
  version?: string;

  /** Zod schema for input validation */
  schema: TSchema;

  /** Tool metadata for AI enhancement tracking (required) */
  metadata: ToolMetadata;

  /**
   * Execute the tool with validated input
   * Validation happens at the MCP server level
   */
  run: (input: z.infer<TSchema>, context: ToolContext) => Promise<Result<TOut>>;
}

// Utility types for tool creation
export type ToolInput<T> = T extends MCPTool<infer S, unknown> ? z.infer<S> : never;

export type ToolOutput<T> = T extends MCPTool<z.ZodTypeAny, infer O> ? O : never;

// Tool factory for consistency
export function createTool<TSchema extends z.ZodTypeAny, TOut>(
  config: MCPTool<TSchema, TOut>,
): MCPTool<TSchema, TOut> {
  return config;
}
