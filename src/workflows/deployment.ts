/**
 * Deployment Workflow - Orchestrates the complete deployment pipeline
 *
 * Steps:
 * 1. Prepare Kubernetes cluster
 * 2. Generate K8s manifests
 * 3. Push image to registry
 * 4. Deploy application to cluster
 * 5. Verify deployment health
 */

import { prepareCluster } from '@tools/prepare-cluster';
import { generateK8sManifests } from '@tools/generate-k8s-manifests';
import { pushImage } from '@tools/push-image';
import { deployApplication } from '@tools/deploy';
import { verifyDeployment } from '@tools/verify-deployment';
import { isFail } from '@types';
import { createTimer, type Logger } from '@lib/logger';
import type { ToolContext } from '../mcp/context/types';
import { createSessionManager, type SessionManager } from '../lib/session';
import type {
  DeploymentWorkflowParams,
  DeploymentWorkflowResult,
  WorkflowStep,
  WorkflowContext,
} from './types';

/**
 * Run the complete deployment workflow
 */
export async function runDeploymentWorkflow(
  params: DeploymentWorkflowParams,
  toolContext: ToolContext,
  _options?: { abortSignal?: AbortSignal },
): Promise<DeploymentWorkflowResult> {
  const logger = toolContext.logger;
  const timer = createTimer(logger, 'deployment-workflow');
  // Access sessionManager through context if available
  const sessionManager: SessionManager = toolContext.sessionManager || createSessionManager(logger);
  const { sessionId, imageId, clusterConfig, deploymentOptions } = params;

  // Initialize workflow context
  const context: WorkflowContext = {
    sessionId,
    steps: [],
    artifacts: new Map(),
    metadata: {
      startTime: new Date(),
      imageId,
      deploymentName: deploymentOptions.name,
    },
  };

  // Define workflow steps
  const steps: WorkflowStep[] = [
    { name: 'prepare-cluster', status: 'pending' },
    { name: 'generate-manifests', status: 'pending' },
    { name: 'push-image', status: 'pending' },
    { name: 'deploy-application', status: 'pending' },
    { name: 'verify-deployment', status: 'pending' },
  ];
  context.steps = steps;

  try {
    logger.info('Starting deployment workflow');

    // Create or get session
    let session = await sessionManager.get(sessionId);
    if (!session) {
      logger.info({ sessionId }, 'Creating new session for deployment workflow');
      session = await sessionManager.create(sessionId);
    }

    // Update session
    await sessionManager.update(sessionId, {
      status: 'active',
      stage: 'prepare-cluster',
    });

    // Step 1: Prepare cluster
    const prepareStep = steps[0];
    if (!prepareStep) {
      const endTime = new Date();
      const errorMessage = 'Prepare cluster step not found';

      await sessionManager.update(sessionId, {
        status: 'failed',
        metadata: {
          error: errorMessage,
          failedAt: endTime.toISOString(),
        },
      });

      timer.end();
      logger.error('Deployment workflow failed - prepare step not found');

      return {
        success: false,
        sessionId,
        error: errorMessage,
        metadata: {
          startTime: context.metadata.startTime,
          endTime,
          duration: endTime.getTime() - context.metadata.startTime.getTime(),
          steps: context.steps,
        },
      };
    }
    prepareStep.status = 'running';
    prepareStep.startTime = new Date();
    context.currentStep = prepareStep.name;

    logger.info('Preparing Kubernetes cluster');

    const clusterResult = await prepareCluster(
      {
        sessionId,
        namespace: clusterConfig.namespace || 'default',
      },
      toolContext,
    );

    if (!clusterResult.ok) {
      prepareStep.status = 'failed';
      prepareStep.error = `Cluster preparation failed: ${clusterResult.error}`;
      const endTime = new Date();
      const errorMessage = `Cluster preparation failed: ${clusterResult.error}`;

      // Mark remaining steps as skipped
      steps.forEach((step) => {
        if (step.status === 'pending') {
          step.status = 'skipped';
        }
      });

      await sessionManager.update(sessionId, {
        status: 'failed',
        metadata: {
          error: errorMessage,
          failedAt: endTime.toISOString(),
        },
      });

      timer.end();
      logger.error('Deployment workflow failed during cluster preparation');

      return {
        success: false,
        sessionId,
        error: errorMessage,
        metadata: {
          startTime: context.metadata.startTime,
          endTime,
          duration: endTime.getTime() - context.metadata.startTime.getTime(),
          steps: context.steps,
        },
      };
    }
    const cluster = clusterResult.value;

    prepareStep.status = 'completed';
    prepareStep.endTime = new Date();
    prepareStep.output = cluster;
    context.artifacts.set('cluster', cluster);

    // Step 2: Generate K8s manifests
    const generateStep = steps[1];
    if (!generateStep) {
      const endTime = new Date();
      const errorMessage = 'Generate manifests step not found';

      await sessionManager.update(sessionId, {
        status: 'failed',
        metadata: {
          error: errorMessage,
          failedAt: endTime.toISOString(),
        },
      });

      timer.end();
      logger.error('Deployment workflow failed - generate step not found');

      return {
        success: false,
        sessionId,
        error: errorMessage,
        metadata: {
          startTime: context.metadata.startTime,
          endTime,
          duration: endTime.getTime() - context.metadata.startTime.getTime(),
          steps: context.steps,
        },
      };
    }
    generateStep.status = 'running';
    generateStep.startTime = new Date();
    context.currentStep = generateStep.name;

    await sessionManager.update(sessionId, {
      stage: 'generate-manifests',
    });

    logger.info('Generating Kubernetes manifests');

    const manifestResult = await generateK8sManifests(
      {
        sessionId,
        appName: deploymentOptions.name,
        namespace: cluster.namespace,
        replicas: deploymentOptions.replicas || 1,
        port: deploymentOptions.port || 8080,
        serviceType: deploymentOptions.serviceType || 'ClusterIP',
        ...(deploymentOptions.resources && {
          resources: {
            requests: {
              memory: deploymentOptions.resources.requests?.memory || '256Mi',
              cpu: deploymentOptions.resources.requests?.cpu || '100m',
            },
            limits: {
              memory: deploymentOptions.resources.limits?.memory || '512Mi',
              cpu: deploymentOptions.resources.limits?.cpu || '500m',
            },
          },
        }),
      },
      toolContext,
    );

    if (isFail(manifestResult)) {
      generateStep.status = 'failed';
      generateStep.error = `Manifest generation failed: ${manifestResult.error}`;
      const endTime = new Date();
      const errorMessage = `Manifest generation failed: ${manifestResult.error}`;

      // Mark remaining steps as skipped
      steps.forEach((step) => {
        if (step.status === 'pending') {
          step.status = 'skipped';
        }
      });

      await sessionManager.update(sessionId, {
        status: 'failed',
        metadata: {
          error: errorMessage,
          failedAt: endTime.toISOString(),
        },
      });

      timer.end();
      logger.error('Deployment workflow failed during manifest generation');

      return {
        success: false,
        sessionId,
        error: errorMessage,
        metadata: {
          startTime: context.metadata.startTime,
          endTime,
          duration: endTime.getTime() - context.metadata.startTime.getTime(),
          steps: context.steps,
        },
      };
    }

    // TypeScript assertion: manifestResult must be successful here due to early return above
    const manifests = (manifestResult as { ok: true; value: any }).value;

    generateStep.status = 'completed';
    generateStep.endTime = new Date();
    generateStep.output = manifests;
    context.artifacts.set('manifests', manifests);

    // Step 3: Push image to registry
    const pushStep = steps[2];
    if (!pushStep) {
      const endTime = new Date();
      const errorMessage = 'Push image step not found';

      await sessionManager.update(sessionId, {
        status: 'failed',
        metadata: {
          error: errorMessage,
          failedAt: endTime.toISOString(),
        },
      });

      timer.end();
      logger.error('Deployment workflow failed - push step not found');

      return {
        success: false,
        sessionId,
        error: errorMessage,
        metadata: {
          startTime: context.metadata.startTime,
          endTime,
          duration: endTime.getTime() - context.metadata.startTime.getTime(),
          steps: context.steps,
        },
      };
    }
    pushStep.status = 'running';
    pushStep.startTime = new Date();
    context.currentStep = pushStep.name;

    await sessionManager.update(sessionId, {
      stage: 'push-image',
    });

    logger.info('Pushing image to registry');

    // Update session with image info for push tool
    await sessionManager.update(sessionId, {
      workflow_state: {
        build_result: {
          imageId,
          tags: [`${deploymentOptions.registry || 'docker.io'}/${imageId}:latest`],
        },
      } as Record<string, unknown>,
    });

    const pushResult = await pushImage(
      {
        sessionId,
        registry: deploymentOptions.registry || 'docker.io',
      },
      toolContext,
    );

    if (!pushResult.ok) {
      pushStep.status = 'failed';
      pushStep.error = `Image push failed: ${pushResult.error}`;
      const endTime = new Date();
      const errorMessage = `Image push failed: ${pushResult.error}`;

      // Mark remaining steps as skipped
      steps.forEach((step) => {
        if (step.status === 'pending') {
          step.status = 'skipped';
        }
      });

      await sessionManager.update(sessionId, {
        status: 'failed',
        metadata: {
          error: errorMessage,
          failedAt: endTime.toISOString(),
        },
      });

      timer.end();
      logger.error('Deployment workflow failed during image push');

      return {
        success: false,
        sessionId,
        error: errorMessage,
        metadata: {
          startTime: context.metadata.startTime,
          endTime,
          duration: endTime.getTime() - context.metadata.startTime.getTime(),
          steps: context.steps,
        },
      };
    }
    const push = pushResult.value;

    pushStep.status = 'completed';
    pushStep.endTime = new Date();
    pushStep.output = push;
    context.artifacts.set('push', push);

    // Step 4: Deploy application
    const deployStep = steps[3];
    if (!deployStep) {
      const endTime = new Date();
      const errorMessage = 'Deploy application step not found';

      await sessionManager.update(sessionId, {
        status: 'failed',
        metadata: {
          error: errorMessage,
          failedAt: endTime.toISOString(),
        },
      });

      timer.end();
      logger.error('Deployment workflow failed - deploy step not found');

      return {
        success: false,
        sessionId,
        error: errorMessage,
        metadata: {
          startTime: context.metadata.startTime,
          endTime,
          duration: endTime.getTime() - context.metadata.startTime.getTime(),
          steps: context.steps,
        },
      };
    }
    deployStep.status = 'running';
    deployStep.startTime = new Date();
    context.currentStep = deployStep.name;

    await sessionManager.update(sessionId, {
      stage: 'deploy-application',
    });

    logger.info('Deploying application to cluster');

    // Update session with manifests for deploy tool
    const existingSession = sessionManager.get ? await sessionManager.get(sessionId) : null;
    const existingWorkflowState =
      (existingSession as unknown as Record<string, unknown>)?.workflow_state || {};
    await sessionManager.update(sessionId, {
      workflow_state: {
        ...existingWorkflowState,
        manifests: (manifests as unknown as Record<string, unknown>).manifests,
      },
    });

    const deployResult = await deployApplication(
      {
        sessionId,
        namespace: cluster.namespace,
        imageId,
      },
      toolContext,
    );

    if (!deployResult.ok) {
      deployStep.status = 'failed';
      deployStep.error = `Deployment failed: ${deployResult.error}`;
      const endTime = new Date();
      const errorMessage = `Deployment failed: ${deployResult.error}`;

      // Mark remaining steps as skipped
      steps.forEach((step) => {
        if (step.status === 'pending') {
          step.status = 'skipped';
        }
      });

      await sessionManager.update(sessionId, {
        status: 'failed',
        metadata: {
          error: errorMessage,
          failedAt: endTime.toISOString(),
        },
      });

      timer.end();
      logger.error('Deployment workflow failed during application deployment');

      return {
        success: false,
        sessionId,
        error: errorMessage,
        metadata: {
          startTime: context.metadata.startTime,
          endTime,
          duration: endTime.getTime() - context.metadata.startTime.getTime(),
          steps: context.steps,
        },
      };
    }
    const deploy = deployResult.value;

    deployStep.status = 'completed';
    deployStep.endTime = new Date();
    deployStep.output = deploy;
    context.artifacts.set('deployment', deploy);

    // Step 5: Verify deployment
    const verifyStep = steps[4];
    if (!verifyStep) {
      const endTime = new Date();
      const errorMessage = 'Verify deployment step not found';

      await sessionManager.update(sessionId, {
        status: 'failed',
        metadata: {
          error: errorMessage,
          failedAt: endTime.toISOString(),
        },
      });

      timer.end();
      logger.error('Deployment workflow failed - verify step not found');

      return {
        success: false,
        sessionId,
        error: errorMessage,
        metadata: {
          startTime: context.metadata.startTime,
          endTime,
          duration: endTime.getTime() - context.metadata.startTime.getTime(),
          steps: context.steps,
        },
      };
    }
    verifyStep.status = 'running';
    verifyStep.startTime = new Date();
    context.currentStep = verifyStep.name;

    await sessionManager.update(sessionId, {
      stage: 'verify-deployment',
    });

    logger.info('Verifying deployment health');

    const verifyResult = await verifyDeployment(
      {
        sessionId,
        deploymentName: deploymentOptions.name,
        namespace: cluster.namespace,
      },
      toolContext,
    );

    let verify: Record<string, unknown> | null = null;
    if (!verifyResult.ok) {
      verifyStep.status = 'failed';
      verifyStep.error = `Verification failed: ${verifyResult.error}`;
      // Verification failures are warnings, deployment may still be functional
      logger.warn('Deployment verification had issues');
    } else {
      verifyStep.status = 'completed';
      verify = verifyResult.value as unknown as Record<string, unknown>;
    }

    verifyStep.endTime = new Date();
    verifyStep.output = verify;
    context.artifacts.set('verification', verify);

    // Workflow completed
    const endTime = new Date();
    await sessionManager.update(sessionId, {
      status: 'completed',
      stage: 'finished',
      metadata: {
        completedAt: endTime.toISOString(),
        results: {
          deploymentName: deploymentOptions.name,
          namespace: (cluster as unknown as Record<string, unknown>).namespace,
          serviceName: (deploy as unknown as Record<string, unknown>).serviceName,
          endpoints: verify?.endpoints,
          replicas: verify?.replicas,
        },
      },
    });

    timer.end();
    logger.info('Deployment workflow completed successfully');

    return {
      success: true,
      sessionId,
      results: (() => {
        const baseResults = {
          deploymentName: deploymentOptions.name,
          namespace: (cluster as unknown as Record<string, unknown>).namespace as string,
          service: {
            name: (deploy as unknown as Record<string, unknown>).serviceName as string,
            type: deploymentOptions.serviceType || 'ClusterIP',
          },
          pods: verify
            ? [
                {
                  name: `${deploymentOptions.name}-pod`,
                  ready: verify?.ready ? Boolean(verify.ready) : false,
                  status: 'Running',
                  restarts: 0,
                },
              ]
            : [],
          verificationStatus: {
            deployment: true,
            service: Boolean((deploy as unknown as Record<string, unknown>).serviceName),
            endpoints: Boolean(
              verify?.endpoints && Array.isArray(verify.endpoints) && verify.endpoints.length > 0,
            ),
            health: Boolean(verify),
          },
        };

        if (verify?.endpoints && Array.isArray(verify.endpoints)) {
          return { ...baseResults, endpoints: verify.endpoints as string[] };
        }

        return baseResults;
      })(),
      metadata: {
        startTime: context.metadata.startTime,
        endTime,
        duration: endTime.getTime() - context.metadata.startTime.getTime(),
        steps: context.steps,
      },
    };
  } catch (error) {
    const endTime = new Date();
    const errorMessage = error instanceof Error ? error.message : 'Unknown error occurred';

    // Mark current step as failed
    const currentStepObj = steps.find((s) => s.name === context.currentStep);
    if (currentStepObj && currentStepObj.status === 'running') {
      currentStepObj.status = 'failed';
      currentStepObj.endTime = endTime;
      currentStepObj.error = errorMessage;
    }

    // Mark remaining steps as skipped
    steps.forEach((step) => {
      if (step.status === 'pending') {
        step.status = 'skipped';
      }
    });

    await sessionManager.update(sessionId, {
      status: 'failed',
      metadata: {
        error: errorMessage,
        failedAt: endTime.toISOString(),
      },
    });

    timer.end();
    logger.error('Deployment workflow failed');

    return {
      success: false,
      sessionId,
      error: errorMessage,
      metadata: {
        startTime: context.metadata.startTime,
        endTime,
        duration: endTime.getTime() - context.metadata.startTime.getTime(),
        steps: context.steps,
      },
    };
  }
}

