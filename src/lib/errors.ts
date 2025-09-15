/**
 * Functional Error System for Containerization Assist
 *
 * Provides structured error handling using data interfaces and factory functions
 * for better error handling, debugging, and recovery throughout the application.
 *
 * Uses a functional approach with:
 * - Error data interfaces (ContainerizationErrorData, SessionErrorData)
 * - Factory functions for error creation
 * - Structural type guards for error checking
 * - Result<T> pattern support
 */

import { Result, Failure } from '../types';

/**
 * Error codes for standardized error handling
 */
export const ErrorCodes = {
  // Validation errors
  VALIDATION_FAILED: 'VALIDATION_FAILED',
  MISSING_REQUIRED_FIELD: 'MISSING_REQUIRED_FIELD',
  INVALID_PARAMETER: 'INVALID_PARAMETER',

  // Docker errors
  DOCKER_BUILD_FAILED: 'DOCKER_BUILD_FAILED',
  DOCKER_PUSH_FAILED: 'DOCKER_PUSH_FAILED',
  DOCKER_TAG_FAILED: 'DOCKER_TAG_FAILED',
  DOCKER_CONNECTION_FAILED: 'DOCKER_CONNECTION_FAILED',
  DOCKERFILE_NOT_FOUND: 'DOCKERFILE_NOT_FOUND',
  IMAGE_NOT_FOUND: 'IMAGE_NOT_FOUND',

  // Kubernetes errors
  KUBERNETES_DEPLOY_FAILED: 'KUBERNETES_DEPLOY_FAILED',
  KUBERNETES_CONNECTION_FAILED: 'KUBERNETES_CONNECTION_FAILED',
  CLUSTER_NOT_READY: 'CLUSTER_NOT_READY',
  NAMESPACE_NOT_FOUND: 'NAMESPACE_NOT_FOUND',
  RESOURCE_NOT_FOUND: 'RESOURCE_NOT_FOUND',

  // Session errors
  SESSION_NOT_FOUND: 'SESSION_NOT_FOUND',
  SESSION_EXPIRED: 'SESSION_EXPIRED',
  SESSION_LIMIT_EXCEEDED: 'SESSION_LIMIT_EXCEEDED',

  // AI Service errors
  AI_SERVICE_UNAVAILABLE: 'AI_SERVICE_UNAVAILABLE',
  AI_GENERATION_FAILED: 'AI_GENERATION_FAILED',
  AI_ANALYSIS_FAILED: 'AI_ANALYSIS_FAILED',

  // File system errors
  FILE_NOT_FOUND: 'FILE_NOT_FOUND',
  DIRECTORY_NOT_FOUND: 'DIRECTORY_NOT_FOUND',
  PERMISSION_DENIED: 'PERMISSION_DENIED',

  // Security errors
  SECURITY_SCAN_FAILED: 'SECURITY_SCAN_FAILED',
  VULNERABILITY_FOUND: 'VULNERABILITY_FOUND',

  // Generic errors
  INTERNAL_ERROR: 'INTERNAL_ERROR',
  TIMEOUT: 'TIMEOUT',
  NOT_IMPLEMENTED: 'NOT_IMPLEMENTED',
} as const;

export type ErrorCode = (typeof ErrorCodes)[keyof typeof ErrorCodes];

// ============================================================================
// FUNCTIONAL APPROACH - Data Interfaces and Factory Functions
// ============================================================================

/**
 * Data interface for containerization errors
 * This is the preferred approach for new code
 */
export interface ContainerizationErrorData {
  readonly message: string;
  readonly code: ErrorCode;
  readonly details: Record<string, unknown>;
  readonly cause?: Error;
  readonly timestamp: Date;
  readonly name: string;
  readonly stack?: string;
}

/**
 * Data interface for session errors
 */
export interface SessionErrorData extends ContainerizationErrorData {
  readonly sessionId?: string;
}

/**
 * Factory function to create a containerization error data object
 * @param message - Error message
 * @param code - Error code
 * @param details - Additional error details
 * @param cause - Original error that caused this error
 * @returns ContainerizationErrorData object
 */
