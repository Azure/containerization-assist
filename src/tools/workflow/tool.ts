/**
 * Workflow Tool - Standardized Implementation
 *
 * Orchestrates containerization workflows using standardized helpers
 * for consistency and improved error handling
 *
 * @example
 * ```typescript
 * const result = await workflow({
 *   sessionId: 'session-123', // optional
 *   workflow: 'containerization',
 *   options: { skipSecurity: false, deploy: true }
 * }, context, logger);
 *
 * if (result.ok) {
 *   console.log('Workflow:', result.workflowName);
 *   console.log('Status:', result.status);
 * }
 * ```
 */

import { ensureSession, defineToolIO, useSessionSlice } from '@mcp/tool-session-helpers';
import { extractErrorMessage } from '../../lib/error-utils';
import { createStandardProgress } from '@mcp/progress-helper';
import type { ToolContext } from '../../mcp/context';
import { createTimer, createLogger, type Logger } from '../../lib/logger';
import { Success, Failure, type Result, type Tool } from '../../types';

// Import tool registry at the module level
import { analyzeRepo } from '@tools/analyze-repo';
import { generateDockerfile } from '@tools/generate-dockerfile';
import { buildImage } from '@tools/build-image';
import { scanImage } from '@tools/scan';
import { pushImage } from '@tools/push-image';
import { tagImage } from '@tools/tag-image';
import { fixDockerfile } from '@tools/fix-dockerfile';
import { resolveBaseImages } from '@tools/resolve-base-images';
import { prepareCluster } from '@tools/prepare-cluster';
import { deployApplication } from '@tools/deploy';
import { generateK8sManifests } from '@tools/generate-k8s-manifests';
import { verifyDeployment } from '@tools/verify-deployment';
import { workflowSchema, type WorkflowParams } from './schema';
import { z } from 'zod';
import type { SessionData } from '../session-types';

// Export specific workflow tool result
export interface WorkflowToolResult {
  ok: boolean;
  sessionId: string;
  workflowId: string;
  status: 'started' | 'completed' | 'failed' | 'already_running';
  message: string;
  workflowName: string;
  estimatedDuration?: number;
  steps: string[];
  completedSteps: string[];
  failedSteps?: string[];
  nextSteps?: string[];
  metadata?: {
    workflowType: string;
    automated: boolean;
    options?: Record<string, unknown>;
    startedAt: string;
    completedAt?: string;
    duration?: number;
  };
}

export interface WorkflowStatusResult {
  status: string;
  workflowId?: string;
  currentStep?: string;
  completedSteps: string[];
  failedSteps: string[];
  progress: number;
}

// Define the result schema for type safety
const WorkflowToolResultSchema = z.object({
  ok: z.boolean(),
  sessionId: z.string(),
  workflowId: z.string(),
  status: z.enum(['started', 'completed', 'failed', 'already_running']),
  message: z.string(),
  workflowName: z.string(),
  estimatedDuration: z.number().optional(),
  steps: z.array(z.string()),
  completedSteps: z.array(z.string()),
  failedSteps: z.array(z.string()).optional(),
  nextSteps: z.array(z.string()).optional(),
  metadata: z
    .object({
      workflowType: z.string(),
      automated: z.boolean(),
      options: z.record(z.unknown()).optional(),
      startedAt: z.string(),
      completedAt: z.string().optional(),
      duration: z.number().optional(),
    })
    .optional(),
});

// Define tool IO for type-safe session operations
const io = defineToolIO(workflowSchema, WorkflowToolResultSchema);

// Tool-specific state schema for comprehensive workflow tracking
const StateSchema = z.object({
  lastExecutedAt: z.date().optional(),
  lastWorkflowType: z.string().optional(),
  lastWorkflowId: z.string().optional(),
  totalWorkflowsExecuted: z.number().optional(),
  lastWorkflowStatus: z.enum(['started', 'completed', 'failed', 'already_running']).optional(),
  lastStepsCompleted: z.number().optional(),
  lastStepsFailed: z.number().optional(),
  lastDuration: z.number().optional(),
  lastCurrentStep: z.string().optional(),
  averageStepDuration: z.number().optional(),
});

/**
 * Get workflow steps based on type
 */