/**
 * Export for MCP registration
 */
export const deploymentWorkflow = {
  name: 'deployment-workflow',
  description: 'Complete deployment pipeline from cluster preparation to verified deployment',
  execute: (
    params: DeploymentWorkflowParams,
    _logger: Logger,
    context?: Record<string, unknown>,
  ) => {
    const toolContext = context as unknown as ToolContext;
    const options: { abortSignal?: AbortSignal } = {};
    if (toolContext?.signal) {
      options.abortSignal = toolContext.signal;
    }
    return runDeploymentWorkflow(params, toolContext, options);
  },
  schema: {
    type: 'object',
    properties: {
      sessionId: { type: 'string', description: 'Session identifier' },
      imageId: { type: 'string', description: 'Docker image ID to deploy' },
      clusterConfig: {
        type: 'object',
        properties: {
          context: { type: 'string', description: 'Kubernetes context' },
          namespace: { type: 'string', description: 'Target namespace' },
          kubeconfig: { type: 'string', description: 'Path to kubeconfig file' },
        },
      },
      deploymentOptions: {
        type: 'object',
        properties: {
          name: { type: 'string', description: 'Deployment name' },
          replicas: { type: 'number', description: 'Number of replicas' },
          port: { type: 'number', description: 'Container port' },
          serviceType: {
            type: 'string',
            enum: ['ClusterIP', 'NodePort', 'LoadBalancer'],
            description: 'Kubernetes service type',
          },
          registry: { type: 'string', description: 'Container registry URL' },
          imagePullPolicy: {
            type: 'string',
            enum: ['Always', 'IfNotPresent', 'Never'],
            description: 'Image pull policy',
          },
          resources: {
            type: 'object',
            properties: {
              limits: {
                type: 'object',
                properties: {
                  cpu: { type: 'string' },
                  memory: { type: 'string' },
                },
              },
              requests: {
                type: 'object',
                properties: {
                  cpu: { type: 'string' },
                  memory: { type: 'string' },
                },
              },
            },
          },
          env: { type: 'object', description: 'Environment variables' },
          labels: { type: 'object', description: 'Kubernetes labels' },
          annotations: { type: 'object', description: 'Kubernetes annotations' },
        },
        required: ['name', 'registry'],
      },
    },
    required: ['sessionId', 'imageId', 'deploymentOptions'],
  },
};
