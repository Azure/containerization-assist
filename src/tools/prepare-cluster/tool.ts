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

import { getToolLogger, createToolTimer } from '@/lib/tool-helpers';
import { extractErrorMessage } from '@/lib/error-utils';
import type { ToolContext } from '@/mcp/context';
import { createKubernetesClient, type K8sManifest } from '@/infra/kubernetes/client';
import { getSystemInfo, getDownloadOS, getDownloadArch } from '@/lib/platform-utils';
import { downloadFile, makeExecutable, createTempFile, deleteTempFile } from '@/lib/file-utils';

import type * as pino from 'pino';
import { Success, Failure, type Result, type ErrorGuidance } from '@/types';
import { prepareClusterSchema, type PrepareClusterParams } from './schema';
import { exec } from 'node:child_process';
import { promisify } from 'node:util';

const execAsync = promisify(exec);

const KIND_VERSION = 'v0.20.0';

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

interface K8sClientAdapter {
  ping(): Promise<boolean>;
  namespaceExists(namespace: string): Promise<boolean>;
  ensureNamespace(
    namespace: string,
  ): Promise<{ success: boolean; error?: string; guidance?: ErrorGuidance }>;
  applyManifest(
    manifest: Record<string, unknown>,
    namespace?: string,
  ): Promise<{ success: boolean; error?: string; guidance?: ErrorGuidance }>;
  checkIngressController(): Promise<boolean>;
  checkPermissions(namespace: string): Promise<boolean>;
}

function createK8sClientAdapter(
  k8sClient: ReturnType<typeof createKubernetesClient>,
): K8sClientAdapter {
  return {
    ping: () => k8sClient.ping(),
    namespaceExists: (namespace: string) => k8sClient.namespaceExists(namespace),
    ensureNamespace: async (namespace: string) => {
      const result = await k8sClient.ensureNamespace(namespace);
      if (result.ok) {
        return { success: true };
      } else {
        return {
          success: false,
          error: result.error,
          ...(result.guidance && { guidance: result.guidance }),
        };
      }
    },
    applyManifest: async (manifest: Record<string, unknown>, namespace?: string) => {
      const result = await k8sClient.applyManifest(manifest as unknown as K8sManifest, namespace);
      if (result.ok) {
        return { success: true };
      } else {
        return {
          success: false,
          error: result.error,
          ...(result.guidance && { guidance: result.guidance }),
        };
      }
    },
    checkIngressController: () => k8sClient.checkIngressController(),
    checkPermissions: (namespace: string) => k8sClient.checkPermissions(namespace),
  };
}

async function checkConnectivity(
  k8sClient: K8sClientAdapter,
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
  k8sClient: K8sClientAdapter,
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
  k8sClient: K8sClientAdapter,
  namespace: string,
  logger: pino.Logger,
): Promise<void> {
  try {
    const serviceAccount = {
      apiVersion: 'v1',
      kind: 'ServiceAccount',
      metadata: {
        name: 'app-service-account',
        namespace,
      },
    };

    const result = await k8sClient.applyManifest(serviceAccount, namespace);
    if (result.success) {
      logger.info({ namespace }, 'RBAC configured');
    } else {
      logger.warn({ namespace, error: result.error }, 'RBAC setup failed');
    }
  } catch (error) {
    logger.warn({ namespace, error }, 'RBAC setup failed');
  }
}

