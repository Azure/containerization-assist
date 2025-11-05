/**
 * Prepare Cluster Tool - Standardized Implementation
 *
 * Prepares and validates Kubernetes cluster for deployment using standardized
 * helpers for consistency and improved error handling
 *
 * @example
 * ```typescript
 * const result = await prepareCluster({
 *   namespace: 'my-app',
 *   environment: 'production'
 * }, context);
 *
 * if (result.success) {
 *   logger.info('Cluster ready', {
 *     ready: result.clusterReady,
 *     checks: result.checks
 *   });
 * }
 * ```
 */

import { setupToolContext } from '@/lib/tool-context-helpers';
import { extractErrorMessage } from '@/lib/errors';
import { validateNamespace } from '@/lib/validation';
import type { ToolContext } from '@/mcp/context';
import { DEFAULT_TIMEOUTS, DOCKER, KUBERNETES } from '@/config/constants';
import {
  createKubernetesClient,
  type K8sManifest,
  type KubernetesClient,
} from '@/infra/kubernetes/client';
import {
  getSystemInfo,
  getDownloadOS,
  getDownloadArch,
  mapNodeArchToPlatform,
} from '@/lib/platform';
import { downloadFile, makeExecutable, createTempFile, deleteTempFile } from '@/lib/file-utils';
import { findRegistryPort } from '@/lib/port-utils';
import type { DockerPlatform } from '@/tools/shared/schemas';

import type * as pino from 'pino';
import { Success, Failure, type Result } from '@/types';
import { prepareClusterSchema, type PrepareClusterParams, RegistryType } from './schema';
import { spawnSync } from 'node:child_process';
import { pluralize } from '@/lib/summary-helpers';

const KIND_VERSION = 'v0.20.0';

/**
 * Validate cluster name to ensure it follows Kubernetes naming conventions.
 * With spawnSync and argument arrays, we no longer need shell escaping.
 *
 * @param clusterName - The cluster name to validate
 * @returns Success with the validated name, or Failure with error details
 */
function validateClusterName(clusterName: string): Result<string> {
  // Kubernetes resource names must be lowercase alphanumeric with dashes
  const nameRegex = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/;

  if (!nameRegex.test(clusterName)) {
    return Failure(
      `Invalid cluster name: "${clusterName}". Must contain only lowercase letters, numbers, and hyphens.`,
      {
        message: `Invalid cluster name: "${clusterName}". Must contain only lowercase letters, numbers, and hyphens.`,
        hint: 'Cluster names must follow Kubernetes naming conventions',
        resolution:
          'Use only lowercase letters (a-z), numbers (0-9), and hyphens (-). Start and end with alphanumeric characters',
      },
    );
  }

  if (clusterName.length > 63) {
    return Failure(`Cluster name too long: "${clusterName}". Must be 63 characters or less.`, {
      message: `Cluster name too long: "${clusterName}". Must be 63 characters or less.`,
      hint: 'Kubernetes resource names have a maximum length of 63 characters',
      resolution: 'Shorten the cluster name to 63 characters or fewer',
    });
  }

  return Success(clusterName);
}

/**
 * Detect the platform architecture of Kubernetes cluster nodes.
 * Returns the detected platform or null if detection fails.
 */
async function detectClusterPlatform(logger: pino.Logger): Promise<DockerPlatform | null> {
  try {
    logger.debug('Detecting cluster node platform...');

    // Get node architecture information
    const archResult = spawnSync('kubectl', [
      'get',
      'nodes',
      '-o',
      "jsonpath={.items[0].status.nodeInfo.architecture}",
    ]);

    if (archResult.error || archResult.status !== 0) {
      logger.warn({ error: archResult.error, stderr: archResult.stderr?.toString() }, 'Could not detect cluster node architecture');
      return null;
    }

    const arch = archResult.stdout.toString().trim();
    if (!arch) {
      logger.warn('Could not detect cluster node architecture');
      return null;
    }

    // Get OS if available (usually linux for Kubernetes)
    let os = 'linux';
    const osResult = spawnSync('kubectl', [
      'get',
      'nodes',
      '-o',
      "jsonpath={.items[0].status.nodeInfo.operatingSystem}",
    ]);

    if (osResult.status === 0 && osResult.stdout) {
      const detectedOS = osResult.stdout.toString().trim().toLowerCase();
      if (detectedOS) {
        os = detectedOS;
      }
    } else {
      logger.debug('Could not detect OS, defaulting to linux');
    }

    const platform = mapNodeArchToPlatform(arch, os);
    logger.debug({ arch, os, platform }, 'Cluster platform detection result');

    return platform;
  } catch (error) {
    logger.warn({ error }, 'Failed to detect cluster platform');
    return null;
  }
}


export interface PrepareClusterResult {
  /**
   * Natural language summary for user display.
   * 1-3 sentences describing the cluster preparation outcome.
   * @example "✅ Cluster prepared. Namespace 'production' created. 5 resources configured. Ready for deployment."
   */
  summary?: string;
  success: boolean;
  clusterReady: boolean;
  cluster: string;
  namespace: string;
  clusterArchitecture?: DockerPlatform | null;
  checks: {
    connectivity: boolean;
    permissions: boolean;
    namespaceExists: boolean;
    ingressController?: boolean;
    rbacConfigured?: boolean;
    kindInstalled?: boolean;
    kindClusterCreated?: boolean;
    localRegistryCreated?: boolean;
    registryAccessValidated?: boolean;
  };
  warnings?: string[];
  localRegistryUrl?: string;
  localRegistry?: {
    externalUrl: string;
    internalEndpoint: string;
    containerName: string;
    healthy: boolean;
    reachableFromCluster: boolean;
  };
  remoteRegistry?: {
    url: string;
    type: string;
    accessValidated: boolean;
  };
}

async function checkConnectivity(
  k8sClient: KubernetesClient,
  logger: pino.Logger,
): Promise<boolean> {
  try {
    const connected = await k8sClient.ping();
    logger.debug({ connected }, 'Cluster connectivity check');
    return connected;
  } catch (error) {
    logger.warn({ error }, 'Cluster connectivity check failed');
    return false;
  }
}

async function checkNamespace(
  k8sClient: KubernetesClient,
  namespace: string,
  logger: pino.Logger,
): Promise<boolean> {
  try {
    const exists = await k8sClient.namespaceExists(namespace);
    logger.debug({ namespace, exists }, 'Checking namespace');
    return exists;
  } catch (error) {
    logger.warn({ namespace, error }, 'Namespace check failed');
    return false;
  }
}

async function setupRbac(
  k8sClient: KubernetesClient,
  namespace: string,
  logger: pino.Logger,
): Promise<void> {
  try {
    const serviceAccount: K8sManifest = {
      apiVersion: 'v1',
      kind: 'ServiceAccount',
      metadata: {
        name: 'app-service-account',
        namespace,
      },
    };

    const result = await k8sClient.applyManifest(serviceAccount, namespace);
    if (result.ok) {
      logger.info({ namespace }, 'RBAC configured');
    } else {
      logger.warn({ namespace, error: result.error }, 'RBAC setup failed');
    }
  } catch (error) {
    logger.warn({ namespace, error }, 'RBAC setup failed');
  }
}

