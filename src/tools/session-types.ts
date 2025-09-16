/**
 * Common session types for tools
 *
 * These types represent the session data structures used across tools
 */

import type { WorkflowState } from '@types';

/**
 * Analysis result stored in session
 */
export interface SessionAnalysisResult {
  language?: string;
  framework?: string;
  frameworkVersion?: string;
  dependencies?: Array<{ name: string; version?: string }>;
  ports?: number[];
  buildSystem?: {
    type?: string;
    buildFile?: string;
    buildCommand?: string;
  };
  summary?: string;
  confidence?: number;
  detectionMethod?: 'signature' | 'extension' | 'provided' | 'fallback' | 'ai-enhanced';
  detectionDetails?: {
    signatureMatches: number;
    extensionMatches: number;
    frameworkSignals: number;
    buildSystemSignals: number;
  };
  recommendations?: {
    baseImage?: string;
    buildStrategy?: string;
    securityNotes?: string[];
  };
  modules?: string[]; // Detected module paths for multi-module projects
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

// Note: Helper functions removed - use getSessionSlice from @mcp/tool-session-helpers instead
// This provides proper type-safe access to session data across tools
