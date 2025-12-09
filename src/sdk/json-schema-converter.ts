/**
 * Type-safe wrapper for zod-to-json-schema conversion.
 *
 * This module isolates the necessary type coercion to a single location,
 * avoiding scattered eslint-disable comments and improving maintainability.
 *
 * The zod-to-json-schema library has slightly misaligned types between its
 * exported function signature and actual usage patterns. This wrapper
 * provides a clean interface while handling the type gymnastics internally.
 */

import { zodToJsonSchema, type JsonSchema7Type } from 'zod-to-json-schema';
import type { ZodTypeAny } from 'zod';

/**
 * Options for JSON Schema conversion.
 */
export interface JsonSchemaConversionOptions {
  /**
   * Strategy for handling $ref references.
   * - 'none': Inline all definitions (best for VS Code compatibility)
   * - 'root': Use $ref with definitions at root
   * - 'relative': Use relative $ref paths
   * @default 'none'
   */
  $refStrategy?: 'none' | 'root' | 'relative';

  /**
   * Target JSON Schema version.
   * @default 'jsonSchema7'
   */
  target?: 'jsonSchema7' | 'jsonSchema2019-09' | 'openApi3';
}

const defaultOptions: Required<JsonSchemaConversionOptions> = {
  $refStrategy: 'none',
  target: 'jsonSchema7',
};

/**
 * Convert a Zod schema to JSON Schema.
 *
 * Uses jsonSchema7 target for VS Code compatibility by default.
 * Disables $ref generation for simpler, self-contained schemas.
 *
 * @param schema - Zod schema to convert
 * @param options - Conversion options
 * @returns JSON Schema (Draft 7 by default)
 *
 * @example
 * ```typescript
 * import { z } from 'zod';
 * import { convertZodToJsonSchema } from './json-schema-converter';
 *
 * const mySchema = z.object({
 *   name: z.string(),
 *   age: z.number().optional(),
 * });
 *
 * const jsonSchema = convertZodToJsonSchema(mySchema);
 * // {
 * //   type: 'object',
 * //   properties: {
 * //     name: { type: 'string' },
 * //     age: { type: 'number' }
 * //   },
 * //   required: ['name']
 * // }
 * ```
 */
export function convertZodToJsonSchema(
  schema: ZodTypeAny,
  options: JsonSchemaConversionOptions = {},
): JsonSchema7Type {
  const opts = { ...defaultOptions, ...options };

  // The zodToJsonSchema function has complex generic types that cause
  // "Type instantiation is excessively deep" errors in CJS builds.
  // We use explicit typing to avoid deep type instantiation issues.
  //
  // This is safe because:
  // 1. We control the input (ZodTypeAny)
  // 2. We control the options (known valid values)
  // 3. The output is always a valid JsonSchema7Type for our use case
  //
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const convert = zodToJsonSchema as (schema: any, options: any) => any;
  return convert(schema, {
    $refStrategy: opts.$refStrategy,
    target: opts.target,
  }) as JsonSchema7Type;
}