async function checkIngressController(
  k8sClient: KubernetesClient,
  logger: pino.Logger,
): Promise<boolean> {
  try {
    const hasIngress = await k8sClient.checkIngressController();
    logger.debug({ hasIngress }, 'Checking for ingress controller');
    return hasIngress;
  } catch (error) {
    logger.warn({ error }, 'Ingress controller check failed');
    return false;
  }
}

async function checkKindInstalled(logger: pino.Logger): Promise<boolean> {
  const result = spawnSync('kind', ['version']);
  if (result.status === 0) {
    logger.debug('Kind is already installed');
    return true;
  }
  logger.debug('Kind is not installed');
  return false;
}

async function installKind(logger: pino.Logger): Promise<void> {
  try {
    logger.info('Installing kind...');

    const systemInfo = getSystemInfo();
    const downloadOS = getDownloadOS();
    const downloadArch = getDownloadArch();

    let kindUrl: string;
    let kindExecutable: string;

    if (systemInfo.isWindows) {
      kindUrl = `https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-windows-${downloadArch}.exe`;
      kindExecutable = 'kind.exe';
    } else {
      kindUrl = `https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-${downloadOS}-${downloadArch}`;
      kindExecutable = 'kind';
    }

    logger.debug({ kindUrl, kindExecutable }, 'Downloading kind binary');
    await downloadFile(kindUrl, `./${kindExecutable}`);

    if (!systemInfo.isWindows) {
      await makeExecutable(`./${kindExecutable}`);
    }

    if (systemInfo.isWindows) {
      // Try to move to Program Files
      let moveResult = spawnSync('cmd', ['/c', 'move', kindExecutable, `%ProgramFiles%\\kind\\${kindExecutable}`]);

      if (moveResult.error || moveResult.status !== 0) {
        // Try user bin directory as fallback
        spawnSync('cmd', ['/c', 'mkdir', '%USERPROFILE%\\bin']); // mkdir might fail if directory exists, that's ok
        moveResult = spawnSync('cmd', ['/c', 'move', kindExecutable, `%USERPROFILE%\\bin\\${kindExecutable}`]);

        if (moveResult.error || moveResult.status !== 0) {
          logger.warn('Failed to move kind executable to PATH, it may need manual installation');
        }
      }
    } else {
      // Try to move to /usr/local/bin with sudo
      let moveResult = spawnSync('sudo', ['mv', `./${kindExecutable}`, `/usr/local/bin/${kindExecutable}`]);

      if (moveResult.error || moveResult.status !== 0) {
        // Try user local bin directory as fallback
        spawnSync('mkdir', ['-p', `${process.env.HOME}/.local/bin`]); // mkdir might fail, that's ok if directory exists
        moveResult = spawnSync('mv', [`./${kindExecutable}`, `${process.env.HOME}/.local/bin/${kindExecutable}`]);

        if (moveResult.error || moveResult.status !== 0) {
          logger.warn('Failed to move kind executable to PATH, it may need manual installation');
        }
      }
    }

    logger.info('Kind installed successfully');
  } catch (error) {
    logger.error({ error }, 'Failed to install kind');
    throw new Error(`Kind installation failed: ${extractErrorMessage(error)}`);
  }
}

/**
 * Check if kind cluster exists.
 * Returns the cluster existence status.
 */
async function checkKindClusterExists(
  clusterName: string,
  logger: pino.Logger,
): Promise<Result<boolean>> {
  const validationResult = validateClusterName(clusterName);
  if (!validationResult.ok) {
    return validationResult;
  }

  const result = spawnSync('kind', ['get', 'clusters']);
  if (result.error || result.status !== 0) {
    logger.debug({ error: result.error, stderr: result.stderr?.toString() }, 'Error checking kind clusters');
    return Success(false);
  }

  const stdout = result.stdout.toString();
  const clusters = stdout
    .trim()
    .split('\n')
    .filter((line: string) => line.trim());
  const exists = clusters.includes(clusterName);
  logger.debug({ clusterName, exists, clusters }, 'Checking kind cluster existence');
  return Success(exists);
}


async function createKindCluster(
  clusterName: string,
  port: number,
  logger: pino.Logger,
): Promise<Result<void>> {
  const validationResult = validateClusterName(clusterName);
  if (!validationResult.ok) {
    return validationResult;
  }

  try {
    logger.info({ clusterName }, 'Creating kind cluster...');

    // Use native architecture for kind cluster
    const hostArch = process.arch;
    logger.info(
      { hostArch },
      'Creating kind cluster with native architecture',
    );

    const nodeImageLine = ''; // Let kind use default image for native architecture

    const kindConfig = `
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."${DOCKER.REGISTRY_HOST}:${port}"]
    endpoint = ["http://${DOCKER.REGISTRY_CONTAINER_NAME}:${DOCKER.REGISTRY_INTERNAL_PORT}"]
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."${DOCKER.REGISTRY_CONTAINER_NAME}:${DOCKER.REGISTRY_INTERNAL_PORT}"]
    endpoint = ["http://${DOCKER.REGISTRY_CONTAINER_NAME}:${DOCKER.REGISTRY_INTERNAL_PORT}"]
nodes:
- role: control-plane
${nodeImageLine}
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: ${KUBERNETES.DEFAULT_HTTP_PORT}
    hostPort: ${KUBERNETES.DEFAULT_HTTP_PORT}
    protocol: TCP
  - containerPort: ${KUBERNETES.DEFAULT_HTTPS_PORT}
    hostPort: ${KUBERNETES.DEFAULT_HTTPS_PORT}
    protocol: TCP
`;

    const configPath = await createTempFile(kindConfig, '.yaml');

    try {
      const result = spawnSync('kind', ['create', 'cluster', '--name', clusterName, '--config', configPath]);

      if (result.error || result.status !== 0) {
        const errorMsg = result.error?.message || result.stderr?.toString() || 'Unknown error';
        logger.error({ clusterName, error: errorMsg, stderr: result.stderr?.toString() }, 'Failed to create kind cluster');
        return Failure(`Kind cluster creation failed: ${errorMsg}`);
      }

      logger.info({ clusterName }, 'Kind cluster created successfully');
      return Success(undefined);
    } finally {
      await deleteTempFile(configPath);
    }
  } catch (error) {
    logger.error({ clusterName, error }, 'Failed to create kind cluster');
    return Failure(`Kind cluster creation failed: ${extractErrorMessage(error)}`);
  }
}

/**
 * Check if local registry exists and is running.
 * If container exists but is stopped, start it.
 * Returns the port number if registry is running and ready, or null if not.
 */
