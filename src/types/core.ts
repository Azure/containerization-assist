/**
 * Core type definitions for the containerization assist system.
 * Consolidated Result types and tool system interfaces.
 */

// ===== RESULT TYPE SYSTEM =====

/**
 * Result type for functional error handling
 *
 * Provides explicit error handling without exceptions to ensure:
 * - Type-safe error propagation
 * - MCP protocol compatibility (no exception bubbling)
 * - Clean async chain composition
 *
 * @example
 * ```typescript
 * const result = await riskyOperation();
 * if (result.ok) {
 *   console.log(result.value);
 * } else {
 *   console.error(result.error);
 * }
 * ```
 */
export type Result<T> = { ok: true; value: T } | { ok: false; error: string };

/** Create a success result */
export const Success = <T>(value: T): Result<T> => ({ ok: true, value });

/** Create a failure result */
export const Failure = <T>(error: string): Result<T> => ({ ok: false, error });

/** Type guard to check if result is a failure */
export const isFail = <T>(result: Result<T>): result is { ok: false; error: string } => !result.ok;

/** Type guard to check if result is successful */
export const isSuccess = <T>(result: Result<T>): result is { ok: true; value: T } => result.ok;

/** Type guard to check if result is a failure */
export const isFailure = <T>(result: Result<T>): result is { ok: false; error: string } =>
  !result.ok;

// ===== COMMON TOOL METADATA =====

/**
 * Base metadata that all tool executions should have
 */
export interface ToolExecutionMetadata {
  sessionId: string;
  executedAt: Date;
  duration: number;
}

/**
 * Validation result structure used across tools
 */
export interface ValidationResult {
  valid: boolean;
  issues: Array<{ severity: 'error' | 'warning'; message: string }>;
}

/**
 * Analysis capability for tools that perform repository/code analysis
 */
export interface AnalysisCapability {
  confidence: number;
  detectionMethod: 'signature' | 'extension' | 'ai-enhanced' | 'fallback';
}

/**
 * Build capability for tools that create artifacts
 */
export interface BuildCapability {
  artifacts: string[];
  size?: number;
  digest?: string;
}

/**
 * Deployment capability for tools that handle deployments
 */
export interface DeploymentCapability {
  namespace?: string;
  replicas?: number;
  status?: 'pending' | 'running' | 'failed' | 'completed';
}
