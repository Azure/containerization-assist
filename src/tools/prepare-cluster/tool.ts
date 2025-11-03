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
import { DEFAULT_TIMEOUTS, DOCKER } from '@/config/constants';
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
  isPlatformCompatible,
} from '@/lib/platform';
import { downloadFile, makeExecutable, createTempFile, deleteTempFile } from '@/lib/file-utils';
import { findRegistryPort } from '@/lib/port-utils';
import type { DockerPlatform } from '@/tools/shared/schemas';

import type * as pino from 'pino';
import { Success, Failure, type Result } from '@/types';
import { prepareClusterSchema, type PrepareClusterParams } from './schema';
import { exec } from 'node:child_process';
import { promisify } from 'node:util';
import { pluralize } from '@/lib/summary-helpers';

const execAsync = promisify(exec);

const KIND_VERSION = 'v0.20.0';

/**
 * Validate and escape cluster name to prevent command injection.
 * Cluster names must follow Kubernetes naming conventions.
 *
 * SECURITY MODEL:
 * - Primary defense: Strict regex validation allowing only [a-z0-9-] characters
 * - Secondary defense: Shell escaping with single quotes (redundant but defensive)
 * - The regex makes command injection impossible as no shell metacharacters are allowed
 *
 * IMPORTANT: Returns the cluster name wrapped in single quotes for shell safety.
 * The returned value must be used with template literal interpolation only.
 * DO NOT use with string concatenation or you may get double-quoting issues.
 *
 * @example
 * ```typescript
 * const result = validateAndEscapeClusterName("my-cluster");
 * if (result.ok) {
 *   // ✅ Correct - template literal interpolation
 *   await execAsync(`kind create cluster --name ${result.value}`);
 *   // Result: kind create cluster --name 'my-cluster'
 *
 *   // ❌ Wrong - string concatenation causes double quoting
 *   await execAsync("kind create cluster --name " + result.value);
 * }
 * ```
 */
