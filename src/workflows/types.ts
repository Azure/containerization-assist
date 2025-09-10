/**
 * Workflow type definitions for containerization pipelines
 *
 * Provides standardized interfaces for multi-step containerization workflows
 * including progress tracking, error handling, and result reporting.
 */

import type { Logger } from 'pino';

/**
 * Base parameters required by all workflow implementations
 *
 * Common configuration shared across different workflow types.
 */
export interface BaseWorkflowParams {
  sessionId: string;
  repoPath: string;
  logger: Logger;
}

/**
 * Standard result format for workflow execution
 *
 * Provides consistent success/failure reporting with optional data and error details.
 * All workflows should return this interface for predictable handling.
 */
export interface WorkflowResult {
  success: boolean;
  data?: unknown;
  error?: string;
}

/**
 * Individual step within a workflow execution
 *
 * Tracks the lifecycle and results of each stage in a multi-step workflow.
 * Used for progress reporting and debugging failed executions.
 */
export interface WorkflowStep {
  name: string;
  status: 'pending' | 'running' | 'completed' | 'failed' | 'skipped';
  startTime?: Date;
  endTime?: Date;
  error?: string;
  output?: unknown;
}

/**
 * Execution context shared across all steps in a workflow
 *
 * Maintains state, artifacts, and metadata throughout workflow execution.
 * Provides a communication channel between workflow steps.
 */
export interface WorkflowContext {
  sessionId: string;
  steps: WorkflowStep[];
  artifacts: Map<string, unknown>;
  metadata: {
    startTime: Date;
    [key: string]: unknown;
  };
  currentStep?: string;
}

/**
 * Parameters for containerization workflow execution
 *
 * Defines the configuration and options for the complete containerization pipeline
 * from repository analysis through image building and tagging.
 */
export interface ContainerizationWorkflowParams {
  sessionId: string;
  projectPath: string;
  moduleRoots?: string[];
  language?: 'java' | 'dotnet' | 'other';
  buildOptions?: {
    dockerfilePath?: string;
    contextPath?: string;
    buildArgs?: Record<string, string>;
    target?: string;
    platform?: string;
    tags?: string[];
    noCache?: boolean;
  };
  scanOptions?: {
    severity?: 'low' | 'medium' | 'high' | 'critical';
    ignoreUnfixed?: boolean;
  };
}

export interface ContainerizationWorkflowResult {
  success: boolean;
  sessionId: string;
  error?: string;
  data?: {
    imageId?: string;
    imageTags?: string[];
    dockerfilePath?: string;
    scanResults?: {
      vulnerabilities: unknown[];
      summary: unknown;
    };
    analysisData: {
      language: string;
    };
  };
  metadata: {
    startTime: Date;
    endTime: Date;
    duration: number;
    steps: WorkflowStep[];
  };
}

// Deployment workflow types
export interface DeploymentWorkflowParams {
  sessionId: string;
  imageId: string;
  clusterConfig: {
    namespace?: string;
    context?: string;
    kubeconfig?: string;
  };
  deploymentOptions: {
    name: string;
    replicas?: number;
    port?: number;
    serviceType?: 'ClusterIP' | 'NodePort' | 'LoadBalancer';
    registry?: string;
    env?: Record<string, string>;
    resources?: {
      requests?: { cpu?: string; memory?: string };
      limits?: { cpu?: string; memory?: string };
    };
  };
}

export interface DeploymentWorkflowResult {
  success: boolean;
  sessionId: string;
  error?: string;
  results?: {
    deploymentName: string;
    namespace: string;
    endpoints?: string[];
    service: {
      name: string;
      type: string;
    };
    pods: Array<{
      name: string;
      ready: boolean;
      status: string;
      restarts: number;
    }>;
    verificationStatus: {
      deployment: boolean;
      service: boolean;
      endpoints: boolean;
      health: boolean;
    };
  };
  metadata: {
    startTime: Date;
    endTime: Date;
    duration: number;
    steps: WorkflowStep[];
  };
}
