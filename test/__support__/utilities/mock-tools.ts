/**
 * Mock Tool Utilities for Testing
 *
 * Provides properly-typed mock tool creation for SDK and executor tests.
 * Eliminates the need for `as never` type assertions.
 */

import { z } from 'zod';
import type { ToolContext } from '../../../src/core/context';
import type { Result } from '../../../src/types/core';
import { tool } from '../../../src/types/tool';
import type { ToolName } from '../../../src/tools';

/**
 * Default mock schema for simple test cases.
 */
export const mockSchema = z.object({
  input: z.string(),
  optional: z.number().optional(),
});

export type MockSchemaInput = z.infer<typeof mockSchema>;

/**
 * Create a properly-typed mock tool for testing.
 *
 * @param handler - Handler function that receives validated input and context
 * @param options - Optional overrides for tool configuration
 * @returns A properly-typed Tool instance
 *
 * @example
 * ```typescript
 * const mockHandler = jest.fn().mockResolvedValue(Success({ result: 'success' }));
 * const mockTool = createMockTool(mockHandler);
 *
 * const result = await executeTool(mockTool, { input: 'test' });
 * ```
 */
export function createMockTool<TOut>(
  handler: (input: MockSchemaInput, ctx: ToolContext) => Promise<Result<TOut>>,
  options?: {
    name?: ToolName;
    description?: string;
    knowledgeEnhanced?: boolean;
  },
) {
  return tool({
    name: options?.name ?? ('analyze-repo' as ToolName),
    description: options?.description ?? 'Mock tool for testing',
    schema: mockSchema,
    metadata: { knowledgeEnhanced: options?.knowledgeEnhanced ?? false },
    handler,
  });
}

/**
 * Create a mock tool with a custom schema for testing.
 *
 * Use this when you need to test with a specific schema shape.
 *
 * @param schema - Zod schema for input validation
 * @param handler - Handler function
 * @param options - Optional tool configuration
 * @returns A properly-typed Tool instance
 *
 * @example
 * ```typescript
 * const customSchema = z.object({
 *   config: z.object({
 *     name: z.string(),
 *     port: z.number(),
 *   }),
 * });
 *
 * const mockTool = createMockToolWithSchema(
 *   customSchema,
 *   async (input, ctx) => Success({ processed: input.config.name })
 * );
 * ```
 */
export function createMockToolWithSchema<TSchema extends z.ZodTypeAny, TOut>(
  schema: TSchema,
  handler: (input: z.infer<TSchema>, ctx: ToolContext) => Promise<Result<TOut>>,
  options?: {
    name?: ToolName;
    description?: string;
    knowledgeEnhanced?: boolean;
  },
) {
  return tool({
    name: options?.name ?? ('analyze-repo' as ToolName),
    description: options?.description ?? 'Mock tool for testing',
    schema,
    metadata: { knowledgeEnhanced: options?.knowledgeEnhanced ?? false },
    handler,
  });
}
