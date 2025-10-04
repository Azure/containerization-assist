/**
 * Docker socket validation and auto-detection module
 */

import { existsSync, statSync } from 'fs';
import { homedir } from 'os';
import { join } from 'path';
import { extractErrorMessage } from '@/lib/error-utils';

/**
 * Result of Docker socket validation
 */
export interface SocketValidationResult {
  /** The validated Docker socket path (empty string if invalid) */
  dockerSocket: string;
  /** Warning messages encountered during validation */
  warnings: string[];
}

/**
 * Get Colima socket paths in order of preference.
 */
function getColimaSockets(): string[] {
  const homeDir = homedir();
  return [
    join(homeDir, '.colima/default/docker.sock'),
    join(homeDir, '.colima/docker/docker.sock'),
    join(homeDir, '.lima/colima/sock/docker.sock'), // Lima-based colima
  ];
}

/**
 * Find the first available Docker socket from the given paths (synchronous file-based check).
 */
function findAvailableDockerSocket(socketPaths: string[]): string | null {
  for (const socketPath of socketPaths) {
    try {
      if (existsSync(socketPath)) {
        const stat = statSync(socketPath);
        if (stat.isSocket()) {
          return socketPath;
        }
      }
    } catch {
      // Continue to next socket path
    }
  }
  return null;
}

/**
 * Auto-detect Docker socket path with Colima support (synchronous).
 * Exported for use in other modules that need to detect the socket path.
 */
export function autoDetectDockerSocket(): string {
  // If Windows, use default Windows socket
  if (process.platform === 'win32') {
    return 'npipe://./pipe/docker_engine'; // Windows default pipe, not a socket
  }

  const unixDefaultPaths = [
    '/var/run/docker.sock', // Standard Unix Docker socket
    ...getColimaSockets(), // Colima sockets
  ];

  const availableSocket = findAvailableDockerSocket(unixDefaultPaths);
  return availableSocket || '/var/run/docker.sock'; // Fallback to default
}

/**
 * Determine if output should be logged based on quiet mode and MCP environment
 */
function shouldLogOutput(quiet: boolean): boolean {
  return !quiet && !process.env.MCP_MODE && !process.env.MCP_QUIET;
}

/**
 * Validate Docker socket path and provide warnings if invalid.
 * Handles Windows named pipes, Unix sockets, and provides user-friendly error messages.
 *
 * @param options - Options object containing optional dockerSocket property
 * @param options.dockerSocket - Explicit Docker socket path (CLI option)
 * @param quiet - If true, suppress console output (default: false)
 * @returns Validation result with socket path and any warnings
 */
export function validateDockerSocket(
  options: { dockerSocket?: string },
  quiet = false,
): SocketValidationResult {
  const warnings: string[] = [];
  let dockerSocket = '';
  const defaultDockerSocket = autoDetectDockerSocket();

  // Priority order: CLI option -> Environment variable -> Default
  if (options.dockerSocket) {
    dockerSocket = options.dockerSocket;
  } else if (process.env.DOCKER_SOCKET) {
    dockerSocket = process.env.DOCKER_SOCKET;
  } else {
    dockerSocket = defaultDockerSocket;
  }

  // Validate the selected socket
  try {
    // Handle Windows named pipes specially - they can't be stat()'d
    if (dockerSocket.includes('pipe')) {
      // For Windows named pipes, assume they're valid and let Docker client handle validation
      if (shouldLogOutput(quiet)) {
        console.error(`✅ Using Docker named pipe: ${dockerSocket}`);
      }
      return { dockerSocket, warnings };
    }

    // For Unix sockets and other paths, check if they exist and are valid
    const stat = statSync(dockerSocket);
    if (!stat.isSocket()) {
      warnings.push(`${dockerSocket} exists but is not a socket`);
      return {
        dockerSocket: '',
        warnings: [
          ...warnings,
          'No valid Docker socket found',
          'Docker operations require a valid Docker connection',
          'Consider: 1) Starting Docker Desktop, 2) Specifying --docker-socket <path>',
        ],
      };
    }

    // Only log when not in quiet mode or pure MCP mode
    if (shouldLogOutput(quiet)) {
      console.error(`✅ Using Docker socket: ${dockerSocket}`);
    }
  } catch (error) {
    const errorMsg = extractErrorMessage(error);
    warnings.push(`Cannot access Docker socket: ${dockerSocket} - ${errorMsg}`);
    return {
      dockerSocket: '',
      warnings: [
        ...warnings,
        'No valid Docker socket found',
        'Docker operations require a valid Docker connection',
        'Consider: 1) Starting Docker Desktop, 2) Specifying --docker-socket <path>',
      ],
    };
  }

  return { dockerSocket, warnings };
}
