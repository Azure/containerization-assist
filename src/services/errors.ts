/**
 * Docker error handling utilities leveraging dockerode's native error structure
 */

/**
 * Interface for dockerode error objects
 * Based on dockerode's actual error structure
 */
interface DockerodeError extends Error {
  statusCode?: number;
  json?: Record<string, unknown>;
  reason?: string;
  code?: string;
}

/**
 * Type guard to check if an error has dockerode-specific properties
 */
function hasDockerodeProperties(error: Error): error is DockerodeError {
  const dockerError = error as DockerodeError;
  return (
    typeof dockerError.statusCode === 'number' ||
    typeof dockerError.json === 'object' ||
    typeof dockerError.reason === 'string' ||
    typeof dockerError.code === 'string'
  );
}

/**
 * Extract meaningful error message from dockerode errors
 * The goal is to provide specific, actionable error messages to the LLM instead of generic "Unknown error"
 */
export function extractDockerErrorMessage(error: unknown): {
  message: string;
  details: Record<string, unknown>;
} {
  const details: Record<string, unknown> = {};

  if (error instanceof Error) {
    // Check if this error has dockerode-specific properties
    if (hasDockerodeProperties(error)) {
      // Capture all available properties for debugging
      // Note: Sensitive data will be redacted by Pino logger's built-in redaction
      if (error.statusCode) details.statusCode = error.statusCode;
      if (error.json) details.json = error.json;
      if (error.reason) details.reason = error.reason;
      if (error.code) details.code = error.code;

      // Provide specific, meaningful messages based on error patterns

      // Network connectivity issues (most fundamental)
      if (error.code === 'ENOTFOUND') {
        return {
          message: `Network error: Cannot resolve Docker registry hostname - ${error.message}`,
          details,
        };
      }

      if (error.code === 'ECONNREFUSED') {
        return {
          message: `Network error: Connection refused to Docker registry - ${error.message}`,
          details,
        };
      }

      // HTTP status code errors with specific meanings
      if (error.statusCode === 401) {
        return {
          message: `Authentication error: Invalid registry credentials - ${error.message}`,
          details,
        };
      }

      if (error.statusCode === 403) {
        return {
          message: `Authorization error: Access denied to registry resource - ${error.message}`,
          details,
        };
      }

      if (error.statusCode === 404) {
        return {
          message: `Image not found: The requested image or tag does not exist - ${error.message}`,
          details,
        };
      }

      if (error.statusCode && error.statusCode >= 500) {
        return {
          message: `Registry server error (${error.statusCode}): The Docker registry is experiencing issues - ${error.message}`,
          details,
        };
      }

      // Any other HTTP error
      if (error.statusCode) {
        return {
          message: `Registry error (HTTP ${error.statusCode}): ${error.message}`,
          details,
        };
      }
    }

    // For regular errors, provide the message or a meaningful fallback
    return {
      message: error.message || 'Docker operation failed with unknown error',
      details,
    };
  }

  // Last resort for non-Error objects
  return {
    message: String(error) || 'Unknown Docker error occurred',
    details,
  };
}