async function checkLocalRegistryExists(logger: pino.Logger): Promise<number | null> {
  try {
    // Check if container exists (running or stopped)
    const allContainersResult = spawnSync('docker', [
      'ps',
      '-a',
      '--filter',
      `name=${DOCKER.REGISTRY_CONTAINER_NAME}`,
      '--format',
      '{{.Names}}',
    ]);

    if (allContainersResult.error || allContainersResult.status !== 0) {
      logger.debug({ error: allContainersResult.error }, 'Error checking for registry container');
      return null;
    }

    const allContainers = allContainersResult.stdout.toString();
    const containerExists = allContainers.trim() === DOCKER.REGISTRY_CONTAINER_NAME;

    if (!containerExists) {
      logger.debug('Local registry container does not exist');
      return null;
    }

    // Get the port mapping for the existing container
    const portMappingResult = spawnSync('docker', [
      'inspect',
      DOCKER.REGISTRY_CONTAINER_NAME,
      '--format',
      '{{range $p, $conf := .NetworkSettings.Ports}}{{if eq $p "5000/tcp"}}{{(index $conf 0).HostPort}}{{end}}{{end}}',
    ]);

    if (portMappingResult.error || portMappingResult.status !== 0) {
      logger.warn('Could not determine registry port mapping');
      return null;
    }

    const portMapping = portMappingResult.stdout.toString();
    const port = parseInt(portMapping.trim(), 10);

    if (isNaN(port)) {
      logger.warn('Could not determine registry port mapping');
      return null;
    }

    // Check if container is running
    const runningResult = spawnSync('docker', [
      'ps',
      '--filter',
      `name=${DOCKER.REGISTRY_CONTAINER_NAME}`,
      '--format',
      '{{.Names}}',
    ]);

    const runningContainers = runningResult.status === 0 ? runningResult.stdout.toString() : '';
    const isRunning = runningContainers.trim() === DOCKER.REGISTRY_CONTAINER_NAME;

    if (isRunning) {
      logger.debug({ port }, 'Local registry is running');
      return port;
    }

    // Container exists but is stopped - try to start it
    logger.info('Local registry container exists but is stopped, starting it...');
    try {
      const startResult = spawnSync('docker', ['start', DOCKER.REGISTRY_CONTAINER_NAME]);
      if (startResult.error || startResult.status !== 0) {
        logger.error({ error: startResult.error, stderr: startResult.stderr?.toString() }, 'Failed to start existing registry container');
        return null;
      }
      logger.info({ port }, 'Local registry started successfully');

      // Validate registry health after restart with enhanced health check
      logger.debug('Validating registry health after restart...');
      const healthCheck = await validateRegistryHealth(port, logger);
      if (!healthCheck.healthy) {
        logger.warn(
          { attempts: healthCheck.attempts },
          'Registry health check failed after restart - may not be fully ready',
        );
        // Continue anyway - health issues will be caught in later validation
      } else {
        logger.info(
          { attempts: healthCheck.attempts },
          'Registry health validated successfully after restart',
        );
      }

      // After starting, check if it needs to be reconnected to kind network
      // (containers can lose network connections when stopped/restarted)
      const kindNetworkExists = await checkDockerNetworkExists('kind', logger);
      if (kindNetworkExists) {
        logger.debug('Checking if restarted registry is connected to kind network...');
        const isConnected = await checkContainerNetworkConnection(
          DOCKER.REGISTRY_CONTAINER_NAME,
          'kind',
          logger,
        );

        if (!isConnected) {
          logger.info('Registry not connected to kind network, reconnecting...');
          try {
            const connectResult = spawnSync('docker', ['network', 'connect', 'kind', DOCKER.REGISTRY_CONTAINER_NAME]);
            if (connectResult.error || connectResult.status !== 0) {
              const errorMsg = connectResult.error?.message || connectResult.stderr?.toString() || 'Unknown error';
              throw new Error(errorMsg);
            }
            logger.info('Registry reconnected to kind network successfully');

            // Verify reconnection
            const reconnected = await checkContainerNetworkConnection(
              DOCKER.REGISTRY_CONTAINER_NAME,
              'kind',
              logger,
            );
            if (!reconnected) {
              logger.warn('Failed to verify registry reconnection to kind network');
            }
          } catch (reconnectError) {
            const errorMsg = extractErrorMessage(reconnectError);
            if (errorMsg.includes('already') || errorMsg.includes('duplicate')) {
              logger.debug('Registry already connected to kind network');
            } else {
              logger.warn(
                { error: errorMsg },
                'Failed to reconnect registry to kind network (non-fatal)',
              );
            }
          }
        } else {
          logger.debug('Registry already connected to kind network');
        }
      }

      return port;
    } catch (startError) {
      logger.error({ error: startError }, 'Failed to start existing registry container');
      return null;
    }
  } catch (error) {
    logger.debug({ error }, 'Error checking local registry');
    return null;
  }
}

/**
 * Validate registry health by checking HTTP endpoint.
 * Enhanced with exponential backoff and container status checks.
 * Retries up to 10 times with exponential backoff (500ms to 5s).
 */
async function validateRegistryHealth(
  port: number,
  logger: pino.Logger,
): Promise<{ healthy: boolean; attempts: number }> {
  const maxAttempts = 10;
  let delayMs = 500; // Start with 500ms
  const maxDelay = 5000; // Cap at 5s

  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    // First check if container is running
    const statusResult = spawnSync('docker', [
      'inspect',
      DOCKER.REGISTRY_CONTAINER_NAME,
      '--format',
      '{{.State.Status}}',
    ]);

    if (statusResult.status === 0) {
      const status = statusResult.stdout.toString();
      if (status.trim() !== 'running') {
        logger.debug({ attempt, status: status.trim() }, 'Registry container not running yet');
        await new Promise((resolve) => setTimeout(resolve, delayMs));
        delayMs = Math.min(delayMs * 1.5, maxDelay); // Exponential backoff
        continue;
      }
    } else {
      logger.debug({ attempt, error: statusResult.stderr?.toString() }, 'Container status check failed');
    }

    // Then check HTTP endpoint
    const curlResult = spawnSync(
      'curl',
      ['-sf', '--max-time', '3', `http://${DOCKER.REGISTRY_HOST}:${port}/v2/`],
      { timeout: 4000 },
    );

    if (curlResult.status === 0) {
      logger.debug({ attempt }, 'Registry health check passed');
      return { healthy: true, attempts: attempt };
    } else {
      logger.debug({ attempt, error: curlResult.stderr?.toString() }, 'Registry health check attempt failed');
    }

    if (attempt < maxAttempts) {
      logger.debug({ attempt, delayMs }, 'Waiting before retry...');
      await new Promise((resolve) => setTimeout(resolve, delayMs));
      delayMs = Math.min(delayMs * 1.5, maxDelay);
    }
  }

  // Check container logs on failure for debugging
  const logsResult = spawnSync('docker', ['logs', '--tail', '20', DOCKER.REGISTRY_CONTAINER_NAME]);
  if (logsResult.status === 0) {
    const logs = logsResult.stdout.toString();
    logger.error({ logs }, 'Registry container logs (failed health check)');
  }

  logger.warn({ maxAttempts }, 'Registry health check failed after all attempts');
  return { healthy: false, attempts: maxAttempts };
}

/**
 * Check if Docker network exists.
 */
