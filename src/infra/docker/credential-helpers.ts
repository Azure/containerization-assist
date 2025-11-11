/**
 * Docker Credential Helper Integration
 *
 * Provides integration with Docker credential helpers to automatically
 * retrieve authentication credentials for registries, similar to how
 * Docker CLI works with `az acr login` and other credential providers.
 */

import { readFile } from 'fs/promises';
import { homedir } from 'os';
import { join } from 'path';
import { spawn } from 'child_process';
import type { Logger } from 'pino';
import { Success, Failure, type Result } from '@/types';

/**
 * Docker configuration structure from ~/.docker/config.json
 */
export interface DockerConfig {
  auths?: Record<string, { auth?: string; username?: string; password?: string }>;
  credsStore?: string;
  credHelpers?: Record<string, string>;
}

/**
 * Credentials returned by credential helpers
 */
export interface CredentialHelperResult {
  ServerURL: string;
  Username: string;
  Secret: string;
}

/**
 * Authentication configuration for Dockerode
 */
export interface DockerAuthConfig {
  username: string;
  password: string;
  serveraddress: string;
}

/**
 * Read and parse Docker configuration file
 */
async function readDockerConfig(logger: Logger): Promise<Result<DockerConfig>> {
  try {
    const configPath = join(homedir(), '.docker', 'config.json');
    const configContent = await readFile(configPath, 'utf-8');
    const config = JSON.parse(configContent) as DockerConfig;

    logger.debug({ configPath, hasCredsStore: !!config.credsStore }, 'Read Docker config');
    return Success(config);
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : String(error);

    // If config file doesn't exist, return empty config (not an error)
    if (errorMessage.includes('ENOENT') || errorMessage.includes('no such file')) {
      logger.debug('Docker config file not found, using empty config');
      return Success({});
    }

    return Failure(`Failed to read Docker config: ${errorMessage}`, {
      message: 'Unable to read Docker configuration',
      hint: 'Docker config file is corrupted or inaccessible',
      resolution: 'Check ~/.docker/config.json file permissions and format',
      details: { error: errorMessage },
    });
  }
}

