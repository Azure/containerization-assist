/**
 * Verify Deployment Tool - Standardized Implementation
 *
 * Verifies Kubernetes deployment health and retrieves endpoints using
 * standardized helpers for consistency and improved error handling
 *
 * @example
 * ```typescript
 * const result = await verifyDeployment({
 *   sessionId: 'session-123',
 *   deploymentName: 'my-app',
 *   namespace: 'production',
 *   checks: ['pods', 'services', 'health']
 * }, context, logger);
 *
 * if (result.success) {
 *   logger.info('Deployment verified', {
 *     ready: result.ready,
 *     endpoints: result.endpoints
 *   });
 * }
 * ```
 */

import { getToolLogger, createToolTimer } from '@/lib/tool-helpers';
import { extractErrorMessage } from '@/lib/error-utils';
import type { ToolContext } from '@/mcp/context';
import { createKubernetesClient, KubernetesClient } from '@/lib/kubernetes';

import { DEFAULT_TIMEOUTS } from '@/config/defaults';
import { Success, Failure, type Result, TOPICS } from '@/types';
import { verifyDeploymentSchema, type VerifyDeploymentParams } from './schema';
import { sampleWithRerank } from '@/mcp/ai/sampling-runner';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';

export interface VerifyDeploymentResult extends Record<string, unknown> {
  success: boolean;
  sessionId: string;
  namespace: string;
  deploymentName: string;
  serviceName: string;
  endpoints: Array<{
    type: 'internal' | 'external';
    url: string;
    port: number;
    healthy?: boolean;
  }>;
  ready: boolean;
  replicas: number;
  status: {
    readyReplicas: number;
    totalReplicas: number;
    conditions: Array<{
      type: string;
      status: string;
      message: string;
    }>;
  };
  healthCheck?: {
    status: 'healthy' | 'unhealthy' | 'unknown';
    message: string;
    checks?: Array<{
      name: string;
      status: 'pass' | 'fail';
      message?: string;
    }>;
  };
  validationInsights?: DeploymentValidationInsights;
  workflowHints?: {
    nextStep: string;
    message: string;
  };
}

// Additional interface for AI validation insights
export interface DeploymentValidationInsights {
  troubleshootingSteps: string[];
  healthRecommendations: string[];
  performanceInsights: string[];
  confidence: number;
}

/**
 * Check deployment health
 */
async function checkDeploymentHealth(
  k8sClient: KubernetesClient,
  namespace: string,
  deploymentName: string,
  timeout: number,
): Promise<{
  ready: boolean;
  readyReplicas: number;
  totalReplicas: number;
  status: 'healthy' | 'unhealthy' | 'unknown';
  message: string;
}> {
  const startTime = Date.now();
  const pollInterval = DEFAULT_TIMEOUTS.healthCheck || 5000;

  while (Date.now() - startTime < timeout * 1000) {
    const statusResult = await k8sClient.getDeploymentStatus(namespace, deploymentName);

    if (statusResult.ok && statusResult.value?.ready) {
      return {
        ready: true,
        readyReplicas: statusResult.value.readyReplicas ?? 0,
        totalReplicas: statusResult.value.totalReplicas ?? 0,
        status: 'healthy',
        message: 'Deployment is healthy and ready',
      };
    }

    // Wait before checking again using configured interval
    await new Promise((resolve) => setTimeout(resolve, pollInterval));
  }

  return {
    ready: false,
    readyReplicas: 0,
    totalReplicas: 1,
    status: 'unhealthy',
    message: 'Deployment health check timed out',
  };
}

/**
 * Check endpoint health
 */
async function checkEndpointHealth(url: string): Promise<boolean> {
  try {
    // Make HTTP health check request
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), DEFAULT_TIMEOUTS.healthCheck || 5000);

    try {
      const response = await fetch(url, {
        method: 'GET',
        signal: controller.signal,
        headers: {
          'User-Agent': 'containerization-assist-health-check',
        },
      });

      clearTimeout(timeoutId);

      // Consider 2xx and 3xx responses as healthy
      return response.ok || (response.status >= 300 && response.status < 400);
    } catch (fetchError: unknown) {
      clearTimeout(timeoutId);

      // If it's an abort error, the request timed out
      if (fetchError instanceof Error && fetchError.name === 'AbortError') {
        return false;
      }

      // For other errors (network issues, etc.), consider unhealthy
      return false;
    }
  } catch {
    return false;
  }
}

