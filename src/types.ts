/**
 * Core type definitions for the containerization assist system.
 * Provides Result type for error handling and tool system interfaces.
 */

import type { Logger } from 'pino';
import type { ToolContext } from './mcp/context';
import type { ZodRawShape } from 'zod';

// Export enhanced category types
export * from './types/categories';

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

export type { ToolContext } from './mcp/context';

/**
 * Tool definition for MCP server operations.
 */
export interface Tool {
  /** Unique tool identifier */
  name: string;
  /** Human-readable tool description */
  description?: string;
  /** JSON schema for parameter validation */
  schema?: Record<string, unknown>;
  /** Zod schema for McpServer compatibility (optional) */
  zodSchema?: ZodRawShape;
  /**
   * Executes the tool with provided parameters.
   * @param params - Tool-specific parameters
   * @param logger - Logger instance for tool execution
   * @param context - Optional ToolContext for AI capabilities and progress reporting
   * @returns Promise resolving to Result with tool output or error
   */
  execute: (
    params: Record<string, unknown>,
    logger: Logger,
    context?: ToolContext,
  ) => Promise<Result<unknown>>;
}

// ===== SESSION =====

/**
 * Represents the state of a tool execution session.
 */
export interface WorkflowState {
  /** Unique session identifier */
  sessionId: string;
  /** Currently executing tool */
  currentStep?: string;
  /** Overall progress (0-100) */
  progress?: number;
  /** Results from completed tools */
  results?: Record<string, unknown>;
  /** Additional metadata */
  metadata?: Record<string, unknown>;
  /** List of completed step names */
  completed_steps?: string[];
  /** Session creation timestamp */
  createdAt: Date;
  /** Last update timestamp */
  updatedAt: Date;
  /** Allow additional properties for extensibility */
  [key: string]: unknown;
}

// ===== AI SERVICE TYPES =====

export interface AIService {
  isAvailable(): boolean;
  generateResponse(prompt: string, context?: Record<string, unknown>): Promise<Result<string>>;
  analyzeCode(code: string, language: string): Promise<Result<unknown>>;
  enhanceDockerfile(
    dockerfile: string,
    requirements?: Record<string, unknown>,
  ): Promise<Result<string>>;
  validateParameters?(params: Record<string, unknown>): Promise<Result<unknown>>;
  analyzeResults?(results: unknown): Promise<Result<unknown>>;
}
