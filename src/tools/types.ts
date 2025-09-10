/**
 * Shared types for tools to prevent circular dependencies
 */

// ToolContext should now be imported directly from '@mcp/context/types'

export interface AnalyzeRepoResult {
  ok: boolean;
  sessionId: string;
  language: string;
  languageVersion?: string;
  framework?: string;
  frameworkVersion?: string;
  buildSystem?: {
    type: string;
    buildFile: string;
    buildCommand: string;
    testCommand?: string;
  };
  dependencies: Array<{
    name: string;
    version?: string;
    type: string;
  }>;
  ports: number[];
  hasDockerfile: boolean;
  hasDockerCompose: boolean;
  hasKubernetes: boolean;
  recommendations?: {
    baseImage?: string;
    buildStrategy?: string;
    securityNotes?: string[];
  };
  metadata?: {
    repoPath: string;
    depth: number;
    includeTests: boolean;
    timestamp: string;
    aiInsights?: string;
  };
}

export type AnalysisPerspective = 'comprehensive' | 'security-focused' | 'performance-focused';

export interface PerspectiveConfig {
  perspective: AnalysisPerspective;
  emphasis: string[];
  additionalChecks: string[];
}
