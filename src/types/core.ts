/**
 * Core type definitions for the containerization assist system.
 * Consolidated Result types and tool system interfaces.
 */

// ===== RESULT TYPE SYSTEM =====

/**
 * Structured error information with actionable guidance
 */
export interface ErrorGuidance {
  /** Primary error message */
  message: string;
  /** Actionable hint for the operator (what went wrong in user terms) */
  hint?: string;
  /** Specific resolution steps to fix the issue */
  resolution?: string;
  /** Additional context or details */
  details?: Record<string, unknown>;
}

/**
 * Result type for functional error handling
 *
 * Provides explicit error handling without exceptions to ensure:
 * - Type-safe error propagation
 * - MCP protocol compatibility (no exception bubbling)
 * - Clean async chain composition
 * - Actionable operator guidance for failures
 *
 * @example
 * ```typescript
 * const result = await riskyOperation();
 * if (result.ok) {
 *   console.log(result.value);
 * } else {
 *   console.error(result.error); // string for backward compatibility
 *   if (result.guidance) {
 *     console.error('Hint:', result.guidance.hint);
 *     console.error('Resolution:', result.guidance.resolution);
 *   }
 * }
 * ```
 */
export type Result<T> =
  | { ok: true; value: T }
  | { ok: false; error: string; guidance?: ErrorGuidance };

/** Create a success result */
export const Success = <T>(value: T): Result<T> => ({ ok: true, value });

/**
 * Create a failure result with optional guidance
 * @param error - Error message (required for backward compatibility)
 * @param guidance - Optional structured guidance for operators
 */
export const Failure = <T>(error: string, guidance?: ErrorGuidance): Result<T> => {
  // Always create a new guidance object to avoid mutating the input parameter
  const resultGuidance = guidance ? { ...guidance, message: guidance.message || error } : undefined;
  return resultGuidance ? { ok: false, error, guidance: resultGuidance } : { ok: false, error };
};

