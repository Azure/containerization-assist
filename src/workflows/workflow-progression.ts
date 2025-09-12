/**
 * Centralized Workflow Progression
 *
 * Dynamically determines next steps based on current tool, success/failure,
 * session context, and workflow configuration rather than hardcoded hints.
 */

import { TOOL_NAMES } from '../exports/tool-names.js';

export interface SessionContext {
  completed_steps?: string[];
  dockerfile_result?: { content?: string };
  analysis_result?: { language?: string };
}

export interface WorkflowStep {
  tool: string;
  params?: Record<string, unknown>;
  description: string;
  condition?: (sessionContext: SessionContext) => boolean;
}

export interface WorkflowProgression {
  nextSteps: WorkflowStep[];
  summary: string;
}

/**
 * Individual tool sequences - simple linear progression
 */
const INDIVIDUAL_TOOL_SEQUENCES = {
  basic: [
    TOOL_NAMES.ANALYZE_REPO,
    TOOL_NAMES.RESOLVE_BASE_IMAGES,
    TOOL_NAMES.GENERATE_DOCKERFILE,
    TOOL_NAMES.BUILD_IMAGE,
    TOOL_NAMES.SCAN_IMAGE,
    TOOL_NAMES.TAG_IMAGE,
    TOOL_NAMES.PUSH_IMAGE,
    TOOL_NAMES.GENERATE_K8S_MANIFESTS,
    TOOL_NAMES.PREPARE_CLUSTER,
    TOOL_NAMES.DEPLOY_APPLICATION,
  ],
} as const;

const hasNotCompleted = (tool: string) => (ctx: SessionContext) =>
  !ctx.completed_steps?.includes(tool);

const hasCompleted = (tool: string) => (ctx: SessionContext) => ctx.completed_steps?.includes(tool);

/**
 * Recovery workflows for failures
 */
