/**
 * Workflow Coordinator
 *
 * This module serves as the workflow coordinator for orchestrating various
 * containerization workflows and intelligent workflow execution.
 */

import { Result, Success, type Tool } from '@types';
import { runContainerizationWorkflow } from '@workflows/containerization';
import {
  runBuildOnlyWorkflow,
  type ContainerizationConfig,
} from '@workflows/containerization-workflow';
import type { ToolContext } from '@mcp/context/types';
import type {
  ContainerizationWorkflowParams as ContainerizationWorkflowConfig,
  ContainerizationWorkflowResult as ContainerizationResult,
} from '@workflows/types';
import {
  executeWorkflow as executeIntelligentWorkflow,
  type WorkflowContext,
  type WorkflowResult,
} from '@workflows/intelligent-orchestration';

interface EnhancedWorkflowConfig extends Omit<ContainerizationWorkflowConfig, 'sessionId'> {
  toolFactory?: {
    getTool?: (toolName: string) => Tool;
    [key: string]: Tool | ((toolName: string) => Tool) | undefined;
  };
  aiService?: Record<string, unknown>;
  sessionManager?: Record<string, unknown>;
  sessionId?: string;
}

/**
 * Execute containerization workflow for a repository
 * @param repositoryPath - Path to the repository to containerize
 * @param logger - Logger instance for workflow execution
 * @param config - Optional workflow configuration
 * @returns Promise resolving to workflow execution result
 */
export const executeBasicContainerizationWorkflow = async (
  repositoryPath: string,
  context: ToolContext,
  config?: Partial<ContainerizationWorkflowConfig>,
): Promise<Result<ContainerizationResult>> => {
  const params: ContainerizationWorkflowConfig = {
    sessionId: config?.sessionId || `workflow-${Date.now()}`,
    projectPath: repositoryPath,
    ...(config?.buildOptions && { buildOptions: config.buildOptions }),
    ...(config?.scanOptions && { scanOptions: config.scanOptions }),
  };
  return runContainerizationWorkflow(params, context) as unknown as Promise<
    Result<ContainerizationResult>
  >;
};

/**
 * Execute build-only workflow for a repository
 * @param repositoryPath - Path to the repository to build
 * @param logger - Logger instance for workflow execution
 * @param config - Optional workflow configuration
 * @returns Promise resolving to build workflow execution result
 */
export const executeBuildWorkflow = async (
  repositoryPath: string,
  context: ToolContext,
  config?: Partial<ContainerizationWorkflowConfig>,
): Promise<Result<ContainerizationResult>> => {
  const result = await runBuildOnlyWorkflow(
    repositoryPath,
    context,
    config as ContainerizationConfig,
  );
  if (!result.ok) {
    return result as Result<ContainerizationResult>;
  }

  // Convert the build-only result to ContainerizationResult format
  return Success({
    success: true,
    sessionId: config?.sessionId || `build-${Date.now()}`,
    data: {
      imageId: result.value.imageId,
      analysisData: {
        language: 'unknown',
      },
    },
    metadata: {
      startTime: new Date(Date.now() - result.value.duration),
      endTime: new Date(),
      duration: result.value.duration,
      steps: [],
    },
  } as ContainerizationResult);
};

/**
 * Execute enhanced workflow using intelligent orchestration
 * @param repositoryPath - Path to the repository
 * @param workflowType - Type of workflow to execute (e.g., 'deployment', 'security')
 * @param context - Tool context for workflow execution
 * @param config - Optional workflow configuration with AI service and session management
 * @returns Promise resolving to enhanced workflow execution result
 */
export const executeWorkflow = async (
  repositoryPath: string,
  workflowType: string,
  context: ToolContext,
  config?: Partial<EnhancedWorkflowConfig>,
): Promise<Result<WorkflowResult>> => {
  const workflowContext: WorkflowContext = {
    ...(config?.sessionId ? { sessionId: config.sessionId } : {}),
    logger: context.logger,
  };

  const params = {
    repoPath: repositoryPath,
    ...config,
  };

  return executeIntelligentWorkflow(
    workflowType,
    params,
    workflowContext,
    (config?.toolFactory ?? {}) as {
      getTool?: (toolName: string) => Tool;
      [key: string]: Tool | ((toolName: string) => Tool) | undefined;
    },
  );
};
