/**
 * Session Type Definitions
 *
 * Provides type-safe session slices for tools while maintaining backward compatibility
 * with existing session infrastructure.
 */

import { z } from 'zod';

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
 * Tool IO definition pairing input and output schemas
 */
export interface ToolIO<In, Out> {
  input: z.ZodType<In>;
  output: z.ZodType<Out>;
}

/**
 * Define tool input/output schemas for type-safe session operations
 * @param input - Zod schema for tool input parameters
 * @param output - Zod schema for tool output results
 * @returns ToolIO object with paired schemas
 */
export function defineToolIO<In, Out>(
  input: z.ZodType<In>,
  output: z.ZodType<Out>,
): ToolIO<In, Out> {
  return { input, output };
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
    typeof (metadata as any).toolSlices === 'object'
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
