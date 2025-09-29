/**
 * Deploy Application Tool - Standardized Implementation
 *
 * Deploys applications to Kubernetes clusters using standardized helpers
 * for consistency and improved error handling
 * @example
 * ```typescript
 * const result = await deployApplication({
 *   sessionId: 'session-123',
 *   namespace: 'my-app',
 *   environment: 'production'
 * }, context, logger);
 * if (result.success) {
 *   logger.info('Application deployed', {
 *     deployment: result.deploymentName,
 *     endpoints: result.endpoints
 *   });
 * }
 * ```
 */

import * as yaml from 'js-yaml';
import { getToolLogger, createToolTimer } from '@/lib/tool-helpers';
import type { Logger } from '@/lib/logger';
import { extractErrorMessage } from '@/lib/error-utils';
import type { ToolContext } from '@/mcp/context';
import { createKubernetesClient } from '@/lib/kubernetes';
import type { K8sManifest } from '@/infra/kubernetes/client';

import { Success, Failure, type Result, TOPICS } from '@/types';
import { DEFAULT_TIMEOUTS } from '@/config/defaults';
import { deployApplicationSchema, type DeployApplicationParams } from './schema';
import { sampleWithRerank } from '@/mcp/ai/sampling-runner';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages, type MCPMessage } from '@/mcp/ai/message-converter';

// Type definitions for Kubernetes manifests
interface KubernetesManifest {
  kind?: string;
  metadata?: {
    name?: string;
    namespace?: string;
  };
}

interface DeploymentManifest extends KubernetesManifest {
  kind: 'Deployment';
  spec?: {
    replicas?: number;
  };
}

interface ServiceManifest extends KubernetesManifest {
  kind: 'Service';
  spec?: {
    ports?: Array<{ port?: number; targetPort?: number; nodePort?: number }>;
    type?: 'ClusterIP' | 'NodePort' | 'LoadBalancer';
  };
}

interface IngressManifest extends KubernetesManifest {
  kind: 'Ingress';
  spec?: {
    rules?: Array<{ host?: string; http?: unknown }>;
  };
}

// Configuration constants
const DEPLOYMENT_CONFIG = {
  DEFAULT_NAMESPACE: 'default',
  DEFAULT_REPLICAS: 1,
  DEFAULT_ENVIRONMENT: 'development',
  DEFAULT_CLUSTER: 'default',
  DEFAULT_PORT: 80,
  WAIT_TIMEOUT_SECONDS: 300,
  DRY_RUN: false,
  WAIT_FOR_READY: true,
  PENDING_LB_URL: 'http://pending-loadbalancer',
  DEFAULT_INGRESS_HOST: 'app.example.com',
} as const;

// Manifest deployment order for proper resource creation
const MANIFEST_ORDER = [
  'Namespace',
  'ConfigMap',
  'Secret',
  'PersistentVolume',
  'PersistentVolumeClaim',
  'ServiceAccount',
  'Role',
  'RoleBinding',
  'ClusterRole',
  'ClusterRoleBinding',
  'Service',
  'Deployment',
  'StatefulSet',
  'DaemonSet',
  'Job',
  'CronJob',
  'Ingress',
  'HorizontalPodAutoscaler',
  'VerticalPodAutoscaler',
  'NetworkPolicy',
] as const;

// Additional interface for AI deployment analysis
export interface DeploymentAnalysis {
  recommendations: string[];
  optimizations: string[];
  troubleshooting: string[];
  confidence: number;
}

export interface DeployApplicationResult {
  success: boolean;
  sessionId: string;
  namespace: string;
  deploymentName: string;
  serviceName: string;
  endpoints: Array<{
    type: 'internal' | 'external';
    url: string;
    port: number;
  }>;
  ready: boolean;
  replicas: number;
  deploymentAnalysis?: DeploymentAnalysis;
  status?: {
    readyReplicas: number;
    totalReplicas: number;
    conditions: Array<{
      type: string;
      status: string;
      message: string;
    }>;
  };
  chainHint?: string; // Hint for next tool in workflow chain
  workflowHints?: {
    nextStep: string;
    message: string;
  };
}

// Define the result schema for type safety

/**
 * Score deployment analysis based on content quality and relevance
 */
