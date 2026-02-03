import type { z } from 'zod';
import type { ToolCategory } from '@/types/categories';
import type { ToolMetadata } from '@/types/tool-metadata';
import type { ChainHints } from '@/types/tool';

/**
 * Tool definition containing metadata without the handler implementation.
 * This is a lightweight interface for importing tool definitions without
 * pulling in the heavy handler implementations and their dependencies.
 */
export interface IToolDefinition<TName extends string = string> {
  /** Unique tool identifier */
  name: TName;
  /** Human-readable description */
  description: string;
  /** Tool category for organization and grouping */
  category?: ToolCategory;
  /** Optional semantic version */
  version?: string;
  /** Zod schema for validation */
  schema: z.ZodTypeAny;
  /** Tool metadata for AI enhancement tracking */
  metadata: ToolMetadata;
  /** Optional workflow guidance hints for tool chaining */
  chainHints?: ChainHints;
}