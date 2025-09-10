/**
 * Docker-specific error interface for enhanced error handling
 */
export interface DockerError extends Error {
  statusCode?: number;
  json?: Record<string, unknown>;
  reason?: string;
  code?: string;
}

/**
 * Type guard to check if an error is a Docker-specific error
 */
export function isDockerError(error: unknown): error is DockerError {
  return (
    error instanceof Error &&
    (typeof (error as DockerError).statusCode === 'number' ||
      typeof (error as DockerError).code === 'string' ||
      typeof (error as DockerError).reason === 'string')
  );
}

/**
 * Safely sanitize error details for logging, removing potential sensitive information
 */
export function sanitizeErrorDetails(details: Record<string, unknown>): Record<string, unknown> {
  const sanitized = { ...details };

  // Remove potentially sensitive fields that might contain tokens or credentials
  if (sanitized.json && typeof sanitized.json === 'object' && sanitized.json !== null) {
    const jsonObj = sanitized.json as Record<string, unknown>;
    const sanitizedJson = { ...jsonObj };

    // Remove common sensitive field patterns
    const sensitiveKeys = /auth|token|password|secret|key|credential/i;
    Object.keys(sanitizedJson).forEach((key) => {
      if (sensitiveKeys.test(key)) {
        sanitizedJson[key] = '[REDACTED]';
      }
    });

    sanitized.json = sanitizedJson;
  }

  return sanitized;
}

/**
 * Docker error handling utilities for extracting meaningful error messages from Docker operations.
 *
 * This module provides comprehensive error handling for various Docker operations including:
 * - Image building and management
 * - Container operations
 * - Registry operations (push/pull)
 * - Network and volume operations
 *
 * Error patterns are ordered by specificity to ensure the most precise error messages are extracted first.
 * More specific patterns (like registry authentication errors) are checked before generic patterns.
 *
 * @see https://distribution.github.io/distribution/spec/api/ - Docker Registry HTTP API V2 specification
 * @see https://docs.docker.com/reference/api/engine/ - Docker Engine API reference
 */
export function extractDockerErrorMessage(error: unknown): {
  message: string;
  details: Record<string, unknown>;
} {
  const details: Record<string, unknown> = {};

  if (isDockerError(error)) {
    // First, capture ALL available Docker error properties for debugging
    if (error.statusCode) details.statusCode = error.statusCode;
    if (error.json) details.json = error.json;
    if (error.reason) details.reason = error.reason;
    if (error.code) details.code = error.code;

    // Handle specific Docker error scenarios in PRIORITY ORDER
    // Order matters: Most specific → Least specific to avoid misclassification

    // 1. FIRST: Network-level errors (most fundamental)
    // These occur BEFORE any HTTP communication happens
    // Network connectivity issues (DNS resolution failures, connection refused)
    // Ref: https://distribution.github.io/distribution/spec/api/#overview
    // WHY FIRST: If network fails, no HTTP status codes will be available
    if (error.code === 'ENOTFOUND' || error.code === 'ECONNREFUSED') {
      details.networkError = true;
      return {
        message: `Network connectivity issue - ${error.message || 'Cannot reach Docker registry'}`,
        details,
      };
    }

    // 2. SECOND: Authentication/Authorization failures (specific HTTP codes)
    // Ref: https://distribution.github.io/distribution/spec/auth/
    // HTTP 401 = Invalid credentials, HTTP 403 = Valid credentials but insufficient permissions
    // WHY SECOND: More specific than generic HTTP errors, needs special handling for auth flows
    if (error.statusCode === 401 || error.statusCode === 403) {
      details.authError = true;
      return {
        message: `Registry authentication issue - ${error.message || 'Access denied'}`,
        details,
      };
    }

    // 3. THIRD: Image/manifest not found errors (very common, specific meaning)
    // Ref: https://distribution.github.io/distribution/spec/api/#pulling-an-image
    // HTTP 404 = Image, tag, or manifest does not exist in registry
    // WHY THIRD: 404 has special meaning in Docker context (missing image vs generic not found)
    if (error.statusCode === 404) {
      details.imageNotFound = true;
      return {
        message: `Base image not found - ${error.message || 'Image does not exist'}`,
        details,
      };
    }

    // 4. LAST: Generic HTTP errors from Docker daemon or registry (catch-all)
    // Ref: https://docs.docker.com/reference/api/engine/
    // Covers 400 (bad request), 500 (server error), etc.
    // WHY LAST: Fallback for any HTTP status code not handled above
    if (error.statusCode) {
      return {
        message: `HTTP ${error.statusCode} - ${error.message || 'Registry error'}`,
        details,
      };
    }
  }

  if (error instanceof Error) {
    details.message = error.message;
    details.stack = error.stack;
    return {
      message: error.message || 'Unknown error',
      details,
    };
  }

  return {
    message: String(error) || 'Unknown error',
    details,
  };
}
