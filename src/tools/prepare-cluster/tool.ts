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
import { getSystemInfo, getDownloadOS, getDownloadArch } from '@/lib/platform';
import { downloadFile, makeExecutable, createTempFile, deleteTempFile } from '@/lib/file-utils';

import type * as pino from 'pino';
import { Success, Failure, type Result } from '@/types';
import { prepareClusterSchema, type PrepareClusterParams } from './schema';
import { exec } from 'node:child_process';
import { promisify } from 'node:util';

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
    );
  }

  if (clusterName.length > 63) {
    return Failure(`Cluster name too long: "${clusterName}". Must be 63 characters or less.`);
  }

  // Wrap in single quotes for defense-in-depth shell safety
  // Note: The regex already prevents single quotes, so the replace is technically
  // unnecessary, but we keep it as a defensive measure in case validation changes
  return Success(`'${clusterName.replace(/'/g, "'\\''")}'`);
}

export interface PrepareClusterResult {
  success: boolean;
  clusterReady: boolean;
  cluster: string;
  namespace: string;
  checks: {
    connectivity: boolean;
    permissions: boolean;
    namespaceExists: boolean;
    ingressController?: boolean;
    rbacConfigured?: boolean;
    kindInstalled?: boolean;
    kindClusterCreated?: boolean;
    localRegistryCreated?: boolean;
  };
  warnings?: string[];
  localRegistryUrl?: string;
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

async function createKindCluster(clusterName: string, logger: pino.Logger): Promise<Result<void>> {
  const escapedNameResult = validateAndEscapeClusterName(clusterName);
  if (!escapedNameResult.ok) {
    return escapedNameResult;
  }
  const escapedName = escapedNameResult.value;

  try {
    logger.info({ clusterName }, 'Creating kind cluster...');

    // Note: ${DOCKER.LOCAL_REGISTRY_PORT} is interpolated by JavaScript (to 5001),
    // not by YAML. This template literal produces a valid YAML string with literal port numbers.
    const kindConfig = `
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${DOCKER.LOCAL_REGISTRY_PORT}"]
    endpoint = ["http://kind-registry:${DOCKER.LOCAL_REGISTRY_PORT}"]
nodes:
- role: control-plane
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

async function checkLocalRegistryExists(logger: pino.Logger): Promise<boolean> {
  try {
    const { stdout } = await execAsync(
      'docker ps -a --filter "name=kind-registry" --format "{{.Names}}"',
    );
    const exists = stdout.trim() === 'kind-registry';
    logger.debug({ exists }, 'Checking local registry existence');
    return exists;
  } catch (error) {
    logger.debug({ error }, 'Error checking local registry');
    return false;
  }
}

async function createLocalRegistry(logger: pino.Logger): Promise<string> {
  try {
    logger.info('Creating local Docker registry...');

    await execAsync(`docker run -d --restart=always -p ${DOCKER.LOCAL_REGISTRY_PORT}:${DOCKER.INTERNAL_REGISTRY_PORT} --name kind-registry registry:2`);

    try {
      await execAsync('docker network connect kind kind-registry');
    } catch {
      logger.debug('Registry might already be connected to kind network');
    }

    const registryUrl = `localhost:${DOCKER.LOCAL_REGISTRY_PORT}`;
    logger.info({ registryUrl }, 'Local Docker registry created successfully');
    return registryUrl;
  } catch (error) {
    logger.error({ error }, 'Failed to create local registry');
    throw new Error(`Local registry creation failed: ${extractErrorMessage(error)}`);
  }
}

/**
 * Setup Kind cluster if needed
 */
async function setupKindCluster(
  cluster: string,
  logger: pino.Logger,
  checks: {
    kindInstalled: boolean | undefined;
    kindClusterCreated: boolean | undefined;
  },
): Promise<Result<void>> {
  // Validate cluster name upfront
  const escapedNameResult = validateAndEscapeClusterName(cluster);
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

  const clusterExistsResult = await checkKindClusterExists(cluster, logger);
  if (!clusterExistsResult.ok) {
    return clusterExistsResult;
  }
  const kindClusterExists = clusterExistsResult.value;

  if (!kindClusterExists) {
    const createResult = await createKindCluster(cluster, logger);
    if (!createResult.ok) {
      return createResult;
    }
    checks.kindClusterCreated = true;
    logger.info({ clusterName: cluster }, 'Kind cluster creation completed');

    // Wait for cluster to stabilize
    await new Promise((resolve) => setTimeout(resolve, DEFAULT_TIMEOUTS.clusterStabilization));
  } else {
    checks.kindClusterCreated = true;
    logger.info({ clusterName: cluster }, 'Kind cluster already exists');
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
 */
async function setupLocalRegistry(
  logger: pino.Logger,
  checks: {
    localRegistryCreated: boolean | undefined;
  },
): Promise<string> {
  const registryExists = await checkLocalRegistryExists(logger);
  if (!registryExists) {
    const registryUrl = await createLocalRegistry(logger);
    checks.localRegistryCreated = true;
    logger.info({ registryUrl }, 'Local registry creation completed');
    return registryUrl;
  } else {
    const registryUrl = `localhost:${DOCKER.LOCAL_REGISTRY_PORT}`;
    checks.localRegistryCreated = true;
    logger.info({ registryUrl }, 'Local registry already exists');
    return registryUrl;
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
    return Failure('Cannot connect to Kubernetes cluster');
  }

  // Check permissions
  checks.permissions = await k8sClient.checkPermissions(namespace);
  if (!checks.permissions) {
    warnings.push('Limited permissions - some operations may fail');
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

  const { environment = 'development', namespace = 'default' } = params;

  // Validate namespace
  const namespaceValidation = validateNamespace(namespace);
  if (!namespaceValidation.ok) {
    return namespaceValidation;
  }

  const cluster = environment === 'development' ? 'kind' : 'default';
  const shouldCreateNamespace = environment === 'production';
  const shouldSetupRbac = environment === 'production';
  const installIngress = false;
  const checkRequirements = true;
  const shouldSetupKind = environment === 'development';
  const shouldCreateLocalRegistry = environment === 'development';

  try {
    logger.info({ environment, namespace }, 'Starting Kubernetes cluster preparation');

    const k8sClient = createKubernetesClient(logger);

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
    };
    let localRegistryUrl: string | undefined;

    // Setup Kind cluster if in development environment
    if (shouldSetupKind) {
      const setupResult = await setupKindCluster(cluster, logger, checks);
      if (!setupResult.ok) {
        return setupResult;
      }
    }

    // Setup local Docker registry if in development environment
    if (shouldCreateLocalRegistry) {
      localRegistryUrl = await setupLocalRegistry(logger, checks);
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

    const result: PrepareClusterResult = {
      success: true,
      clusterReady,
      cluster,
      namespace,
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
      },
      ...(warnings.length > 0 && { warnings }),
      ...(localRegistryUrl && { localRegistryUrl }),
    };

    logger.info({ clusterReady, checks }, 'Cluster preparation completed');

    timer.end({ clusterReady, environment });

    return Success(result);
  } catch (error) {
    timer.error(error);

    const errorMessage = error instanceof Error ? error.message : String(error);
    return Failure(errorMessage);
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
    success: 'Cluster preparation successful. Next: Call deploy to deploy to the kind cluster.',
    failure:
      'Cluster preparation found issues. Check connectivity, permissions, and namespace configuration.',
  },
  handler: handlePrepareCluster,
});
