/**
 * Session Type Definitions
 *
 * Provides type-safe session slices for tools while maintaining backward compatibility
 * with existing session infrastructure.
 */

import { z, type ZodTypeAny } from 'zod';

/**
 * Typed session slice for tool-specific data
 * @template In - Input type (from tool parameters)
 * @template Out - Output type (from tool result)
 * @template State - Tool-specific state type
 */
export interface SessionSlice<In, Out, State = Record<string, unknown>> {
  /** Last validated input to the tool */
  input: In;
  /** Last validated output from the tool */
  output?: Out;
  /** Tool-specific progress/flags */
  state: State;
  /** Timestamp of last update */
  updatedAt?: Date;
}

/**
 * Tool IO definition with proper type inference
 */
export interface ToolIO<TInputSchema extends ZodTypeAny, TOutputSchema extends ZodTypeAny> {
  input: TInputSchema;
  output: TOutputSchema;
}

/**
 * Inferred types from ToolIO schemas
 */
export type InferredInput<T> =
  T extends ToolIO<infer I, ZodTypeAny> ? (I extends ZodTypeAny ? z.infer<I> : never) : never;

export type InferredOutput<T> =
  T extends ToolIO<ZodTypeAny, infer O> ? (O extends ZodTypeAny ? z.infer<O> : never) : never;

/**
 * Define tool input/output schemas for type-safe session operations
 * @param input - Zod schema for tool input parameters
 * @param output - Zod schema for tool output results
 * @returns ToolIO object with paired schemas and inferred types
 */
export function defineToolIO<TInputSchema extends ZodTypeAny, TOutputSchema extends ZodTypeAny>(
  input: TInputSchema,
  output: TOutputSchema,
): ToolIO<TInputSchema, TOutputSchema> {
  return {
    input,
    output,
  };
}

/**
 * Session slice store interface for type-safe operations
 */
export interface SessionSliceStore {
  /**
   * Get raw data from session
   */
  get<T>(sessionId: string, key: string): Promise<T | null>;

  /**
   * Set data in session
   */
  set<T>(sessionId: string, key: string, value: T): Promise<void>;

  /**
   * Delete data from session
   */
  delete(sessionId: string, key: string): Promise<void>;

  /**
   * Check if session exists
   */
  has(sessionId: string): Promise<boolean>;
}

/**
 * Session slice operations interface
 */
export interface SessionSliceOperations<In, Out, State> {
  /**
   * Get typed slice for a tool
   * @returns SessionSlice or null if not found
   */
  get(sessionId: string): Promise<SessionSlice<In, Out, State> | null>;

  /**
   * Set entire slice for a tool
   */
  set(sessionId: string, slice: SessionSlice<In, Out, State>): Promise<void>;

  /**
   * Partial update to slice
   */
  patch(sessionId: string, partial: Partial<SessionSlice<In, Out, State>>): Promise<void>;

  /**
   * Clear slice for a tool
   */
  clear(sessionId: string): Promise<void>;
}

/**
 * Tool slice metadata stored in session
 */
export interface ToolSliceMetadata {
  /** Map of tool names to their slices */
  toolSlices: Record<string, unknown>;
  /** Last accessed timestamps for cleanup */
  lastAccessed: Record<string, Date>;
}

/**
 * Type guard for checking if metadata has tool slices
 */
export function hasToolSlices(
  metadata: unknown,
): metadata is { toolSlices: Record<string, unknown> } {
  return (
    typeof metadata === 'object' &&
    metadata !== null &&
    'toolSlices' in metadata &&
    typeof (metadata as Record<string, unknown>).toolSlices === 'object'
  );
}

/**
 * Create default slice metadata
 */
export function createSliceMetadata(): ToolSliceMetadata {
  return {
    toolSlices: {},
    lastAccessed: {},
  };
}
