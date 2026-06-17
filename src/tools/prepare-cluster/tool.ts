/**
 * Prepare Cluster Tool - Advisory Implementation
 *
 * Probes Kubernetes/host state read-only and returns a ClusterPlan describing the
 * exact commands and manifests the calling agent should run to prepare the cluster.
 * The tool never executes commands or mutates state itself.
 *
 * @example
 * ```typescript
 * const result = await prepareCluster({
 *   clusterType: 'kind',
 *   namespace: 'my-app',
 * }, context);
 *
 * if (result.ok) {
 *   const plan = result.value;
 *   logger.info('Cluster plan', {
 *     clusterType: plan.clusterType,
 *     commands: plan.recommendations.setupCommands.length,
 *     manifests: plan.recommendations.manifests.length,
 *   });
 * }
 * ```
 */

import { setupToolContext } from '@/lib/tool-context-helpers';
import { validateNamespace } from '@/lib/validation';
import type { ToolContext } from '@/core/context';
import { DOCKER, KUBERNETES } from '@/config/constants';
import { prepareClusterToolDefinition } from './types';
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
import { findRegistryPort } from '@/lib/port-utils';
import type { DockerPlatform, ToolNextAction } from '@/tools/shared/schemas';

import type * as pino from 'pino';
import yaml from 'js-yaml';
import { Success, Failure, type Result } from '@/types';
import {
  type PrepareClusterParams,
  type ClusterPlan,
  type ClusterSetupCommand,
  type ClusterManifestPlan,
  type ClusterPlatformGuidance,
} from './schema';
import { exec } from 'node:child_process';
import { promisify } from 'node:util';
import { pluralize } from '@/lib/summary-helpers';

const execAsync = promisify(exec);

const KIND_VERSION = 'v0.20.0';
const KIND_AMD64_NODE_IMAGE =
  'kindest/node:v1.27.3@sha256:3966ac761ae0136263ffdb6cfd4db23ef8a83cba8a463690e98317add2c9ba72';

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
      "kubectl get nodes -o jsonpath='{.items[0].status.nodeInfo.architecture}'",
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
        "kubectl get nodes -o jsonpath='{.items[0].status.nodeInfo.operatingSystem}'",
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
 * Build read-only platform compatibility guidance between the target platform and
 * the detected cluster platform. Never fails — surfaces guidance and pushes warnings.
 */