function validateAndEscapeClusterName(clusterName: string): Result<string> {
  // Kubernetes resource names must be lowercase alphanumeric with dashes
  // This regex is the primary security mechanism - it prevents ALL shell metacharacters
  const nameRegex = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/;

  if (!nameRegex.test(clusterName)) {
    return Failure(
      `Invalid cluster name: "${clusterName}". Must contain only lowercase letters, numbers, and hyphens.`,
      {
        message: `Invalid cluster name: "${clusterName}". Must contain only lowercase letters, numbers, and hyphens.`,
        hint: 'Cluster names must follow Kubernetes naming conventions',
        resolution: 'Use only lowercase letters (a-z), numbers (0-9), and hyphens (-). Start and end with alphanumeric characters',
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

  // Wrap in single quotes for defense-in-depth shell safety
  // Note: The regex already prevents single quotes, so the replace is technically
  // unnecessary, but we keep it as a defensive measure in case validation changes
  return Success(`'${clusterName.replace(/'/g, "'\\''")}'`);
}

/**
 * Detect the platform architecture of Kubernetes cluster nodes.
 * Returns the detected platform or null if detection fails.
 */
async function detectClusterPlatform(logger: pino.Logger): Promise<DockerPlatform | null> {
  try {
    logger.debug('Detecting cluster node platform...');

    // Get node architecture information
    const { stdout } = await execAsync(
      'kubectl get nodes -o jsonpath=\'{.items[0].status.nodeInfo.architecture}\'',
    );
    const arch = stdout.trim().replace(/'/g, '');

    if (!arch) {
      logger.warn('Could not detect cluster node architecture');
      return null;
    }

    // Get OS if available (usually linux for Kubernetes)
    let os = 'linux';
    try {
      const { stdout: osOutput } = await execAsync(
        'kubectl get nodes -o jsonpath=\'{.items[0].status.nodeInfo.operatingSystem}\'',
      );
      const detectedOS = osOutput.trim().replace(/'/g, '').toLowerCase();
      if (detectedOS) {
        os = detectedOS;
      }
    } catch {
      // If OS detection fails, default to linux
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

/**
 * Validate that target platform is compatible with cluster platform.
 * Returns validation result with detailed guidance.
 */
async function validatePlatformCompatibility(
  targetPlatform: DockerPlatform,
  logger: pino.Logger,
  warnings: string[],
): Promise<{
  clusterPlatform: DockerPlatform | null;
  compatible: boolean;
  requiresEmulation: boolean;
}> {
  const clusterPlatform = await detectClusterPlatform(logger);

  if (!clusterPlatform) {
    warnings.push(
      `Could not detect cluster platform - unable to verify compatibility with target platform ${targetPlatform}`,
    );
    return {
      clusterPlatform: null,
      compatible: false,
      requiresEmulation: false,
    };
  }

  const compatible = isPlatformCompatible(targetPlatform, clusterPlatform);

  if (!compatible) {
    const requiresEmulation = targetPlatform !== clusterPlatform;

    if (requiresEmulation) {
      // Check if this is a known emulation scenario (e.g., ARM Mac with AMD64 kind cluster)
      const systemInfo = getSystemInfo();
      const hostArch = process.arch;
      const isArmMacWithAmd64 =
        systemInfo.isMac &&
        hostArch === 'arm64' &&
        targetPlatform === 'linux/amd64' &&
        clusterPlatform === 'linux/amd64';

      if (isArmMacWithAmd64) {
        // This is expected for ARM Mac development targeting AMD64
        logger.info(
          { targetPlatform, clusterPlatform },
          'ARM Mac detected with AMD64 cluster - Docker emulation will be used',
        );
        warnings.push(
          `Running AMD64 cluster on ARM Mac - images will use Docker emulation (may have performance impact)`,
        );
        return {
          clusterPlatform,
          compatible: true, // Allow this scenario
          requiresEmulation: true,
        };
      }

      // Other emulation scenarios - warn but don't block
      logger.warn(
        { targetPlatform, clusterPlatform },
        'Target platform does not match cluster platform - may require emulation',
      );
      warnings.push(
        `Platform mismatch: Building for ${targetPlatform} but cluster runs ${clusterPlatform}. Images may not run or may require emulation.`,
      );
    }

    return {
      clusterPlatform,
      compatible: false,
      requiresEmulation,
    };
  }

  logger.info({ targetPlatform, clusterPlatform }, 'Platform compatibility validated successfully');
  return {
    clusterPlatform,
    compatible: true,
    requiresEmulation: false,
  };
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
  platform?: {
    target: DockerPlatform;
    cluster: DockerPlatform | null;
    compatible: boolean;
    requiresEmulation: boolean;
  };
  checks: {
    connectivity: boolean;
    permissions: boolean;
    namespaceExists: boolean;
    ingressController?: boolean;
    rbacConfigured?: boolean;
    kindInstalled?: boolean;
    kindClusterCreated?: boolean;
    localRegistryCreated?: boolean;
    platformCompatible?: boolean;
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
  try {
    await execAsync('kind version');
    logger.debug('Kind is already installed');
    return true;
  } catch {
    logger.debug('Kind is not installed');
    return false;
  }
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
      try {
        await execAsync(`move ${kindExecutable} "%ProgramFiles%\\kind\\${kindExecutable}"`);
      } catch {
        try {
          await execAsync(
            `mkdir "%USERPROFILE%\\bin" 2>nul & move ${kindExecutable} "%USERPROFILE%\\bin\\${kindExecutable}"`,
          );
        } catch {
          logger.warn('Failed to move kind executable to PATH, it may need manual installation');
        }
      }
    } else {
      try {
        await execAsync(`sudo mv ./${kindExecutable} /usr/local/bin/${kindExecutable}`);
      } catch {
        try {
          await execAsync(
            `mkdir -p ~/.local/bin && mv ./${kindExecutable} ~/.local/bin/${kindExecutable}`,
          );
        } catch {
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

async function checkKindClusterExists(
  clusterName: string,
  logger: pino.Logger,
): Promise<Result<boolean>> {
  const escapedNameResult = validateAndEscapeClusterName(clusterName);
  if (!escapedNameResult.ok) {
    return escapedNameResult;
  }

  try {
    const { stdout } = await execAsync('kind get clusters');
    const clusters = stdout
      .trim()
      .split('\n')
      .filter((line: string) => line.trim());
    const exists = clusters.includes(clusterName);
    logger.debug({ clusterName, exists, clusters }, 'Checking kind cluster existence');
    return Success(exists);
  } catch (error) {
    logger.debug({ error }, 'Error checking kind clusters');
    return Success(false);
  }
}

async function createKindCluster(clusterName: string, port: number, logger: pino.Logger): Promise<Result<void>> {
  const escapedNameResult = validateAndEscapeClusterName(clusterName);
  if (!escapedNameResult.ok) {
    return escapedNameResult;
  }
  const escapedName = escapedNameResult.value;

  try {
    logger.info({ clusterName }, 'Creating kind cluster...');

    // Detect system architecture to determine if we need cross-platform emulation
    const systemInfo = getSystemInfo();
    const hostArch = process.arch;

    // Check if running on ARM Mac - if so, use AMD64 kind node for broader compatibility
    // This enables testing AMD64 images on ARM development machines via Docker emulation
    const shouldUseAMD64Node = systemInfo.isMac && hostArch === 'arm64';

    if (shouldUseAMD64Node) {
      logger.info({ hostArch, targetArch: 'amd64' },
        'Detected ARM Mac - creating AMD64 kind cluster for cross-platform compatibility (will use Docker emulation)',
      );
    }

    // Build node configuration - add explicit AMD64 image on ARM Mac
    // Use a stable AMD64 node image that works well with Docker Desktop's x86 emulation
    const nodeImageLine = shouldUseAMD64Node
      ? '  image: kindest/node:v1.27.3@sha256:3966ac761ae0136263ffdb6cfd4db23ef8a83cba8a463690e98317add2c9ba72'
      : '';

    const kindConfig = `
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."${DOCKER.REGISTRY_HOST}:${port}"]
    endpoint = ["http://${DOCKER.REGISTRY_CONTAINER_NAME}:5000"]
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
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
`;

    const configPath = await createTempFile(kindConfig, '.yaml');

    try {
      // escapedName is already wrapped in single quotes for shell safety
      await execAsync(`kind create cluster --name ${escapedName} --config "${configPath}"`);
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
    const { stdout: allContainers } = await execAsync(
      `docker ps -a --filter "name=${DOCKER.REGISTRY_CONTAINER_NAME}" --format "{{.Names}}"`,
    );
    const containerExists = allContainers.trim() === DOCKER.REGISTRY_CONTAINER_NAME;

    if (!containerExists) {
      logger.debug('Local registry container does not exist');
      return null;
    }

    // Get the port mapping for the existing container
    const { stdout: portMapping } = await execAsync(
      `docker inspect ${DOCKER.REGISTRY_CONTAINER_NAME} --format '{{range $p, $conf := .NetworkSettings.Ports}}{{if eq $p "5000/tcp"}}{{(index $conf 0).HostPort}}{{end}}{{end}}'`,
    );
    const port = parseInt(portMapping.trim(), 10);

    if (isNaN(port)) {
      logger.warn('Could not determine registry port mapping');
      return null;
    }

    // Check if container is running
    const { stdout: runningContainers } = await execAsync(
      `docker ps --filter "name=${DOCKER.REGISTRY_CONTAINER_NAME}" --format "{{.Names}}"`,
    );
    const isRunning = runningContainers.trim() === DOCKER.REGISTRY_CONTAINER_NAME;

    if (isRunning) {
      logger.debug({ port }, 'Local registry is running');
      return port;
    }

    // Container exists but is stopped - try to start it
    logger.info('Local registry container exists but is stopped, starting it...');
    try {
      await execAsync(`docker start ${DOCKER.REGISTRY_CONTAINER_NAME}`);
      logger.info({ port }, 'Local registry started successfully');

      // After starting, check if it needs to be reconnected to kind network
      // (containers can lose network connections when stopped/restarted)
      const kindNetworkExists = await checkDockerNetworkExists('kind', logger);
      if (kindNetworkExists) {
        logger.debug('Checking if restarted registry is connected to kind network...');
        const isConnected = await checkContainerNetworkConnection(DOCKER.REGISTRY_CONTAINER_NAME, 'kind', logger);

        if (!isConnected) {
          logger.info('Registry not connected to kind network, reconnecting...');
          try {
            await execAsync(`docker network connect kind ${DOCKER.REGISTRY_CONTAINER_NAME}`);
            logger.info('Registry reconnected to kind network successfully');

            // Verify reconnection
            const reconnected = await checkContainerNetworkConnection(DOCKER.REGISTRY_CONTAINER_NAME, 'kind', logger);
            if (!reconnected) {
              logger.warn('Failed to verify registry reconnection to kind network');
            }
          } catch (reconnectError) {
            const errorMsg = extractErrorMessage(reconnectError);
            if (errorMsg.includes('already') || errorMsg.includes('duplicate')) {
              logger.debug('Registry already connected to kind network');
            } else {
              logger.warn({ error: errorMsg }, 'Failed to reconnect registry to kind network (non-fatal)');
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
 * Retries up to 3 times with 1 second delays.
 */
async function validateRegistryHealth(port: number, logger: pino.Logger): Promise<boolean> {
  const maxAttempts = 3;
  const delayMs = 1000;

  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    try {
      const { stdout } = await execAsync(
        `curl -sf http://${DOCKER.REGISTRY_HOST}:${port}/v2/ || echo "failed"`,
      );
      if (!stdout.includes('failed')) {
        logger.debug({ attempt }, 'Registry health check passed');
        return true;
      }
    } catch (error) {
      logger.debug({ attempt, error }, 'Registry health check attempt failed');
    }

    if (attempt < maxAttempts) {
      await new Promise((resolve) => setTimeout(resolve, delayMs));
    }
  }

  logger.warn('Registry health check failed after all attempts');
  return false;
}

/**
 * Check if Docker network exists.
 */
async function checkDockerNetworkExists(networkName: string, logger: pino.Logger): Promise<boolean> {
  try {
    const { stdout } = await execAsync(`docker network ls --filter "name=${networkName}" --format "{{.Name}}"`);
    const exists = stdout.split('\n').includes(networkName);
    logger.debug({ networkName, exists }, 'Checking Docker network existence');
    return exists;
  } catch (error) {
    logger.debug({ networkName, error }, 'Error checking Docker network');
    return false;
  }
}

/**
 * Check if container is already connected to network.
 */
async function checkContainerNetworkConnection(
  containerName: string,
  networkName: string,
  logger: pino.Logger,
): Promise<boolean> {
  try {
    const { stdout } = await execAsync(
      `docker inspect ${containerName} --format '{{range $net, $v := .NetworkSettings.Networks}}{{$net}} {{end}}'`,
    );
    const networks = stdout.trim().split(' ').filter(Boolean);
    const connected = networks.includes(networkName);
    logger.debug({ containerName, networkName, connected, networks }, 'Checking container network connection');
    return connected;
  } catch (error) {
    logger.debug({ containerName, networkName, error }, 'Error checking container network connection');
    return false;
  }
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
  try {
    const { stdout } = await execAsync(
      `docker inspect ${containerName} --format '{{.NetworkSettings.Networks.${networkName}.IPAddress}}'`,
    );
    const ip = stdout.trim();
    if (ip && ip !== '<no value>') {
      logger.debug({ containerName, networkName, ip }, 'Got container network IP');
      return ip;
    }
    return null;
  } catch (error) {
    logger.debug({ containerName, networkName, error }, 'Error getting container network IP');
    return null;
  }
}

/**
 * Verify registry is accessible from within the kind cluster.
 * Uses kubectl run to create a test pod that curls the registry endpoint.
 */
async function verifyRegistryFromCluster(_port: number, logger: pino.Logger): Promise<boolean> {
  try {
    logger.debug('Testing registry reachability from within cluster...');

    // Create a temporary test pod that curls the registry
    const testPodName = `registry-test-${Date.now()}`;
    const curlCommand = `curl -sf http://${DOCKER.REGISTRY_CONTAINER_NAME}:5000/v2/ && echo "success" || echo "failed"`;

    try {
      // Run test pod and wait for completion (timeout 30s)
      const { stdout } = await execAsync(
        `kubectl run ${testPodName} --image=curlimages/curl:latest --restart=Never --rm -i --timeout=30s -- sh -c '${curlCommand}'`,
        { timeout: 35000 },
      );

      const success = stdout.includes('success');
      logger.debug({ testPodName, success, output: stdout.trim() }, 'In-cluster registry test result');

      return success;
    } catch (error) {
      // If pod creation fails, try to clean it up
      try {
        await execAsync(`kubectl delete pod ${testPodName} --ignore-not-found=true`);
      } catch {
        // Ignore cleanup errors
      }
      logger.debug({ error }, 'In-cluster registry test failed');
      return false;
    }
  } catch (error) {
    logger.warn({ error }, 'Error testing registry from cluster');
    return false;
  }
}

/**
 * Validate containerd mirror configuration on kind node.
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
    const { stdout } = await execAsync(
      `docker exec ${nodeContainerName} cat /etc/containerd/config.toml`,
    );

    // Check for the registry mirror configuration
    const hasLocalRegistryMirror = stdout.includes(`${DOCKER.REGISTRY_HOST}:${port}`);
    const hasKindRegistryEndpoint = stdout.includes(`${DOCKER.REGISTRY_CONTAINER_NAME}:5000`);

    const isValid = hasLocalRegistryMirror && hasKindRegistryEndpoint;

    logger.debug(
      {
        clusterName,
        hasLocalRegistryMirror,
        hasKindRegistryEndpoint,
        isValid,
      },
      'Containerd config validation result',
    );

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

async function createLocalRegistry(logger: pino.Logger): Promise<{ url: string; port: number }> {
  try {
    logger.info('Creating local Docker registry...');

    // Find an available port
    const port = await findRegistryPort();
    logger.debug({ port }, 'Found available port for registry');

    await execAsync(`docker run -d --restart=always -p ${port}:5000 --name ${DOCKER.REGISTRY_CONTAINER_NAME} registry:2`);
    logger.debug({ port }, 'Registry container created');

    // Check if kind network exists before attempting connection
    const kindNetworkExists = await checkDockerNetworkExists('kind', logger);
    if (!kindNetworkExists) {
      logger.warn('Kind Docker network does not exist - registry will not be accessible from cluster yet');
      const registryUrl = `${DOCKER.REGISTRY_HOST}:${port}`;
      return { url: registryUrl, port };
    }

    // Check if registry is already connected to kind network
    const alreadyConnected = await checkContainerNetworkConnection(DOCKER.REGISTRY_CONTAINER_NAME, 'kind', logger);
    if (alreadyConnected) {
      logger.debug('Registry already connected to kind network');
    } else {
      // Connect registry to kind network
      try {
        logger.debug('Connecting registry to kind Docker network...');
        await execAsync(`docker network connect kind ${DOCKER.REGISTRY_CONTAINER_NAME}`);
        logger.info('Registry connected to kind network successfully');
      } catch (networkError) {
        const errorMsg = extractErrorMessage(networkError);
        // Only treat as non-fatal if it's "already connected" error
        if (errorMsg.includes('already') || errorMsg.includes('duplicate')) {
          logger.debug({ error: errorMsg }, 'Registry already connected to kind network');
        } else {
          logger.error({ error: errorMsg }, 'Failed to connect registry to kind network');
          throw new Error(`Failed to connect registry to kind network: ${errorMsg}`);
        }
      }

      // Verify connection succeeded by checking again
      logger.debug('Verifying registry network connection...');
      const connectedAfter = await checkContainerNetworkConnection(DOCKER.REGISTRY_CONTAINER_NAME, 'kind', logger);
      if (!connectedAfter) {
        logger.warn('Registry network connection verification failed - may not be accessible from cluster');
      } else {
        // Get and log the IP address on the kind network
        const registryIP = await getContainerNetworkIP(DOCKER.REGISTRY_CONTAINER_NAME, 'kind', logger);
        if (registryIP) {
          logger.info({ registryIP }, 'Registry network connection verified successfully');
        } else {
          logger.info('Registry network connection verified (IP not available)');
        }
      }
    }

    // Validate registry health
    logger.debug('Validating registry health...');
    const isHealthy = await validateRegistryHealth(port, logger);
    if (!isHealthy) {
      logger.warn('Registry health check failed - registry may not be fully ready');
    }

    const registryUrl = `${DOCKER.REGISTRY_HOST}:${port}`;
    logger.info({ registryUrl, healthy: isHealthy }, 'Local Docker registry created successfully');
    return { url: registryUrl, port };
  } catch (error) {
    logger.error({ error }, 'Failed to create local registry');
    throw new Error(`Local registry creation failed: ${extractErrorMessage(error)}`);
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
  const escapedNameResult = validateAndEscapeClusterName(clusterName);
  if (!escapedNameResult.ok) {
    return escapedNameResult;
  }
  const escapedName = escapedNameResult.value;

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
    try {
      const { stdout } = await execAsync('kubectl get nodes --no-headers');
      const nodesReady = stdout.includes('Ready');
      logger.debug({ nodesReady, output: stdout.trim() }, 'Cluster node readiness check');
      if (!nodesReady) {
        logger.warn('Cluster nodes may not be fully ready yet');
      }
    } catch (error) {
      logger.debug({ error }, 'Could not check node readiness (non-fatal)');
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
  try {
    // escapedName is already wrapped in single quotes for shell safety
    await execAsync(`kind export kubeconfig --name ${escapedName}`);
  } catch (error) {
    logger.warn({ error: String(error) }, 'Failed to export kubeconfig, continuing anyway');
  }

  return Success(undefined);
}

/**
 * Setup local Docker registry if needed
 * Returns detailed registry information including health and connectivity status
 */
async function setupLocalRegistry(
  logger: pino.Logger,
  checks: {
    localRegistryCreated: boolean | undefined;
  },
): Promise<{
  url: string;
  port: number;
  healthy: boolean;
  reachableFromCluster: boolean;
}> {
  logger.debug('Starting local registry setup');
  const existingPort = await checkLocalRegistryExists(logger);
  let healthy = false;

  if (existingPort === null) {
    const { url: registryUrl, port } = await createLocalRegistry(logger);
    checks.localRegistryCreated = true;

    // Check health after creation
    healthy = await validateRegistryHealth(port, logger);

    logger.info({ registryUrl, healthy }, 'Local registry creation completed');
    return {
      url: registryUrl,
      port,
      healthy,
      reachableFromCluster: false, // Will be checked later
    };
  } else {
    const registryUrl = `${DOCKER.REGISTRY_HOST}:${existingPort}`;
    checks.localRegistryCreated = true;

    // Check health of existing registry
    healthy = await validateRegistryHealth(existingPort, logger);

    logger.info({ registryUrl, healthy }, 'Local registry already exists');
    return {
      url: registryUrl,
      port: existingPort,
      healthy,
      reachableFromCluster: false, // Will be checked later
    };
  }
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
      resolution: 'Ensure Kubernetes is installed and a cluster is accessible (kubectl cluster-info)',
    });
  }

  // Check permissions
  checks.permissions = await k8sClient.checkPermissions(namespace);
  if (!checks.permissions) {
    return Failure('Insufficient permissions for Kubernetes operations', {
      message: 'Kubernetes permissions check failed',
      hint: 'Current user/service account lacks required permissions',
      resolution: 'Verify current privileges with RBAC: run `kubectl auth can-i <verb> <resource> --namespace <namespace>` for required operations',
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

  const { environment = 'development', namespace = 'default', targetPlatform = 'linux/amd64' } = params;

  // Validate namespace
  const namespaceValidation = validateNamespace(namespace);
  if (!namespaceValidation.ok) {
    return namespaceValidation;
  }

  const clusterName = environment === 'development' ? 'containerization-assist' : 'default';
  const shouldCreateNamespace = environment === 'production';
  const shouldSetupRbac = environment === 'production';
  const installIngress = false;
  const checkRequirements = true;
  const shouldSetupKind = environment === 'development';
  const shouldCreateLocalRegistry = environment === 'development';

  try {
    logger.info({ environment, namespace }, 'Starting Kubernetes cluster preparation');

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
      platformCompatible: undefined as boolean | undefined,
    };
    let localRegistryUrl: string | undefined;
    let localRegistryInfo: PrepareClusterResult['localRegistry'] | undefined;
    let registryPort: number | undefined;

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

    // Setup Kind cluster if in development environment
    // Pass the registry port so it can be configured in the cluster
    if (shouldSetupKind && registryPort !== undefined) {
      const setupResult = await setupKindCluster(clusterName, registryPort, logger, checks);
      if (!setupResult.ok) {
        return setupResult;
      }
    }

    const k8sClient = createKubernetesClient(logger);

    // Setup local Docker registry if in development environment
    if (shouldCreateLocalRegistry && registryPort !== undefined) {
      const registrySetup = await setupLocalRegistry(logger, checks);
      localRegistryUrl = registrySetup.url;
      registryPort = registrySetup.port;

      // Create registry ConfigMap after cluster and registry are set up
      if (shouldSetupKind) {
        await createLocalRegistryConfigMap(k8sClient, registryPort, logger);

        // Validate containerd mirror configuration
        logger.debug('Validating containerd registry mirror configuration...');
        const containerdConfigValid = await validateContainerdConfig(clusterName, registryPort, logger);
        if (!containerdConfigValid) {
          warnings.push(`Containerd registry mirror configuration validation failed - image pulls from ${DOCKER.REGISTRY_HOST}:${registryPort} may not work`);
        } else {
          logger.info('Containerd registry mirror configuration validated successfully');
        }

        // Test registry reachability from within cluster
        logger.debug('Testing registry reachability from cluster...');
        const registryReachable = await verifyRegistryFromCluster(registryPort, logger);
        if (!registryReachable) {
          warnings.push('Registry is not reachable from within cluster - deployment may fail');
        } else {
          logger.info('Registry reachability from cluster validated successfully');
        }

        // Populate detailed registry information
        localRegistryInfo = {
          externalUrl: registrySetup.url,
          internalEndpoint: `${DOCKER.REGISTRY_CONTAINER_NAME}:5000`,
          containerName: DOCKER.REGISTRY_CONTAINER_NAME,
          healthy: registrySetup.healthy,
          reachableFromCluster: registryReachable,
        };
      }
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

    // Validate platform compatibility
    logger.debug({ targetPlatform }, 'Validating platform compatibility...');
    const platformValidation = await validatePlatformCompatibility(targetPlatform, logger, warnings);
    checks.platformCompatible = platformValidation.compatible;

    const platformInfo: PrepareClusterResult['platform'] = {
      target: targetPlatform,
      cluster: platformValidation.clusterPlatform,
      compatible: platformValidation.compatible,
      requiresEmulation: platformValidation.requiresEmulation,
    };

    // Generate summary
    const namespaceAction = checks.namespaceExists ? 'verified' : 'created';
    const resourcesConfigured = Object.values(checks).filter(Boolean).length;
    const summary = `✅ Cluster prepared. Namespace '${namespace}' ${namespaceAction}. ${pluralize(resourcesConfigured, 'resource')} configured. Ready for deployment.`;

    const result: PrepareClusterResult = {
      summary,
      success: true,
      clusterReady,
      cluster: clusterName,
      namespace,
      ...(platformInfo && { platform: platformInfo }),
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
        ...(checks.platformCompatible !== undefined && {
          platformCompatible: checks.platformCompatible,
        }),
      },
      ...(warnings.length > 0 && { warnings }),
      ...(localRegistryUrl && { localRegistryUrl }),
      ...(localRegistryInfo && { localRegistry: localRegistryInfo }),
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
      resolution: 'Check the error message for details. Common issues include Docker not running (for kind clusters), kubectl not configured, or insufficient permissions',
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
    success: 'Cluster preparation successful. Next: Use `kubectl apply -f <manifest-folder>` to deploy your manifests to the cluster, then call verify-deploy to check deployment status.',
    failure:
      'Cluster preparation found issues. Check connectivity, permissions, and namespace configuration.',
  },
  handler: handlePrepareCluster,
});
