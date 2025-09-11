import { Success, Failure, type Result, type Tool } from '@types';
import type { Logger } from 'pino';
import type { ProgressReporter } from '@mcp/context/types';

type WorkflowStep = {
  toolName: string;
  parameters: Record<string, unknown>;
  description?: string;
  required?: boolean;
  condition?: (previousResults: Record<string, unknown>[]) => boolean;
};

type WorkflowContext = {
  sessionId?: string;
  progressReporter?: ProgressReporter;
  signal?: AbortSignal;
  logger: Logger;
};

type WorkflowResult = {
  workflowType: string;
  completedSteps: string[];
  results: Record<string, unknown>[];
  sessionId?: string | undefined;
  recommendations: string[];
  executionTime: number;
  metadata?: Record<string, unknown>;
};

/**
 * Plans workflow steps based on type and session state.
 * Skips already completed steps when resuming workflows.
 *
 * @param workflowType - The type of workflow to plan ('containerization', 'deployment', 'security')
 * @param params - Workflow parameters including optional flags for build, scan, and push
 * @param sessionId - Optional session identifier for resuming workflows
 * @param sessionManager - Manager for tracking workflow session state
 * @returns Array of workflow steps to execute
 */
const planWorkflowSteps = async (
  workflowType: string,
  params: Record<string, unknown>,
  sessionId: string | undefined,
  sessionManager: { getState?: (sessionId: string) => Promise<{ completed_steps?: string[] }> },
): Promise<WorkflowStep[]> => {
  const sessionState =
    sessionId && sessionManager.getState ? await sessionManager.getState(sessionId) : undefined;

  if (workflowType === 'containerization') {
    const steps: WorkflowStep[] = [];

    // Always start with analyze-repo if not already completed
    if (!sessionState?.completed_steps?.includes('analyze-repo')) {
      steps.push({
        toolName: 'analyze-repo',
        parameters: { ...params, sessionId },
        description: 'Analyzing repository structure and dependencies',
        required: true,
      });
    }

    // Only add generate-dockerfile if not already completed
    // It depends on analyze-repo being completed first
    if (!sessionState?.completed_steps?.includes('generate-dockerfile')) {
      steps.push({
        toolName: 'generate-dockerfile',
        parameters: { ...params, sessionId },
        description: 'Generating optimized Dockerfile',
        required: true,
        // Only run if analyze-repo has been completed (either in session state or just executed)
        condition: (results) => {
          // Check if analyze-repo was just executed in this workflow run
          const analyzeRepoJustRan = results.some(
            (r) => '_toolName' in r && r._toolName === 'analyze-repo',
          );
          // Or if it was previously completed in the session
          const analyzeRepoCompleted = sessionState?.completed_steps?.includes('analyze-repo');
          return analyzeRepoJustRan || analyzeRepoCompleted || false;
        },
      });
    }

    if (params.buildImage !== false && !sessionState?.completed_steps?.includes('build-image')) {
      steps.push({
        toolName: 'build-image',
        parameters: { ...params, sessionId },
        description: 'Building Docker image',
        required: false,
      });
    }

    if (params.scanImage !== false && !sessionState?.completed_steps?.includes('scan')) {
      steps.push({
        toolName: 'scan',
        parameters: { ...params, sessionId },
        description: 'Scanning image for vulnerabilities',
        required: false,
        condition: (results) => {
          const lastResult = results[results.length - 1];
          return lastResult?.imageId !== undefined;
        },
      });
    }

    if (params.pushImage && params.registry && !sessionState?.completed_steps?.includes('push')) {
      steps.push({
        toolName: 'push',
        parameters: { ...params, sessionId },
        description: 'Pushing image to registry',
        required: false,
        condition: (results) => {
          const lastResult = results[results.length - 1];
          return lastResult?.imageId !== undefined;
        },
      });
    }

    return steps;
  }

  if (workflowType === 'deployment') {
    return [
      {
        toolName: 'generate-k8s-manifests',
        parameters: { ...params, sessionId },
        description: 'Generating Kubernetes manifests',
        required: true,
      },
      {
        toolName: 'prepare-cluster',
        parameters: { ...params, sessionId },
        description: 'Preparing cluster for deployment',
        required: true,
      },
      {
        toolName: 'deploy',
        parameters: { ...params, sessionId },
        description: 'Deploying application to cluster',
        required: true,
      },
      {
        toolName: 'verify-deployment',
        parameters: { ...params, sessionId },
        description: 'Verifying deployment health',
        required: false,
      },
    ];
  }

  if (workflowType === 'security') {
    return [
      {
        toolName: 'analyze-repo',
        parameters: { ...params, sessionId, detectSecrets: true },
        description: 'Analyzing repository for security issues',
        required: true,
      },
      {
        toolName: 'scan',
        parameters: { ...params, sessionId, severity: 'MEDIUM' },
        description: 'Scanning for vulnerabilities',
        required: true,
      },
      {
        toolName: 'fix-dockerfile',
        parameters: { ...params, sessionId },
        description: 'Fixing security issues in Dockerfile',
        required: false,
        condition: (results) =>
          results.some((r) => {
            const vulnerabilities = r.vulnerabilities;
            return Array.isArray(vulnerabilities) && vulnerabilities.length > 0;
          }),
      },
    ];
  }

  // Optimization workflow
  if (workflowType === 'optimization') {
    return [
      {
        toolName: 'analyze-repo',
        parameters: { ...params, sessionId },
        description: 'Analyzing repository',
        required: true,
      },
      {
        toolName: 'resolve-base-images',
        parameters: { ...params, sessionId },
        description: 'Resolving optimal base images',
        required: true,
      },
      {
        toolName: 'generate-dockerfile',
        parameters: { ...params, sessionId, optimizationLevel: 'aggressive' },
        description: 'Generating optimized Dockerfile',
        required: true,
      },
      {
        toolName: 'build-image',
        parameters: { ...params, sessionId },
        description: 'Building optimized image',
        required: false,
      },
    ];
  }

  return [];
};

