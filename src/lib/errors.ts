/**
 * Structured Error Classes for Containerization Assist
 *
 * Provides a hierarchy of error classes with rich metadata for better
 * error handling, debugging, and recovery throughout the application.
 */

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

/**
 * Base error class for all containerization errors
 */
export class ContainerizationError extends Error {
  public readonly code: ErrorCode;
  public readonly details: Record<string, unknown> | undefined;
  public override readonly cause: Error | undefined;
  public readonly timestamp: Date;

  constructor(
    message: string,
    code: ErrorCode = ErrorCodes.INTERNAL_ERROR,
    details?: Record<string, unknown>,
    cause?: Error,
  ) {
    super(message);
    this.name = 'ContainerizationError';
    this.code = code;
    this.details = details || {};
    this.cause = cause || undefined;
    this.timestamp = new Date();

    // Maintains proper stack trace for where error was thrown
    Error.captureStackTrace(this, this.constructor);
  }

  /**
   * Convert to plain object for serialization
   */
  toJSON(): Record<string, unknown> {
    return {
      name: this.name,
      message: this.message,
      code: this.code,
      details: this.details,
      timestamp: this.timestamp,
      stack: this.stack,
      cause: this.cause
        ? {
            message: this.cause.message,
            stack: this.cause.stack,
          }
        : undefined,
    };
  }

  /**
   * Get a user-friendly error message
   */
  getUserMessage(): string {
    return `${this.message} (${this.code})`;
  }
}

/**
 * Session management errors
 */
export class SessionError extends ContainerizationError {
  constructor(
    message: string,
    code: ErrorCode = ErrorCodes.SESSION_NOT_FOUND,
    details?: Record<string, unknown>,
    cause?: Error,
  ) {
    super(message, code, details, cause);
    this.name = 'SessionError';
  }
}

/**
 * Type guard to check if an error is a ContainerizationError
 */
export function isContainerizationError(error: unknown): error is ContainerizationError {
  return error instanceof ContainerizationError;
}

/**
 * Convert ContainerizationError to Result type (for MCP boundaries)
 */
export function errorToResult(error: ContainerizationError): { ok: false; error: string } {
  return {
    ok: false,
    error: `${error.code}: ${error.message}`,
  };
}