const RECOVERY_WORKFLOWS = {
  // Analysis failures - retry with different parameters or fallback to manual input
  [TOOL_NAMES.ANALYZE_REPO]: [
    { tool: TOOL_NAMES.GENERATE_DOCKERFILE, condition: () => true }, // Skip to generation with basic defaults
  ],

  // Base image resolution failures - fallback to generation with defaults
  [TOOL_NAMES.RESOLVE_BASE_IMAGES]: [
    { tool: TOOL_NAMES.ANALYZE_REPO, condition: hasNotCompleted(TOOL_NAMES.ANALYZE_REPO) },
    { tool: TOOL_NAMES.GENERATE_DOCKERFILE, condition: () => true }, // Continue with default base images
  ],

  // Dockerfile generation failures - try analysis first, then fix approach
  [TOOL_NAMES.GENERATE_DOCKERFILE]: [
    { tool: TOOL_NAMES.ANALYZE_REPO, condition: hasNotCompleted(TOOL_NAMES.ANALYZE_REPO) },
    {
      tool: TOOL_NAMES.RESOLVE_BASE_IMAGES,
      condition: hasNotCompleted(TOOL_NAMES.RESOLVE_BASE_IMAGES),
    },
    { tool: TOOL_NAMES.FIX_DOCKERFILE, condition: () => true },
  ],

  // Dockerfile fix failures - go back to analysis or try regeneration
  [TOOL_NAMES.FIX_DOCKERFILE]: [
    { tool: TOOL_NAMES.ANALYZE_REPO, condition: hasNotCompleted(TOOL_NAMES.ANALYZE_REPO) },
    { tool: TOOL_NAMES.GENERATE_DOCKERFILE, condition: () => true },
  ],

  // Build failures - most complex recovery workflow
  [TOOL_NAMES.BUILD_IMAGE]: [
    { tool: TOOL_NAMES.FIX_DOCKERFILE, condition: hasNotCompleted(TOOL_NAMES.FIX_DOCKERFILE) },
    { tool: TOOL_NAMES.GENERATE_DOCKERFILE, condition: hasCompleted(TOOL_NAMES.FIX_DOCKERFILE) },
    {
      tool: TOOL_NAMES.RESOLVE_BASE_IMAGES,
      condition: hasNotCompleted(TOOL_NAMES.RESOLVE_BASE_IMAGES),
    },
    { tool: TOOL_NAMES.ANALYZE_REPO, condition: hasNotCompleted(TOOL_NAMES.ANALYZE_REPO) },
  ],

  // Scan failures - continue workflow but note security concerns
  [TOOL_NAMES.SCAN_IMAGE]: [
    { tool: TOOL_NAMES.TAG_IMAGE, condition: () => true }, // Continue even if scan fails
  ],

  // Tag failures - rebuild image or continue with existing tags
  [TOOL_NAMES.TAG_IMAGE]: [
    { tool: TOOL_NAMES.BUILD_IMAGE, condition: () => true }, // Rebuild if tagging fails
  ],

  // Push failures - retry tagging or check registry connectivity
  [TOOL_NAMES.PUSH_IMAGE]: [
    { tool: TOOL_NAMES.TAG_IMAGE, condition: () => true }, // Retry with proper tagging
  ],

  // K8s manifest generation failures - ensure prerequisites are met
  [TOOL_NAMES.GENERATE_K8S_MANIFESTS]: [
    { tool: TOOL_NAMES.ANALYZE_REPO, condition: hasNotCompleted(TOOL_NAMES.ANALYZE_REPO) },
    { tool: TOOL_NAMES.TAG_IMAGE, condition: hasNotCompleted(TOOL_NAMES.TAG_IMAGE) },
    { tool: TOOL_NAMES.BUILD_IMAGE, condition: hasNotCompleted(TOOL_NAMES.BUILD_IMAGE) },
  ],

  // Cluster preparation failures - retry or suggest manual setup
  [TOOL_NAMES.PREPARE_CLUSTER]: [
    {
      tool: TOOL_NAMES.GENERATE_K8S_MANIFESTS,
      condition: hasCompleted(TOOL_NAMES.GENERATE_K8S_MANIFESTS),
    }, // Continue with manifests if ready
  ],

  // Deployment failures - check cluster and manifests
  [TOOL_NAMES.DEPLOY_APPLICATION]: [
    { tool: TOOL_NAMES.PREPARE_CLUSTER, condition: hasNotCompleted(TOOL_NAMES.PREPARE_CLUSTER) },
    {
      tool: TOOL_NAMES.GENERATE_K8S_MANIFESTS,
      condition: hasNotCompleted(TOOL_NAMES.GENERATE_K8S_MANIFESTS),
    },
    { tool: TOOL_NAMES.PUSH_IMAGE, condition: hasNotCompleted(TOOL_NAMES.PUSH_IMAGE) },
  ],

  // Deployment verification failures - check deployment status
  [TOOL_NAMES.VERIFY_DEPLOYMENT]: [
    {
      tool: TOOL_NAMES.DEPLOY_APPLICATION,
      condition: hasNotCompleted(TOOL_NAMES.DEPLOY_APPLICATION),
    },
    { tool: TOOL_NAMES.PREPARE_CLUSTER, condition: hasNotCompleted(TOOL_NAMES.PREPARE_CLUSTER) },
  ],
} as const;

/**
 * Determine next steps on successful tool completion
 */
export function getSuccessProgression(
  completedTool: string,
  sessionContext: SessionContext,
  workflowType: string = 'basic',
): WorkflowProgression {
  const sequence =
    INDIVIDUAL_TOOL_SEQUENCES[workflowType as keyof typeof INDIVIDUAL_TOOL_SEQUENCES] ||
    INDIVIDUAL_TOOL_SEQUENCES.basic;

  const currentIndex = sequence.findIndex((tool) => tool === completedTool);
  const completedSteps = sessionContext.completed_steps || [];

  if (currentIndex === -1) {
    return {
      nextSteps: [],
      summary: 'Tool not in standard workflow sequence',
    };
  }

  // Find the next uncompleted tool in the sequence
  const nextTool = sequence.slice(currentIndex + 1).find((tool) => !completedSteps.includes(tool));

  if (!nextTool) {
    return {
      nextSteps: [],
      summary: `${completedTool} tool execution completed. Workflow finished`,
    };
  }

  return {
    nextSteps: [
      {
        tool: nextTool,
        description: getToolDescription(nextTool),
      },
    ],
    summary: `${completedTool} tool execution completed successfully. Continue with calling ${nextTool} tool.`,
  };
}