function getWorkflowSteps(workflowType: string, options?: Record<string, unknown>): string[] {
  const baseSteps = {
    full: [
      'analyze-repo',
      'resolve-base-images',
      'generate-dockerfile',
      'build-image',
      'scan-image',
      'tag-image',
      'push-image',
      'generate-k8s-manifests',
      'prepare-cluster',
      'deploy',
      'verify-deployment',
    ],
    'build-only': [
      'analyze-repo',
      'resolve-base-images',
      'generate-dockerfile',
      'build-image',
      'scan-image',
      'tag-image',
    ],
    'deploy-only': ['generate-k8s-manifests', 'prepare-cluster', 'deploy', 'verify-deployment'],
    quick: ['analyze-repo', 'generate-dockerfile', 'build-image'],
    containerization: [
      'analyze-repo',
      'resolve-base-images',
      'generate-dockerfile',
      'build-image',
      'scan-image',
      'tag-image',
      'push-image',
      'generate-k8s-manifests',
      'deploy',
      'verify-deployment',
    ],
  };

  let steps = baseSteps[workflowType as keyof typeof baseSteps] ?? baseSteps.full;

  // Filter steps based on options
  if (options?.skipTests) {
    steps = steps.filter((s) => !s.includes('test'));
  }
  if (options?.skipSecurity) {
    steps = steps.filter((s) => s !== 'scan-image');
  }
  if (!options?.deploy && workflowType === 'full') {
    steps = steps.filter((s) => !['deploy', 'verify-deployment'].includes(s));
  }

  return steps;
}

/**
 * Estimate workflow duration in seconds
 */
function estimateWorkflowDuration(steps: string[]): number {
  const stepDurations: Record<string, number> = {
    'analyze-repo': 5,
    'resolve-base-images': 3,
    'generate-dockerfile': 3,
    'build-image': 30,
    'scan-image': 15,
    'tag-image': 2,
    'push-image': 10,
    'generate-k8s-manifests': 5,
    'prepare-cluster': 10,
    deploy: 20,
    'verify-deployment': 10,
  };

  return steps.reduce((total, step) => total + (stepDurations[step] ?? 5), 0);
}

// Type guard to safely cast params
const castParams = <T>(params: Record<string, unknown>): T => {
  return params as T;
};

// Create tool mapping with wrapped tools (which are functions directly)
const toolMap: Record<string, Tool> = {
  'analyze-repo': {
    name: 'analyze-repo',
    execute: (params: Record<string, unknown>, _logger: Logger, context?: unknown) =>
      analyzeRepo(castParams(params), context as ToolContext),
  },
  'generate-dockerfile': {
    name: 'generate-dockerfile',
    execute: (params: Record<string, unknown>, _logger: Logger, context?: unknown) =>
      generateDockerfile(castParams(params), context as ToolContext),
  },
  'build-image': {
    name: 'build-image',
    execute: (params: Record<string, unknown>, _logger: Logger, context?: unknown) =>
      buildImage(castParams(params), context as ToolContext),
  },
  'scan-image': {
    name: 'scan-image',
    execute: (params: Record<string, unknown>, _logger: Logger, context?: unknown) =>
      scanImage(castParams(params), context as ToolContext),
  },
  'push-image': {
    name: 'push-image',
    execute: (params: Record<string, unknown>, _logger: Logger, context?: unknown) =>
      pushImage(castParams(params), context as ToolContext),
  },
  'tag-image': {
    name: 'tag-image',
    execute: (params: Record<string, unknown>, _logger: Logger, context?: unknown) =>
      tagImage(castParams(params), context as ToolContext),
  },
  'fix-dockerfile': {
    name: 'fix-dockerfile',
    execute: (params: Record<string, unknown>, _logger: Logger, context?: unknown) =>
      fixDockerfile(castParams(params), context as ToolContext),
  },
  'resolve-base-images': {
    name: 'resolve-base-images',
    execute: (params: Record<string, unknown>, _logger: Logger, context?: unknown) =>
      resolveBaseImages(castParams(params), context as ToolContext),
  },
  'prepare-cluster': {
    name: 'prepare-cluster',
    execute: (params: Record<string, unknown>, _logger: Logger, context?: unknown) =>
      prepareCluster(castParams(params), context as ToolContext),
  },
  deploy: {
    name: 'deploy',
    execute: (params: Record<string, unknown>, _logger: Logger, context?: unknown) =>
      deployApplication(castParams(params), context as ToolContext),
  },
  'generate-k8s-manifests': {
    name: 'generate-k8s-manifests',
    execute: (params: Record<string, unknown>, _logger: Logger, context?: unknown) =>
      generateK8sManifests(castParams(params), context as ToolContext),
  },
  'verify-deployment': {
    name: 'verify-deployment',
    execute: (params: Record<string, unknown>, _logger: Logger, context?: unknown) =>
      verifyDeployment(castParams(params), context as ToolContext),
  },
};

