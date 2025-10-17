import type { z, ZodRawShape } from 'zod';
import type { Result } from './core';
import type { ToolContext } from '@/mcp/context';
import type { ToolCategory } from './categories';
import type { ToolMetadata } from './tool-metadata';
import { extractSchemaShape } from '@/lib/zod-utils';

/**
 * Unified tool interface for all MCP tools with external telemetry support
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

  /** Raw Zod schema shape for MCP registration */
  inputSchema: ZodRawShape;

  /** Zod schema for validation (kept internally for parsing) */
  schema: TSchema;

  /** Tool metadata for AI enhancement tracking (required) */
  metadata: ToolMetadata;

  /** Parse and validate untyped arguments to strongly-typed input (matches Zod API) */
  parse: (args: unknown) => z.infer<TSchema>;

  /** Tool handler with pre-validated, strongly-typed input */
  handler: (input: z.infer<TSchema>, context: ToolContext) => Promise<Result<TOut>>;
}

/**
 * Lightweight helper to create tools with reduced boilerplate
 * Automatically extracts inputSchema and creates parse method from Zod schema
 */
export function tool<TSchema extends z.ZodTypeAny, TOut>(config: {
  name: string;
  description: string;
  schema: TSchema;
  metadata: ToolMetadata;
  handler: (input: z.infer<TSchema>, context: ToolContext) => Promise<Result<TOut>>;
  category?: ToolCategory;
  version?: string;
}): MCPTool<TSchema, TOut> {
  return {
    ...config,
    inputSchema: extractSchemaShape(config.schema),
    parse: (args: unknown) => config.schema.parse(args), // Uses Zod's parse, throws on invalid
  };
}