const executeWorkflowStep = async (
  step: WorkflowStep,
  toolFactory: {
    getTool?: (toolName: string) => Tool;
    [key: string]: Tool | ((toolName: string) => Tool) | undefined;
  },
  context: WorkflowContext,
  previousResults: Record<string, unknown>[],
): Promise<Result<Record<string, unknown>>> => {
  const { logger } = context;

  // Check step condition
  if (step.condition && !step.condition(previousResults)) {
    logger.info({ step: step.toolName }, 'Skipping step due to condition');
    return Success({ skipped: true, reason: 'Condition not met' });
  }

  // Get tool from factory
  let tool: Tool | undefined;
  if (toolFactory.getTool) {
    tool = toolFactory.getTool(step.toolName);
  } else {
    const toolOrFunction = toolFactory[step.toolName];
    if (typeof toolOrFunction === 'function') {
      tool = toolOrFunction(step.toolName);
    } else {
      tool = toolOrFunction as Tool;
    }
  }

  if (!tool) {
    return Failure(`Tool not found: ${step.toolName}`);
  }

  // Execute with enhanced context if available
  const toolWithEnhanced = tool as Tool & {
    executeEnhanced?: (
      params: Record<string, unknown>,
      context: WorkflowContext,
    ) => Promise<Result<Record<string, unknown>>>;
  };
  if (toolWithEnhanced.executeEnhanced) {
    return toolWithEnhanced.executeEnhanced(step.parameters, context);
  }

  // Fallback to standard execution
  const result = await tool.execute(step.parameters, logger);
  return result as Result<Record<string, unknown>>;
};

