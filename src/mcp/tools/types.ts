/**
 * Tool Types - Discriminated union for different tool export patterns
 */

import type { z } from 'zod';
import type { ToolContext } from '@/mcp/context';
import type { Result } from '@/types';

/**
 * Standard tool export pattern - direct function export
 * Used by most tools like analyze-repo, generate-dockerfile, etc.
 */
export interface StandardToolExport {
  type: 'standard';
  name: string;
  description?: string;
  inputSchema: z.ZodTypeAny;
  execute: (params: any, ctx: ToolContext) => Promise<Result<unknown>>;
}

/**
 * Nested tool export pattern - nested execute function
 * Used by prompt-backed tools that return { execute: { execute: ... } }
 */
export interface NestedToolExport {
  type: 'nested';
  name: string;
  description?: string;
  inputSchema: z.ZodTypeAny;
  outputSchema?: z.ZodTypeAny;
  execute: {
    execute: (params: unknown, helpers: unknown, ctx: ToolContext) => Promise<Result<unknown>>;
  };
}

/**
 * Factory tool export pattern - function that returns handler
 */
export interface FactoryToolExport {
  type: 'factory';
  name: string;
  description?: string;
  inputSchema: z.ZodTypeAny;
  execute: (helpers: any) => (params: any, ctx: ToolContext) => Promise<Result<unknown>>;
}

/**
 * Discriminated union of all supported tool export patterns
 */
export type ToolExport = StandardToolExport | NestedToolExport | FactoryToolExport;

/**
 * Normalized tool interface for the router
 */
export interface NormalizedTool {
  name: string;
  schema?: z.ZodTypeAny;
  handler: (params: Record<string, unknown>, ctx: ToolContext) => Promise<Result<unknown>>;
}

/**
 * Normalize a tool export to the standard router interface
 */
export function normalizeToolExport(toolMod: ToolExport): NormalizedTool {
  switch (toolMod.type) {
    case 'standard':
      return {
        name: toolMod.name,
        schema: toolMod.inputSchema,
        handler: async (params, ctx) => {
          return toolMod.execute(params, ctx);
        },
      };

    case 'nested':
      return {
        name: toolMod.name,
        schema: toolMod.inputSchema,
        handler: async (params, ctx) => {
          return toolMod.execute.execute(params, { logger: ctx.logger }, ctx);
        },
      };

    case 'factory':
      return {
        name: toolMod.name,
        schema: toolMod.inputSchema,
        handler: async (params, ctx) => {
          const handler = toolMod.execute({ logger: ctx.logger });
          return handler(params, ctx);
        },
      };
  }
}
