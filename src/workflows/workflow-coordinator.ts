/**
 * Workflow Coordinator
 *
 * This module serves as the workflow coordinator for orchestrating various
 * containerization workflows and intelligent workflow execution.
 */

import { Result, type Tool } from '@types';
import type { ToolContext } from '@mcp/context/types';
import type { ContainerizationWorkflowParams as ContainerizationWorkflowConfig } from '@workflows/types';
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
