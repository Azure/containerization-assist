import type { z } from 'zod';
import type { Result } from './core';
import type { ToolContext } from '@/mcp/context';
import type { ToolCategory } from './categories';

/**
 * Unified tool interface for all MCP tools
 */
export interface Tool<TSchema extends z.ZodTypeAny = z.ZodAny, TOut = unknown> {
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

  /**
   * Execute the tool with validated input
   * Validation happens at the MCP server level
   */
  run: (input: z.infer<TSchema>, context: ToolContext) => Promise<Result<TOut>>;
}

// Utility types for tool creation
export type ToolInput<T> = T extends Tool<infer S, any> ? z.infer<S> : never;

export type ToolOutput<T> = T extends Tool<any, infer O> ? O : never;

// Tool factory for consistency
export function createTool<TSchema extends z.ZodTypeAny, TOut>(
  config: Tool<TSchema, TOut>,
): Tool<TSchema, TOut> {
  return config;
}
