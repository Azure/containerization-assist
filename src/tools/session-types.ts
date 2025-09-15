/**
 * Common session types for tools
 *
 * These types represent the session data structures used across tools
 */

import type { WorkflowState } from '../types';

/**
 * Analysis result stored in session
 */
export interface SessionAnalysisResult {
  language?: string;
  framework?: string;
  dependencies?: Array<{ name: string; version?: string }>;
  ports?: number[];
  buildSystem?: {
    type?: string;
    buildFile?: string;
    buildCommand?: string;
  };
  summary?: string;
}

/**
 * Build result stored in session
 */
export interface SessionBuildResult {
  imageId?: string;
  tags?: string[];
  error?: string;
  digest?: string;
}

/**
 * Dockerfile result stored in session
 */
export interface SessionDockerfileResult {
  content?: string;
  path?: string;
  multistage?: boolean;
  fixed?: boolean;
  fixes?: string[];
}

/**
 * K8s result stored in session
 */
export interface SessionK8sResult {
  manifests?: Array<{
    kind: string;
    name: string;
    namespace: string;
    content?: string;
    filePath?: string;
  }>;
  replicas?: number;
  resources?: unknown;
  outputPath?: string;
}

/**
 * Session metadata
 */
export interface SessionMetadata {
  repoPath?: string;
  dockerfileBaseImage?: string;
  dockerfileOptimization?: boolean;
  dockerfileWarnings?: string[];
  aiEnhancementUsed?: boolean;
  aiGenerationType?: string;
  timestamp?: string;
  k8sWarnings?: string[];
  [key: string]: unknown;
}

/**
 * Complete session data structure
 */
export interface SessionData {
  workflowState?: WorkflowState & {
    metadata?: SessionMetadata;
  };
  metadata?: SessionMetadata;
  completedSteps?: string[];
  currentStep?: string;
  results?: Record<string, unknown>;
  [key: string]: unknown;
}

/**
 * Helper functions for safe access to session results
 * All data is stored in the results object indexed by tool name
 */
export function getAnalysisResult(
  session: WorkflowState | SessionData | undefined | null,
): SessionAnalysisResult | undefined {
  if (!session) return undefined;

  // Check results object (standard pattern)
  if ('results' in session && session.results?.['analyze-repo']) {
    return session.results['analyze-repo'] as SessionAnalysisResult;
  }

  // Check nested workflowState
  if ('workflowState' in session && session.workflowState) {
    const ws = session.workflowState;
    if (typeof ws === 'object' && ws !== null && 'results' in ws) {
      const results = (ws as WorkflowState).results;
      if (results?.['analyze-repo']) {
        return results['analyze-repo'] as SessionAnalysisResult;
      }
    }
  }

  return undefined;
}

export function getBaseImages(session: WorkflowState | SessionData | undefined | null): unknown {
  if (!session) return undefined;

  // Check results object (standard pattern)
  if ('results' in session && session.results?.['resolve-base-images']) {
    return session.results['resolve-base-images'];
  }

  // Check nested workflowState
  if ('workflowState' in session && session.workflowState) {
    const ws = session.workflowState;
    if (typeof ws === 'object' && ws !== null && 'results' in ws) {
      const results = (ws as WorkflowState).results;
      if (results?.['resolve-base-images']) {
        return results['resolve-base-images'];
      }
    }
  }

  return undefined;
}
