/**
 * Docker socket validation and auto-detection module.
 * Supports Docker Engine, Docker Desktop, Colima, OrbStack, Rancher Desktop,
 * Podman, rootless Docker, Snap Docker, and DOCKER_HOST env var.
 */

import { existsSync, statSync } from 'fs';
import { homedir } from 'os';
import { join } from 'path';
import { extractErrorMessage } from '@/lib/errors';

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
 * Parsed result from a DOCKER_HOST value.
 */
export interface ParsedDockerHost {
  type: 'unix' | 'tcp' | 'npipe';
  /** Socket path for unix/npipe, or full host string for tcp */
  value: string;
  /** Extracted host for tcp connections */
  host?: string;
  /** Extracted port for tcp connections */
  port?: number;
}

/**
 * Parse a DOCKER_HOST value into a structured result.
 * Handles: unix://, tcp://, http://, https://, npipe://, raw paths.
 * Throws on unsupported schemes (e.g. fd://, ssh://).
 */
export function parseDockerHost(value: string): ParsedDockerHost {
  const trimmed = value.trim();
  if (!trimmed) {
    throw new Error('DOCKER_HOST is empty');
  }

  // unix:// scheme
  if (trimmed.startsWith('unix://')) {
    const path = trimmed.slice('unix://'.length);
    if (!path) throw new Error('DOCKER_HOST unix:// has no path');
    return { type: 'unix', value: path };
  }

  // npipe:// scheme (Windows named pipes)
  if (trimmed.startsWith('npipe://')) {
    const path = trimmed.slice('npipe://'.length);
    if (!path) throw new Error('DOCKER_HOST npipe:// has no path');
    return { type: 'npipe', value: path };
  }

  // tcp://, http://, https:// schemes → TCP connection
  if (/^(tcp|https?):/.test(trimmed)) {
    try {
      // Replace tcp:// with http:// for URL parsing
      const urlStr = trimmed.replace(/^tcp:/, 'http:');
      const url = new URL(urlStr);
      const host = url.hostname || 'localhost';
      const port = url.port ? parseInt(url.port, 10) : trimmed.startsWith('https:') ? 2376 : 2375;
      return { type: 'tcp', value: trimmed, host, port };
    } catch {
      throw new Error(`DOCKER_HOST has invalid URL: ${trimmed}`);
    }
  }

  // Unsupported schemes
  if (/^[a-z][a-z0-9+.-]*:/.test(trimmed)) {
    throw new Error(`DOCKER_HOST scheme not supported: ${trimmed}`);
  }

  // Windows pipe raw path (check before generic / prefix since //./pipe/ starts with /)
  if (trimmed.startsWith('//./pipe/') || trimmed.startsWith('\\\\.\\pipe\\')) {
    return { type: 'npipe', value: trimmed };
  }

  // Raw path — treat as unix socket
  if (trimmed.startsWith('/') || trimmed.startsWith('~')) {
    return { type: 'unix', value: trimmed };
  }

  throw new Error(`DOCKER_HOST value not recognized: ${trimmed}`);
}

/**
 * Get all candidate Unix socket paths for Docker-compatible runtimes,
 * ordered by popularity / likelihood.
 */
