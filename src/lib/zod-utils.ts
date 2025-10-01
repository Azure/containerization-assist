/**
 * Zod utility functions
 */

import type { z, ZodRawShape } from 'zod';

/**
 * Extract the shape from a Zod schema for MCP protocol compatibility
 * Handles different Zod schema types safely
 */
export function extractSchemaShape(schema: z.ZodTypeAny): ZodRawShape {
  // ZodObject has .shape property
  if ('shape' in schema && typeof schema.shape === 'object' && schema.shape !== null) {
    return schema.shape as ZodRawShape;
  }

  // Other schemas may have ._def.shape() method
  if (schema._def && 'shape' in schema._def && typeof schema._def.shape === 'function') {
    return schema._def.shape();
  }

  // For ZodAny or other types without shape, return empty object
  return {};
}
