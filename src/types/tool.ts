import type { z, ZodRawShape } from 'zod';
import type { Result } from './core';
import type { ToolContext } from '@/mcp/context';
import type { ToolCategory } from './categories';
import type { ToolMetadata } from './tool-metadata';
import type { ToolName } from '@/tools';
import { extractSchemaShape } from '@/lib/zod-utils';

/**
 * Chain hints for tool workflow guidance
 */
export interface ChainHints {
  /** Guidance message shown after successful execution */
  success: string;
  /** Guidance message shown after failed execution */
  failure: string;
}

/**
 * Tool interface for all MCP tools with external telemetry support
 *
 * @see {@link ../../docs/adr/002-tool-interface.md ADR-002: Unified Tool Interface}
 */
export interface Tool<TSchema extends z.ZodTypeAny = z.ZodTypeAny, TOut = unknown> {
  /** Unique tool identifier - must be a valid ToolName */
  name: ToolName;

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

  /** Optional workflow guidance hints for tool chaining */
  chainHints?: ChainHints;

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
  name: ToolName;
  description: string;
  schema: TSchema;
  metadata: ToolMetadata;
  handler: (input: z.infer<TSchema>, context: ToolContext) => Promise<Result<TOut>>;
  category?: ToolCategory;
  version?: string;
  chainHints?: ChainHints;
}): Tool<TSchema, TOut> {
  return {
    ...config,
    inputSchema: extractSchemaShape(config.schema),
    parse: (args: unknown) => config.schema.parse(args), // Uses Zod's parse, throws on invalid
  };
}