/**
 * Execute a single workflow step
 */
export async function executeStep(
  step: string,
  sessionId: string,
  config: { workflowType: string; options?: Record<string, unknown> },
  logger: Logger,
  context: ToolContext,
): Promise<{ ok: boolean; error?: string }> {
  const timer = createTimer(logger, `workflow-step-${step}`);

  try {
    logger.info({ step, sessionId }, `Executing workflow step: ${step}`);

    // Get the tool from the map
    const tool = toolMap[step];

    if (!tool) {
      throw new Error(`Tool not found: ${step}`);
    }

    // Prepare tool configuration based on the step
    const toolConfig: Record<string, unknown> = {
      sessionId,
    };

    // Add step-specific configuration
    switch (step) {
      case 'scan-image':
        toolConfig.skipIfNoScanner = config.options?.skipSecurity;
        break;
      case 'push-image':
        if (config.options?.registryUrl) {
          toolConfig.registryUrl = config.options.registryUrl;
        }
        break;
      case 'tag-image':
        if (config.options?.imageTag) {
          toolConfig.tag = config.options.imageTag;
        }
        break;
      case 'deploy':
        if (config.options?.namespace) {
          toolConfig.namespace = config.options.namespace;
        }
        break;
      case 'generate-k8s-manifests':
        if (config.options?.namespace) {
          toolConfig.namespace = config.options.namespace;
        }
        break;
    }

    // Execute the actual tool
    const result = await tool.execute(toolConfig, logger, context);

    // Handle Result<T> pattern
    if (result && typeof result === 'object' && 'ok' in result) {
      if (result.ok) {
        timer.end({ step, success: true });
        return { ok: true };
      } else {
        timer.end({ step, success: false, error: result.error });
        return { ok: false, error: result.error };
      }
    }

    // Handle direct response (backward compatibility)
    timer.end({ step, success: true });
    return { ok: true };
  } catch (error) {
    timer.error(error);
    logger.error({ step, error }, `Workflow step failed: ${step}`);
    return {
      ok: false,
      error: extractErrorMessage(error),
    };
  }
}

/**
 * Workflow implementation with selective progress reporting
 */