function scoreDeploymentAnalysis(text: string): number {
  let score = 0;

  // Basic content quality (30 points)
  if (text.length > 150) score += 10;
  if (text.includes('\n')) score += 10;
  if (!text.toLowerCase().includes('error')) score += 10;

  // Deployment analysis indicators (40 points)
  if (/deployment|pod|service|ingress/i.test(text)) score += 15;
  if (/troubleshoot|debug|monitor|health/i.test(text)) score += 15;
  if (/optimize|scale|resource|performance/i.test(text)) score += 10;

  // Structure and actionability (30 points)
  if (/\d+\.|-|\*/.test(text)) score += 10; // Has list structure
  if (/check|verify|monitor|improve/i.test(text)) score += 10;
  if (text.split('\n').length >= 4) score += 10; // Multi-line content

  return Math.min(score, 100);
}

/**
 * Build deployment analysis prompt for AI enhancement
 */
async function buildDeploymentAnalysisPrompt(
  deploymentResult: DeployApplicationResult,
  manifests: KubernetesManifest[],
  deployedResources: Array<{ kind: string; name: string; namespace: string }>,
  failedResources: Array<{ kind: string; name: string; error: string }>,
): Promise<{ messages: MCPMessage[]; maxTokens: number }> {
  const basePrompt = `You are a Kubernetes deployment expert. Analyze deployment results and provide specific recommendations.

Focus on:
1. Deployment health and readiness
2. Resource optimization and scaling
3. Troubleshooting common deployment issues
4. Security and best practices
5. Monitoring and observability

Provide concrete, actionable recommendations.

Analyze this Kubernetes deployment and provide optimization recommendations:

**Deployment Summary:**
- Name: ${deploymentResult.deploymentName}
- Namespace: ${deploymentResult.namespace}
- Ready: ${deploymentResult.ready}
- Replicas: ${deploymentResult.replicas} (${deploymentResult.status?.readyReplicas}/${deploymentResult.status?.totalReplicas})
- Endpoints: ${deploymentResult.endpoints.length}

**Deployed Resources:**
${deployedResources.map((r) => `- ${r.kind}: ${r.name}`).join('\n')}

**Failed Resources:**
${failedResources.length > 0 ? failedResources.map((r) => `- ${r.kind}: ${r.name} (${r.error})`).join('\n') : 'None'}

**Manifest Types:**
${manifests.map((m) => `- ${m.kind}: ${m.metadata?.name}`).join('\n')}

Please provide:
1. **Health Recommendations:** Ways to improve deployment health and reliability
2. **Optimizations:** Performance and resource optimization suggestions
3. **Troubleshooting:** Common issues and debugging steps
4. **Best Practices:** Kubernetes deployment best practices to consider

Format your response as clear, actionable recommendations.`;

  const messages = await buildMessages({
    basePrompt,
    topic: TOPICS.GENERATE_K8S_MANIFESTS,
    tool: 'deploy',
    environment: 'kubernetes',
  });

  return { messages: toMCPMessages(messages).messages, maxTokens: 2048 };
}

/**
 * Generate AI-powered deployment analysis
 */