const generateWorkflowRecommendations = (
  workflowType: string,
  results: Record<string, unknown>[],
): string[] => {
  const recommendations: string[] = [];

  // General recommendations
  recommendations.push('Review generated artifacts for accuracy');
  recommendations.push('Test container functionality before production deployment');

  // Workflow-specific recommendations
  if (workflowType === 'containerization') {
    if (
      results.some((r) => {
        const vulnerabilities = r.vulnerabilities;
        return Array.isArray(vulnerabilities) && vulnerabilities.length > 0;
      })
    ) {
      recommendations.push('Address security vulnerabilities before deployment');
    }
    recommendations.push('Configure CI/CD pipeline for automated builds');
  }

  if (workflowType === 'deployment') {
    recommendations.push('Monitor deployment health metrics');
    recommendations.push('Set up alerts for critical issues');
    recommendations.push('Configure autoscaling based on load patterns');
  }

  if (workflowType === 'security') {
    recommendations.push('Enable security scanning in CI/CD pipeline');
    recommendations.push('Regularly update base images and dependencies');
    recommendations.push('Implement runtime security monitoring');
  }

  if (workflowType === 'optimization') {
    const imageSizes = results
      .filter((r) => typeof r.imageSize === 'number')
      .map((r) => r.imageSize as number);
    if (imageSizes.length > 1) {
      const first = imageSizes[0];
      const last = imageSizes[imageSizes.length - 1];
      if (first && last && first > 0) {
        const reduction = ((first - last) / first) * 100;
        recommendations.push(`Image size reduced by ${reduction.toFixed(1)}% through optimization`);
      }
    }
    recommendations.push('Consider using distroless images for further size reduction');
  }

  return recommendations;
};

const updateSessionWithWorkflowProgress = async (
  sessionId: string,
  sessionManager: {
    updateWorkflowProgress?: (
      sessionId: string,
      progress: Record<string, unknown>,
    ) => Promise<void>;
    addCompletedStep?: (sessionId: string, stepName: string) => Promise<void>;
    storeStepResult?: (
      sessionId: string,
      stepName: string,
      result: Record<string, unknown>,
    ) => Promise<void>;
  },
  step: number,
  totalSteps: number,
  stepName: string,
  result: Record<string, unknown>,
): Promise<void> => {
  if (sessionManager.updateWorkflowProgress) {
    await sessionManager.updateWorkflowProgress(sessionId, {
      step,
      totalSteps,
      currentStep: stepName,
      result,
      timestamp: new Date().toISOString(),
    });
  }

  // Store completed step
  if (sessionManager.addCompletedStep) {
    await sessionManager.addCompletedStep(sessionId, stepName);
  }

  // Store result if significant
  if (result && !result.skipped && sessionManager.storeStepResult) {
    await sessionManager.storeStepResult(sessionId, stepName, result);
  }
};

/**
 * Execute a workflow with intelligent orchestration
 */