async function checkDockerNetworkExists(
  networkName: string,
  logger: pino.Logger,
): Promise<boolean> {
  const result = spawnSync('docker', [
    'network',
    'ls',
    '--filter',
    `name=${networkName}`,
    '--format',
    '{{.Name}}',
  ]);

  if (result.error || result.status !== 0) {
    logger.debug({ networkName, error: result.error }, 'Error checking Docker network');
    return false;
  }

  const stdout = result.stdout.toString();
  const exists = stdout.split('\n').includes(networkName);
  logger.debug({ networkName, exists }, 'Checking Docker network existence');
  return exists;
}

/**
 * Check if container is already connected to network.
 */
async function checkContainerNetworkConnection(
  containerName: string,
  networkName: string,
  logger: pino.Logger,
): Promise<boolean> {
  const result = spawnSync('docker', [
    'inspect',
    containerName,
    '--format',
    '{{range $net, $v := .NetworkSettings.Networks}}{{$net}} {{end}}',
  ]);

  if (result.error || result.status !== 0) {
    logger.debug(
      { containerName, networkName, error: result.error },
      'Error checking container network connection',
    );
    return false;
  }

  const stdout = result.stdout.toString();
  const networks = stdout.trim().split(' ').filter(Boolean);
  const connected = networks.includes(networkName);
  logger.debug(
    { containerName, networkName, connected, networks },
    'Checking container network connection',
  );
  return connected;
}

/**
 * Wait for Docker network to exist with retries.
 * Returns true if network exists within timeout.
 */
async function waitForDockerNetwork(
  networkName: string,
  logger: pino.Logger,
  maxAttempts: number = 10,
  delayMs: number = 1000,
): Promise<boolean> {
  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    const exists = await checkDockerNetworkExists(networkName, logger);
    if (exists) {
      logger.debug({ networkName, attempt }, 'Docker network found');
      return true;
    }

    if (attempt < maxAttempts) {
      logger.debug({ networkName, attempt, maxAttempts }, 'Waiting for Docker network...');
      await new Promise((resolve) => setTimeout(resolve, delayMs));
    }
  }

  logger.warn({ networkName, maxAttempts }, 'Docker network not found after all attempts');
  return false;
}

/**
 * Get container's IP address on a specific network.
 */
async function getContainerNetworkIP(
  containerName: string,
  networkName: string,
  logger: pino.Logger,
): Promise<string | null> {
  const result = spawnSync('docker', [
    'inspect',
    containerName,
    '--format',
    `{{.NetworkSettings.Networks.${networkName}.IPAddress}}`,
  ]);

  if (result.error || result.status !== 0) {
    logger.debug({ containerName, networkName, error: result.error }, 'Error getting container network IP');
    return null;
  }

  const stdout = result.stdout.toString();
  const ip = stdout.trim();
  if (ip && ip !== '<no value>') {
    logger.debug({ containerName, networkName, ip }, 'Got container network IP');
    return ip;
  }
  return null;
}

/**
 * Validate DNS resolution for registry from within the cluster.
 * Uses kubectl run to create a test pod that performs DNS lookup for the registry hostname.
 * This specifically tests if pods can resolve the registry's DNS name to its IP address.
 */
async function verifyRegistryDNSResolution(logger: pino.Logger): Promise<{
  resolves: boolean;
  resolvedIP?: string;
}> {
  try {
    logger.debug('Testing registry DNS resolution from within cluster...');

    // Create a temporary test pod that performs DNS lookup
    const testPodName = `registry-dns-test-${Date.now()}`;

    try {
      // Run test pod and wait for completion (timeout 30s)
      const result = spawnSync(
        'kubectl',
        [
          'run',
          testPodName,
          '--image=busybox:latest',
          '--restart=Never',
          '--rm',
          '-i',
          '--timeout=30s',
          '--',
          'sh',
          '-c',
          `nslookup ${DOCKER.REGISTRY_CONTAINER_NAME} && echo "DNS_SUCCESS" || echo "DNS_FAILED"`,
        ],
        { timeout: 35000 },
      );

      if (result.error) {
        logger.debug({ error: result.error }, 'In-cluster DNS resolution test failed');
        return { resolves: false };
      }

      const stdout = result.stdout.toString();
      const success = stdout.includes('DNS_SUCCESS') && !stdout.includes('DNS_FAILED');

      // Try to extract the resolved IP address from nslookup output
      let resolvedIP: string | undefined;
      if (success) {
        // nslookup output format: "Address 1: <IP> <hostname>"
        const ipMatch = stdout.match(/Address\s+\d+:\s+(\d+\.\d+\.\d+\.\d+)/);
        if (ipMatch?.[1]) {
          resolvedIP = ipMatch[1];
        }
      }

      logger.debug(
        { testPodName, resolves: success, resolvedIP, output: stdout.trim() },
        'In-cluster DNS resolution test result',
      );

      return resolvedIP ? { resolves: success, resolvedIP } : { resolves: success };
    } catch (error) {
      // If pod creation fails, try to clean it up
      const cleanupResult = spawnSync('kubectl', ['delete', 'pod', testPodName, '--ignore-not-found=true']);
      if (cleanupResult.error) {
        logger.debug({ error: cleanupResult.error }, 'Failed to cleanup test pod');
      }
      logger.debug({ error }, 'In-cluster DNS resolution test failed');
      return { resolves: false };
    }
  } catch (error) {
    logger.warn({ error }, 'Error testing registry DNS resolution from cluster');
    return { resolves: false };
  }
}

/**
 * Validate containerd mirror configuration on kind node.
 * Enhanced to detect actual mirror configuration structure dynamically.
 * Checks if the registry mirror config was properly applied.
 */