async function generateDeploymentAnalysis(
  deploymentResult: DeployApplicationResult,
  manifests: KubernetesManifest[],
  deployedResources: Array<{ kind: string; name: string; namespace: string }>,
  failedResources: Array<{ kind: string; name: string; error: string }>,
  ctx: ToolContext,
): Promise<Result<DeploymentAnalysis>> {
  try {
    const analysisResult = await sampleWithRerank(
      ctx,
      async () =>
        buildDeploymentAnalysisPrompt(
          deploymentResult,
          manifests,
          deployedResources,
          failedResources,
        ),
      scoreDeploymentAnalysis,
      { count: 2, stopAt: 85 },
    );

    if (!analysisResult.ok) {
      return Failure(`Failed to generate deployment analysis: ${analysisResult.error}`);
    }

    const text = analysisResult.value.text;

    // Parse the AI response to extract structured analysis
    const recommendations: string[] = [];
    const optimizations: string[] = [];
    const troubleshooting: string[] = [];

    const lines = text
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => line.length > 0);

    let currentSection = '';
    for (const line of lines) {
      if (
        line.includes('Health Recommendations') ||
        line.includes('health') ||
        line.includes('Health')
      ) {
        currentSection = 'health';
        continue;
      }
      if (
        line.includes('Optimizations') ||
        line.includes('optimization') ||
        line.includes('Optimization')
      ) {
        currentSection = 'optimization';
        continue;
      }
      if (
        line.includes('Troubleshooting') ||
        line.includes('troubleshoot') ||
        line.includes('Troubleshoot')
      ) {
        currentSection = 'troubleshooting';
        continue;
      }
      if (line.includes('Best Practices') || line.includes('practices')) {
        currentSection = 'practices';
        continue;
      }

      if (line.startsWith('-') || line.startsWith('*') || line.match(/^\d+\./)) {
        const cleanLine = line.replace(/^[-*\d.]\s*/, '');
        if (cleanLine.length > 10) {
          if (currentSection === 'health') {
            recommendations.push(cleanLine);
          } else if (currentSection === 'optimization') {
            optimizations.push(cleanLine);
          } else if (currentSection === 'troubleshooting') {
            troubleshooting.push(cleanLine);
          } else if (currentSection === 'practices') {
            recommendations.push(`Best Practice: ${cleanLine}`);
          } else {
            recommendations.push(cleanLine);
          }
        }
      }
    }

    // Add general recommendations if none found
    if (recommendations.length === 0) {
      recommendations.push('Monitor pod status and resource usage');
      recommendations.push('Configure health checks for better reliability');
    }

    if (troubleshooting.length === 0) {
      troubleshooting.push('Check pod logs if deployment is not ready');
      troubleshooting.push('Verify resource limits and requests');
      troubleshooting.push('Check service selectors match pod labels');
    }

    return Success({
      recommendations,
      optimizations,
      troubleshooting,
      confidence: analysisResult.value.winner.score,
    });
  } catch (error) {
    return Failure(`Failed to generate deployment analysis: ${extractErrorMessage(error)}`);
  }
}

/**
 * Parse YAML/JSON manifest content with validation
 */
function parseManifest(content: string, logger: Logger): KubernetesManifest[] {
  try {
    // Try parsing as JSON first
    const parsed = JSON.parse(content);
    const manifests = Array.isArray(parsed) ? parsed : [parsed];
    return validateManifests(manifests, logger);
  } catch {
    // Parse YAML documents (supports multi-document YAML)
    try {
      // In js-yaml v4, loadAll is safe by default (no code execution)
      const documents = yaml.loadAll(content);
      const filtered = documents.filter((doc: unknown) => doc !== null && doc !== undefined);
      return validateManifests(filtered, logger);
    } catch (yamlError) {
      logger.error({ error: yamlError }, 'Failed to parse manifests as YAML');
      throw new Error(`Invalid manifest format: ${extractErrorMessage(yamlError)}`);
    }
  }
}

/**
 * Validate manifests have required structure
 */
function validateManifests(manifests: unknown[], logger: Logger): KubernetesManifest[] {
  const validated: KubernetesManifest[] = [];

  for (const manifest of manifests) {
    if (!manifest || typeof manifest !== 'object') {
      logger.warn({ manifest }, 'Skipping invalid manifest: not an object');
      continue;
    }

    const m = manifest as KubernetesManifest;
    if (!m.kind) {
      logger.warn({ manifest }, 'Skipping manifest without kind');
      continue;
    }

    if (!m.metadata?.name) {
      logger.warn({ kind: m.kind }, 'Manifest missing metadata.name');
    }

    validated.push(m);
  }

  return validated;
}

/**
 * Check if a kind is in the manifest order
 */
function getManifestOrderIndex(kind: string | undefined): number {
  if (!kind) return 999;
  const index = MANIFEST_ORDER.indexOf(kind as (typeof MANIFEST_ORDER)[number]);
  return index >= 0 ? index : 999;
}

/**
 * Order manifests for deployment based on resource dependencies
 */
function orderManifests(manifests: KubernetesManifest[]): KubernetesManifest[] {
  return manifests.sort((a, b) => {
    const aIndex = getManifestOrderIndex(a.kind);
    const bIndex = getManifestOrderIndex(b.kind);
    return aIndex - bIndex;
  });
}

/**
 * Find manifest by kind with type safety
 */
function findManifestByKind<T extends KubernetesManifest>(
  manifests: KubernetesManifest[],
  kind: string,
): T | undefined {
  return manifests.find((m) => m.kind === kind) as T | undefined;
}