export const executeWorkflow = async (
  workflowType: string,
  params: Record<string, unknown>,
  context: WorkflowContext,
  toolFactory: {
    getTool?: (toolName: string) => Tool;
    [key: string]: Tool | ((toolName: string) => Tool) | undefined;
  },
  aiService?: {
    analyzeResults?: (params: Record<string, unknown>) => Promise<Result<{ nextSteps?: string[] }>>;
  },
  sessionManager?: {
    getState?: (sessionId: string) => Promise<{ completed_steps?: string[] }>;
    updateWorkflowProgress?: (
      sessionId: string,
      progress: Record<string, unknown>,
    ) => Promise<void>;
    addCompletedStep?: (sessionId: string, stepName: string) => Promise<void>;
    storeStepResult?: (
      sessionId: string,
      stepName: string,
      result: Record<string, unknown>,
    ) => Promise<void>;
  },
): Promise<Result<WorkflowResult>> => {
  const { sessionId, progressReporter, signal, logger } = context;
  const startTime = Date.now();

  try {
    // Validate workflow type
    const validWorkflows = ['containerization', 'deployment', 'security', 'optimization'];
    if (!validWorkflows.includes(workflowType)) {
      return Failure(
        `Invalid workflow type: ${workflowType}. Valid types: ${validWorkflows.join(', ')}`,
      );
    }

    // Plan workflow steps
    void progressReporter?.('Planning workflow steps...', 5);
    const steps = await planWorkflowSteps(workflowType, params, sessionId, sessionManager || {});

    if (steps.length === 0) {
      return Failure(`No steps planned for workflow: ${workflowType}`);
    }

    logger.info(
      {
        workflowType,
        stepCount: steps.length,
        steps: steps.map((s) => s.toolName),
      },
      'Workflow execution started',
    );

    const results: Record<string, unknown>[] = [];
    const completedSteps: string[] = [];

    // Execute each step
    for (let i = 0; i < steps.length; i++) {
      const step = steps[i];
      if (!step) {
        logger.warn(`Step ${i} is undefined, skipping`);
        continue;
      }
      const progressPercent = 10 + (i / steps.length) * 80;

      void progressReporter?.(step.description || `Executing ${step.toolName}...`, progressPercent);

      // Check for cancellation
      if (signal?.aborted) {
        logger.info({ workflowType, completedSteps }, 'Workflow cancelled by user');
        return Failure('Workflow cancelled by user');
      }

      // Execute step with context
      const stepResult = await executeWorkflowStep(
        step,
        toolFactory,
        {
          sessionId: sessionId || 'default',
          signal: signal || new AbortController().signal,
          logger: logger.child({ step: step.toolName }),
          progressReporter: async (message: string, progress?: number, total?: number) => {
            await progressReporter?.(
              message,
              progressPercent + ((progress || 0) * 0.8) / steps.length,
              total,
            );
          },
        } as WorkflowContext,
        results,
      );

      if (!stepResult.ok) {
        if (step.required) {
          // Required step failed - abort workflow
          const error = `Required step ${i + 1} (${step.toolName}) failed: ${stepResult.error}`;
          logger.error({ step: step.toolName, error: stepResult.error }, 'Workflow step failed');

          const suggestions = [
            'Review error message and fix the issue',
            'Check input parameters are correct',
            'Verify prerequisites are met',
            sessionId ? 'Review session state for conflicts' : null,
          ].filter(Boolean);

          return Failure(`${error}\n\nRecovery options:\n- ${suggestions.join('\n- ')}`);
        } else {
          // Optional step failed - log and continue
          logger.warn(
            { step: step.toolName, error: stepResult.error },
            'Optional step failed, continuing',
          );
          results.push({ error: stepResult.error, stepName: step.toolName });
        }
      } else {
        // Add the tool name to the result for condition checking
        const resultWithToolName = {
          ...stepResult.value,
          _toolName: step.toolName,
        };
        results.push(resultWithToolName);

        if (!stepResult.value.skipped) {
          completedSteps.push(step.toolName);
        }
      }

      // Update session state if available
      if (sessionId && sessionManager?.updateWorkflowProgress) {
        await updateSessionWithWorkflowProgress(
          sessionId,
          sessionManager,
          i + 1,
          steps.length,
          step.toolName,
          stepResult.ok ? stepResult.value : { error: stepResult.error },
        );
      }
    }

    void progressReporter?.('Finalizing workflow...', 95);

    // Get final session state for recommendations
    const finalSessionState =
      sessionId && sessionManager?.getState ? await sessionManager.getState(sessionId) : undefined;

    // Generate recommendations
    const recommendations = generateWorkflowRecommendations(workflowType, results);

    // Add AI insights if available
    if (aiService?.analyzeResults && sessionId) {
      const aiAnalysis = await aiService.analyzeResults({
        toolName: 'workflow',
        parameters: { workflowType, ...params },
        result: { completedSteps, results },
        sessionId,
        context: finalSessionState,
      });

      if (aiAnalysis.ok && aiAnalysis.value.nextSteps) {
        recommendations.push(...aiAnalysis.value.nextSteps);
      }
    }

    const executionTime = Date.now() - startTime;

    // Generate workflow summary
    const summary: WorkflowResult = {
      workflowType,
      completedSteps,
      results,
      sessionId: sessionId ?? undefined,
      recommendations: [...new Set(recommendations)], // Remove duplicates
      executionTime,
      metadata: {
        totalSteps: steps.length,
        successfulSteps: completedSteps.length,
        skippedSteps: results.filter((r) => Boolean(r.skipped)).length,
        failedSteps: results.filter((r) => Boolean(r.error)).length,
        aiEnhanced: !!aiService,
        sessionTracked: !!sessionId,
      },
    };

    void progressReporter?.('Workflow complete', 100);

    logger.info(
      {
        workflowType,
        executionTime,
        completedSteps: completedSteps.length,
        totalSteps: steps.length,
      },
      'Workflow execution completed',
    );

    return Success(summary);
  } catch (error: unknown) {
    logger.error({ error, workflowType }, 'Workflow execution failed');
    const message = error instanceof Error ? error.message : String(error);
    return Failure(`Workflow execution failed: ${message}`);
  }
};

// Export types
export type { WorkflowStep, WorkflowContext, WorkflowResult };
