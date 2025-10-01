/**
 * Docker error handling utilities leveraging dockerode's native error structure
 */

import type { ErrorGuidance } from '@/types';

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
 * Extract error with actionable guidance for operators
 */
export function extractDockerErrorGuidance(error: unknown): ErrorGuidance {
  const details: Record<string, unknown> = {};

  if (error instanceof Error) {
    // Check if this error has dockerode-specific properties
    if (hasDockerodeProperties(error)) {
      // Capture all available properties for debugging
      if (error.statusCode) details.statusCode = error.statusCode;
      if (error.json) details.json = error.json;
      if (error.reason) details.reason = error.reason;
      if (error.code) details.code = error.code;

      // Extract detailed error message from json field or reason if available (Docker API errors)
      let detailedMessage = error.message;

      // Try to get detailed message from json.message field
      if (error.json && typeof error.json === 'object' && 'message' in error.json) {
        const jsonMessage = String(error.json.message);
        if (jsonMessage && jsonMessage.length > 0) {
          detailedMessage = jsonMessage;
        }
      }

      // If json is null but we have a reason, use that
      if (!detailedMessage || (error.reason && error.reason.length > detailedMessage.length)) {
        detailedMessage = error.reason || detailedMessage;
      }

      // Network connectivity issues (most fundamental)
      if (error.code === 'ENOTFOUND') {
        return {
          message: `Cannot resolve Docker registry hostname`,
          hint: 'The Docker registry hostname could not be found in DNS',
          resolution:
            'Check your internet connection and verify the registry URL is correct. For private registries, ensure DNS is configured properly.',
          details,
        };
      }

      if (error.code === 'ECONNREFUSED') {
        return {
          message: `Docker daemon is not available`,
          hint: 'Connection to Docker daemon was refused',
          resolution:
            'Ensure Docker is installed and running: `docker ps` should succeed. Check Docker daemon logs if the service is running.',
          details,
        };
      }

      if (error.code === 'ETIMEDOUT') {
        return {
          message: `Docker operation timed out`,
          hint: 'The operation took too long and was cancelled',
          resolution:
            'Check network connectivity, firewall rules, or try again. For large images, consider increasing timeout settings.',
          details,
        };
      }

      if (error.code === 'ECONNRESET') {
        return {
          message: `Connection reset - Connection was forcibly closed`,
          hint: 'The Docker daemon or registry closed the connection unexpectedly',
          resolution:
            'Retry the operation. If it persists, check Docker daemon health and network stability.',
          details,
        };
      }

      if (error.code === 'EAI_AGAIN') {
        return {
          message: `DNS lookup failed - Temporary failure in name resolution`,
          hint: 'Temporary DNS resolution issue',
          resolution:
            'Retry the operation. If it persists, check your DNS configuration and network connectivity.',
          details,
        };
      }

      if (error.code === 'EHOSTUNREACH') {
        return {
          message: `Host unreachable - Cannot reach the Docker registry host`,
          hint: 'No network route to the Docker registry host',
          resolution:
            'Check firewall rules, VPN connection, and network configuration. Verify the registry is accessible from your network.',
          details,
        };
      }

      if (error.code === 'ENETUNREACH') {
        return {
          message: `Network unreachable - No route to the Docker registry network`,
          hint: 'No network route to the Docker registry',
          resolution:
            'Check firewall rules, VPN connection, and network configuration. Verify the registry is accessible from your network.',
          details,
        };
      }

      if (error.code === 'EPIPE') {
        return {
          message: `Broken pipe - Connection was unexpectedly closed`,
          hint: 'Docker daemon or registry closed the connection during operation',
          resolution: 'Retry the operation. Check Docker daemon logs for issues.',
          details,
        };
      }

      // HTTP status code errors with specific meanings
      if (error.statusCode === 401) {
        return {
          message: `Docker registry authentication failed`,
          hint: 'Invalid or missing registry credentials',
          resolution:
            'Run `docker login <registry>` to authenticate, or verify credentials in your Docker config (~/.docker/config.json).',
          details,
        };
      }

      if (error.statusCode === 403) {
        return {
          message: `Authorization error - Access denied to registry resource`,
          hint: 'Your credentials lack permission for this operation',
          resolution:
            'Verify you have push/pull permissions for this repository. Contact registry administrator if needed.',
          details,
        };
      }

      if (error.statusCode === 404) {
        return {
          message: `Image or tag not found`,
          hint: 'The requested image or tag does not exist in the registry',
          resolution:
            'Verify the image name and tag are correct. Use `docker images` to list local images or check registry catalog.',
          details,
        };
      }

      if (error.statusCode && error.statusCode >= 500) {
        return {
          message:
            detailedMessage ||
            `Registry server error (${error.statusCode}) - The registry is experiencing issues`,
          hint: `Registry returned HTTP ${error.statusCode} - server-side issue`,
          resolution:
            'The registry is experiencing problems. Retry after a moment or check registry status page.',
          details,
        };
      }

      // Any other HTTP error
      if (error.statusCode) {
        const guidance: ErrorGuidance = {
          message: detailedMessage || `Registry error (HTTP ${error.statusCode})`,
          resolution: 'Check Docker registry documentation for this error code.',
          details,
        };
        if (error.message && error.message !== detailedMessage) {
          guidance.hint = error.message;
        }
        return guidance;
      }
    }

    // Check for Docker daemon availability
    if (error.message.includes('connect ENOENT')) {
      return {
        message: 'Docker daemon is not running',
        hint: 'Cannot connect to Docker socket',
        resolution:
          'Start Docker daemon: `sudo systemctl start docker` (Linux) or start Docker Desktop (Mac/Windows).',
        details,
      };
    }

    // Check for Dockerfile syntax errors
    if (error.message.match(/unknown instruction/i)) {
      return {
        message: error.message,
        hint: 'Invalid Dockerfile instruction detected',
        resolution:
          'Check your Dockerfile for syntax errors. Valid instructions include FROM, RUN, COPY, ADD, CMD, ENTRYPOINT, etc.',
        details,
      };
    }

    if (error.message.match(/dockerfile parse error|dockerfile must begin with|no build stage/i)) {
      return {
        message: error.message,
        hint: 'Dockerfile structure is invalid',
        resolution:
          'Ensure your Dockerfile starts with a FROM instruction and follows proper syntax.',
        details,
      };
    }

    // Check for missing Dockerfile
    if (error.message.match(/cannot find|no such file|ENOENT.*dockerfile/i)) {
      return {
        message: error.message,
        hint: 'Dockerfile not found in the specified location',
        resolution:
          'Verify the Dockerfile path is correct and the file exists in the build context.',
        details,
      };
    }

    // For regular errors, provide the message
    return {
      message: error.message || 'Docker operation failed',
      hint: 'An error occurred during the Docker operation',
      resolution: 'Check Docker daemon logs and ensure Docker is functioning correctly.',
      details,
    };
  }

  // Last resort for non-Error objects
  return {
    message: String(error) || 'Unknown Docker error',
    hint: 'An unexpected error occurred',
    resolution: 'Check Docker daemon status and logs for more information.',
    details,
  };
}

/**
 * Extract meaningful error message from dockerode errors
 * Wraps extractDockerErrorGuidance for backwards compatibility
 */
export function extractDockerErrorMessage(error: unknown): {
  message: string;
  details: Record<string, unknown>;
} {
  const guidance = extractDockerErrorGuidance(error);
  return {
    message: guidance.message,
    details: guidance.details || {},
  };
}
