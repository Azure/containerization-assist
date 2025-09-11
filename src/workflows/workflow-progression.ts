/**
 * Centralized Workflow Progression
 *
 * Dynamically determines next steps based on current tool, success/failure,
 * session context, and workflow configuration rather than hardcoded hints.
 */

import type { SessionContext } from '../lib/chain-hints';
import { TOOL_NAMES } from '../exports/tools.js';

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

/**
 * Recovery workflows for failures
 */
const RECOVERY_WORKFLOWS = {
  [TOOL_NAMES.BUILD_IMAGE]: [
    {
      tool: TOOL_NAMES.FIX_DOCKERFILE,
      condition: (ctx: SessionContext) => !ctx.completed_steps?.includes(TOOL_NAMES.FIX_DOCKERFILE),
    },
    {
      tool: TOOL_NAMES.GENERATE_DOCKERFILE,
      condition: (ctx: SessionContext) => ctx.completed_steps?.includes(TOOL_NAMES.FIX_DOCKERFILE),
    },
    {
      tool: TOOL_NAMES.ANALYZE_REPO,
      condition: (ctx: SessionContext) => !ctx.completed_steps?.includes(TOOL_NAMES.ANALYZE_REPO),
    },
  ],
  [TOOL_NAMES.FIX_DOCKERFILE]: [
    {
      tool: TOOL_NAMES.ANALYZE_REPO,
      condition: (ctx: SessionContext) => !ctx.completed_steps?.includes(TOOL_NAMES.ANALYZE_REPO),
    },
    { tool: TOOL_NAMES.GENERATE_DOCKERFILE, condition: () => true },
  ],
  [TOOL_NAMES.GENERATE_DOCKERFILE]: [
    {
      tool: TOOL_NAMES.ANALYZE_REPO,
      condition: (ctx: SessionContext) => !ctx.completed_steps?.includes(TOOL_NAMES.ANALYZE_REPO),
    },
    { tool: TOOL_NAMES.FIX_DOCKERFILE, condition: () => true },
  ],
  [TOOL_NAMES.SCAN_IMAGE]: [
    { tool: TOOL_NAMES.TAG_IMAGE, condition: () => true }, // Continue even if scan fails
  ],
  [TOOL_NAMES.TAG_IMAGE]: [
    { tool: TOOL_NAMES.BUILD_IMAGE, condition: () => true }, // Rebuild if tagging fails
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
        summary: `${failedTool} failed. Recover with ${option.tool}`,
      };
    }
  }

  return {
    nextSteps: [],
    summary: `${failedTool} failed. Manual intervention needed`,
  };
}

/**
 * Get human-readable description for tools
 */
function getToolDescription(tool: string): string {
  const descriptions: Record<string, string> = {
    [TOOL_NAMES.ANALYZE_REPO]: 'Analyze repository structure and dependencies',
    [TOOL_NAMES.GENERATE_DOCKERFILE]: 'Generate optimized Dockerfile',
    [TOOL_NAMES.FIX_DOCKERFILE]: 'Fix Dockerfile issues and errors',
    [TOOL_NAMES.BUILD_IMAGE]: 'Build Docker image from Dockerfile',
    [TOOL_NAMES.SCAN_IMAGE]: 'Scan image for security vulnerabilities',
    [TOOL_NAMES.TAG_IMAGE]: 'Tag image for deployment',
    [TOOL_NAMES.PUSH_IMAGE]: 'Push image to container registry',
    [TOOL_NAMES.GENERATE_K8S_MANIFESTS]: 'Generate Kubernetes deployment manifests',
    [TOOL_NAMES.VERIFY_DEPLOYMENT]: 'Verify deployment configuration',
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
