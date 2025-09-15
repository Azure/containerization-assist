/**
 * Consolidated tool result types using composition pattern.
 * Uses intersection types to combine capabilities rather than inheritance.
 */

import type {
  Result,
  ToolExecutionMetadata,
  ValidationResult,
  AnalysisCapability,
  BuildCapability,
  DeploymentCapability,
} from './core';

// ===== ANALYSIS TYPES =====

export interface RepositoryAnalysis {
  language: string;
  languageVersion?: string;
  framework?: string;
  frameworkVersion?: string;
  buildSystem?: {
    type: string;
    file: string;
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
  summary?: string;
}

export type AnalyzeRepoResult = Result<RepositoryAnalysis> &
  ToolExecutionMetadata &
  AnalysisCapability;

// ===== BUILD TYPES =====

export interface ImageBuild {
  imageId: string;
  tags: string[];
  size: number;
  layers?: number;
  buildTime: number;
  logs: string[];
  securityWarnings?: string[];
}

export type BuildImageResult = Result<ImageBuild> & ToolExecutionMetadata & BuildCapability;

export interface DockerfileGeneration {
  content: string;
  path: string;
  multistage: boolean;
  baseImage: string;
  optimizations: string[];
  warnings?: string[];
}

export type GenerateDockerfileResult = Result<DockerfileGeneration> & ToolExecutionMetadata;

export interface DockerfileFix {
  content: string;
  fixes: Array<{
    issue: string;
    fix: string;
    severity: 'error' | 'warning' | 'info';
  }>;
  optimizations?: string[];
}

export type FixDockerfileResult = Result<DockerfileFix> & ToolExecutionMetadata & ValidationResult;

// ===== CONTAINER MANAGEMENT TYPES =====

export interface ImageTag {
  originalTag: string;
  newTags: string[];
  imageId: string;
}

export type TagImageResult = Result<ImageTag> & ToolExecutionMetadata;

export interface ImagePush {
  imageId: string;
  tags: string[];
  registry: string;
  size: number;
  digest: string;
}

export type PushImageResult = Result<ImagePush> & ToolExecutionMetadata & BuildCapability;

// ===== SECURITY TYPES =====

export interface SecurityScan {
  vulnerabilities: {
    critical: number;
    high: number;
    medium: number;
    low: number;
    unknown: number;
    total: number;
  };
  remediationGuidance?: Array<{
    vulnerability: string;
    recommendation: string;
    severity?: string;
    example?: string;
  }>;
  scanTime: string;
  passed: boolean;
}

export type ScanImageResult = Result<SecurityScan> & ToolExecutionMetadata & ValidationResult;

// ===== KUBERNETES TYPES =====

export interface KubernetesManifests {
  manifests: Array<{
    kind: string;
    name: string;
    namespace: string;
    content: string;
    filePath?: string;
  }>;
  outputPath?: string;
  replicas?: number;
  resources?: unknown;
}

export type GenerateK8sManifestsResult = Result<KubernetesManifests> & ToolExecutionMetadata;

export interface ClusterPreparation {
  namespace: string;
  created: boolean;
  configured: boolean;
  ready: boolean;
}

export type PrepareClusterResult = Result<ClusterPreparation> & ToolExecutionMetadata;

export interface ApplicationDeployment {
  namespace: string;
  deploymentName: string;
  serviceName: string;
  endpoints: Array<{
    type: 'internal' | 'external';
    url: string;
    port: number;
  }>;
  ready: boolean;
}

export type DeployApplicationResult = Result<ApplicationDeployment> &
  ToolExecutionMetadata &
  DeploymentCapability;

export interface DeploymentVerification {
  deploymentName: string;
  namespace: string;
  ready: boolean;
  replicas: {
    desired: number;
    ready: number;
    available: number;
  };
  conditions: Array<{
    type: string;
    status: string;
    message: string;
  }>;
  endpoints?: Array<{
    url: string;
    ready: boolean;
  }>;
}

export type VerifyDeploymentResult = Result<DeploymentVerification> & ToolExecutionMetadata;

// ===== HELM TYPES =====

export interface HelmCharts {
  chartName: string;
  chartPath: string;
  templates: string[];
  values: Record<string, unknown>;
  version: string;
}

export type GenerateHelmChartsResult = Result<HelmCharts> & ToolExecutionMetadata;

// ===== ACA TYPES =====

export interface AcaManifests {
  manifests: Array<{
    name: string;
    type: string;
    content: string;
  }>;
  outputPath: string;
}

export type GenerateAcaManifestsResult = Result<AcaManifests> & ToolExecutionMetadata;

export interface AcaToK8sConversion {
  convertedManifests: Array<{
    original: string;
    converted: string;
    kind: string;
    name: string;
  }>;
  warnings?: string[];
}

export type ConvertAcaToK8sResult = Result<AcaToK8sConversion> &
  ToolExecutionMetadata &
  ValidationResult;

// ===== UTILITY TYPES =====

export interface BaseImageRecommendations {
  recommendations: Array<{
    image: string;
    tag: string;
    reason: string;
    size?: string;
    security?: string;
  }>;
  preferred?: string;
}

export type ResolveBaseImagesResult = Result<BaseImageRecommendations> & ToolExecutionMetadata;

// ===== UNION TYPE FOR ALL TOOL RESULTS =====

export type AnyToolResult =
  | AnalyzeRepoResult
  | BuildImageResult
  | GenerateDockerfileResult
  | FixDockerfileResult
  | TagImageResult
  | PushImageResult
  | ScanImageResult
  | GenerateK8sManifestsResult
  | PrepareClusterResult
  | DeployApplicationResult
  | VerifyDeploymentResult
  | GenerateHelmChartsResult
  | GenerateAcaManifestsResult
  | ConvertAcaToK8sResult
  | ResolveBaseImagesResult;

// ===== UTILITY INTERSECTION TYPES =====

export type WithAnalysis<T> = T & AnalysisCapability;
export type WithValidation<T> = T & ValidationResult;
export type WithBuild<T> = T & BuildCapability;
export type WithDeployment<T> = T & DeploymentCapability;

// ===== LEGACY COMPATIBILITY =====

export type AnalysisPerspective = 'comprehensive' | 'security-focused' | 'performance-focused';

export interface PerspectiveConfig {
  perspective: AnalysisPerspective;
  emphasis: string[];
  additionalChecks: string[];
}
