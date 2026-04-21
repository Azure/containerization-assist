/**
 * Docker context resolution utility.
 *
 * Provides a function to resolve a Docker CLI context name to its
 * daemon endpoint, enabling tools to target specific Docker daemons.
 *
 * Docker contexts are managed by the Docker CLI (`docker context`) and each
 * context maps to a Docker daemon endpoint (Unix socket or TCP URL).
 */

import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import type { Logger } from 'pino';

import { extractErrorMessage } from '@/lib/errors';
import { Result, Success, Failure } from '@/types';

const execFileAsync = promisify(execFile);

/**
 * Resolve a Docker context name to its daemon endpoint.
 *
 * Returns the Docker endpoint (socket path or TCP URL) for the specified context.
 * The endpoint can be used as DOCKER_HOST or to create a targeted Docker client.
 *
 * @param contextName - Name of the Docker context to resolve
 * @param logger - Logger instance for debug output
 * @returns Result containing the Docker endpoint string (e.g., "unix:///var/run/docker.sock")
 */
export async function resolveDockerContext(
  contextName: string,
  logger: Logger,
): Promise<Result<string>> {
  try {
    const { stdout } = await execFileAsync(
      'docker',
      ['context', 'inspect', contextName, '--format', '{{.Endpoints.docker.Host}}'],
      { timeout: 10000 },
    );

    const endpoint = stdout.trim();
    if (!endpoint) {
      return Failure(`Docker context '${contextName}' has no endpoint`, {
        message: `Context '${contextName}' does not have a Docker endpoint configured`,
        hint: 'The context may be misconfigured or incomplete',
        resolution: `Run "docker context inspect ${contextName}" to check its configuration`,
      });
    }

    logger.debug({ contextName, endpoint }, 'Resolved Docker context endpoint');
    return Success(endpoint);
  } catch (error) {
    const errorMessage = extractErrorMessage(error);
    logger.debug({ error: errorMessage, contextName }, 'Failed to resolve Docker context');

    return Failure(`Failed to resolve Docker context '${contextName}': ${errorMessage}`, {
      message: `Docker context '${contextName}' could not be resolved`,
      hint: 'The context name may be incorrect or Docker CLI may not be available',
      resolution: `Run "docker context ls" to see available contexts. Available context names can be used with the context parameter.`,
    });
  }
}