export const createContainerizationError = (
  message: string,
  code: ErrorCode = ErrorCodes.INTERNAL_ERROR,
  details: Record<string, unknown> = {},
  cause?: Error,
): ContainerizationErrorData => {
  const error = new Error(message);
  return {
    message,
    code,
    details,
    timestamp: new Date(),
    name: 'ContainerizationError',
    ...(cause && { cause }),
    ...(error.stack && { stack: error.stack }),
  };
};

/**
 * Factory function to create a session error data object
 * @param message - Error message
 * @param code - Error code
 * @param details - Additional error details
 * @param cause - Original error that caused this error
 * @param sessionId - Optional session ID
 * @returns SessionErrorData object
 */
export const createSessionError = (
  message: string,
  code: ErrorCode = ErrorCodes.SESSION_NOT_FOUND,
  details: Record<string, unknown> = {},
  cause?: Error,
  sessionId?: string,
): SessionErrorData => {
  const baseError = createContainerizationError(message, code, details, cause);
  return {
    ...baseError,
    name: 'SessionError',
    ...(sessionId && { sessionId }),
  };
};

/**
 * Factory function to create a validation error
 */
export const createValidationError = (
  message: string,
  details: Record<string, unknown> = {},
  cause?: Error,
): ContainerizationErrorData =>
  createContainerizationError(message, ErrorCodes.VALIDATION_FAILED, details, cause);

/**
 * Factory function to create a Docker error
 */
export const createDockerError = (
  message: string,
  code: ErrorCode = ErrorCodes.DOCKER_BUILD_FAILED,
  details: Record<string, unknown> = {},
  cause?: Error,
): ContainerizationErrorData => createContainerizationError(message, code, details, cause);

/**
 * Factory function to create a Kubernetes error
 */
export const createKubernetesError = (
  message: string,
  code: ErrorCode = ErrorCodes.KUBERNETES_DEPLOY_FAILED,
  details: Record<string, unknown> = {},
  cause?: Error,
): ContainerizationErrorData => createContainerizationError(message, code, details, cause);

/**
 * Factory function to create an AI service error
 */
export const createAIServiceError = (
  message: string,
  code: ErrorCode = ErrorCodes.AI_SERVICE_UNAVAILABLE,
  details: Record<string, unknown> = {},
  cause?: Error,
): ContainerizationErrorData => createContainerizationError(message, code, details, cause);

/**
 * Convert error data to a user-friendly message
 */
export const getUserMessage = (error: ContainerizationErrorData): string =>
  `${error.message} (${error.code})`;

/**
 * Serialize error data for logging or transmission
 */
export const serializeError = (error: ContainerizationErrorData): Record<string, unknown> => ({
  name: error.name,
  message: error.message,
  code: error.code,
  details: error.details,
  timestamp: error.timestamp,
  stack: error.stack,
  cause: error.cause
    ? {
        message: error.cause.message,
        stack: error.cause.stack,
      }
    : undefined,
});

/**
 * Convert error data to Result type for function returns
 */
export const errorDataToResult = <T>(error: ContainerizationErrorData): Result<T> =>
  Failure(`${error.code}: ${error.message}`);

// ============================================================================
// TYPE GUARDS
// ============================================================================

/**
 * Type guard to check if an object is ContainerizationErrorData
 * This is the preferred approach for new code
 */
export function isContainerizationErrorData(error: unknown): error is ContainerizationErrorData {
  return (
    typeof error === 'object' &&
    error !== null &&
    'code' in error &&
    'message' in error &&
    'name' in error &&
    'timestamp' in error &&
    'details' in error
  );
}

/**
 * Type guard to check if an object is SessionErrorData
 */
export function isSessionErrorData(error: unknown): error is SessionErrorData {
  return isContainerizationErrorData(error) && error.name === 'SessionError';
}

// ============================================================================
// CONVERSION UTILITIES
// ============================================================================

/**
 * Convert error data to Result type (for MCP boundaries)
 */
export function errorToResult(error: ContainerizationErrorData): { ok: false; error: string } {
  return {
    ok: false,
    error: `${error.code}: ${error.message}`,
  };
}
