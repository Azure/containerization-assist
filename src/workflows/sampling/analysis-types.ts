/**
 * Analysis Sampling Types - Core interfaces for repository analysis sampling
 */

import type { Logger } from 'pino';
import type { Result } from '@types';

/**
 * Analysis context for sampling different perspectives
 */
export interface AnalysisContext {
  repoPath: string;
  language: 'java' | 'dotnet' | 'other';
  framework?: string;
  dependencies?: Array<{ name: string; version?: string; type: string }>;
  ports?: number[];
  depth?: number;
  includeTests?: boolean;
  securityFocus?: boolean;
  performanceFocus?: boolean;
  moduleRoots?: string[];
}

/**
 * Enhanced analysis result with sampling metadata
 */
export interface AnalysisVariant {
  id: string;
  strategy: string;
  perspective: 'comprehensive' | 'security' | 'performance' | 'architecture' | 'deployment';

  // Core analysis data from AnalyzeRepoResult
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
  recommendations?: string[];

  // Extended analysis properties
  files?: Array<{ path: string; type: string; content?: string }>;
  frameworks?: Array<{ name: string; version?: string }>;
  patterns?: Record<string, unknown>;
  security?: Record<string, unknown>;
  deployment?: Record<string, unknown>;

  // Sampling metadata
  insights: {
    keyFindings: string[];
    riskAssessments: string[];
    optimizationOpportunities: string[];
    architecturalPatterns: string[];
    deploymentReadiness: string[];
  };
  confidence: number; // 0-100
  completeness: number; // 0-100
  analysisTime: number;
  filesAnalyzed: number;
  generated: Date;
  metadata?: Record<string, unknown>;
}

/**
 * Analysis scoring criteria for variant evaluation
 */
export interface AnalysisScoringCriteria {
  accuracy: { weight: number; minScore: number }; // How accurate are the findings
  completeness: { weight: number; minScore: number }; // How comprehensive is the analysis
  relevance: { weight: number; minScore: number }; // How relevant to containerization
  actionability: { weight: number; minScore: number }; // How actionable are the insights
}

/**
 * Detailed analysis score breakdown
 */
export interface AnalysisScoreDetails {
  total: number; // Weighted total score 0-100
  breakdown: {
    accuracy: number; // Individual score 0-100
    completeness: number; // Individual score 0-100
    relevance: number; // Individual score 0-100
    actionability: number; // Individual score 0-100
  };
  strengths: string[]; // What this analysis does well
  weaknesses: string[]; // What this analysis misses
  recommendations: string[]; // How to improve the analysis
  confidence: number; // Overall confidence in this analysis
}

/**
 * Analysis variant with computed score
 */
export interface ScoredAnalysisVariant extends AnalysisVariant {
  score: AnalysisScoreDetails;
  rank: number;
}

/**
 * Analysis sampling strategy interface
 */
export interface AnalysisStrategy {
  name: string;
  description: string;
  perspective: 'comprehensive' | 'security' | 'performance' | 'architecture' | 'deployment';

  /**
   * Generate analysis variant using this strategy's perspective
   */
  analyzeRepository(context: AnalysisContext, logger: Logger): Promise<Result<AnalysisVariant>>;

  /**
   * Score an analysis variant based on this strategy's criteria
   */
  scoreAnalysis(
    variant: AnalysisVariant,
    criteria: AnalysisScoringCriteria,
    logger: Logger,
  ): Promise<Result<AnalysisScoreDetails>>;
}

/**
 * Analysis selection constraints
 */
export interface AnalysisSelectionConstraints {
  mustInclude?: string[]; // Required findings/insights
  mustNotInclude?: string[]; // Forbidden findings
  minConfidence?: number; // Minimum confidence level
  minCompleteness?: number; // Minimum completeness level
  preferredPerspective?:
    | 'comprehensive'
    | 'security'
    | 'performance'
    | 'architecture'
    | 'deployment';
}

/**
 * Complete analysis sampling result
 */
export interface AnalysisSamplingResult {
  bestVariant: ScoredAnalysisVariant;
  variants: ScoredAnalysisVariant[];
  metadata: {
    totalVariants: number;
    executionTime: number;
    criteria: AnalysisScoringCriteria;
    strategies: string[];
    timestamp: string;
  };
}

/**
 * Analysis sampling configuration
 */
export interface AnalysisSamplingConfig {
  variantCount?: number; // Default: 3
  strategies?: string[]; // Default: all available
  criteria?: Partial<AnalysisScoringCriteria>; // Default: balanced weights
  constraints?: AnalysisSelectionConstraints;
  focus?: 'comprehensive' | 'security' | 'performance' | 'architecture' | 'deployment';
  enableCaching?: boolean; // Default: true
  timeout?: number; // Default: 120000ms
}

/**
 * Analysis sampling options for workflow integration
 */
export interface AnalysisSamplingOptions {
  focus?: 'comprehensive' | 'security' | 'performance' | 'architecture' | 'deployment';
  priority?: 'speed' | 'accuracy' | 'completeness';
  customCriteria?: Partial<AnalysisScoringCriteria>;
}

/**
 * File analysis metadata
 */
export interface FileAnalysisMetadata {
  path: string;
  size: number;
  language: string;
  importance: 'critical' | 'important' | 'useful' | 'optional';
  analysisDepth: 'full' | 'partial' | 'metadata-only';
  insights: string[];
  risks: string[];
}
