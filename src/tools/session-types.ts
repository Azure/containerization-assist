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
  build_system?: {
    type?: string;
    build_file?: string;
    build_command?: string;
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
    file_path?: string;
  }>;
  replicas?: number;
  resources?: unknown;
  output_path?: string;
}

/**
 * Session metadata
 */
export interface SessionMetadata {
  repo_path?: string;
  dockerfile_baseImage?: string;
  dockerfile_optimization?: boolean;
  dockerfile_warnings?: string[];
  ai_enhancement_used?: boolean;
  ai_generation_type?: string;
  timestamp?: string;
  k8s_warnings?: string[];
  [key: string]: unknown;
}

/**
 * Complete session data structure
 */
export interface SessionData {
  analysis_result?: SessionAnalysisResult;
  build_result?: SessionBuildResult;
  dockerfile_result?: SessionDockerfileResult;
  k8s_result?: SessionK8sResult;
  workflow_state?: WorkflowState & {
    analysis_result?: SessionAnalysisResult;
    build_result?: SessionBuildResult;
    dockerfile_result?: SessionDockerfileResult;
    k8s_result?: SessionK8sResult;
    metadata?: SessionMetadata;
  };
  metadata?: SessionMetadata;
  completed_steps?: string[];
  currentStep?: string;
  [key: string]: unknown;
}
