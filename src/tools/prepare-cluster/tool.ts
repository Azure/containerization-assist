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
import { createKubernetesClient, type K8sManifest } from '@/infra/kubernetes/client';
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
    // Wrap in `cmd /c` so `%USERPROFILE%` is expanded by cmd.exe regardless of the
    // parent shell. PowerShell (the common Windows shell) does not expand `%VAR%`
    // syntax, so an unwrapped command would move the binary into a literal
    // "%USERPROFILE%" folder. Create the target dir first (move fails if it is
    // missing) and use `move /Y` so re-runs overwrite without an interactive
    // prompt. Inner quotes keep the destination path space-safe under cmd.exe.
    return {
      command: `cmd /c "curl.exe -Lo kind.exe ${url} && if not exist "%USERPROFILE%\\bin" mkdir "%USERPROFILE%\\bin" && move /Y kind.exe "%USERPROFILE%\\bin\\kind.exe""`,
      goal: 'Install the kind binary (Windows) and place it on your PATH',
    };
  }

  const url = `https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-${downloadOS}-${downloadArch}`;
  // Avoid a hard `sudo` dependency: in advisory mode the plan may be run by a
  // non-root user (or non-interactively, where a sudo password prompt would hang).
  // Prefer the on-PATH system location when it is writable, otherwise fall back to
  // a user-writable bin dir so the install still succeeds without elevation.
  return {
    command: `curl -Lo ./kind ${url} && chmod +x ./kind && (mv ./kind /usr/local/bin/kind 2>/dev/null || { mkdir -p "$HOME/.local/bin" && mv ./kind "$HOME/.local/bin/kind"; })`,
    goal: 'Install the kind binary to a PATH location (no sudo required; falls back to ~/.local/bin)',
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
 * Build the `kind create cluster` command that streams the cluster config via stdin.
 * The config is piped using a heredoc on POSIX shells and a PowerShell here-string on
 * Windows (Bash heredocs do not work in PowerShell/cmd). The cluster name is the
 * already-escaped, single-quote-wrapped form, which is literal in both shells.
 */
function buildKindCreateClusterCommand(
  shellSafeClusterName: string,
  clusterName: string,
  kindConfig: string,
): ClusterSetupCommand {
  const { isWindows } = getSystemInfo();
  // kindConfig always ends with a newline, so the heredoc/here-string terminator
  // lands on its own line in both forms.
  const command = isWindows
    ? `@'\n${kindConfig}'@ | kind create cluster --name ${shellSafeClusterName} --config=-`
    : `cat <<'EOF' | kind create cluster --name ${shellSafeClusterName} --config=-\n${kindConfig}EOF`;
  return {
    command,
    goal: `Create the kind cluster '${clusterName}' with local-registry support`,
  };
}

/**
 * Read-only status of the local registry container.
 */
interface LocalRegistryStatus {
  /** Container is currently running. */
  running: boolean;
  /** Container exists (running or stopped). */
  exists: boolean;
  /** Host port mapped to the registry's internal port, if discoverable. */
  port: number | null;
}

/**
 * Read-only inspection of the local registry container.
 * Distinguishes three states without mutating anything (advisory mode):
 *  - running: container is up; port read from its runtime network settings.
 *  - stopped: container exists but is not running; port reused from its host config.
 *  - absent:  no container with the registry name exists.
 * Never starts containers or mutates networks.
 */
async function checkLocalRegistryStatus(logger: pino.Logger): Promise<LocalRegistryStatus> {
  const absent: LocalRegistryStatus = { running: false, exists: false, port: null };
  const containerName = DOCKER.REGISTRY_CONTAINER_NAME;

  try {
    // Is the container currently running?
    const { stdout: runningContainers } = await execAsync(
      `docker ps --filter "name=${containerName}" --format "{{.Names}}"`,
    );
    const running = runningContainers
      .trim()
      .split('\n')
      .some((name) => name.trim() === containerName);

    // If not running, does it still exist in a stopped state?
    let exists = running;
    if (!running) {
      const { stdout: allContainers } = await execAsync(
        `docker ps -a --filter "name=${containerName}" --format "{{.Names}}"`,
      );
      exists = allContainers
        .trim()
        .split('\n')
        .some((name) => name.trim() === containerName);

      if (!exists) {
        logger.debug('Local registry container does not exist');
        return absent;
      }
      logger.debug('Local registry container exists but is stopped');
    }

    // Resolve the mapped host port. Prefer the runtime network settings for a
    // running container (authoritative for the port actually published), and fall
    // back to the configured host bindings (which persist for stopped containers).
    // Both sources are tried because either can be empty depending on how the
    // container was created and how the Docker version reports it.
    const portKey = `${DOCKER.REGISTRY_INTERNAL_PORT}/tcp`;
    const portSources = running
      ? ['.NetworkSettings.Ports', '.HostConfig.PortBindings']
      : ['.HostConfig.PortBindings', '.NetworkSettings.Ports'];

    let port: number | null = null;
    for (const source of portSources) {
      const { stdout: portMapping } = await execAsync(
        `docker inspect ${containerName} --format '{{range $p, $conf := ${source}}}{{if eq $p "${portKey}"}}{{(index $conf 0).HostPort}}{{end}}{{end}}'`,
      );
      const parsedPort = parseInt(portMapping.trim(), 10);
      if (!isNaN(parsedPort)) {
        port = parsedPort;
        break;
      }
    }

    if (port === null) {
      logger.warn(
        { containerName },
        'Could not determine the existing registry host port from Docker; the generated plan may use an incorrect port',
      );
    }

    logger.debug({ running, exists, port }, 'Local registry status');
    return { running, exists, port };
  } catch (error) {
    logger.debug({ error }, 'Error checking local registry');
    return absent;
  }
}

/**
 * Build the `docker run` command that creates and starts the local registry container.
 */
function buildRegistryRunCommand(port: number): ClusterSetupCommand {
  return {
    command: `docker run -d --restart=always -p ${port}:${DOCKER.REGISTRY_INTERNAL_PORT} --name ${DOCKER.REGISTRY_CONTAINER_NAME} registry:2`,
    goal: `Start a local Docker registry on port ${port}`,
  };
}

/**
 * Build the command that starts an existing (stopped) local registry container,
 * reusing its original port mapping.
 */
function buildRegistryStartCommand(port: number): ClusterSetupCommand {
  return {
    command: `docker start ${DOCKER.REGISTRY_CONTAINER_NAME}`,
    goal: `Start the existing local Docker registry container (reuses port ${port})`,
  };
}

/**
 * Build the command that connects the local registry to the kind Docker network.
 * Required for in-cluster image pulls: the kind config points containerd mirrors at
 * `http://${REGISTRY_CONTAINER_NAME}:5000`, which only resolves once the registry is
 * attached to the kind network. Emitted only for a newly created registry (an existing
 * registry is already attached — Docker network membership persists across stop/start).
 */
function buildRegistryNetworkConnectCommand(): ClusterSetupCommand {
  return {
    command: `docker network connect kind ${DOCKER.REGISTRY_CONTAINER_NAME}`,
    goal: 'Connect the local registry to the kind network so the cluster can pull from it',
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

  // Validate + escape the cluster name up front so any emitted commands are injection-safe.
  // validateAndEscapeClusterName returns the name wrapped in single quotes for shell safety;
  // emitted kind commands must interpolate this escaped form, never the raw name.
  let shellSafeClusterName = clusterName;
  if (isKind) {
    const nameResult = validateAndEscapeClusterName(clusterName);
    if (!nameResult.ok) {
      return nameResult;
    }
    shellSafeClusterName = nameResult.value;
  }

  try {
    logger.info(
      { environment, namespace, clusterType: effectiveClusterType },
      'Analyzing cluster state (read-only) to build preparation plan',
    );

    const warnings: string[] = [];

    // ---- Probe read-only state (no mutations) ----
    // The cluster may not exist yet (e.g. a kind cluster we are about to create), in
    // which case there is no reachable kubeconfig. For kind that is expected — the
    // plan itself creates the cluster, so we continue. For a generic/already-existing
    // cluster, an unreachable kubeconfig means the emitted manifests would fail on
    // apply, so we record it here and surface it in the generic branch below.
    let namespaceExists = false;
    let clusterReachable = true;
    try {
      const k8sClient = createKubernetesClient(logger);
      namespaceExists = await k8sClient.namespaceExists(namespace);
      logger.debug({ namespace, namespaceExists }, 'Checked namespace existence');
    } catch (error) {
      clusterReachable = false;
      logger.debug(
        { error: error instanceof Error ? error.message : String(error) },
        'Cluster/kubeconfig not reachable while probing namespace',
      );
    }
    const clusterPlatform = await detectClusterPlatform(logger);

    let kindInstalled = false;
    let clusterExists = false;
    let registryStatus: LocalRegistryStatus = { running: false, exists: false, port: null };

    if (isKind) {
      kindInstalled = await checkKindInstalled(logger);
      if (kindInstalled) {
        const existsResult = await checkKindClusterExists(clusterName, logger);
        if (existsResult.ok) {
          clusterExists = existsResult.value;
        }
      }
      registryStatus = await checkLocalRegistryStatus(logger);
    }

    // ---- Compute recommendations from the unsatisfied state ----
    const setupCommands: ClusterSetupCommand[] = [];
    const manifests: ClusterManifestPlan[] = [];

    if (isKind) {
      // Reuse the existing registry port (running or stopped); otherwise pick a free port.
      const registryPort = registryStatus.port ?? (await findRegistryPort());

      // If a registry already exists but its port could not be read, we are about to
      // emit config (kind mirror + ConfigMap) for a freshly picked port that will not
      // match the running registry. Surface that so the caller can verify/correct it.
      if (registryStatus.exists && registryStatus.port === null) {
        warnings.push(
          `An existing local registry container ("${DOCKER.REGISTRY_CONTAINER_NAME}") was detected but its published host port could not be determined. The plan uses port ${registryPort}, which may not match the existing registry — verify the registry's actual port before applying the kind config and ConfigMap.`,
        );
      }

      // 1. Install kind if it is missing.
      if (!kindInstalled) {
        setupCommands.push(buildKindInstallCommand());
      }

      // 2. Ensure the local registry is running.
      const registryNewlyCreated = !registryStatus.exists;
      if (!registryStatus.exists) {
        // No registry container yet — create one.
        setupCommands.push(buildRegistryRunCommand(registryPort));
      } else if (!registryStatus.running) {
        // Container exists but is stopped — start it (reusing its existing port)
        // rather than `docker run`, which would fail with a name conflict.
        setupCommands.push(buildRegistryStartCommand(registryPort));
      }

      // 3. Create the kind cluster (with local-registry mirror config) if it does not exist.
      if (!clusterExists) {
        const kindConfig = buildKindClusterConfig(registryPort, strictPlatformValidation);
        setupCommands.push(
          buildKindCreateClusterCommand(shellSafeClusterName, clusterName, kindConfig),
        );
      }

      // 4. Connect the registry to the kind network. Required for in-cluster pulls,
      //    but only when the registry container is newly created: an existing registry
      //    is already attached (network membership persists across stop/start), and
      //    re-running `docker network connect` on an attached container errors.
      if (registryNewlyCreated) {
        setupCommands.push(buildRegistryNetworkConnectCommand());
      }

      // 5. Point kubectl at the cluster.
      setupCommands.push({
        command: `kind export kubeconfig --name ${shellSafeClusterName}`,
        goal: 'Point kubectl at the kind cluster',
      });

      // Registry-hosting ConfigMap documents the registry for tools/users (kind best practice).
      manifests.push(buildLocalRegistryConfigMapManifest(registryPort));
    } else {
      // Generic cluster (AKS/EKS/GKE/minikube/...): the cluster is expected to already
      // exist. If we could not reach it, every manifest below will fail on apply —
      // surface a warning and a required connectivity check so the agent fixes its
      // kubeconfig/context first instead of acting on a plan that looks ready but is not.
      if (!clusterReachable) {
        warnings.push(
          `The target ${effectiveClusterType} cluster is not reachable (missing or invalid kubeconfig/context). The setup commands and manifests below will fail until kubectl is pointed at the cluster — configure your kubeconfig (e.g. az aks get-credentials, aws eks update-kubeconfig, or gcloud container clusters get-credentials) and re-run this tool.`,
        );
        setupCommands.push({
          command: 'kubectl cluster-info',
          goal: 'Verify kubectl can reach the target cluster before applying manifests (configure your kubeconfig/context first if this fails)',
        });
      }

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
    // High confidence when the cluster platform was detected; lower when it could not
    // be (e.g. the cluster does not exist yet, including kind bootstrap) because the
    // platform-compatibility guidance is then only partial; lowest for a generic
    // (already-existing) cluster we could not reach at all, since the plan is then
    // built without any real cluster state.
    const confidence = !isKind && !clusterReachable ? 0.4 : clusterPlatform === null ? 0.7 : 0.9;

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
        registryRunning: registryStatus.running,
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
