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

import { getToolLogger, createToolTimer } from '@/lib/tool-helpers';
import { extractErrorMessage } from '@/lib/error-utils';
import { randomUUID } from 'node:crypto';
import type { ToolContext } from '@/mcp/context';
import { createKubernetesClient } from '@/lib/kubernetes';
import type { K8sManifest } from '@/infra/kubernetes/client';
import { getSystemInfo, getDownloadOS, getDownloadArch } from '@/lib/platform-utils';
import { downloadFile, makeExecutable, createTempFile, deleteTempFile } from '@/lib/file-utils';

import type * as pino from 'pino';
import { Success, Failure, type Result, TOPICS } from '@/types';
import { prepareClusterSchema, type PrepareClusterParams } from './schema';
import { exec } from 'node:child_process';
import { promisify } from 'node:util';
import { sampleWithRerank } from '@/mcp/ai/sampling-runner';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';

// Use proper async command execution
const execAsync = promisify(exec);

// Configuration constants
const KIND_VERSION = 'v0.20.0'; // Use latest stable version - easily configurable

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
  clusterOptimizations?: ClusterOptimizationInsights;
  workflowHints?: {
    nextStep: string;
    message: string;
  };
}

// Additional interface for AI cluster optimization insights
export interface ClusterOptimizationInsights {
  resourceRecommendations: string[];
  securityEnhancements: string[];
  performanceOptimizations: string[];
  infrastructureAdvice: string[];
  confidence: number;
}

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
      const result = await k8sClient.applyManifest(manifest as unknown as K8sManifest, namespace);
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
 * Install kind if not present - Cross-platform implementation
 */
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

    // Download kind binary
    logger.debug({ kindUrl, kindExecutable }, 'Downloading kind binary');
    await downloadFile(kindUrl, `./${kindExecutable}`);

    // Make executable (Unix-like systems only)
    if (!systemInfo.isWindows) {
      await makeExecutable(`./${kindExecutable}`);
    }

    // Move to appropriate location
    if (systemInfo.isWindows) {
      // On Windows, move to a directory in PATH or create one
      try {
        await execAsync(`move ${kindExecutable} "%ProgramFiles%\\kind\\${kindExecutable}"`);
      } catch {
        // Fallback: try to add to user's local bin
        try {
          await execAsync(
            `mkdir "%USERPROFILE%\\bin" 2>nul & move ${kindExecutable} "%USERPROFILE%\\bin\\${kindExecutable}"`,
          );
        } catch {
          logger.warn('Failed to move kind executable to PATH, it may need manual installation');
        }
      }
    } else {
      // Unix-like systems
      try {
        await execAsync(`sudo mv ./${kindExecutable} /usr/local/bin/${kindExecutable}`);
      } catch {
        // Fallback: try without sudo to user's local bin
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

/**
 * Check if kind cluster exists
 */
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

/**
 * Create kind cluster with local registry - Cross-platform implementation
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

    // Write config to temporary file using cross-platform utilities
    const configPath = await createTempFile(kindConfig, '.yaml');

    try {
      // Create cluster
      await execAsync(`kind create cluster --name ${clusterName} --config "${configPath}"`);
      logger.info({ clusterName }, 'Kind cluster created successfully');
    } finally {
      // Clean up config file
      await deleteTempFile(configPath);
    }
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
    await execAsync('docker run -d --restart=always -p 5001:5000 --name kind-registry registry:2');

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
 * Score cluster optimization insights based on quality and relevance
 */
function scoreClusterOptimizations(
  insights: ClusterOptimizationInsights,
  _preparationResult: Record<string, unknown>,
  _checks: Record<string, unknown>,
): number {
  let score = 0;

  // Quality scoring for resource recommendations (0-25 points)
  if (insights.resourceRecommendations && insights.resourceRecommendations.length > 0) {
    score += Math.min(insights.resourceRecommendations.length * 4, 16);

    // Bonus for specific resource recommendations
    const specificRecommendations = insights.resourceRecommendations.filter((rec) =>
      /memory|cpu|disk|storage|quota|limit|request|pv|pvc|node/i.test(rec),
    ).length;
    score += Math.min(specificRecommendations * 3, 9);
  }

  // Quality scoring for security enhancements (0-25 points)
  if (insights.securityEnhancements && insights.securityEnhancements.length > 0) {
    score += Math.min(insights.securityEnhancements.length * 4, 16);

    // Bonus for security-specific recommendations
    const securitySpecific = insights.securityEnhancements.filter((enh) =>
      /rbac|networkpolicy|psp|admission|tls|encryption|secret|serviceaccount/i.test(enh),
    ).length;
    score += Math.min(securitySpecific * 3, 9);
  }

  // Quality scoring for performance optimizations (0-25 points)
  if (insights.performanceOptimizations && insights.performanceOptimizations.length > 0) {
    score += Math.min(insights.performanceOptimizations.length * 4, 16);

    // Bonus for performance-specific optimizations
    const performanceSpecific = insights.performanceOptimizations.filter((opt) =>
      /performance|scaling|hpa|vpa|cluster-autoscaler|node-pool|affinity/i.test(opt),
    ).length;
    score += Math.min(performanceSpecific * 3, 9);
  }

  // Quality scoring for infrastructure advice (0-25 points)
  if (insights.infrastructureAdvice && insights.infrastructureAdvice.length > 0) {
    score += Math.min(insights.infrastructureAdvice.length * 4, 16);

    // Bonus for infrastructure-specific advice
    const infraSpecific = insights.infrastructureAdvice.filter((advice) =>
      /networking|ingress|dns|loadbalancer|proxy|cni|storage|backup/i.test(advice),
    ).length;
    score += Math.min(infraSpecific * 3, 9);
  }

  // Confidence bonus (0-20 points)
  if (insights.confidence >= 0.8) {
    score += 20;
  } else if (insights.confidence >= 0.6) {
    score += 15;
  } else if (insights.confidence >= 0.4) {
    score += 10;
  } else {
    score += 5;
  }

  return Math.min(score, 100);
}

/**
 * Build prompt for generating cluster optimization insights
 */
function buildClusterOptimizationPrompt(
  preparationResult: Record<string, unknown>,
  checks: Record<string, unknown>,
  environment: string,
): string {
  const context = preparationResult.clusterReady
    ? 'that is successfully prepared but may benefit from optimization'
    : 'that has encountered issues during preparation';

  return `As a Kubernetes cluster optimization expert, analyze this cluster preparation result ${context}.

Cluster Details:
- Environment: ${environment}
- Cluster: ${preparationResult.cluster}
- Namespace: ${preparationResult.namespace}
- Cluster Ready: ${preparationResult.clusterReady}
- Local Registry: ${preparationResult.localRegistryUrl || 'None'}

Checks Performed:
- Connectivity: ${checks.connectivity}
- Permissions: ${checks.permissions}
- Namespace Exists: ${checks.namespaceExists}
${checks.ingressController !== undefined ? `- Ingress Controller: ${checks.ingressController}` : ''}
${checks.rbacConfigured !== undefined ? `- RBAC Configured: ${checks.rbacConfigured}` : ''}
${checks.kindInstalled !== undefined ? `- Kind Installed: ${checks.kindInstalled}` : ''}
${checks.kindClusterCreated !== undefined ? `- Kind Cluster Created: ${checks.kindClusterCreated}` : ''}
${checks.localRegistryCreated !== undefined ? `- Local Registry Created: ${checks.localRegistryCreated}` : ''}

${Array.isArray(preparationResult.warnings) && preparationResult.warnings.length > 0 ? `Warnings:\n${preparationResult.warnings.map((w: string) => `- ${w}`).join('\n')}` : ''}

Provide a JSON response with:
1. resourceRecommendations: Array of specific resource allocation and management recommendations
2. securityEnhancements: Array of security improvements for the cluster setup
3. performanceOptimizations: Array of performance tuning suggestions for better cluster efficiency
4. infrastructureAdvice: Array of infrastructure setup and networking recommendations
5. confidence: Number between 0-1 indicating confidence in the analysis

Focus on:
- Resource optimization and scaling strategies
- Security hardening and RBAC best practices
- Network policies and ingress configuration
- Monitoring and observability setup
- Development vs production environment considerations
- Kind cluster optimizations for local development
- Registry and image management improvements

Respond with valid JSON only.`;
}

/**
 * Generate AI-powered cluster optimization insights
 */
async function generateClusterOptimizations(
  preparationResult: Record<string, unknown>,
  checks: Record<string, unknown>,
  environment: string,
  ctx: ToolContext,
): Promise<Result<ClusterOptimizationInsights>> {
  try {
    const prompt = buildClusterOptimizationPrompt(preparationResult, checks, environment);

    const messages = await buildMessages({
      basePrompt: prompt,
      topic: TOPICS.GENERATE_K8S_MANIFESTS,
      tool: 'prepare-cluster',
      environment: environment || 'development',
    });

    const result = await sampleWithRerank(
      ctx,
      async () => ({
        messages: toMCPMessages(messages).messages,
        maxTokens: 1200,
        modelPreferences: { hints: [{ name: 'cluster-optimization' }] },
      }),
      (response: string) => {
        try {
          const parsed = JSON.parse(response);
          const insights: ClusterOptimizationInsights = {
            resourceRecommendations: Array.isArray(parsed.resourceRecommendations)
              ? parsed.resourceRecommendations
              : [],
            securityEnhancements: Array.isArray(parsed.securityEnhancements)
              ? parsed.securityEnhancements
              : [],
            performanceOptimizations: Array.isArray(parsed.performanceOptimizations)
              ? parsed.performanceOptimizations
              : [],
            infrastructureAdvice: Array.isArray(parsed.infrastructureAdvice)
              ? parsed.infrastructureAdvice
              : [],
            confidence:
              typeof parsed.confidence === 'number' &&
              parsed.confidence >= 0 &&
              parsed.confidence <= 1
                ? parsed.confidence
                : 0.5,
          };

          return scoreClusterOptimizations(insights, preparationResult, checks);
        } catch {
          return { overall: 0 };
        }
      },
      {},
    );

    if (result.ok) {
      const parsed = JSON.parse(result.value.text);
      const insights: ClusterOptimizationInsights = {
        resourceRecommendations: Array.isArray(parsed.resourceRecommendations)
          ? parsed.resourceRecommendations
          : [],
        securityEnhancements: Array.isArray(parsed.securityEnhancements)
          ? parsed.securityEnhancements
          : [],
        performanceOptimizations: Array.isArray(parsed.performanceOptimizations)
          ? parsed.performanceOptimizations
          : [],
        infrastructureAdvice: Array.isArray(parsed.infrastructureAdvice)
          ? parsed.infrastructureAdvice
          : [],
        confidence:
          typeof parsed.confidence === 'number' && parsed.confidence >= 0 && parsed.confidence <= 1
            ? parsed.confidence
            : 0.5,
      };
      return Success(insights);
    } else {
      return Failure('Failed to generate cluster optimization insights');
    }
  } catch (error) {
    return Failure(`Error generating cluster optimization insights: ${error}`);
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

  const { environment = 'development', namespace = 'default' } = params;

  const cluster = environment === 'development' ? 'kind' : 'default';
  const shouldCreateNamespace = environment === 'production';
  const shouldSetupRbac = environment === 'production';
  const installIngress = false;
  const checkRequirements = true;
  const shouldSetupKind = environment === 'development';
  const shouldCreateLocalRegistry = environment === 'development';

  try {
    // Ensure session exists and get typed slice operations
    const sessionId = params.sessionId || randomUUID();
    let sessionState = null;

    if (context.sessionManager) {
      const getResult = await context.sessionManager.get(sessionId);
      if (getResult.ok) {
        sessionState = getResult.value;
      }

      // Create if doesn't exist
      if (!sessionState) {
        const createResult = await context.sessionManager.create(sessionId);
        if (!createResult.ok) {
          return Failure(`Failed to create session: ${createResult.error}`);
        }
        sessionState = createResult.value;
      }
    } else {
      return Failure('Session manager not available in context');
    }
    logger.info(
      { sessionId, environment, namespace },
      'Starting Kubernetes cluster preparation with session',
    );

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
      try {
        await execAsync(`kind export kubeconfig --name ${kindClusterName}`);
      } catch (error) {
        logger.warn({ error: String(error) }, 'Failed to export kubeconfig, continuing anyway');
      }
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

    // Store cluster preparation result in session
    const currentSteps = sessionState?.completed_steps || [];
    if (context.sessionManager) {
      await context.sessionManager.update(sessionId, {
        results: {
          'prepare-cluster': result,
        },
        completed_steps: [...currentSteps, 'prepare-cluster'],
      });
    }

    // Generate AI-powered cluster optimization insights
    let clusterOptimizations: ClusterOptimizationInsights | undefined;
    try {
      const optimizationResult = await generateClusterOptimizations(
        result as unknown as Record<string, unknown>,
        checks,
        environment,
        context,
      );

      if (optimizationResult.ok) {
        clusterOptimizations = optimizationResult.value;
        logger.info(
          {
            resourceRecommendations: clusterOptimizations.resourceRecommendations.length,
            confidence: clusterOptimizations.confidence,
          },
          'Generated AI cluster optimization insights',
        );
      } else {
        logger.warn(
          { error: optimizationResult.error },
          'Failed to generate cluster optimization insights',
        );
      }
    } catch (error) {
      logger.warn(
        { error: extractErrorMessage(error) },
        'Error generating cluster optimization insights',
      );
    }

    // Add optimization insights and workflow hints to result
    const enrichedResult: PrepareClusterResult = {
      ...result,
      ...(clusterOptimizations && { clusterOptimizations }),
      workflowHints: {
        nextStep: clusterReady ? 'generate-k8s-manifests' : 'fix-cluster-issues',
        message: clusterReady
          ? `Cluster preparation successful. Use "generate-k8s-manifests" with sessionId ${sessionId} to create deployment manifests.${clusterOptimizations ? ' Review AI optimization insights to enhance cluster performance and security.' : ''}`
          : `Cluster preparation found issues. ${clusterOptimizations ? 'Review AI recommendations to resolve cluster setup problems.' : 'Check connectivity, permissions, and namespace configuration.'}`,
      },
    };

    timer.end({ clusterReady, sessionId, environment });

    return Success(enrichedResult);
  } catch (error) {
    timer.error(error);

    const errorMessage = error instanceof Error ? error.message : String(error);
    return Failure(errorMessage);
  }
}

/**
 * Export the prepare cluster tool directly
 */
export const prepareCluster = prepareClusterImpl;

// New Tool interface export
import type { Tool } from '@/types/tool';

const tool: Tool<typeof prepareClusterSchema, PrepareClusterResult> = {
  name: 'prepare-cluster',
  description: 'Prepare Kubernetes cluster for deployment with AI-powered optimization insights',
  version: '2.0.0',
  schema: prepareClusterSchema,
  metadata: {
    aiDriven: true,
    knowledgeEnhanced: false,
    samplingStrategy: 'single',
    enhancementCapabilities: [
      'cluster-optimization',
      'resource-recommendations',
      'security-enhancements',
      'performance-tuning',
    ],
  },
  run: prepareClusterImpl,
};

export default tool;