async function workflowImpl(
  params: WorkflowParams,
  context: ToolContext,
): Promise<Result<WorkflowToolResult>> {
  // Basic parameter validation
  if (!params || typeof params !== 'object') {
    return Failure('Invalid parameters provided');
  }

  // Progress reporting for complex workflow orchestration
  const progress = context.progress ? createStandardProgress(context.progress) : undefined;
  const logger = context.logger || createLogger({ name: 'workflow' });
  const timer = createTimer(logger, 'workflow');
  const startedAt = new Date().toISOString();

  try {
    const { workflow: workflowType = 'full', options = {} } = params;
    const automated = Boolean(options.automated);

    logger.info({ workflowType, options }, 'Starting workflow execution');

    // Progress: Workflow orchestration started
    if (progress) await progress('EXECUTING');

    // Ensure session exists and get typed slice operations
    const sessionResult = await ensureSession(context, params.sessionId);
    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }

    const { id: sessionId, state: session } = sessionResult.value;
    const slice = useSessionSlice('workflow', io, context, StateSchema);

    if (!slice) {
      return Failure('Session manager not available');
    }

    logger.info(
      { sessionId, workflowType, automated },
      'Starting containerization workflow with session',
    );

    // Record input in session slice
    await slice.patch(sessionId, { input: params });

    // Check if workflow is already running
    const sessionData = session as SessionData;
    const workflowState = sessionData?.workflow_state;
    if (workflowState?.status === 'running') {
      return Success({
        ok: false,
        sessionId,
        workflowId: (workflowState.workflowId as string) ?? `workflow-${sessionId}`,
        status: 'already_running',
        message: 'A workflow is already running for this session',
        workflowName: `${workflowType} workflow`,
        steps: (workflowState.steps as string[]) ?? [],
        completedSteps: (workflowState.completedSteps as string[]) ?? [],
      });
    }

    // Generate workflow ID
    const workflowId = `workflow-${sessionId}-${Date.now()}`;

    // Get workflow steps
    const steps = getWorkflowSteps(workflowType, options);
    const estimatedDuration = estimateWorkflowDuration(steps);

    logger.info(
      {
        sessionId,
        workflowId,
        workflowType,
        stepCount: steps.length,
      },
      'Starting workflow',
    );

    // Update session metadata for backward compatibility - initial workflow state
    const sessionManager = context.sessionManager;
    if (sessionManager) {
      try {
        await sessionManager.update(sessionId, {
          metadata: {
            ...session.metadata,
            workflow_state: {
              status: 'running',
              workflowId,
              workflowType,
              steps,
              completedSteps: [],
              currentStep: steps[0],
              startedAt,
            },
          },
        });
      } catch (error) {
        logger.warn(
          { error: (error as Error).message },
          'Failed to update session with workflow start',
        );
      }
    }

    // Execute workflow steps
    const completedSteps: string[] = [];
    const failedSteps: string[] = [];
    let workflowFailed = false;

    for (const step of steps) {
      // Update current step in session metadata for backward compatibility
      if (sessionManager) {
        try {
          await sessionManager.update(sessionId, {
            metadata: {
              ...session.metadata,
              workflow_state: {
                status: 'running',
                workflowId,
                workflowType,
                steps,
                currentStep: step,
                completedSteps,
                failedSteps,
                startedAt,
              },
            },
          });
        } catch (error) {
          logger.warn(
            { error: (error as Error).message, step },
            'Failed to update session for step',
          );
        }
      }

      // Execute step
      const result = await executeStep(step, sessionId, { workflowType, options }, logger, context);

      if (result.ok) {
        completedSteps.push(step);
      } else {
        failedSteps.push(step);

        // Check if we should continue or abort
        if (!options.autoRollback && !automated) {
          workflowFailed = true;
          logger.error({ step, error: result.error }, 'Workflow aborted due to step failure');
          break;
        }
      }
    }

    const completedAt = new Date().toISOString();
    const duration = Date.now() - new Date(startedAt).getTime();

    // Prepare the final result
    const finalStatus = workflowFailed ? 'failed' : 'completed';
    const result: WorkflowToolResult = {
      ok: finalStatus === 'completed',
      sessionId,
      workflowId,
      status: finalStatus,
      message:
        finalStatus === 'completed'
          ? `Workflow completed successfully with ${completedSteps.length}/${steps.length} steps`
          : `Workflow failed after ${completedSteps.length}/${steps.length} steps`,
      workflowName: `${workflowType} workflow`,
      estimatedDuration,
      steps,
      completedSteps,
      ...(failedSteps.length > 0 && { failedSteps }),
      metadata: {
        workflowType,
        automated,
        options,
        startedAt,
        completedAt,
        duration,
      },
    };

    // Update typed session slice with final output and comprehensive state
    await slice.patch(sessionId, {
      output: result,
      state: {
        lastExecutedAt: new Date(),
        lastWorkflowType: workflowType,
        lastWorkflowId: workflowId,
        totalWorkflowsExecuted:
          (sessionData?.completed_steps || []).filter((s) => s === 'workflow').length + 1,
        lastWorkflowStatus: finalStatus,
        lastStepsCompleted: completedSteps.length,
        lastStepsFailed: failedSteps.length,
        lastDuration: duration,
        lastCurrentStep:
          finalStatus === 'completed' ? undefined : completedSteps[completedSteps.length - 1],
        averageStepDuration: steps.length > 0 ? duration / steps.length : undefined,
      },
    });

    // Update session metadata for backward compatibility - final state
    if (sessionManager) {
      try {
        await sessionManager.update(sessionId, {
          metadata: {
            ...session.metadata,
            workflow_state: {
              status: finalStatus,
              workflowId,
              workflowType,
              steps,
              completedSteps,
              failedSteps,
              currentStep: null,
              startedAt,
              completedAt,
              duration,
            },
          },
          completed_steps: [...(session.completed_steps || []), 'workflow'],
        });
      } catch (error) {
        logger.warn(
          { error: (error as Error).message },
          'Failed to update session with final workflow state',
        );
      }
    }

    timer.end({
      workflowId,
      status: finalStatus,
      completedSteps: completedSteps.length,
      failedSteps: failedSteps.length,
    });

    logger.info(
      {
        workflowId,
        status: finalStatus,
        completedSteps: completedSteps.length,
        failedSteps: failedSteps.length,
        duration,
      },
      'Workflow completed',
    );

    return Success({
      ...result,
      nextSteps: steps.filter((s) => !completedSteps.includes(s)),
    });
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'Workflow execution failed');

    return Failure(extractErrorMessage(error));
  }
}

/**
 * Wrapped workflow tool with standardized behavior
 */
/**
 * Workflow tool with selective progress reporting
 */
export const workflow = workflowImpl;
