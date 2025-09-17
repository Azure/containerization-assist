/**
 * Prepare Cluster Tool - Standardized Implementation
 *
 * Prepares and validates Kubernetes cluster for deployment using standardized
 * helpers for consistency and improved error handling
 *
 * @example
 * ```typescript
 * const result = await prepareCluster({
 *   sessionId: 'session-123',
 *   namespace: 'my-app',
 *   environment: 'production'
 * }, context, logger);
 *
 * if (result.success) {
 *   logger.info('Cluster ready', {
 *     ready: result.clusterReady,
 *     checks: result.checks
 *   });
 * }
 * ```
 */

import { ensureSession, defineToolIO, useSessionSlice } from '@mcp/tool-session-helpers';
import { getToolLogger, createToolTimer } from '@lib/tool-helpers';
import { extractErrorMessage } from '@lib/error-utils';
import type { ToolContext } from '@mcp/context';
import { createKubernetesClient } from '@lib/kubernetes';

import type * as pino from 'pino';
import { Success, Failure, type Result } from '@types';
import { prepareClusterSchema, type PrepareClusterParams } from './schema';
import { exec } from 'child_process';
import { promisify } from 'util';
import { z } from 'zod';
import type { SessionData } from '@tools/session-types';

const execAsync = promisify(exec);