function getUnixSocketPaths(): string[] {
  const home = homedir();
  const paths: string[] = [
    // Standard Docker Engine
    '/var/run/docker.sock',
    // Docker Desktop (macOS 4.13+, Linux)
    join(home, '.docker/run/docker.sock'),
    join(home, '.docker/desktop/docker.sock'),
    // OrbStack
    join(home, '.orbstack/run/docker.sock'),
    // Rancher Desktop
    join(home, '.rd/docker.sock'),
    // Colima
    join(home, '.colima/default/docker.sock'),
    join(home, '.colima/docker/docker.sock'),
    join(home, '.lima/colima/sock/docker.sock'),
  ];

  // Rootless Docker / Podman — need UID, which is unavailable on Windows
  if (process.platform !== 'win32') {
    const xdgRuntime = process.env.XDG_RUNTIME_DIR;
    const uid = typeof process.getuid === 'function' ? String(process.getuid()) : undefined;

    if (xdgRuntime) {
      paths.push(join(xdgRuntime, 'docker.sock'), join(xdgRuntime, 'podman/podman.sock'));
    }
    if (uid) {
      paths.push(`/run/user/${uid}/docker.sock`);
      paths.push(`/run/user/${uid}/podman/podman.sock`);
    }

    // Podman machine (macOS)
    paths.push(join(home, '.local/share/containers/podman/machine/podman.sock'));
    // Podman rootful
    paths.push('/run/podman/podman.sock');
    // Alt Linux location
    paths.push('/run/docker.sock');
    // Snap-installed Docker
    paths.push('/run/snap.docker/docker.sock');
  }

  return paths;
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
 * Auto-detect Docker socket path with broad runtime support (synchronous).
 * Checks DOCKER_HOST env var, then probes known socket paths.
 * Exported for use in other modules that need to detect the socket path.
 */
export function autoDetectDockerSocket(): string {
  // If Windows, use default Windows pipe
  if (process.platform === 'win32') {
    return '//./pipe/docker_engine';
  }

  // Check DOCKER_HOST env var (lower priority than DOCKER_SOCKET, which is handled by validateDockerSocket)
  const dockerHost = process.env.DOCKER_HOST;
  if (dockerHost) {
    try {
      const parsed = parseDockerHost(dockerHost);
      // For unix sockets, verify the file exists before committing
      if (parsed.type === 'unix') {
        try {
          const stat = statSync(parsed.value);
          if (stat.isSocket()) return parsed.value;
        } catch {
          // Socket from DOCKER_HOST doesn't exist — fall through to probe list
        }
      } else {
        // TCP / npipe — return as-is, let Docker client validate connectivity
        return parsed.value;
      }
    } catch {
      // Unparseable DOCKER_HOST — fall through to probe list
    }
  }

  const socketPaths = getUnixSocketPaths();
  const availableSocket = findAvailableDockerSocket(socketPaths);
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
 * Handles Windows named pipes, Unix sockets, TCP connections, and provides user-friendly error messages.
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

  // Priority order: CLI option -> DOCKER_SOCKET env -> DOCKER_HOST env -> auto-detect
  if (options.dockerSocket) {
    dockerSocket = options.dockerSocket;
  } else if (process.env.DOCKER_SOCKET) {
    dockerSocket = process.env.DOCKER_SOCKET;
  } else if (process.env.DOCKER_HOST) {
    try {
      const parsed = parseDockerHost(process.env.DOCKER_HOST);
      // Use the parsed (trimmed) value for all types
      dockerSocket = parsed.value;
    } catch (err) {
      warnings.push(`Invalid DOCKER_HOST: ${extractErrorMessage(err)}`);
      dockerSocket = defaultDockerSocket;
    }
  } else {
    dockerSocket = defaultDockerSocket;
  }

  // Validate the selected socket
  try {
    // Handle Windows named pipes specially - they can't be stat()'d
    if (
      dockerSocket.startsWith('//./pipe/') ||
      dockerSocket.startsWith('\\\\.\\pipe\\') ||
      dockerSocket.startsWith('npipe://')
    ) {
      // For Windows named pipes, assume they're valid and let Docker client handle validation
      if (shouldLogOutput(quiet)) {
        console.error(`✅ Using Docker named pipe: ${dockerSocket}`);
      }
      return { dockerSocket, warnings };
    }

    // Handle TCP connections — skip file-based validation
    if (/^(tcp|https?):/.test(dockerSocket)) {
      if (shouldLogOutput(quiet)) {
        console.error(`✅ Using Docker TCP connection: ${dockerSocket}`);
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
          'Consider: 1) Start Docker Desktop/Rancher Desktop/OrbStack/Podman, 2) Set DOCKER_HOST, 3) Specify --docker-socket <path>',
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
        'Consider: 1) Start Docker Desktop/Rancher Desktop/OrbStack/Podman, 2) Set DOCKER_HOST, 3) Specify --docker-socket <path>',
      ],
    };
  }

  return { dockerSocket, warnings };
}