function buildPlatformGuidance(
  targetPlatform: DockerPlatform,
  clusterPlatform: DockerPlatform | null,
  warnings: string[],
): ClusterPlatformGuidance {
  if (!clusterPlatform) {
    return {
      target: targetPlatform,
      cluster: null,
      compatible: false,
      requiresEmulation: false,
      note: `Cluster platform not detected (the cluster may not exist yet). Images are built for ${targetPlatform}; re-check compatibility after the cluster is created.`,
    };
  }

  const compatible = isPlatformCompatible(targetPlatform, clusterPlatform);
  if (compatible) {
    return {
      target: targetPlatform,
      cluster: clusterPlatform,
      compatible: true,
      requiresEmulation: false,
      note: `Target platform ${targetPlatform} is compatible with cluster platform ${clusterPlatform}.`,
    };
  }

  const requiresEmulation = targetPlatform !== clusterPlatform;
  const note = `Platform mismatch: cluster is ${clusterPlatform} but target is ${targetPlatform}. Images may require emulation (performance impact) or recreate the cluster with a matching architecture.`;
  warnings.push(note);
  return {
    target: targetPlatform,
    cluster: clusterPlatform,
    compatible: false,
    requiresEmulation,
    note,
  };
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
 * Check if kind cluster exists. Returns the cluster existence status.
 */
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

/**
 * Build the kind install command appropriate for the host OS (read-only host detection).
 */
function buildKindInstallCommand(): { command: string; goal: string } {
  const systemInfo = getSystemInfo();
  const downloadOS = getDownloadOS();
  const downloadArch = getDownloadArch();

  if (systemInfo.isWindows) {
    const url = `https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-windows-${downloadArch}.exe`;
    return {
      command: `curl.exe -Lo kind.exe ${url} && move kind.exe "%USERPROFILE%\\bin\\kind.exe"`,
      goal: 'Install the kind binary (Windows) and place it on your PATH',
    };
  }

  const url = `https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-${downloadOS}-${downloadArch}`;
  return {
    command: `curl -Lo ./kind ${url} && chmod +x ./kind && sudo mv ./kind /usr/local/bin/kind`,
    goal: 'Install the kind binary and place it on your PATH',
  };
}

/**
 * Build the kind cluster config YAML (pure). Emitted in the plan so the agent can apply it
 * via `kind create cluster --config -`. On ARM Mac (non-strict) an explicit AMD64 node image
 * is pinned for cross-platform compatibility.
 */
function buildKindClusterConfig(port: number, strictMode: boolean): string {
  const systemInfo = getSystemInfo();
  const hostArch = process.arch;
  const shouldUseAMD64Node = !strictMode && systemInfo.isMac && hostArch === 'arm64';
  const nodeImageLine = shouldUseAMD64Node ? `  image: ${KIND_AMD64_NODE_IMAGE}` : '';

  return `kind: Cluster
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
}

/**
 * Read-only check for whether the local registry container is running.
 * Returns the mapped host port if the container is running, otherwise null.
 * Does NOT start stopped containers or mutate networks (advisory mode).
 */
async function checkLocalRegistryExists(logger: pino.Logger): Promise<number | null> {
  try {
    // Check if the container is currently running
    const { stdout: runningContainers } = await execAsync(
      `docker ps --filter "name=${DOCKER.REGISTRY_CONTAINER_NAME}" --format "{{.Names}}"`,
    );
    const isRunning = runningContainers.trim() === DOCKER.REGISTRY_CONTAINER_NAME;

    if (!isRunning) {
      logger.debug('Local registry container is not running');
      return null;
    }

    // Get the port mapping for the running container
    const { stdout: portMapping } = await execAsync(
      `docker inspect ${DOCKER.REGISTRY_CONTAINER_NAME} --format '{{range $p, $conf := .NetworkSettings.Ports}}{{if eq $p "5000/tcp"}}{{(index $conf 0).HostPort}}{{end}}{{end}}'`,
    );
    const port = parseInt(portMapping.trim(), 10);

    if (isNaN(port)) {
      logger.warn('Could not determine registry port mapping');
      return null;
    }

    logger.debug({ port }, 'Local registry is running');
    return port;
  } catch (error) {
    logger.debug({ error }, 'Error checking local registry');
    return null;
  }
}

/**
 * Build the `docker run` command that starts the local registry container.
 */
function buildRegistryRunCommand(port: number): ClusterSetupCommand {
  return {
    command: `docker run -d --restart=always -p ${port}:${DOCKER.REGISTRY_INTERNAL_PORT} --name ${DOCKER.REGISTRY_CONTAINER_NAME} registry:2`,
    goal: `Start a local Docker registry on port ${port}`,
  };
}

/**
 * Build the command that connects the local registry to the kind Docker network.
 * Marked optional because the registry may already be connected.
 */
function buildRegistryNetworkConnectCommand(): ClusterSetupCommand {
  return {
    command: `docker network connect kind ${DOCKER.REGISTRY_CONTAINER_NAME}`,
    goal: 'Connect the local registry to the kind network so the cluster can pull from it',
    optional: true,
  };
}

/**
 * Build the local-registry-hosting ConfigMap manifest (kind best practice).
 */
function buildLocalRegistryConfigMapManifest(port: number): ClusterManifestPlan {
  const manifest: K8sManifest = {
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
  return {
    kind: 'ConfigMap',
    namespace: 'kube-public',
    yaml: yaml.dump(manifest),
  };
}

/**
 * Build the application ServiceAccount manifest for the target namespace.
 */
function buildServiceAccountManifest(namespace: string): ClusterManifestPlan {
  const manifest: K8sManifest = {
    apiVersion: 'v1',
    kind: 'ServiceAccount',
    metadata: {
      name: 'app-service-account',
      namespace,
    },
  };
  return {
    kind: 'ServiceAccount',
    namespace,
    yaml: yaml.dump(manifest),
  };
}

/**
 * Core cluster preparation implementation
 */
async function handlePrepareCluster(
  params: PrepareClusterParams,
  context: ToolContext,
): Promise<Result<ClusterPlan>> {
  const { logger, timer } = setupToolContext(context, 'prepare-cluster');

  const {
    clusterType: explicitClusterType,
    environment = 'development',
    namespace = 'default',
    targetPlatform = 'linux/amd64',
    strictPlatformValidation = true,
  } = params;

  // Resolve effective cluster type: explicit clusterType wins, otherwise infer from environment for backwards compat
  const effectiveClusterType =
    explicitClusterType ?? (environment === 'development' ? 'kind' : 'generic');
  const isKind = effectiveClusterType === 'kind';

  // Validate namespace
  const namespaceValidation = validateNamespace(namespace);
  if (!namespaceValidation.ok) {
    return namespaceValidation;
  }

  const clusterName = isKind ? 'containerization-assist' : 'default';

  // Validate the cluster name up front so any emitted commands are injection-safe.
  if (isKind) {
    const nameResult = validateAndEscapeClusterName(clusterName);
    if (!nameResult.ok) {
      return nameResult;
    }
  }

  try {
    logger.info(
      { environment, namespace, clusterType: effectiveClusterType },
      'Analyzing cluster state (read-only) to build preparation plan',
    );

    const warnings: string[] = [];

    // ---- Probe read-only state (no mutations) ----
    // createKubernetesClient throws when no kubeconfig is found (e.g. fresh CI runner with
    // no cluster yet).  Since this tool is purely advisory, we treat a missing kubeconfig
    // as "no cluster available" and default all K8s-probe results to false / null.
    let namespaceExists = false;
    try {
      const k8sClient = createKubernetesClient(logger);
      namespaceExists = await checkNamespace(k8sClient, namespace, logger);
    } catch (err) {
      logger.debug({ err }, 'No kubeconfig available - skipping namespace probe');
    }
    const clusterPlatform = await detectClusterPlatform(logger);

    let kindInstalled = false;
    let clusterExists = false;
    let registryRunning: number | null = null;

    if (isKind) {
      kindInstalled = await checkKindInstalled(logger);
      if (kindInstalled) {
        const existsResult = await checkKindClusterExists(clusterName, logger);
        if (existsResult.ok) {
          clusterExists = existsResult.value;
        }
      }
      registryRunning = await checkLocalRegistryExists(logger);
    }

    // ---- Compute recommendations from the unsatisfied state ----
    const setupCommands: ClusterSetupCommand[] = [];
    const manifests: ClusterManifestPlan[] = [];

    if (isKind) {
      // Use the existing registry port if one is running, otherwise pick a free port.
      const registryPort = registryRunning ?? (await findRegistryPort());

      // 1. Install kind if it is missing.
      if (!kindInstalled) {
        setupCommands.push(buildKindInstallCommand());
      }

      // 2. Start the local registry if it is not already running.
      if (registryRunning === null) {
        setupCommands.push(buildRegistryRunCommand(registryPort));
      }

      // 3. Create the kind cluster (with local-registry mirror config) if it does not exist.
      if (!clusterExists) {
        const kindConfig = buildKindClusterConfig(registryPort, strictPlatformValidation);
        setupCommands.push({
          command: `cat <<'EOF' | kind create cluster --name ${clusterName} --config=-\n${kindConfig}EOF`,
          goal: `Create the kind cluster '${clusterName}' with local-registry support`,
        });
      }

      // 4. Connect the registry to the kind network (optional — may already be connected).
      setupCommands.push(buildRegistryNetworkConnectCommand());

      // 5. Point kubectl at the cluster.
      setupCommands.push({
        command: `kind export kubeconfig --name ${clusterName}`,
        goal: 'Point kubectl at the kind cluster',
      });

      // Registry-hosting ConfigMap documents the registry for tools/users (kind best practice).
      manifests.push(buildLocalRegistryConfigMapManifest(registryPort));
    } else {
      // Generic cluster (AKS/EKS/GKE/minikube/...): the cluster already exists.
      if (!namespaceExists) {
        setupCommands.push({
          command: `kubectl create namespace ${namespace}`,
          goal: `Create the '${namespace}' namespace`,
          optional: true,
        });
      }
      // Application ServiceAccount for the workload namespace.
      manifests.push(buildServiceAccountManifest(namespace));
    }

    // ---- Platform compatibility guidance (read-only, never fails) ----
    const platformGuidance = buildPlatformGuidance(targetPlatform, clusterPlatform, warnings);

    // ---- Confidence + summary ----
    // High confidence when state was probed cleanly; lower when the cluster platform
    // could not be detected (e.g., no cluster exists yet) since guidance is partial.
    const confidence = clusterPlatform === null && !isKind ? 0.7 : 0.9;

    const requiredCommands = setupCommands.filter((c) => !c.optional).length;
    const summary =
      requiredCommands === 0
        ? `✅ Cluster '${clusterName}' (${effectiveClusterType}) appears ready. Apply ${pluralize(manifests.length, 'manifest')} and verify before deploying.`
        : `🔧 ACTION REQUIRED: Prepare ${effectiveClusterType} cluster '${clusterName}'. Run ${pluralize(requiredCommands, 'setup command')} and apply ${pluralize(manifests.length, 'manifest')}, then verify before deploying.`;

    const nextAction: ToolNextAction = {
      action: 'review-and-decide',
      instruction:
        'Review the detected cluster state, then run the setup commands (in order, skipping optional ones already satisfied) and apply the manifests via your terminal/kubectl tools. Re-check platform compatibility after the cluster exists, then call verify-deploy.',
      files: [],
    };

    const plan: ClusterPlan = {
      nextAction,
      clusterType: effectiveClusterType,
      detected: {
        kindInstalled,
        clusterExists,
        namespaceExists,
        registryRunning: registryRunning !== null,
        clusterPlatform,
      },
      recommendations: {
        setupCommands,
        manifests,
        platformGuidance,
      },
      ...(warnings.length > 0 && { warnings }),
      confidence,
      summary,
    };

    logger.info(
      {
        clusterType: effectiveClusterType,
        requiredCommands,
        manifests: manifests.length,
        detected: plan.detected,
      },
      'Cluster preparation plan generated',
    );

    timer.end({ clusterType: effectiveClusterType, environment });

    return Success(plan);
  } catch (error) {
    timer.error(error);

    const errorMessage = error instanceof Error ? error.message : String(error);
    return Failure(errorMessage, {
      message: errorMessage,
      hint: 'An unexpected error occurred while analyzing the cluster state',
      resolution:
        'Check the error message for details. Common issues include Docker not running (for kind clusters) or kubectl not configured',
    });
  }
}

export const prepareCluster = handlePrepareCluster;

import { tool } from '@/types/tool';

export default tool({
  ...prepareClusterToolDefinition,
  handler: handlePrepareCluster,
});