/**
 * Determine recovery steps on tool failure
 */
export function getFailureProgression(
  failedTool: string,
  _errorMessage: string,
  sessionContext: SessionContext,
): WorkflowProgression {
  const recoveryOptions = RECOVERY_WORKFLOWS[failedTool as keyof typeof RECOVERY_WORKFLOWS];

  if (!recoveryOptions) {
    return {
      nextSteps: [
        {
          tool: TOOL_NAMES.ANALYZE_REPO,
          description: 'Re-analyze project to understand the issue',
        },
      ],
      summary: `${failedTool} failed. Fallback to analysis`,
    };
  }

  for (const option of recoveryOptions) {
    if (!option.condition || option.condition(sessionContext)) {
      return {
        nextSteps: [
          {
            tool: option.tool,
            description: getToolDescription(option.tool),
          },
        ],
        summary: `${failedTool} tool failed. Recover by calling ${option.tool} tool`,
      };
    }
  }

  return {
    nextSteps: [],
    summary: `${failedTool} tool failed. Manual intervention needed`,
  };
}

/**
 * Get human-readable description for tools
 */
function getToolDescription(tool: string): string {
  const descriptions: Record<string, string> = {
    [TOOL_NAMES.ANALYZE_REPO]: 'Analyze repository structure and dependencies',
    [TOOL_NAMES.RESOLVE_BASE_IMAGES]: 'Resolve and recommend optimal base images',
    [TOOL_NAMES.GENERATE_DOCKERFILE]: 'Generate optimized Dockerfile',
    [TOOL_NAMES.FIX_DOCKERFILE]: 'Fix Dockerfile issues and errors',
    [TOOL_NAMES.BUILD_IMAGE]: 'Build Docker image from Dockerfile',
    [TOOL_NAMES.SCAN_IMAGE]: 'Scan image for security vulnerabilities',
    [TOOL_NAMES.TAG_IMAGE]: 'Tag image for deployment',
    [TOOL_NAMES.PUSH_IMAGE]: 'Push image to container registry',
    [TOOL_NAMES.GENERATE_K8S_MANIFESTS]: 'Generate Kubernetes deployment manifests',
    [TOOL_NAMES.PREPARE_CLUSTER]: 'Prepare Kubernetes cluster environment',
    [TOOL_NAMES.DEPLOY_APPLICATION]: 'Deploy application to Kubernetes cluster',
    [TOOL_NAMES.VERIFY_DEPLOYMENT]: 'Verify deployment status and health',
  };

  return descriptions[tool] || `Execute ${tool}`;
}

/**
 * Create chain hint from workflow progression
 */
export function createWorkflowChainHint(progression: WorkflowProgression): string {
  if (progression.nextSteps.length === 0) {
    return progression.summary;
  }

  const nextStep = progression.nextSteps[0];
  return `${progression.summary}. Next Step: ${nextStep?.tool ? `Call ${nextStep.tool} tool` : 'unknown'}`;
}

/**
 * Format failure progression into a chain hint string
 */
export function formatFailureChainHint(
  failedTool: string,
  progression: WorkflowProgression,
): string {
  const nextStep = progression.nextSteps[0];
  if (!nextStep) {
    return `Error: ${failedTool} failed. Re-analyze the repository to understand the issue. Next: ${TOOL_NAMES.ANALYZE_REPO}`;
  }

  const reason = progression.summary
    .replace(`${failedTool} tool failed. `, '')
    .replace(`${failedTool} failed. `, '');
  return `Error: ${reason}. Next: ${nextStep.tool}`;
}
