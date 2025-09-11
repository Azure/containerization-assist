/**
 * Chain hints using centralized workflow progression
 */

import { getFailureProgression, getSuccessProgression } from '../workflows/workflow-progression';
import { TOOL_NAMES } from '../exports/tools.js';

export interface FailureHint {
  tool: string;
  reason: string;
}

export interface SessionContext {
  completed_steps?: string[];
  dockerfile_result?: { content?: string };
  analysis_result?: { language?: string };
}

/**
 * Get failure hint using centralized workflow progression
 */
export function getFailureHint(
  failedTool: string,
  errorMessage: string,
  sessionContext?: SessionContext,
): FailureHint {
  const progression = getFailureProgression(
    failedTool,
    errorMessage,
    sessionContext || { completed_steps: [] },
  );

  const nextStep = progression.nextSteps[0];
  if (!nextStep) {
    return {
      tool: TOOL_NAMES.ANALYZE_REPO,
      reason: 'Re-analyze to understand the issue',
    };
  }

  return {
    tool: nextStep.tool,
    reason: progression.summary.replace(`${failedTool} failed. `, ''),
  };
}

/**
 * Get success chain hint using centralized workflow progression
 */
export function getSuccessChainHint(
  completedTool: string,
  sessionContext: SessionContext,
  workflowType: string = 'basic',
): string {
  const progression = getSuccessProgression(completedTool, sessionContext, workflowType);
  return progression.summary;
}

/**
 * Format hint for chain hint string
 */
export function formatChainHint(hint: FailureHint): string {
  return `Error: ${hint.reason}. Next: ${hint.tool}`;
}