async function validateContainerdConfig(
  clusterName: string,
  port: number,
  logger: pino.Logger,
): Promise<boolean> {
  try {
    logger.debug({ clusterName }, 'Validating containerd mirror config on kind node...');

    // Get the kind node container name
    const nodeContainerName = `${clusterName}-control-plane`;

    // Read containerd config from the node
    const result = spawnSync('docker', ['exec', nodeContainerName, 'cat', '/etc/containerd/config.toml']);

    if (result.error || result.status !== 0) {
      logger.warn({ clusterName, error: result.error, stderr: result.stderr?.toString() }, 'Error validating containerd config');
      return false;
    }

    const stdout = result.stdout.toString();

    // Enhanced validation: check for the registry mirror configuration structure
    // The config should have a [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:PORT"] section
    // with an endpoint = ["http://ca-registry:${DOCKER.REGISTRY_INTERNAL_PORT}"] entry

    // Pattern 1: Check for mirror registry host (external access point)
    const mirrorHostPattern = new RegExp(
      `\\[plugins\\."io\\.containerd\\.grpc\\.v1\\.cri"\\.registry\\.mirrors\\."${DOCKER.REGISTRY_HOST}:${port}"\\]`,
    );
    const hasLocalRegistryMirror = mirrorHostPattern.test(stdout);

    // Pattern 2: Check for endpoint configuration (internal cluster access)
    // This can appear in different formats:
    // endpoint = ["http://ca-registry:${DOCKER.REGISTRY_INTERNAL_PORT}"]
    // or
    // endpoint = ["http://registry-name:${DOCKER.REGISTRY_INTERNAL_PORT}"]
    const endpointPattern = new RegExp(
      `endpoint\\s*=\\s*\\["http://${DOCKER.REGISTRY_CONTAINER_NAME}:${DOCKER.REGISTRY_INTERNAL_PORT}"\\]`,
    );
    const hasKindRegistryEndpoint = endpointPattern.test(stdout);

    const isValid = hasLocalRegistryMirror && hasKindRegistryEndpoint;

    // If validation fails, log a snippet of the config for debugging
    if (!isValid) {
      // Extract the relevant registry config section for debugging
      const registryConfigMatch = stdout.match(
        /\[plugins\."io\.containerd\.grpc\.v1\.cri"\.registry\.mirrors.*?\n(?:.*?\n){0,5}/,
      );
      const configSnippet = registryConfigMatch
        ? registryConfigMatch[0]
        : 'Config section not found';

      logger.debug(
        {
          clusterName,
          hasLocalRegistryMirror,
          hasKindRegistryEndpoint,
          isValid,
          configSnippet,
        },
        'Containerd config validation failed - config snippet shown for debugging',
      );
    } else {
      logger.debug(
        {
          clusterName,
          hasLocalRegistryMirror,
          hasKindRegistryEndpoint,
          isValid,
        },
        'Containerd config validation result',
      );
    }

    return isValid;
  } catch (error) {
    logger.warn({ clusterName, error }, 'Error validating containerd config');
    return false;
  }
}

/**
 * Create local registry ConfigMap in kube-public namespace.
 * This documents the registry location for tools and users (kind best practice).
 */
async function createLocalRegistryConfigMap(
  k8sClient: KubernetesClient,
  port: number,
  logger: pino.Logger,
): Promise<void> {
  try {
    logger.debug('Creating local registry ConfigMap in kube-public namespace');

    const configMap: K8sManifest = {
      apiVersion: 'v1',
      kind: 'ConfigMap',
      metadata: {
        name: 'local-registry-hosting',
        namespace: 'kube-public',
      },
      data: {
        'localRegistryHosting.v1': `host: "${DOCKER.REGISTRY_HOST}:${port}"\nhelp: "https://kind.sigs.k8s.io/docs/user/local-registry/"`,
      },
    };

    const result = await k8sClient.applyManifest(configMap, 'kube-public');
    if (result.ok) {
      logger.info('Local registry ConfigMap created in kube-public namespace');
    } else {
      logger.warn({ error: result.error }, 'Failed to create local registry ConfigMap (non-fatal)');
    }
  } catch (error) {
    logger.warn({ error }, 'Failed to create local registry ConfigMap (non-fatal)');
  }
}

/**
 * Phase 1: Create registry container only (without network connection).
 * This allows registry to be created before kind cluster exists.
 */
async function createRegistryContainerOnly(port: number, logger: pino.Logger): Promise<void> {
  try {
    logger.info({ port }, 'Creating local Docker registry container...');

    const result = spawnSync('docker', [
      'run',
      '-d',
      '--restart=always',
      '-p',
      `${port}:${DOCKER.REGISTRY_INTERNAL_PORT}`,
      '--name',
      DOCKER.REGISTRY_CONTAINER_NAME,
      'registry:2',
    ]);

    if (result.error || result.status !== 0) {
      const errorMsg = result.error?.message || result.stderr?.toString() || 'Unknown error';
      logger.error({ error: errorMsg }, 'Failed to create registry container');
      throw new Error(`Registry container creation failed: ${errorMsg}`);
    }

    logger.debug({ port }, 'Registry container created (network connection deferred)');
  } catch (error) {
    logger.error({ error }, 'Failed to create registry container');
    throw new Error(`Registry container creation failed: ${extractErrorMessage(error)}`);
  }
}

/**
 * Phase 2: Connect registry to kind network and validate setup.
 * Should be called after kind cluster and network exist.
 */
async function connectRegistryToKindNetwork(
  port: number,
  logger: pino.Logger,
): Promise<{ connected: boolean; healthy: boolean; healthCheckAttempts: number }> {
  logger.debug('Connecting registry to kind network and validating...');

  // Check if kind network exists
  const kindNetworkExists = await checkDockerNetworkExists('kind', logger);
  if (!kindNetworkExists) {
    logger.warn(
      'Kind Docker network does not exist - registry will not be accessible from cluster',
    );
    return { connected: false, healthy: false, healthCheckAttempts: 0 };
  }

  // Check if registry is already connected to kind network
  const alreadyConnected = await checkContainerNetworkConnection(
    DOCKER.REGISTRY_CONTAINER_NAME,
    'kind',
    logger,
  );

  if (alreadyConnected) {
    logger.debug('Registry already connected to kind network');
  } else {
    // Connect registry to kind network
    logger.debug('Connecting registry to kind Docker network...');
    const connectResult = spawnSync('docker', ['network', 'connect', 'kind', DOCKER.REGISTRY_CONTAINER_NAME]);

    if (connectResult.error || connectResult.status !== 0) {
      const errorMsg = connectResult.error?.message || connectResult.stderr?.toString() || '';
      // Only treat as non-fatal if it's "already connected" error
      if (errorMsg.includes('already') || errorMsg.includes('duplicate')) {
        logger.debug({ error: errorMsg }, 'Registry already connected to kind network');
      } else {
        logger.error({ error: errorMsg }, 'Failed to connect registry to kind network');
        throw new Error(`Failed to connect registry to kind network: ${errorMsg}`);
      }
    } else {
      logger.info('Registry connected to kind network successfully');
    }

    // Verify connection succeeded by checking again
    logger.debug('Verifying registry network connection...');
    const connectedAfter = await checkContainerNetworkConnection(
      DOCKER.REGISTRY_CONTAINER_NAME,
      'kind',
      logger,
    );
    if (!connectedAfter) {
      logger.warn(
        'Registry network connection verification failed - may not be accessible from cluster',
      );
      return { connected: false, healthy: false, healthCheckAttempts: 0 };
    }

    // Get and log the IP address on the kind network
    const registryIP = await getContainerNetworkIP(DOCKER.REGISTRY_CONTAINER_NAME, 'kind', logger);
    if (registryIP) {
      logger.info({ registryIP }, 'Registry network connection verified successfully');
    } else {
      logger.info('Registry network connection verified (IP not available)');
    }
  }

  // Validate registry health
  logger.debug('Validating registry health...');
  const healthCheck = await validateRegistryHealth(port, logger);
  if (!healthCheck.healthy) {
    logger.warn(
      { attempts: healthCheck.attempts },
      'Registry health check failed - registry may not be fully ready',
    );
  } else {
    logger.info({ attempts: healthCheck.attempts }, 'Registry health validated successfully');
  }

  return {
    connected: true,
    healthy: healthCheck.healthy,
    healthCheckAttempts: healthCheck.attempts,
  };
}

/**
 * Validate that the user is already logged into the specified registry.
 * Checks docker login status without handling credentials.
 *
 * @param registryUrl - Registry URL (e.g., myregistry.azurecr.io)
 * @param registryType - Type of registry (acr, generic)
 * @param logger - Logger instance
 * @returns Success if logged in, Failure with guidance if not
 */
async function validateRegistryAccess(
  registryUrl: string,
  registryType: string,
  logger: pino.Logger,
): Promise<Result<void>> {
  try {
    logger.debug({ registryUrl, registryType }, 'Checking Docker login status for registry...');

    let testResult: string;

    switch (registryType) {
      case RegistryType.ACR: {
        // For ACR, try to list repositories
        // Extract registry name (before first dot, or entire URL if no dot)
        const registryName = registryUrl.includes('.') ? registryUrl.split('.')[0] : registryUrl;
        if (!registryName) {
          return Failure(`Invalid ACR registry URL: ${registryUrl}`, {
            message: `Could not extract registry name from URL: ${registryUrl}`,
            hint: `ACR registry URL should be in format: <registry-name>.azurecr.io`,
            resolution: `Provide a valid ACR registry URL like 'myregistry.azurecr.io'`,
          });
        }

        const acrResult = spawnSync('az', ['acr', 'repository', 'list', '--name', registryName, '--output', 'json'], {
          stdio: ['ignore', 'pipe', 'ignore'],
        });

        if (acrResult.error || acrResult.status !== 0) {
          testResult = 'not-logged-in';
        } else {
          testResult = acrResult.stdout.toString();
        }
        break;
      }
      default: {
        // Generic: try to check docker config for registry credentials
        const dockerCredResult = spawnSync('docker-credential-desktop', ['list'], {
          stdio: ['ignore', 'pipe', 'ignore'],
        });

        if (dockerCredResult.error || dockerCredResult.status !== 0) {
          testResult = 'not-logged-in';
        } else {
          const credOutput = dockerCredResult.stdout.toString();
          testResult = credOutput.toLowerCase().includes(registryUrl.toLowerCase()) ? credOutput : 'not-logged-in';
        }
      }
    }

    if (testResult.includes('not-logged-in') || testResult.trim() === '') {
      return Failure(
        `Not logged into registry: ${registryUrl}`,
        {
          message: `Docker login credentials not found for registry: ${registryUrl}`,
          hint: `User must be logged into the registry before running prepare-cluster`,
          resolution: `Log in to the registry using:\n  - For ACR: az acr login --name ${registryUrl.split('.')[0]}\n  - For ECR: aws ecr get-login-password --region <region> | docker login --username AWS --password-stdin ${registryUrl}\n  - For GCR: gcloud auth configure-docker\n  - For generic: docker login ${registryUrl}\nThen retry prepare-cluster.`,
        }
      );
    }

    logger.debug({ registryUrl, testResult: testResult.substring(0, 100) }, 'Registry access validation passed');
    return Success(undefined);
  } catch (error) {
    const errorMsg = extractErrorMessage(error);
    logger.warn({ registryUrl, error: errorMsg }, 'Registry access validation failed');

    return Failure(
      `Registry access validation failed: ${errorMsg}`,
      {
        message: `Could not verify login status for registry: ${registryUrl}`,
        hint: `Unable to confirm Docker login credentials for the registry`,
        resolution: `Ensure you are logged in:\n  - For ACR: az acr login --name ${registryUrl.split('.')[0]}\n  - For ECR: aws ecr get-login-password --region <region> | docker login --username AWS --password-stdin ${registryUrl}\n  - For GCR: gcloud auth configure-docker\n  - For generic: docker login ${registryUrl}\nThen retry prepare-cluster.`,
      }
    );
  }
}

/**
 * Setup Kind cluster if needed
 */
async function setupKindCluster(
  clusterName: string,
  port: number,
  logger: pino.Logger,
  checks: {
    kindInstalled: boolean | undefined;
    kindClusterCreated: boolean | undefined;
  },
): Promise<Result<void>> {
  // Validate cluster name upfront
  const validationResult = validateClusterName(clusterName);
  if (!validationResult.ok) {
    return validationResult;
  }

  checks.kindInstalled = await checkKindInstalled(logger);
  if (!checks.kindInstalled) {
    await installKind(logger);
    checks.kindInstalled = true;
    logger.info('Kind installation completed');
  }

  const clusterExistsResult = await checkKindClusterExists(clusterName, logger);
  if (!clusterExistsResult.ok) {
    return clusterExistsResult;
  }
  const kindClusterExists = clusterExistsResult.value;

  if (!kindClusterExists) {
    const createResult = await createKindCluster(clusterName, port, logger);
    if (!createResult.ok) {
      return createResult;
    }
    checks.kindClusterCreated = true;
    logger.info({ clusterName }, 'Kind cluster creation completed');

    // Wait for cluster to stabilize and check for node readiness
    logger.debug('Waiting for cluster to stabilize...');
    await new Promise((resolve) => setTimeout(resolve, DEFAULT_TIMEOUTS.clusterStabilization));

    // Verify cluster nodes are ready
    const nodesResult = spawnSync('kubectl', ['get', 'nodes', '--no-headers']);
    if (nodesResult.status === 0) {
      const stdout = nodesResult.stdout.toString();
      const nodesReady = stdout.includes('Ready');
      logger.debug({ nodesReady, output: stdout.trim() }, 'Cluster node readiness check');
      if (!nodesReady) {
        logger.warn('Cluster nodes may not be fully ready yet');
      }
    } else {
      logger.debug({ error: nodesResult.stderr?.toString() }, 'Could not check node readiness (non-fatal)');
    }

    // Validate that kind network was created
    logger.debug('Validating kind Docker network...');
    const networkExists = await waitForDockerNetwork('kind', logger, 10, 1000);
    if (!networkExists) {
      logger.warn('Kind Docker network not found - registry connectivity may be impaired');
    } else {
      logger.info('Kind Docker network validated successfully');
    }
  } else {
    checks.kindClusterCreated = true;
    logger.info({ clusterName }, 'Kind cluster already exists');
  }

  // Export kubeconfig
  const exportResult = spawnSync('kind', ['export', 'kubeconfig', '--name', clusterName]);
  if (exportResult.error || exportResult.status !== 0) {
    logger.warn(
      { error: exportResult.error?.message, stderr: exportResult.stderr?.toString() },
      'Failed to export kubeconfig, continuing anyway'
    );
  }

  return Success(undefined);
}

/**
 * Setup local Docker registry if needed
 * Returns detailed registry information including health and connectivity status
 */
/**
 * Setup local Docker registry - Phase 1 only (container creation)
 * This function creates the registry container without connecting it to the kind network.
 * Phase 2 (network connection) is handled separately after the kind cluster is ready.
 *
 * Returns detailed registry information including port and initial health status.
 */
async function setupLocalRegistry(
  port: number,
  logger: pino.Logger,
  checks: {
    localRegistryCreated: boolean | undefined;
  },
): Promise<{
  url: string;
  port: number;
  healthy: boolean;
  healthCheckAttempts: number;
  reachableFromCluster: boolean;
}> {
  logger.debug({ port }, 'Starting local registry setup (Phase 1: container creation)');

  // Check if registry already exists
  const existingPort = await checkLocalRegistryExists(logger);

  if (existingPort !== null) {
    // Registry already exists - just check health
    const registryUrl = `${DOCKER.REGISTRY_HOST}:${existingPort}`;
    checks.localRegistryCreated = true;

    // Check health of existing registry
    const healthCheck = await validateRegistryHealth(existingPort, logger);

    logger.info(
      { registryUrl, healthy: healthCheck.healthy, attempts: healthCheck.attempts },
      'Local registry already exists',
    );
    return {
      url: registryUrl,
      port: existingPort,
      healthy: healthCheck.healthy,
      healthCheckAttempts: healthCheck.attempts,
      reachableFromCluster: false, // Will be checked in Phase 2
    };
  }

  // Registry doesn't exist - create it (Phase 1)
  await createRegistryContainerOnly(port, logger);
  checks.localRegistryCreated = true;

  // Check initial health after creation
  const registryUrl = `${DOCKER.REGISTRY_HOST}:${port}`;
  const healthCheck = await validateRegistryHealth(port, logger);

  logger.info(
    { registryUrl, healthy: healthCheck.healthy, attempts: healthCheck.attempts },
    'Local registry container created (network connection deferred)',
  );
  return {
    url: registryUrl,
    port,
    healthy: healthCheck.healthy,
    healthCheckAttempts: healthCheck.attempts,
    reachableFromCluster: false, // Will be checked in Phase 2
  };
}

/**
 * Verify cluster readiness by checking connectivity, permissions, and namespace
 */
async function verifyClusterReadiness(
  k8sClient: KubernetesClient,
  namespace: string,
  shouldCreateNamespace: boolean,
  shouldSetupRbac: boolean,
  checkRequirements: boolean,
  installIngress: boolean,
  logger: pino.Logger,
  checks: {
    connectivity: boolean;
    permissions: boolean;
    namespaceExists: boolean;
    ingressController: boolean | undefined;
    rbacConfigured: boolean | undefined;
  },
  warnings: string[],
): Promise<Result<boolean>> {
  // Check connectivity
  checks.connectivity = await checkConnectivity(k8sClient, logger);
  if (!checks.connectivity) {
    return Failure('Cannot connect to Kubernetes cluster', {
      message: 'Kubernetes cluster connection failed',
      hint: 'Could not establish connection to any Kubernetes cluster',
      resolution:
        'Ensure Kubernetes is installed and a cluster is accessible (kubectl cluster-info)',
    });
  }

  // Check permissions
  checks.permissions = await k8sClient.checkPermissions(namespace);
  if (!checks.permissions) {
    return Failure('Insufficient permissions for Kubernetes operations', {
      message: 'Kubernetes permissions check failed',
      hint: 'Current user/service account lacks required permissions',
      resolution:
        'Verify current privileges with RBAC: run `kubectl auth can-i <verb> <resource> --namespace <namespace>` for required operations',
    });
  }

  // Check/create namespace
  checks.namespaceExists = await checkNamespace(k8sClient, namespace, logger);
  if (!checks.namespaceExists && shouldCreateNamespace) {
    const ensureResult = await k8sClient.ensureNamespace(namespace);
    if (ensureResult.ok) {
      checks.namespaceExists = true;
      logger.info({ namespace }, 'Namespace created successfully');
    } else {
      logger.error({ namespace, error: ensureResult.error }, 'Failed to create namespace');
      return Failure(ensureResult.error || 'Failed to create namespace', ensureResult.guidance);
    }
  } else if (!checks.namespaceExists) {
    warnings.push(`Namespace ${namespace} does not exist - deployment may fail`);
  }

  // Setup RBAC if needed
  if (shouldSetupRbac) {
    await setupRbac(k8sClient, namespace, logger);
    checks.rbacConfigured = true;
  }

  // Check ingress controller if needed
  if (checkRequirements || installIngress) {
    checks.ingressController = await checkIngressController(k8sClient, logger);
    if (!checks.ingressController) {
      warnings.push('No ingress controller found - external access may not work');
    }
  }

  const clusterReady = checks.connectivity && checks.permissions && checks.namespaceExists;
  return Success(clusterReady);
}

/**
 * Core cluster preparation implementation
 */
async function handlePrepareCluster(
  params: PrepareClusterParams,
  context: ToolContext,
): Promise<Result<PrepareClusterResult>> {
  const { logger, timer } = setupToolContext(context, 'prepare-cluster');

  const {
    environment = 'development',
    namespace = 'default',
    useRemoteCluster = false,
    clusterName: userClusterName,
    registry,
  } = params;

  // Validate namespace
  const namespaceValidation = validateNamespace(namespace);
  if (!namespaceValidation.ok) {
    return namespaceValidation;
  }

  // Determine cluster name
  const clusterName = useRemoteCluster
    ? (userClusterName || 'remote-cluster')
    : (environment === 'development' ? 'containerization-assist' : 'default');

  const shouldCreateNamespace = environment === 'production';
  const shouldSetupRbac = environment === 'production';
  const installIngress = false;
  const checkRequirements = true;
  const shouldSetupKind = !useRemoteCluster && environment === 'development';
  const shouldCreateLocalRegistry = !useRemoteCluster && environment === 'development';

  try {
    logger.info({ environment, namespace, useRemoteCluster }, 'Starting Kubernetes cluster preparation');

    // Validate remote registry access if using remote cluster
    if (useRemoteCluster && registry) {
      logger.info({ registryUrl: registry.url, registryType: registry.type }, 'Validating registry access...');

      const registryValidation = await validateRegistryAccess(registry.url, registry.type, logger);
      if (!registryValidation.ok) {
        return registryValidation;
      }

      logger.info({ registryUrl: registry.url }, 'Registry access validated - user is logged in');
    }

    const warnings: string[] = [];
    const checks = {
      connectivity: false,
      permissions: false,
      namespaceExists: false,
      ingressController: undefined as boolean | undefined,
      rbacConfigured: undefined as boolean | undefined,
      kindInstalled: undefined as boolean | undefined,
      kindClusterCreated: undefined as boolean | undefined,
      localRegistryCreated: undefined as boolean | undefined,
      registryAccessValidated: undefined as boolean | undefined,
    };
    let localRegistryUrl: string | undefined;
    let localRegistryInfo: PrepareClusterResult['localRegistry'] | undefined;
    let remoteRegistryInfo: PrepareClusterResult['remoteRegistry'] | undefined;
    let registryPort: number | undefined;

    // Mark registry as validated if we checked it
    if (useRemoteCluster && registry) {
      checks.registryAccessValidated = true;
      remoteRegistryInfo = {
        url: registry.url,
        type: registry.type,
        accessValidated: true,
      };
    }

    // Determine registry port before cluster setup
    // Either detect existing registry or find an available port for a new one
    if (shouldSetupKind || shouldCreateLocalRegistry) {
      const existingPort = await checkLocalRegistryExists(logger);
      if (existingPort !== null) {
        registryPort = existingPort;
        logger.debug({ registryPort }, 'Using existing registry port');
      } else {
        registryPort = await findRegistryPort();
        logger.debug({ registryPort }, 'Found available port for new registry');
      }
    }

    // Phase 1: Create local Docker registry container (BEFORE kind cluster setup)
    // This allows the registry to exist before the cluster, solving the ordering problem
    let registryPhase1Data: Awaited<ReturnType<typeof setupLocalRegistry>> | undefined;
    if (shouldCreateLocalRegistry && registryPort !== undefined) {
      registryPhase1Data = await setupLocalRegistry(registryPort, logger, checks);
      localRegistryUrl = registryPhase1Data.url;
      logger.info(
        { registryUrl: localRegistryUrl, healthy: registryPhase1Data.healthy },
        'Registry container created (Phase 1 complete)',
      );
    }

    // Setup Kind cluster if in development environment
    // Pass the registry port so it can be configured in the cluster
    if (shouldSetupKind && registryPort !== undefined) {
      const setupResult = await setupKindCluster(clusterName, registryPort, logger, checks);
      if (!setupResult.ok) {
        return setupResult;
      }
    }

    const k8sClient = createKubernetesClient(logger);

    // Phase 2: Connect registry to kind network and validate setup (AFTER kind cluster is ready)
    if (
      shouldCreateLocalRegistry &&
      shouldSetupKind &&
      registryPort !== undefined &&
      registryPhase1Data
    ) {
      logger.info('Starting registry Phase 2: network connection and validation...');

      // Connect registry to kind network
      const networkConnection = await connectRegistryToKindNetwork(registryPort, logger);

      if (!networkConnection.connected) {
        warnings.push('Failed to connect registry to kind network - deployment may fail');
      }

      // Create registry ConfigMap
      await createLocalRegistryConfigMap(k8sClient, registryPort, logger);

      // Validate containerd mirror configuration
      logger.debug('Validating containerd registry mirror configuration...');
      const containerdConfigValid = await validateContainerdConfig(
        clusterName,
        registryPort,
        logger,
      );
      if (!containerdConfigValid) {
        warnings.push(
          `Containerd registry mirror configuration validation failed - image pulls from ${DOCKER.REGISTRY_HOST}:${registryPort} may not work`,
        );
      } else {
        logger.info('Containerd registry mirror configuration validated successfully');
      }

      // Test DNS resolution from within cluster
      logger.debug('Testing registry DNS resolution from cluster...');
      const dnsResolution = await verifyRegistryDNSResolution(logger);

      if (!dnsResolution.resolves) {
        warnings.push(
          `Registry DNS resolution failed from within cluster - pods cannot resolve hostname '${DOCKER.REGISTRY_CONTAINER_NAME}'`,
        );
        logger.warn(
          'Registry DNS resolution failed - this may indicate network configuration issues',
        );
      } else {
        const dnsMessage = dnsResolution.resolvedIP
          ? `Registry DNS resolution validated successfully (resolved to ${dnsResolution.resolvedIP})`
          : 'Registry DNS resolution validated successfully';
        logger.info({ resolvedIP: dnsResolution.resolvedIP }, dnsMessage);
      }

      // Populate detailed registry information combining Phase 1 and Phase 2 data
      localRegistryInfo = {
        externalUrl: registryPhase1Data.url,
        internalEndpoint: `${DOCKER.REGISTRY_CONTAINER_NAME}:${DOCKER.REGISTRY_INTERNAL_PORT}`,
        containerName: DOCKER.REGISTRY_CONTAINER_NAME,
        healthy: networkConnection.healthy,
        reachableFromCluster: dnsResolution.resolves,
      };

      logger.info(
        {
          connected: networkConnection.connected,
          healthy: networkConnection.healthy,
        },
        'Registry Phase 2 complete',
      );
    }

    // Verify cluster readiness (connectivity, permissions, namespace, RBAC, ingress)
    const readinessResult = await verifyClusterReadiness(
      k8sClient,
      namespace,
      shouldCreateNamespace,
      shouldSetupRbac,
      checkRequirements,
      installIngress,
      logger,
      checks,
      warnings,
    );

    if (!readinessResult.ok) {
      return readinessResult;
    }

    const clusterReady = readinessResult.value;

    // Detect cluster architecture
    logger.debug('Detecting cluster architecture...');
    const clusterArchitecture = await detectClusterPlatform(logger);

    // Generate summary
    const namespaceAction = checks.namespaceExists ? 'verified' : 'created';
    const resourcesConfigured = Object.values(checks).filter(Boolean).length;
    const architectureText = clusterArchitecture
      ? ` Cluster architecture: ${clusterArchitecture}.`
      : '';
    const summary = `✅ Cluster prepared. Namespace '${namespace}' ${namespaceAction}. ${pluralize(resourcesConfigured, 'resource')} configured.${architectureText} Ready for deployment.`;

    const result: PrepareClusterResult = {
      summary,
      success: true,
      clusterReady,
      cluster: clusterName,
      namespace,
      clusterArchitecture,
      checks: {
        connectivity: checks.connectivity,
        permissions: checks.permissions,
        namespaceExists: checks.namespaceExists,
        ...(checks.ingressController !== undefined && {
          ingressController: checks.ingressController,
        }),
        ...(checks.rbacConfigured !== undefined && { rbacConfigured: checks.rbacConfigured }),
        ...(checks.kindInstalled !== undefined && { kindInstalled: checks.kindInstalled }),
        ...(checks.kindClusterCreated !== undefined && {
          kindClusterCreated: checks.kindClusterCreated,
        }),
        ...(checks.localRegistryCreated !== undefined && {
          localRegistryCreated: checks.localRegistryCreated,
        }),
        ...(checks.registryAccessValidated !== undefined && {
          registryAccessValidated: checks.registryAccessValidated,
        }),
      },
      ...(warnings.length > 0 && { warnings }),
      ...(localRegistryUrl && { localRegistryUrl }),
      ...(localRegistryInfo && { localRegistry: localRegistryInfo }),
      ...(remoteRegistryInfo && { remoteRegistry: remoteRegistryInfo }),
    };

    logger.info({ clusterReady, checks }, 'Cluster preparation completed');

    timer.end({ clusterReady, environment });

    return Success(result);
  } catch (error) {
    timer.error(error);

    const errorMessage = error instanceof Error ? error.message : String(error);
    return Failure(errorMessage, {
      message: errorMessage,
      hint: 'An unexpected error occurred during cluster preparation',
      resolution:
        'Check the error message for details. Common issues include Docker not running (for kind clusters), kubectl not configured, or insufficient permissions',
    });
  }
}

export const prepareCluster = handlePrepareCluster;

import { tool } from '@/types/tool';

export default tool({
  name: 'prepare-cluster',
  description: 'Prepare Kubernetes cluster for deployment',
  category: 'kubernetes',
  version: '2.0.0',
  schema: prepareClusterSchema,
  metadata: {
    knowledgeEnhanced: false,
  },
  chainHints: {
    success: 'Cluster preparation successful. For local kind clusters with local registry, use `localhost:<port>/<image>:<tag>` format when building images. For remote clusters with ACR/ECR/GCR, use the registry URL format. Next: Use `kubectl apply -f <manifest-folder>` to deploy your manifests to the cluster, then call verify-deploy to check deployment status.',
    failure:
      'Cluster preparation found issues. Check connectivity, permissions, namespace configuration, and registry access (if using remote cluster).',
  },
  handler: handlePrepareCluster,
});