export interface PrepareClusterResult {
  success: boolean;
  sessionId: string;
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

// Define the result schema for type safety
const PrepareClusterResultSchema = z.object({
  success: z.boolean(),
  sessionId: z.string(),
  clusterReady: z.boolean(),
  cluster: z.string(),
  namespace: z.string(),
  checks: z.object({
    connectivity: z.boolean(),
    permissions: z.boolean(),
    namespaceExists: z.boolean(),
    ingressController: z.boolean().optional(),
    rbacConfigured: z.boolean().optional(),
    kindInstalled: z.boolean().optional(),
    kindClusterCreated: z.boolean().optional(),
    localRegistryCreated: z.boolean().optional(),
  }),
  warnings: z.array(z.string()).optional(),
  localRegistryUrl: z.string().optional(),
});

// Define tool IO for type-safe session operations
const io = defineToolIO(prepareClusterSchema, PrepareClusterResultSchema);

// Tool-specific state schema
const StateSchema = z.object({
  lastPreparedAt: z.date().optional(),
  lastClusterName: z.string().optional(),
  lastNamespace: z.string().optional(),
  totalPreparations: z.number().optional(),
  lastClusterReady: z.boolean().optional(),
  lastChecksPassed: z.number().optional(),
  lastWarningCount: z.number().optional(),
});

interface K8sClientAdapter {
  ping(): Promise<boolean>;
  namespaceExists(namespace: string): Promise<boolean>;
  applyManifest(
    manifest: Record<string, unknown>,
    namespace?: string,
  ): Promise<{ success: boolean; error?: string }>;
  checkIngressController(): Promise<boolean>;
  checkPermissions(namespace: string): Promise<boolean>;
}

function createK8sClientAdapter(
  k8sClient: ReturnType<typeof createKubernetesClient>,
): K8sClientAdapter {
  return {
    ping: () => k8sClient.ping(),
    namespaceExists: (namespace: string) => k8sClient.namespaceExists(namespace),
    applyManifest: async (manifest: Record<string, unknown>, namespace?: string) => {
      const result = await k8sClient.applyManifest(manifest, namespace);
      if (result.ok) {
        return { success: true };
      } else {
        return { success: false, error: result.error };
      }
    },
    checkIngressController: () => k8sClient.checkIngressController(),
    checkPermissions: (namespace: string) => k8sClient.checkPermissions(namespace),
  };
}

/**
 * Check cluster connectivity
 */
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

/**
 * Check namespace exists
 */
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

/**
 * Create namespace if needed
 */
async function createNamespace(
  k8sClient: K8sClientAdapter,
  namespace: string,
  logger: pino.Logger,
): Promise<void> {
  try {
    const namespaceManifest = {
      apiVersion: 'v1',
      kind: 'Namespace',
      metadata: {
        name: namespace,
      },
    };

    const result = await k8sClient.applyManifest(namespaceManifest);
    if (result.success) {
      logger.info({ namespace }, 'Namespace created');
    } else {
      throw new Error(result.error || 'Failed to create namespace');
    }
  } catch (error) {
    logger.error({ namespace, error }, 'Failed to create namespace');
    throw error;
  }
}

/**
 * Setup RBAC if needed
 */
async function setupRbac(
  k8sClient: K8sClientAdapter,
  namespace: string,
  logger: pino.Logger,
): Promise<void> {
  try {
    // Create service account
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

/**
 * Check for ingress controller
 */
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

/**
 * Check if kind is installed
 */
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

/**
 * Install kind if not present
 */
async function installKind(logger: pino.Logger): Promise<void> {
  try {
    logger.info('Installing kind...');

    // Detect platform
    const { stdout: osStdout } = await execAsync('uname -s');
    const { stdout: archStdout } = await execAsync('uname -m');
    const os = osStdout.trim().toLowerCase();
    const arch = archStdout.trim();

    // Map architecture names
    let kindArch = arch;
    if (arch === 'x86_64') kindArch = 'amd64';
    if (arch === 'aarch64') kindArch = 'arm64';

    const kindVersion = 'v0.20.0'; // Use latest stable version
    const kindUrl = `https://kind.sigs.k8s.io/dl/${kindVersion}/kind-${os}-${kindArch}`;

    // Download and install kind
    await execAsync(`curl -Lo ./kind ${kindUrl}`);
    await execAsync('chmod +x ./kind');
    await execAsync('sudo mv ./kind /usr/local/bin/kind');

    logger.info('Kind installed successfully');
  } catch (error) {
    logger.error({ error }, 'Failed to install kind');
    throw new Error(`Kind installation failed: ${extractErrorMessage(error)}`);
  }
}

/**
 * Check if kind cluster exists
 */
async function checkKindClusterExists(clusterName: string, logger: pino.Logger): Promise<boolean> {
  try {
    const { stdout } = await execAsync('kind get clusters');
    const clusters = stdout
      .trim()
      .split('\n')
      .filter((line) => line.trim());
    const exists = clusters.includes(clusterName);
    logger.debug({ clusterName, exists, clusters }, 'Checking kind cluster existence');
    return exists;
  } catch (error) {
    logger.debug({ error }, 'Error checking kind clusters');
    return false;
  }
}

/**
 * Create kind cluster with local registry
 */
async function createKindCluster(clusterName: string, logger: pino.Logger): Promise<void> {
  try {
    logger.info({ clusterName }, 'Creating kind cluster...');

    // Create kind cluster with registry config
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

    // Write config to temporary file
    await execAsync(`echo '${kindConfig}' > /tmp/kind-config.yaml`);

    // Create cluster
    await execAsync(`kind create cluster --name ${clusterName} --config /tmp/kind-config.yaml`);

    // Clean up config file
    await execAsync('rm /tmp/kind-config.yaml');

    logger.info({ clusterName }, 'Kind cluster created successfully');
  } catch (error) {
    logger.error({ clusterName, error }, 'Failed to create kind cluster');
    throw new Error(`Kind cluster creation failed: ${extractErrorMessage(error)}`);
  }
}

/**
 * Check if local registry container exists
 */
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

/**
 * Create local Docker registry for kind
 */
async function createLocalRegistry(logger: pino.Logger): Promise<string> {
  try {
    logger.info('Creating local Docker registry...');

    // Create registry container
    await execAsync(`
      docker run -d --restart=always -p 5001:5000 --name kind-registry registry:2
    `);

    // Connect registry to kind network
    try {
      await execAsync('docker network connect kind kind-registry');
    } catch {
      // Network might already be connected, ignore error
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
async function prepareClusterImpl(
  params: PrepareClusterParams,
  context: ToolContext,
): Promise<Result<PrepareClusterResult>> {
  const logger = getToolLogger(context, 'prepare-cluster');
  const timer = createToolTimer(logger, 'prepare-cluster');

  try {
    const { environment = 'development', namespace = 'default' } = params;

    const cluster = environment === 'development' ? 'kind' : 'default';
    const shouldCreateNamespace = environment === 'production';
    const shouldSetupRbac = environment === 'production';
    const installIngress = false;
    const checkRequirements = true;
    const shouldSetupKind = environment === 'development';
    const shouldCreateLocalRegistry = environment === 'development';

    logger.info({ cluster, namespace, environment }, 'Starting cluster preparation');

    // Ensure session exists and get typed slice operations
    const sessionResult = await ensureSession(context, params.sessionId);
    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }

    const { id: sessionId, state: session } = sessionResult.value;
    const slice = useSessionSlice('prepare-cluster', io, context, StateSchema);

    if (!slice) {
      return Failure('Session manager not available');
    }

    logger.info(
      { sessionId, environment, namespace },
      'Starting Kubernetes cluster preparation with session',
    );

    // Record input in session slice
    await slice.patch(sessionId, { input: params });

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

    // 0. Setup kind and local registry for development
    if (shouldSetupKind) {
      // Check/install kind
      checks.kindInstalled = await checkKindInstalled(logger);
      if (!checks.kindInstalled) {
        await installKind(logger);
        checks.kindInstalled = true;
        logger.info('Kind installation completed');
      }

      // Check/create kind cluster
      const kindClusterName = cluster;
      const kindClusterExists = await checkKindClusterExists(kindClusterName, logger);
      if (!kindClusterExists) {
        await createKindCluster(kindClusterName, logger);
        checks.kindClusterCreated = true;
        logger.info({ clusterName: kindClusterName }, 'Kind cluster creation completed');

        // Wait a bit for cluster to be ready
        await new Promise((resolve) => setTimeout(resolve, 5000));
      } else {
        checks.kindClusterCreated = true;
        logger.info({ clusterName: kindClusterName }, 'Kind cluster already exists');
      }

      // Setup kubectl context for kind
      await execAsync(`kind export kubeconfig --name ${kindClusterName}`);
    }

    if (shouldCreateLocalRegistry) {
      // Check/create local registry
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

    // 1. Check connectivity
    checks.connectivity = await checkConnectivity(k8sClient, logger);
    if (!checks.connectivity) {
      return Failure('Cannot connect to Kubernetes cluster');
    }

    // 2. Check permissions
    checks.permissions = await k8sClient.checkPermissions(namespace);
    if (!checks.permissions) {
      warnings.push('Limited permissions - some operations may fail');
    }

    // 3. Check/create namespace
    checks.namespaceExists = await checkNamespace(k8sClient, namespace, logger);
    if (!checks.namespaceExists && shouldCreateNamespace) {
      await createNamespace(k8sClient, namespace, logger);
      checks.namespaceExists = true;
    } else if (!checks.namespaceExists) {
      warnings.push(`Namespace ${namespace} does not exist - deployment may fail`);
    }

    // 4. Setup RBAC if requested
    if (shouldSetupRbac) {
      await setupRbac(k8sClient, namespace, logger);
      checks.rbacConfigured = true;
    }

    // 5. Check for ingress controller
    if (checkRequirements || installIngress) {
      checks.ingressController = await checkIngressController(k8sClient, logger);
      if (!checks.ingressController) {
        warnings.push('No ingress controller found - external access may not work');
      }
    }

    // Determine if cluster is ready
    const clusterReady = checks.connectivity && checks.permissions && checks.namespaceExists;

    // Prepare the result
    const result: PrepareClusterResult = {
      success: true,
      sessionId,
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

    // Update typed session slice with output and state
    const sessionData = session as SessionData;
    await slice.patch(sessionId, {
      output: result,
      state: {
        lastPreparedAt: new Date(),
        lastClusterName: cluster,
        lastNamespace: namespace,
        totalPreparations:
          (sessionData?.completedSteps || []).filter((s: string) => s === 'prepare_cluster')
            .length + 1,
        lastClusterReady: clusterReady,
        lastChecksPassed: Object.values(checks).filter(Boolean).length,
        lastWarningCount: warnings.length,
      },
    });

    timer.end({ clusterReady, sessionId, environment });
    logger.info(
      { sessionId, clusterReady, checks, namespace, environment },
      'Kubernetes cluster preparation completed',
    );

    return Success(result);
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'Cluster preparation failed');

    const errorMessage = error instanceof Error ? error.message : String(error);
    return Failure(errorMessage);
  }
}

/**
 * Export the prepare cluster tool directly
 */
export const prepareCluster = prepareClusterImpl;
