/**
 * Centralized Error Mapping for MCP Boundary
 *
 * Single place for converting domain errors to MCP errors.
 * Used at the boundary between tool execution and MCP protocol.
 */

import { McpError, ErrorCode } from '@modelcontextprotocol/sdk/types.js';

/**
 * Map domain error codes to MCP error codes
 */
const ERROR_CODE_MAP: Record<string, ErrorCode> = {
  // Validation errors
  VALIDATION_ERROR: ErrorCode.InvalidParams,
  INVALID_PARAMS: ErrorCode.InvalidParams,
  MISSING_PARAMS: ErrorCode.InvalidParams,

  // Not found errors
  NOT_FOUND: ErrorCode.MethodNotFound,
  TOOL_NOT_FOUND: ErrorCode.MethodNotFound,
  PROMPT_NOT_FOUND: ErrorCode.MethodNotFound,
  RESOURCE_NOT_FOUND: ErrorCode.InternalError,

  // Resource errors (no specific ResourceError in SDK)
  RESOURCE_ERROR: ErrorCode.InternalError,
  RESOURCE_UNAVAILABLE: ErrorCode.InternalError,

  // Everything else is internal error
  INTERNAL_ERROR: ErrorCode.InternalError,
  UNKNOWN_ERROR: ErrorCode.InternalError,
};

/**
 * Convert any error to an MCP error at the protocol boundary
 *
 * This is the single function that should be used when converting
 * errors from tool execution to MCP protocol errors.
 *
 * @example
 * ```typescript
 * try {
 *   const result = await tool.handler(params, context);
 *   return formatSuccess(result);
 * } catch (error) {
 *   throw toMcpError(error);
 * }
 * ```
 */
export function toMcpError(error: unknown): McpError {
  // Already an MCP error
  if (error instanceof McpError) {
    return error;
  }

  // ContainerizationError with code and details
  if (isContainerizationError(error)) {
    const mcpCode = ERROR_CODE_MAP[error.code] || ErrorCode.InternalError;
    return new McpError(mcpCode, error.message, error.details);
  }

  // Standard Error
  if (error instanceof Error) {
    // Check for common error patterns in the message
    if (error.message.includes('not found')) {
      return new McpError(ErrorCode.MethodNotFound, error.message);
    }
    if (error.message.includes('invalid') || error.message.includes('validation')) {
      return new McpError(ErrorCode.InvalidParams, error.message);
    }

    return new McpError(ErrorCode.InternalError, error.message);
  }

  // Unknown error type
  return new McpError(
    ErrorCode.InternalError,
    typeof error === 'string' ? error : 'Unknown error occurred',
  );
}

/**
 * Type guard for domain errors with code and details
 */
interface DomainError {
  code: string;
  message: string;
  details?: unknown;
}

function isContainerizationError(error: unknown): error is DomainError {
  return (
    typeof error === 'object' &&
    error !== null &&
    'code' in error &&
    'message' in error &&
    typeof (error as { code: unknown }).code === 'string' &&
    typeof (error as { message: unknown }).message === 'string'
  );
}

/**
 * Extract error details for logging
 *
 * @example
 * ```typescript
 * logger.error(getErrorDetails(error), 'Tool execution failed');
 * ```
 */
export function getErrorDetails(error: unknown): Record<string, any> {
  if (error instanceof McpError) {
    return {
      code: error.code,
      message: error.message,
      data: error.data,
    };
  }

  if (isContainerizationError(error)) {
    return {
      code: error.code,
      message: error.message,
      details: error.details,
    };
  }

  if (error instanceof Error) {
    return {
      name: error.name,
      message: error.message,
      stack: error.stack,
    };
  }

  return {
    error: String(error),
    type: typeof error,
  };
}