/**
 * Deploy a single manifest with error recovery
 */
async function deployManifest(
  manifest: KubernetesManifest,
  namespace: string,
  k8sClient: ReturnType<typeof createKubernetesClient>,
  logger: Logger,
): Promise<{ success: boolean; resource?: { kind: string; name: string; namespace: string } }> {
  const { kind = 'unknown', metadata } = manifest;
  const name = metadata?.name ?? 'unknown';

  try {
    const applyResult = await k8sClient.applyManifest(manifest as K8sManifest, namespace);

    if (!applyResult.ok) {
      logger.error({ kind, name, error: applyResult.error }, 'Failed to apply manifest');
      return { success: false };
    }

    logger.info({ kind, name }, 'Successfully deployed resource');

    return {
      success: true,
      resource: {
        kind,
        name,
        namespace: metadata?.namespace ?? namespace,
      },
    };
  } catch (error) {
    logger.error({ kind, name, error: extractErrorMessage(error) }, 'Exception deploying resource');
    return { success: false };
  }
}

/**
 * Core deployment implementation
 */
async function deployApplicationImpl(
  params: DeployApplicationParams,
  context: ToolContext,
): Promise<Result<DeployApplicationResult>> {
  const logger = getToolLogger(context, 'deploy');
  const timer = createToolTimer(logger, 'deploy');
  try {
    const {
      namespace = DEPLOYMENT_CONFIG.DEFAULT_NAMESPACE,
      replicas = DEPLOYMENT_CONFIG.DEFAULT_REPLICAS,
      environment = DEPLOYMENT_CONFIG.DEFAULT_ENVIRONMENT,
    } = params;

    const cluster = DEPLOYMENT_CONFIG.DEFAULT_CLUSTER;
    const dryRun = DEPLOYMENT_CONFIG.DRY_RUN;
    const wait = DEPLOYMENT_CONFIG.WAIT_FOR_READY;
    const timeout = DEPLOYMENT_CONFIG.WAIT_TIMEOUT_SECONDS;
    logger.info({ namespace, cluster, dryRun, environment }, 'Starting application deployment');

    // Use session facade directly
    const sessionId = params.sessionId || context.session?.id;
    if (!sessionId) {
      return Failure('Session ID is required for deployment operations');
    }

    logger.info(
      { sessionId, namespace, environment },
      'Starting Kubernetes deployment with session',
    );
    const k8sClient = createKubernetesClient(logger);

    // Get K8s manifests from session results
    let manifestContents: string | undefined;
    try {
      if (context.session) {
        const results = context.session.get('results');
        if (results && typeof results === 'object' && 'generate-k8s-manifests' in results) {
          const k8sResult = (results as any)['generate-k8s-manifests'] as {
            manifests?: string;
          };
          if (k8sResult.manifests) {
            manifestContents = k8sResult.manifests;
            logger.info({ sessionId }, 'Retrieved K8s manifests from session results');
          }
        }
      }
    } catch (error) {
      logger.error({ error }, 'Failed to get K8s manifests from session');
    }

    if (!manifestContents) {
      return Failure(
        'No Kubernetes manifests found in session. Please run generate-k8s-manifests tool first.',
      );
    }

    // Parse and validate manifests
    let manifests: KubernetesManifest[];
    try {
      // The manifests are already a string containing all YAML documents

      if (!manifestContents) {
        return Failure('No valid manifest content found in session');
      }

      manifests = parseManifest(manifestContents, logger);
    } catch (error) {
      return Failure(`Failed to parse manifests: ${extractErrorMessage(error)}`);
    }

    if (manifests.length === 0) {
      return Failure('No valid manifests found in session');
    }
    // Order manifests for deployment
    const orderedManifests = orderManifests(manifests);
    logger.info(
      { manifestCount: orderedManifests.length, dryRun, namespace },
      'Deploying manifests to Kubernetes',
    );
    // Deploy manifests with proper error handling
    const deployedResources: Array<{ kind: string; name: string; namespace: string }> = [];
    const failedResources: Array<{ kind: string; name: string; error: string }> = [];

    if (!dryRun) {
      // Report progress
      logger.info({ totalManifests: orderedManifests.length }, 'Starting manifest deployment');

      for (let i = 0; i < orderedManifests.length; i++) {
        const manifest = orderedManifests[i];
        if (!manifest) continue; // Skip undefined entries

        const progress = `[${i + 1}/${orderedManifests.length}]`;

        logger.debug(
          {
            progress,
            kind: manifest.kind,
            name: manifest.metadata?.name,
          },
          'Deploying manifest',
        );

        const result = await deployManifest(manifest, namespace, k8sClient, logger);

        if (result.success && result.resource) {
          deployedResources.push(result.resource);
        } else {
          failedResources.push({
            kind: manifest.kind ?? 'unknown',
            name: manifest.metadata?.name ?? 'unknown',
            error: 'Deployment failed',
          });
        }
      }

      // Log deployment summary
      logger.info(
        {
          deployed: deployedResources.length,
          failed: failedResources.length,
          total: orderedManifests.length,
        },
        'Manifest deployment completed',
      );

      if (failedResources.length > 0) {
        logger.warn({ failedResources }, 'Some resources failed to deploy');
      }
    } else {
      // For dry run, simulate deployment
      logger.info('Dry run mode - simulating deployment');
      for (const manifest of orderedManifests) {
        deployedResources.push({
          kind: manifest.kind ?? 'unknown',
          name: manifest.metadata?.name ?? 'unknown',
          namespace: manifest.metadata?.namespace ?? namespace,
        });
      }
    }
    // Find deployment and service with type safety
    const deployment = findManifestByKind<DeploymentManifest>(orderedManifests, 'Deployment');
    const service = findManifestByKind<ServiceManifest>(orderedManifests, 'Service');
    const ingress = findManifestByKind<IngressManifest>(orderedManifests, 'Ingress');

    const deploymentName = deployment?.metadata?.name ?? 'app';
    const serviceName = service?.metadata?.name ?? deploymentName;
    // Wait for deployment to be ready
    let ready = false;
    let readyReplicas = 0;
    const totalReplicas = deployment?.spec?.replicas ?? replicas;
    if (wait && !dryRun) {
      // Wait for deployment with configurable retry delay
      logger.info(
        { deploymentName, timeoutSeconds: timeout },
        'Waiting for deployment to be ready',
      );

      const startTime = Date.now();
      const retryDelay = DEFAULT_TIMEOUTS.deploymentPoll || 5000;
      const maxWaitTime = timeout * 1000;
      let attempts = 0;

      while (Date.now() - startTime < maxWaitTime) {
        attempts++;
        const statusResult = await k8sClient.getDeploymentStatus(namespace, deploymentName);

        if (statusResult.ok && statusResult.value?.ready) {
          ready = true;
          readyReplicas = statusResult.value.readyReplicas || 0;
          logger.info(
            {
              deploymentName,
              readyReplicas,
              attempts,
              elapsedSeconds: Math.round((Date.now() - startTime) / 1000),
            },
            'Deployment is ready',
          );
          break;
        }

        // Log progress periodically
        if (attempts % 6 === 0) {
          // Every ~30 seconds at 5s intervals
          logger.debug(
            {
              deploymentName,
              attempt: attempts,
              elapsedSeconds: Math.round((Date.now() - startTime) / 1000),
              currentStatus: statusResult.ok ? statusResult.value : undefined,
            },
            'Still waiting for deployment',
          );
        }

        // Wait before checking again using configured delay
        await new Promise((resolve) => setTimeout(resolve, retryDelay));
      }

      if (!ready) {
        logger.error(
          { deploymentName, timeoutSeconds: timeout, attempts },
          'Deployment did not become ready within timeout - check pod status and logs',
        );
      }
    } else if (dryRun) {
      // For dry runs, mark as ready
      ready = true;
      readyReplicas = totalReplicas;
    }
    // Build endpoints with proper configuration
    const endpoints: Array<{ type: 'internal' | 'external'; url: string; port: number }> = [];

    if (service) {
      const port = service.spec?.ports?.[0]?.port ?? DEPLOYMENT_CONFIG.DEFAULT_PORT;

      // Internal endpoint
      endpoints.push({
        type: 'internal',
        url: `http://${serviceName}.${namespace}.svc.cluster.local`,
        port,
      });

      // External endpoint if LoadBalancer
      if (service.spec?.type === 'LoadBalancer') {
        endpoints.push({
          type: 'external',
          url: DEPLOYMENT_CONFIG.PENDING_LB_URL,
          port,
        });
      }

      // NodePort endpoint
      if (service.spec?.type === 'NodePort') {
        const nodePort = service.spec?.ports?.[0]?.nodePort;
        if (nodePort) {
          endpoints.push({
            type: 'external',
            url: `http://<node-ip>`,
            port: nodePort,
          });
        }
      }
    }

    // Check for ingress
    if (ingress) {
      const host = ingress.spec?.rules?.[0]?.host ?? DEPLOYMENT_CONFIG.DEFAULT_INGRESS_HOST;
      endpoints.push({
        type: 'external',
        url: `http://${host}`,
        port: DEPLOYMENT_CONFIG.DEFAULT_PORT,
      });
    }

    // Generate AI-powered deployment analysis
    let deploymentAnalysis: DeploymentAnalysis | undefined;
    try {
      const analysisResult = await generateDeploymentAnalysis(
        {
          success: ready,
          sessionId,
          namespace,
          deploymentName,
          serviceName,
          endpoints,
          ready,
          replicas: totalReplicas,
          status: {
            readyReplicas,
            totalReplicas,
            conditions: [
              {
                type: 'Available',
                status: ready ? 'True' : 'False',
                message: ready ? 'Deployment is available' : 'Deployment is pending',
              },
            ],
          },
        },
        orderedManifests,
        deployedResources,
        failedResources,
        context,
      );

      if (analysisResult.ok) {
        deploymentAnalysis = analysisResult.value;
        logger.info(
          {
            recommendations: deploymentAnalysis.recommendations.length,
            confidence: deploymentAnalysis.confidence,
          },
          'Generated AI deployment analysis',
        );
      } else {
        logger.warn({ error: analysisResult.error }, 'Failed to generate deployment analysis');
      }
    } catch (error) {
      logger.warn({ error: extractErrorMessage(error) }, 'Error generating deployment analysis');
    }

    // Prepare the result
    const result: DeployApplicationResult = {
      success: ready, // Success depends on deployment readiness
      sessionId,
      namespace,
      deploymentName,
      serviceName,
      endpoints,
      ready,
      replicas: totalReplicas,
      ...(deploymentAnalysis && { deploymentAnalysis }),
      status: {
        readyReplicas,
        totalReplicas,
        conditions: [
          {
            type: 'Available',
            status: ready ? 'True' : 'False',
            message: ready ? 'Deployment is available' : 'Deployment is pending',
          },
        ],
      },
      workflowHints: {
        nextStep: 'verify-deploy',
        message: `Application deployed successfully. Use "verify-deploy" with sessionId ${sessionId} to check deployment status, or access your application at: ${endpoints.length > 0 ? endpoints[0]?.url || 'the service endpoint' : 'the service endpoint'}.${deploymentAnalysis ? ' Review AI deployment analysis for optimization recommendations.' : ''}`,
      },
    };

    // Store deployment result in session
    if (context.session) {
      context.session.set('results', {
        ...((context.session.get('results') as Record<string, any>) || {}),
        deploy: result,
      });
      context.session.set('current_step', 'deploy');
      context.session.pushStep('deploy');
    }
    timer.end({ deploymentName, ready, sessionId });

    if (ready) {
      logger.info(
        { sessionId, deploymentName, serviceName, ready, namespace },
        'Kubernetes deployment completed successfully',
      );
    } else {
      logger.warn(
        { sessionId, deploymentName, serviceName, ready, namespace },
        'Kubernetes deployment completed but pods are not ready',
      );
    }

    return Success(result);
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'Application deployment failed');
    return Failure(extractErrorMessage(error));
  }
}

/**
 * Export the deploy tool directly
 */
export const deployApplication = deployApplicationImpl;

// New Tool interface export
import type { Tool } from '@/types/tool';

const tool: Tool<typeof deployApplicationSchema, DeployApplicationResult> = {
  name: 'deploy',
  description: 'Deploy applications to Kubernetes clusters',
  version: '2.0.0',
  schema: deployApplicationSchema,
  metadata: {
    aiDriven: true,
    knowledgeEnhanced: true,
    samplingStrategy: 'rerank',
    enhancementCapabilities: [
      'deployment-analysis',
      'troubleshooting',
      'optimization-recommendations',
    ],
  },
  run: deployApplicationImpl,
};

export default tool;