async function checkIngressController(
  k8sClient: K8sClientAdapter,
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

async function checkKindClusterExists(clusterName: string, logger: pino.Logger): Promise<boolean> {
  try {
    const { stdout } = await execAsync('kind get clusters');
    const clusters = stdout
      .trim()
      .split('\n')
      .filter((line: string) => line.trim());
    const exists = clusters.includes(clusterName);
    logger.debug({ clusterName, exists, clusters }, 'Checking kind cluster existence');
    return exists;
  } catch (error) {
    logger.debug({ error }, 'Error checking kind clusters');
    return false;
  }
}

async function createKindCluster(clusterName: string, logger: pino.Logger): Promise<void> {
  try {
    logger.info({ clusterName }, 'Creating kind cluster...');

    const kindConfig = `
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:5001"]
    endpoint = ["http://kind-registry:5001"]
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
      await execAsync(`kind create cluster --name ${clusterName} --config "${configPath}"`);
      logger.info({ clusterName }, 'Kind cluster created successfully');
    } finally {
      await deleteTempFile(configPath);
    }
  } catch (error) {
    logger.error({ clusterName, error }, 'Failed to create kind cluster');
    throw new Error(`Kind cluster creation failed: ${extractErrorMessage(error)}`);
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

    await execAsync('docker run -d --restart=always -p 5001:5000 --name kind-registry registry:2');

    try {
      await execAsync('docker network connect kind kind-registry');
    } catch {
      logger.debug('Registry might already be connected to kind network');
    }

    const registryUrl = 'localhost:5001';
    logger.info({ registryUrl }, 'Local Docker registry created successfully');
    return registryUrl;
  } catch (error) {
    logger.error({ error }, 'Failed to create local registry');
    throw new Error(`Local registry creation failed: ${extractErrorMessage(error)}`);
  }
}

/**
 * Core cluster preparation implementation
 */
async function handlePrepareCluster(
  params: PrepareClusterParams,
  context: ToolContext,
): Promise<Result<PrepareClusterResult>> {
  const logger = getToolLogger(context, 'prepare-cluster');
  const timer = createToolTimer(logger, 'prepare-cluster');

  const { environment = 'development', namespace = 'default' } = params;

  const cluster = environment === 'development' ? 'kind' : 'default';
  const shouldCreateNamespace = environment === 'production';
  const shouldSetupRbac = environment === 'production';
  const installIngress = false;
  const checkRequirements = true;
  const shouldSetupKind = environment === 'development';
  const shouldCreateLocalRegistry = environment === 'development';

  try {
    logger.info({ environment, namespace }, 'Starting Kubernetes cluster preparation');

    const k8sClientRaw = createKubernetesClient(logger);
    const k8sClient = createK8sClientAdapter(k8sClientRaw);

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

    if (shouldSetupKind) {
      checks.kindInstalled = await checkKindInstalled(logger);
      if (!checks.kindInstalled) {
        await installKind(logger);
        checks.kindInstalled = true;
        logger.info('Kind installation completed');
      }

      const kindClusterName = cluster;
      const kindClusterExists = await checkKindClusterExists(kindClusterName, logger);
      if (!kindClusterExists) {
        await createKindCluster(kindClusterName, logger);
        checks.kindClusterCreated = true;
        logger.info({ clusterName: kindClusterName }, 'Kind cluster creation completed');

        await new Promise((resolve) => setTimeout(resolve, 5000));
      } else {
        checks.kindClusterCreated = true;
        logger.info({ clusterName: kindClusterName }, 'Kind cluster already exists');
      }

      try {
        await execAsync(`kind export kubeconfig --name ${kindClusterName}`);
      } catch (error) {
        logger.warn({ error: String(error) }, 'Failed to export kubeconfig, continuing anyway');
      }
    }

    if (shouldCreateLocalRegistry) {
      const registryExists = await checkLocalRegistryExists(logger);
      if (!registryExists) {
        localRegistryUrl = await createLocalRegistry(logger);
        checks.localRegistryCreated = true;
        logger.info({ registryUrl: localRegistryUrl }, 'Local registry creation completed');
      } else {
        localRegistryUrl = 'localhost:5001';
        checks.localRegistryCreated = true;
        logger.info({ registryUrl: localRegistryUrl }, 'Local registry already exists');
      }
    }

    checks.connectivity = await checkConnectivity(k8sClient, logger);
    if (!checks.connectivity) {
      return Failure('Cannot connect to Kubernetes cluster');
    }

    checks.permissions = await k8sClient.checkPermissions(namespace);
    if (!checks.permissions) {
      warnings.push('Limited permissions - some operations may fail');
    }

    checks.namespaceExists = await checkNamespace(k8sClient, namespace, logger);
    if (!checks.namespaceExists && shouldCreateNamespace) {
      const ensureResult = await k8sClient.ensureNamespace(namespace);
      if (ensureResult.success) {
        checks.namespaceExists = true;
        logger.info({ namespace }, 'Namespace created successfully');
      } else {
        logger.error({ namespace, error: ensureResult.error }, 'Failed to create namespace');
        return Failure(ensureResult.error || 'Failed to create namespace', ensureResult.guidance);
      }
    } else if (!checks.namespaceExists) {
      warnings.push(`Namespace ${namespace} does not exist - deployment may fail`);
    }

    if (shouldSetupRbac) {
      await setupRbac(k8sClient, namespace, logger);
      checks.rbacConfigured = true;
    }

    if (checkRequirements || installIngress) {
      checks.ingressController = await checkIngressController(k8sClient, logger);
      if (!checks.ingressController) {
        warnings.push('No ingress controller found - external access may not work');
      }
    }

    const clusterReady = checks.connectivity && checks.permissions && checks.namespaceExists;

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
    enhancementCapabilities: [],
  },
  handler: handlePrepareCluster,
});
