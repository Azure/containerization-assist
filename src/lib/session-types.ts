/**
 * Session Type Definitions
 *
 * Simple session storage without complex generics.
 */

/**
 * Simple session slice for tool-specific data
 */
export interface SessionSlice {
  /** Last input to the tool */
  input: Record<string, unknown>;
  /** Last output from the tool */
  output?: unknown;
  /** Tool-specific state */
  state: Record<string, unknown>;
  /** Timestamp of last update */
  updatedAt?: Date;
}

/**
 * Session slice operations interface
 */
export interface SessionSliceOperations {
  /**
   * Get slice for a tool
   * @returns SessionSlice or null if not found
   */
  get(sessionId: string): Promise<SessionSlice | null>;

  /**
   * Set entire slice for a tool
   */
  set(sessionId: string, slice: SessionSlice): Promise<void>;

  /**
   * Partial update to slice
   */
  patch(sessionId: string, partial: Partial<SessionSlice>): Promise<void>;

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
