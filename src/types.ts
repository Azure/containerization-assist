/**
 * Core type definitions for the containerization assist system.
 * Provides Result type for error handling and tool system interfaces.
 */

import type { Logger } from 'pino';
import type { ToolContext } from './mcp/context/types';

/**
 * Result type for functional error handling
 *
 * DESIGN DECISION: Why Result<T> instead of exceptions?
 *
 * This pattern was chosen over traditional try/catch exception handling for several reasons:
 *
 * 1. **Explicit Error Handling**: Forces consumers to handle errors explicitly at the type level
 *    - TypeScript compiler ensures error cases aren't ignored
 *    - Makes error paths visible in the function signature
 *    - Prevents accidental exception bubbling that breaks the MCP protocol
 *
 * 2. **MCP Protocol Compatibility**: The Model Context Protocol expects structured responses
 *    - Exceptions would break the JSON-RPC message flow
 *    - Result<T> ensures all responses are serializable
 *    - Enables graceful error reporting to AI models
 *
 * 3. **Async Chain Safety**: Prevents unhandled promise rejections
 *    - Traditional exceptions can be lost in async chains
 *    - Result<T> makes error propagation explicit and safe
 *    - Enables better error aggregation in workflows
 *
 * 4. **Functional Programming Alignment**: Supports railway-oriented programming
 *    - Enables clean error composition and transformation
 *    - Allows building robust workflows from potentially-failing operations
 *    - Makes error recovery patterns more predictable
 *
 * Trade-offs accepted:
 * - Slightly more verbose than exceptions (requires .ok checks)
 * - Different from typical JavaScript patterns (but aligns with Rust/Go)
 * - Learning curve for developers used to exception-based error handling
 *
 * @example
 * ```typescript
 * // Instead of this (exception-based):
 * try {
 *   const result = await riskyOperation();
 *   return processResult(result);
 * } catch (error) {
 *   logger.error(error);
 *   throw new Error('Operation failed');
 * }
 *
 * // Use this (Result-based):
 * const result = await riskyOperation(); // Returns Result<T>
 * if (!result.ok) {
 *   logger.error(result.error);
 *   return Failure('Operation failed');
 * }
 * return Success(processResult(result.value));
 * ```
 */
export type Result<T> = { ok: true; value: T } | { ok: false; error: string };

/** Create a success result */
export const Success = <T>(value: T): Result<T> => ({ ok: true, value });

/** Create a failure result */
export const Failure = <T>(error: string): Result<T> => ({ ok: false, error });

/** Type guard to check if result is a failure */
export const isFail = <T>(result: Result<T>): result is { ok: false; error: string } => !result.ok;

export type { ToolContext } from './mcp/context/types';

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

// ===== SESSION & WORKFLOW =====

/**
 * Represents the state of a workflow execution session.
 */
export interface WorkflowState {
  /** Unique session identifier */
  sessionId: string;
  /** Currently executing workflow step */
  currentStep?: string;
  /** Overall workflow progress (0-100) */
  progress?: number;
  /** Results from completed workflow steps */
  results?: Record<string, unknown>;
  /** Additional workflow metadata */
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