function normalizeRegistryHostname(registry: string): string {
  let hostname: string;

  try {
    const urlString = registry.includes('://') ? registry : `https://${registry}`;
    const url = new URL(urlString);
    hostname = url.hostname ?? registry;
  } catch {
    hostname = (registry
      .replace(/^https?:\/\//, '')
      .split('/')[0]
      ?.split('?')[0]
      ?.split('#')[0]
      ?.split(':')[0]) ?? registry;
  }

  hostname = hostname.toLowerCase().trim();

  if (
    hostname === 'docker.io' ||
    hostname === 'index.docker.io' ||
    hostname === 'registry-1.docker.io' ||
    hostname === 'registry.hub.docker.com'
  ) {
    return 'docker.io';
  }

  return hostname;
}

function isAzureACR(serverUrl: string): boolean {
  let hostname: string;

  try {
    const urlString = serverUrl.includes('://') ? serverUrl : `https://${serverUrl}`;
    const url = new URL(urlString);
    hostname = url.hostname ?? serverUrl;
  } catch {
    hostname = (serverUrl
      .replace(/^https?:\/\//, '')
      .split('/')[0]
      ?.split(':')[0]) ?? serverUrl;
  }

  hostname = hostname.toLowerCase().trim();

  return hostname.endsWith('.azurecr.io') && hostname.length > '.azurecr.io'.length;
}

/**
 * Validate that credential helper response matches expected registry
 *
 * SECURITY: Prevents credential leakage when helper returns credentials
 * for a different host than requested. This protects against malicious
 * credential helpers or DNS rebinding attacks.
 */
function validateCredentialHelperResponse(
  expectedRegistry: string,
  credentialResponse: CredentialHelperResult,
  logger: Logger,
): boolean {
  const normalizedExpected = normalizeRegistryHostname(expectedRegistry);
  const normalizedResponse = normalizeRegistryHostname(credentialResponse.ServerURL);

  if (normalizedExpected !== normalizedResponse) {
    logger.warn(
      {
        expected: normalizedExpected,
        received: normalizedResponse,
        serverUrl: credentialResponse.ServerURL,
      },
      'SECURITY: Credential helper returned credentials for different host than requested',
    );
    return false;
  }

  return true;
}

/**
 * Execute a credential helper command
 */
async function executeCredentialHelper(
  helperName: string,
  serverUrl: string,
  logger: Logger,
): Promise<Result<CredentialHelperResult>> {
  return new Promise((resolve) => {
    const helperCommand = `docker-credential-${helperName}`;

    logger.debug({ helperCommand, serverUrl }, 'Executing credential helper');

    const child = spawn(helperCommand, ['get'], {
      stdio: ['pipe', 'pipe', 'pipe'],
      timeout: 10000, // 10 second timeout
    });

    let stdout = '';
    let stderr = '';

    child.stdout?.on('data', (data) => {
      stdout += data.toString();
    });

    child.stderr?.on('data', (data) => {
      stderr += data.toString();
    });

    child.on('error', (error) => {
      const errorMessage = error.message;

      if (errorMessage.includes('ENOENT') || errorMessage.includes('command not found')) {
        resolve(Failure(`Credential helper not found: ${helperCommand}`, {
          message: 'Docker credential helper not installed',
          hint: `The credential helper '${helperCommand}' is not available`,
          resolution: 'Install the required credential helper or use explicit credentials',
          details: { helperName, serverUrl },
        }));
      } else {
        resolve(Failure(`Credential helper failed: ${errorMessage}`, {
          message: 'Failed to retrieve credentials from helper',
          hint: 'Credential helper execution failed',
          resolution: 'Check credential helper installation and registry authentication',
          details: { helperName, serverUrl, error: errorMessage },
        }));
      }
    });

    child.on('close', (code) => {
      if (code !== 0) {
        const errorMessage = stderr || `Process exited with code ${code}`;

        if (errorMessage.includes('credentials not found') || errorMessage.includes('not logged in')) {
          resolve(Failure('No credentials found for registry', {
            message: 'Registry credentials not found in credential store',
            hint: 'You may need to log in to the registry first',
            resolution: 'Run the appropriate login command (e.g., az acr login, docker login)',
            details: { helperName, serverUrl },
          }));
        } else {
          resolve(Failure(`Credential helper failed: ${errorMessage}`, {
            message: 'Failed to retrieve credentials from helper',
            hint: 'Credential helper execution failed',
            resolution: 'Check credential helper installation and registry authentication',
            details: { helperName, serverUrl, error: errorMessage },
          }));
        }
        return;
      }

      if (stderr) {
        logger.warn({ stderr, helperCommand }, 'Credential helper produced stderr output');
      }

      try {
        const result = JSON.parse(stdout) as CredentialHelperResult;

        // Validate the result structure
        if (!result.Username || !result.Secret || !result.ServerURL) {
          resolve(Failure('Invalid credential helper response', {
            message: 'Credential helper returned incomplete credentials',
            hint: 'Credential helper response missing required fields',
            resolution: 'Check credential helper configuration and re-run authentication (e.g., az acr login)',
            details: { helperCommand, serverUrl, result },
          }));
          return;
        }

        logger.debug({ helperCommand, serverUrl, username: result.Username }, 'Credential helper executed successfully');
        resolve(Success(result));
      } catch (parseError) {
        const errorMessage = parseError instanceof Error ? parseError.message : String(parseError);
        resolve(Failure(`Failed to parse credential helper response: ${errorMessage}`, {
          message: 'Invalid JSON response from credential helper',
          hint: 'Credential helper returned malformed JSON',
          resolution: 'Check credential helper configuration and try re-authenticating',
          details: { helperCommand, serverUrl, stdout, error: errorMessage },
        }));
      }
    });

    // Write the server URL to stdin and close it
    child.stdin?.write(serverUrl);
    child.stdin?.end();
  });
}

/**
 * Get credentials for a registry using Docker credential helpers
 */
export async function getRegistryCredentials(
  registry: string,
  logger: Logger,
): Promise<Result<DockerAuthConfig | null>> {
  try {
    // SECURITY: Validate input to prevent malicious registry hostnames
    if (!registry || typeof registry !== 'string') {
      return Failure('Invalid registry hostname', {
        message: 'Registry hostname must be a non-empty string',
        hint: 'Registry parameter is missing or invalid',
        resolution: 'Provide a valid registry hostname',
        details: { registry },
      });
    }

    // SECURITY: Enforce reasonable length limits
    if (registry.length > 255) {
      return Failure('Invalid registry hostname', {
        message: 'Registry hostname exceeds maximum length',
        hint: 'Hostname must be 255 characters or less',
        resolution: 'Provide a valid registry hostname',
        details: { registry: `${registry.substring(0, 100)}...` },
      });
    }

    // Read Docker configuration
    const configResult = await readDockerConfig(logger);
    if (!configResult.ok) {
      return configResult;
    }

    const config = configResult.value;
    const normalizedRegistry = normalizeRegistryHostname(registry);

    logger.debug({ registry, normalizedRegistry }, 'Looking up credentials for registry');

    // Check if there are explicit credentials in auths section
    if (config.auths?.[normalizedRegistry]) {
      const auth = config.auths[normalizedRegistry];
      if (auth.username && auth.password) {
        logger.debug({ registry: normalizedRegistry }, 'Using explicit credentials from config');
        return Success({
          username: auth.username,
          password: auth.password,
          serveraddress: normalizedRegistry,
        });
      }

      // Handle base64 encoded auth
      if (auth.auth) {
        try {
          const decoded = Buffer.from(auth.auth, 'base64').toString('utf-8');
          const [username, password] = decoded.split(':', 2);
          if (username && password) {
            logger.debug({ registry: normalizedRegistry }, 'Using base64 encoded credentials from config');
            return Success({
              username,
              password,
              serveraddress: normalizedRegistry,
            });
          }
        } catch (decodeError) {
          logger.warn({ decodeError, registry: normalizedRegistry }, 'Failed to decode base64 auth');
        }
      }
    }

    // Check for registry-specific credential helper
    if (config.credHelpers?.[normalizedRegistry]) {
      const helperName = config.credHelpers[normalizedRegistry];
      logger.debug({ registry: normalizedRegistry, helperName }, 'Using registry-specific credential helper');

      const credResult = await executeCredentialHelper(helperName, normalizedRegistry, logger);
      if (credResult.ok) {
        const creds = credResult.value;

        // SECURITY: Validate that credential helper returned credentials for the correct host
        if (!validateCredentialHelperResponse(normalizedRegistry, creds, logger)) {
          // Don't use mismatched credentials, continue to next method
          logger.debug('Skipping mismatched credentials from registry-specific helper');
        } else {
          // Format serveraddress correctly for different registry types
          let serveraddress = creds.ServerURL;

          // SECURITY: Azure ACR requires https:// protocol for authentication
          // Use secure suffix matching instead of substring matching
          if (isAzureACR(serveraddress) && !serveraddress.startsWith('https://')) {
            serveraddress = `https://${serveraddress}`;
          }

          return Success({
            username: creds.Username,
            password: creds.Secret,
            serveraddress,
          });
        }
      } else {
        // Log the credential helper failure but continue to try global helper
        logger.debug({ error: credResult.error }, 'Registry-specific credential helper failed, trying global helper');
      }
    }

    // Check for global credential store
    if (config.credsStore) {
      const helperName = config.credsStore;
      logger.debug({ registry: normalizedRegistry, helperName }, 'Using global credential store');

      const credResult = await executeCredentialHelper(helperName, normalizedRegistry, logger);
      if (credResult.ok) {
        const creds = credResult.value;

        // SECURITY: Validate that credential helper returned credentials for the correct host
        if (!validateCredentialHelperResponse(normalizedRegistry, creds, logger)) {
          // Don't use mismatched credentials
          logger.debug('Skipping mismatched credentials from global credential store');
        } else {
          // Format serveraddress correctly for different registry types
          let serveraddress = creds.ServerURL;

          // SECURITY: Azure ACR requires https:// protocol for authentication
          // Use secure suffix matching instead of substring matching
          if (isAzureACR(serveraddress) && !serveraddress.startsWith('https://')) {
            serveraddress = `https://${serveraddress}`;
          }

          return Success({
            username: creds.Username,
            password: creds.Secret,
            serveraddress,
          });
        }
      } else {
        // If credential helper fails, log it but don't return error
        // The caller can decide whether to proceed without credentials
        logger.debug({ error: credResult.error }, 'Global credential store failed');
      }
    }

    // No credentials found, but this is not necessarily an error
    logger.debug({ registry: normalizedRegistry }, 'No credentials found for registry');
    return Success(null);
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : String(error);
    return Failure(`Failed to get registry credentials: ${errorMessage}`, {
      message: 'Error retrieving registry credentials',
      hint: 'Failed to access Docker credential system',
      resolution: 'Check Docker configuration and credential helper setup',
      details: { registry, error: errorMessage },
    });
  }
}
