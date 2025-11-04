/**
 * Port utility functions for finding and checking available ports
 */

import { DOCKER } from '@/config';
import { exec } from 'node:child_process';
import { promisify } from 'node:util';

const execAsync = promisify(exec);

/**
 * Check if a port is available on the host
 */
export async function isPortAvailable(port: number): Promise<boolean> {
  try {
    // Try to check if port is in use using lsof (Unix/Mac) or netstat (Windows)
    const platform = process.platform;

    if (platform === 'win32') {
      // Windows: use netstat
      const { stdout } = await execAsync(`netstat -ano | findstr :${port}`);
      // If command succeeds and has output, port is in use
      return !stdout.trim();
    } else {
      // Unix/Mac: use lsof
      try {
        await execAsync(`lsof -i :${port} -sTCP:LISTEN -t`);
        // If command succeeds, port is in use
        return false;
      } catch {
        // If lsof fails, port is available
        return true;
      }
    }
  } catch {
    // If there's an error or no output, assume port is available
    return true;
  }
}

/**
 * Find an available port starting from startPort and checking up to endPort
 * Returns the first available port found, or null if none are available
 */
export async function findAvailablePort(startPort: number, endPort: number): Promise<number | null> {
  for (let port = startPort; port <= endPort; port++) {
    if (await isPortAvailable(port)) {
      return port;
    }
  }
  return null;
}

/**
 * Find an available port for the local registry
 * Searches from port 6000 to 60100
 * Returns the first available port or throws an error if none are available
 */
export async function findRegistryPort(): Promise<number> {
  const startPort = DOCKER.REGISTRY_PORT_START;
  const endPort = DOCKER.REGISTRY_PORT_END;

  const port = await findAvailablePort(startPort, endPort);

  if (port === null) {
    throw new Error(
      `No available ports found in range ${startPort}-${endPort}. Please free up some ports and try again.`,
    );
  }

  return port;
}