/**
 * Score validation insights based on quality and relevance
 */
function scoreValidationInsights(
  insights: DeploymentValidationInsights,
  _verificationResult: VerifyDeploymentResult,
  _healthChecks: Array<{ name: string; status: string; message?: string }>,
): number {
  let score = 0;

  // Quality scoring for troubleshooting steps (0-30 points)
  if (insights.troubleshootingSteps && insights.troubleshootingSteps.length > 0) {
    score += Math.min(insights.troubleshootingSteps.length * 5, 20);

    // Bonus for actionable steps (contain keywords like 'check', 'verify', 'restart')
    const actionableSteps = insights.troubleshootingSteps.filter((step) =>
      /check|verify|restart|scale|update|apply|rollback|debug/i.test(step),
    ).length;
    score += Math.min(actionableSteps * 2, 10);
  }

  // Quality scoring for health recommendations (0-25 points)
  if (insights.healthRecommendations && insights.healthRecommendations.length > 0) {
    score += Math.min(insights.healthRecommendations.length * 4, 16);

    // Bonus for specific health recommendations
    const specificRecommendations = insights.healthRecommendations.filter((rec) =>
      /resource|memory|cpu|probe|readiness|liveness|limit|request/i.test(rec),
    ).length;
    score += Math.min(specificRecommendations * 3, 9);
  }

  // Quality scoring for performance insights (0-25 points)
  if (insights.performanceInsights && insights.performanceInsights.length > 0) {
    score += Math.min(insights.performanceInsights.length * 4, 16);

    // Bonus for performance-specific insights
    const performanceSpecific = insights.performanceInsights.filter((insight) =>
      /scale|performance|optimization|efficiency|throughput|latency|resource/i.test(insight),
    ).length;
    score += Math.min(performanceSpecific * 3, 9);
  }

  // Confidence penalty/bonus (0-20 points)
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
 * Build prompt for generating validation insights
 */
function buildValidationInsightsPrompt(
  verificationResult: VerifyDeploymentResult,
  healthChecks: Array<{ name: string; status: string; message?: string }>,
): string {
  const hasIssues =
    !verificationResult.success || verificationResult.healthCheck?.status !== 'healthy';
  const context = hasIssues
    ? 'with issues that need attention'
    : 'that appears healthy but may benefit from optimization';

  return `As a Kubernetes deployment expert, analyze this deployment verification result ${context}.

Deployment Status:
- Name: ${verificationResult.deploymentName}
- Namespace: ${verificationResult.namespace}
- Success: ${verificationResult.success}
- Ready: ${verificationResult.ready}
- Replicas: ${verificationResult.replicas} (${verificationResult.status.readyReplicas}/${verificationResult.status.totalReplicas} ready)
- Health Status: ${verificationResult.healthCheck?.status || 'unknown'}
- Health Message: ${verificationResult.healthCheck?.message || 'No health check performed'}

${Array.isArray(verificationResult.status.conditions) && verificationResult.status.conditions.length > 0 ? `Conditions:\n${verificationResult.status.conditions.map((c) => `- ${c.type}: ${c.status} - ${c.message}`).join('\n')}` : ''}

${healthChecks.length > 0 ? `Health Checks:\n${healthChecks.map((check) => `- ${check.name}: ${check.status}${check.message ? ` - ${check.message}` : ''}`).join('\n')}` : ''}

${Array.isArray(verificationResult.endpoints) && verificationResult.endpoints.length > 0 ? `Endpoints:\n${verificationResult.endpoints.map((ep) => `- ${ep.type}: ${ep.url}:${ep.port} (healthy: ${ep.healthy})`).join('\n')}` : ''}

Provide a JSON response with:
1. troubleshootingSteps: Array of specific, actionable steps to diagnose and fix issues (if any)
2. healthRecommendations: Array of recommendations to improve deployment health and reliability
3. performanceInsights: Array of insights for optimizing performance and resource usage
4. confidence: Number between 0-1 indicating confidence in the analysis

Focus on:
- Kubernetes-specific best practices and troubleshooting
- Resource optimization and scaling recommendations
- Health check and probe configuration
- Network and service connectivity
- Security and reliability improvements

Respond with valid JSON only.`;
}

/**
 * Generate AI-powered validation insights for deployment verification
 */
async function generateValidationInsights(
  verificationResult: VerifyDeploymentResult,
  healthChecks: Array<{ name: string; status: string; message?: string }>,
  ctx: ToolContext,
): Promise<Result<DeploymentValidationInsights>> {
  try {
    const prompt = buildValidationInsightsPrompt(verificationResult, healthChecks);

    const messages = await buildMessages({
      basePrompt: prompt,
      topic: TOPICS.GENERATE_K8S_MANIFESTS,
      tool: 'verify-deployment',
      environment: 'production',
    });

    const result = await sampleWithRerank(
      ctx,
      async () => ({
        messages: toMCPMessages(messages).messages,
        maxTokens: 1000,
        modelPreferences: { hints: [{ name: 'deployment-validation' }] },
      }),
      (response: string) => {
        try {
          const parsed = JSON.parse(response);
          const insights: DeploymentValidationInsights = {
            troubleshootingSteps: Array.isArray(parsed.troubleshootingSteps)
              ? parsed.troubleshootingSteps
              : [],
            healthRecommendations: Array.isArray(parsed.healthRecommendations)
              ? parsed.healthRecommendations
              : [],
            performanceInsights: Array.isArray(parsed.performanceInsights)
              ? parsed.performanceInsights
              : [],
            confidence:
              typeof parsed.confidence === 'number' &&
              parsed.confidence >= 0 &&
              parsed.confidence <= 1
                ? parsed.confidence
                : 0.5,
          };

          return scoreValidationInsights(insights, verificationResult, healthChecks);
        } catch {
          return { overall: 0 };
        }
      },
      {},
    );

    if (result.ok) {
      const parsed = JSON.parse(result.value.text);
      const insights: DeploymentValidationInsights = {
        troubleshootingSteps: Array.isArray(parsed.troubleshootingSteps)
          ? parsed.troubleshootingSteps
          : [],
        healthRecommendations: Array.isArray(parsed.healthRecommendations)
          ? parsed.healthRecommendations
          : [],
        performanceInsights: Array.isArray(parsed.performanceInsights)
          ? parsed.performanceInsights
          : [],
        confidence:
          typeof parsed.confidence === 'number' && parsed.confidence >= 0 && parsed.confidence <= 1
            ? parsed.confidence
            : 0.5,
      };
      return Success(insights);
    } else {
      return Failure('Failed to generate validation insights');
    }
  } catch (error) {
    return Failure(`Error generating validation insights: ${error}`);
  }
}

/**
 * Deployment verification implementation - direct execution without wrapper
 */
async function verifyDeploymentImpl(
  params: VerifyDeploymentParams,
  context: ToolContext,
): Promise<Result<VerifyDeploymentResult>> {
  if (!params || typeof params !== 'object') {
    return Failure('Invalid parameters provided');
  }
  const logger = getToolLogger(context, 'verify-deploy');
  const timer = createToolTimer(logger, 'verify-deploy');

  const {
    deploymentName: configDeploymentName,
    namespace: configNamespace,
    checks = ['pods', 'services', 'health'],
  } = params;

  const timeout = 60;

  try {
    // Use session facade directly
    const sessionId = params.sessionId || context.session?.id;
    if (!sessionId) {
      return Failure('Session ID is required for deployment verification');
    }

    if (!context.session) {
      return Failure('Session not available');
    }

    logger.info({ sessionId, checks }, 'Starting Kubernetes deployment verification with session');

    const k8sClient = createKubernetesClient(logger);

    if (!configDeploymentName) {
      return Failure('Deployment name is required. Provide deploymentName parameter.');
    }

    const namespace = configNamespace ?? 'default';
    const deploymentName = configDeploymentName;
    const serviceName = deploymentName;
    const endpoints: Array<{ type: string; url: string; port: number; healthy?: boolean }> = [];

    logger.info({ namespace, deploymentName }, 'Checking deployment health');

    // Check deployment health
    const health = await checkDeploymentHealth(k8sClient, namespace, deploymentName, timeout);

    // Initialize health checks
    const healthChecks: Array<{ name: string; status: 'pass' | 'fail'; message?: string }> = [];

    // Check each endpoint if 'health' is in checks
    if (checks.includes('health')) {
      for (const endpoint of endpoints) {
        if (endpoint.type === 'external') {
          const isHealthy = await checkEndpointHealth(endpoint.url);
          endpoint.healthy = isHealthy;
          healthChecks.push({
            name: `${endpoint.type}-endpoint`,
            status: isHealthy ? 'pass' : 'fail',
            message: `${endpoint.url}:${endpoint.port}`,
          });
        }
      }
    }

    // Determine overall health status
    const allHealthy = healthChecks.every((check) => check.status === 'pass');
    const overallStatus =
      health.ready && (healthChecks.length === 0 || allHealthy)
        ? 'healthy'
        : health.ready
          ? 'unhealthy'
          : 'unknown';

    // Prepare the result
    const result: VerifyDeploymentResult = {
      success: overallStatus === 'healthy',
      sessionId,
      namespace,
      deploymentName,
      serviceName,
      endpoints: endpoints as Array<{
        type: 'internal' | 'external';
        url: string;
        port: number;
        healthy?: boolean;
      }>,
      ready: health.ready,
      replicas: health.totalReplicas,
      status: {
        readyReplicas: health.readyReplicas,
        totalReplicas: health.totalReplicas,
        conditions: [
          {
            type: 'Available',
            status: health.ready ? 'True' : 'False',
            message: health.message,
          },
        ],
      },
      healthCheck: {
        status: overallStatus,
        message: health.message,
        ...(healthChecks.length > 0 && { checks: healthChecks }),
      },
    };

    // Session storage is handled by orchestrator automatically

    timer.end({ deploymentName, ready: health.ready, sessionId });

    // Generate AI-powered validation insights
    let validationInsights: DeploymentValidationInsights | undefined;
    try {
      const insightResult = await generateValidationInsights(result, healthChecks, context);

      if (insightResult.ok) {
        validationInsights = insightResult.value;
        logger.info(
          {
            troubleshootingSteps: validationInsights.troubleshootingSteps.length,
            confidence: validationInsights.confidence,
          },
          'Generated AI validation insights',
        );
      } else {
        logger.warn({ error: insightResult.error }, 'Failed to generate validation insights');
      }
    } catch (error) {
      logger.warn({ error: extractErrorMessage(error) }, 'Error generating validation insights');
    }

    // Add validation insights and workflow hints to result
    const finalResult: VerifyDeploymentResult = {
      ...result,
      ...(validationInsights && { validationInsights }),
      workflowHints: {
        nextStep: result.success ? 'ops' : 'fix-deployment-issues',
        message: result.success
          ? `Deployment verification successful. Use "ops" with sessionId ${sessionId} for operational tasks, or review the deployment status.${validationInsights ? ' Check AI validation insights for optimization recommendations.' : ''}`
          : `Deployment verification found issues. ${validationInsights ? 'Review AI troubleshooting steps to resolve problems.' : 'Check deployment status and pod logs to diagnose issues.'}`,
      },
    };

    return Success(finalResult);
  } catch (error) {
    timer.error(error);

    return Failure(extractErrorMessage(error));
  }
}

/**
 * Verify deployment tool
 */
export const verifyDeployment = verifyDeploymentImpl;

// New Tool interface export
import type { MCPTool } from '@/types/tool';

const tool: MCPTool<typeof verifyDeploymentSchema, VerifyDeploymentResult> = {
  name: 'verify-deploy',
  description: 'Verify Kubernetes deployment status with AI-powered validation insights',
  version: '2.0.0',
  schema: verifyDeploymentSchema,
  metadata: {
    aiDriven: true,
    knowledgeEnhanced: false,
    samplingStrategy: 'single',
    enhancementCapabilities: [
      'validation-insights',
      'troubleshooting-guidance',
      'performance-recommendations',
      'health-analysis',
    ],
  },
  run: verifyDeploymentImpl,
};

export default tool;
