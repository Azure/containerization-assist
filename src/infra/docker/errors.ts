/**
 * Docker error handling utilities leveraging dockerode's native error structure
 */

import type { ErrorGuidance } from '@/types';
import {
  createErrorGuidanceBuilder,
  customPattern,
  messagePattern,
  type ErrorPattern,
} from '@/lib/error-guidance';

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
 * Extract detailed error message from dockerode error structure
 */
function extractDockerMessage(error: DockerodeError): string {
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

  return detailedMessage;
}

/**
 * Build details object from dockerode error
 */
function buildDetails(error: Error): Record<string, unknown> {
  const details: Record<string, unknown> = {};

  if (hasDockerodeProperties(error)) {
    if (error.statusCode) details.statusCode = error.statusCode;
    if (error.json) details.json = error.json;
    if (error.reason) details.reason = error.reason;
    if (error.code) details.code = error.code;
  }

  return details;
}

/**
 * Docker error patterns in order of specificity
 * All patterns include details for debugging purposes
 */
const dockerErrorPatterns: ErrorPattern[] = [
  // Network connectivity issues (most fundamental)
  customPattern(
    (error: unknown) => {
      if (!(error instanceof Error)) return false;
      const err = error as Error & { code?: string };
      return err?.code === 'ENOTFOUND';
    },
    (error: unknown) => ({
      message: 'Cannot resolve Docker registry hostname',
      hint: 'The Docker registry hostname could not be found in DNS',
      resolution:
        'Check your internet connection and verify the registry URL is correct. For private registries, ensure DNS is configured properly.',
      details: buildDetails(error as Error),
    }),
  ),

  customPattern(
    (error: unknown) => {
      if (!(error instanceof Error)) return false;
      const err = error as Error & { code?: string };
      return err?.code === 'ECONNREFUSED';
    },
    (error: unknown) => ({
      message: 'Docker daemon is not available',
      hint: 'Connection to Docker daemon was refused',
      resolution:
        'Ensure Docker is installed and running: `docker ps` should succeed. Check Docker daemon logs if the service is running.',
      details: buildDetails(error as Error),
    }),
  ),

  customPattern(
    (error: unknown) => {
      if (!(error instanceof Error)) return false;
      const err = error as Error & { code?: string };
      return err?.code === 'ETIMEDOUT';
    },
    (error: unknown) => ({
      message: 'Docker operation timed out',
      hint: 'The operation took too long and was cancelled',
      resolution:
        'Check network connectivity, firewall rules, or try again. For large images, consider increasing timeout settings.',
      details: buildDetails(error as Error),
    }),
  ),

  customPattern(
    (error: unknown) => {
      if (!(error instanceof Error)) return false;
      const err = error as Error & { code?: string };
      return err?.code === 'ECONNRESET';
    },
    (error: unknown) => ({
      message: 'Connection reset - Connection was forcibly closed',
      hint: 'The Docker daemon or registry closed the connection unexpectedly',
      resolution:
        'Retry the operation. If it persists, check Docker daemon health and network stability.',
      details: buildDetails(error as Error),
    }),
  ),

  customPattern(
    (error: unknown) => {
      if (!(error instanceof Error)) return false;
      const err = error as Error & { code?: string };
      return err?.code === 'EAI_AGAIN';
    },
    (error: unknown) => ({
      message: 'DNS lookup failed - Temporary failure in name resolution',
      hint: 'Temporary DNS resolution issue',
      resolution:
        'Retry the operation. If it persists, check your DNS configuration and network connectivity.',
      details: buildDetails(error as Error),
    }),
  ),

  customPattern(
    (error: unknown) => {
      if (!(error instanceof Error)) return false;
      const err = error as Error & { code?: string };
      return err?.code === 'EHOSTUNREACH';
    },
    (error: unknown) => ({
      message: 'Host unreachable - Cannot reach the Docker registry host',
      hint: 'No network route to the Docker registry host',
      resolution:
        'Check firewall rules, VPN connection, and network configuration. Verify the registry is accessible from your network.',
      details: buildDetails(error as Error),
    }),
  ),

  customPattern(
    (error: unknown) => {
      if (!(error instanceof Error)) return false;
      const err = error as Error & { code?: string };
      return err?.code === 'ENETUNREACH';
    },
    (error: unknown) => ({
      message: 'Network unreachable - No route to the Docker registry network',
      hint: 'No network route to the Docker registry',
      resolution:
        'Check firewall rules, VPN connection, and network configuration. Verify the registry is accessible from your network.',
      details: buildDetails(error as Error),
    }),
  ),

  customPattern(
    (error: unknown) => {
      if (!(error instanceof Error)) return false;
      const err = error as Error & { code?: string };
      return err?.code === 'EPIPE';
    },
    (error: unknown) => ({
      message: 'Broken pipe - Connection was unexpectedly closed',
      hint: 'Docker daemon or registry closed the connection during operation',
      resolution: 'Retry the operation. Check Docker daemon logs for issues.',
      details: buildDetails(error as Error),
    }),
  ),

  // HTTP status code errors with specific meanings
  customPattern(
    (error: unknown) => {
      if (!(error instanceof Error)) return false;
      const err = error as DockerodeError & { response?: { statusCode?: number } };
      return err?.statusCode === 401 || err?.response?.statusCode === 401;
    },
    (error: unknown) => ({
      message: 'Docker registry authentication failed',
      hint: 'Invalid or missing registry credentials',
      resolution:
        'Run `docker login <registry>` to authenticate, or verify credentials in your Docker config (~/.docker/config.json).',
      details: buildDetails(error as Error),
    }),
  ),

  customPattern(
    (error: unknown) => {
      if (!(error instanceof Error)) return false;
      const err = error as DockerodeError & { response?: { statusCode?: number } };
      return err?.statusCode === 403 || err?.response?.statusCode === 403;
    },
    (error: unknown) => ({
      message: 'Authorization error - Access denied to registry resource',
      hint: 'Your credentials lack permission for this operation',
      resolution:
        'Verify you have push/pull permissions for this repository. Contact registry administrator if needed.',
      details: buildDetails(error as Error),
    }),
  ),

  customPattern(
    (error: unknown) => {
      if (!(error instanceof Error)) return false;
      const err = error as DockerodeError & { response?: { statusCode?: number } };
      return err?.statusCode === 404 || err?.response?.statusCode === 404;
    },
    (error: unknown) => ({
      message: 'Image or tag not found',
      hint: 'The requested image or tag does not exist in the registry',
      resolution:
        'Verify the image name and tag are correct. Use `docker images` to list local images or check registry catalog.',
      details: buildDetails(error as Error),
    }),
  ),

  // Server errors (5xx range)
  customPattern(
    (error: unknown) => {
      if (!(error instanceof Error)) return false;
      const err = error as DockerodeError;
      return typeof err.statusCode === 'number' && err.statusCode >= 500 && err.statusCode <= 599;
    },
    (error: unknown) => {
      const dockerError = error as DockerodeError;
      const detailedMessage = hasDockerodeProperties(dockerError)
        ? extractDockerMessage(dockerError)
        : (dockerError as Error).message || 'Registry server error';
      return {
        message:
          detailedMessage ||
          `Registry server error (${dockerError.statusCode}) - The registry is experiencing issues`,
        hint: `Registry returned HTTP ${dockerError.statusCode} - server-side issue`,
        resolution:
          'The registry is experiencing problems. Retry after a moment or check registry status page.',
        details: buildDetails(dockerError as Error),
      };
    },
  ),

  // Any other HTTP error with statusCode
  customPattern(
    (error: unknown) => {
      if (!(error instanceof Error)) return false;
      const err = error as DockerodeError;
      return hasDockerodeProperties(err) && typeof err.statusCode === 'number';
    },
    (error: unknown) => {
      const err = error as DockerodeError;
      const detailedMessage = extractDockerMessage(err);
      const guidance: ErrorGuidance = {
        message: detailedMessage || `Registry error (HTTP ${err.statusCode})`,
        resolution: 'Check Docker registry documentation for this error code.',
        details: buildDetails(err),
      };
      if (err.message && err.message !== detailedMessage) {
        guidance.hint = err.message;
      }
      return guidance;
    },
  ),

  // Docker daemon availability
  messagePattern('connect ENOENT', {
    message: 'Docker daemon is not running',
    hint: 'Cannot connect to Docker socket',
    resolution:
      'Start Docker daemon: `sudo systemctl start docker` (Linux) or start Docker Desktop (Mac/Windows).',
  }),

  // Dockerfile syntax errors
  messagePattern('unknown instruction', {
    message: 'Invalid Dockerfile instruction detected',
    hint: 'Unknown or misspelled Dockerfile instruction',
    resolution:
      'Check your Dockerfile for syntax errors. Valid instructions include FROM, RUN, COPY, ADD, CMD, ENTRYPOINT, etc.',
  }),

  customPattern(
    (error: unknown) => {
      const msg = error instanceof Error ? error.message : String(error);
      return /dockerfile parse error|dockerfile must begin with|no build stage/i.test(msg);
    },
    (error: unknown) => ({
      message: error instanceof Error ? error.message : String(error),
      hint: 'Dockerfile structure is invalid',
      resolution:
        'Ensure your Dockerfile starts with a FROM instruction and follows proper syntax.',
    }),
  ),

  // Missing Dockerfile
  customPattern(
    (error: unknown) => {
      const msg = error instanceof Error ? error.message : String(error);
      return /cannot find|no such file|ENOENT.*dockerfile/i.test(msg);
    },
    (error: unknown) => ({
      message: error instanceof Error ? error.message : String(error),
      hint: 'Dockerfile not found in the specified location',
      resolution: 'Verify the Dockerfile path is correct and the file exists in the build context.',
    }),
  ),
];

/**
 * Default guidance when no pattern matches
 */
function defaultDockerGuidance(error: unknown): ErrorGuidance {
  if (error instanceof Error) {
    return {
      message: error.message || 'Docker operation failed',
      hint: 'An error occurred during the Docker operation',
      resolution: 'Check Docker daemon logs and ensure Docker is functioning correctly.',
      details: buildDetails(error),
    };
  }

  return {
    message: String(error) || 'Unknown Docker error',
    hint: 'An unexpected error occurred',
    resolution: 'Check Docker daemon status and logs for more information.',
    details: {},
  };
}

/**
 * Extract error with actionable guidance for operators
 *
 * @param error - The error to extract guidance from
 * @returns ErrorGuidance with actionable information for operators
 */
export function extractDockerErrorGuidance(error: unknown): ErrorGuidance {
  const baseExtractor = createErrorGuidanceBuilder(dockerErrorPatterns, defaultDockerGuidance);
  return baseExtractor(error);
}
