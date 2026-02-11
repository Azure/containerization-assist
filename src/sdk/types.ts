/**
 * SDK Type Exports
 *
 * Re-exports all types that SDK consumers might need.
 * This provides a single import location for type-only imports.
 */

import type { Logger } from 'pino';

// ===== CORE TYPES =====

export type { ToolContext, ProgressReporter } from '@/core/context';

// ===== RESULT TYPES =====

export type { Result, ErrorGuidance } from '@/types/core';
// Note: Success and Failure are values, exported from main sdk/index.ts

// ===== SDK-SPECIFIC TYPES =====

/**
 * Options for SDK tool execution.
 *
 * These options allow SDK consumers to customize tool behavior
 * without needing to understand the full ToolContext interface.
 */
export interface SDKOptions {
  /**
   * Custom logger instance.
   * Defaults to a quiet logger (warn level) to minimize noise.
   */
  logger?: Logger;

  /**
   * Abort signal for cancellation support.
   * Pass an AbortController's signal to enable cancellation.
   */
  signal?: AbortSignal;

  /**
   * Progress callback for long-running operations.
   * Called with status updates during tool execution.
   */
  onProgress?: (message: string, progress?: number, total?: number) => void;
}

// ===== ANALYZE-REPO TYPES =====

export type { RepositoryAnalysis, ModuleInfo } from '@/tools/analyze-repo/schema';

// ===== GENERATE-DOCKERFILE TYPES =====

export type {
  DockerfilePlan,
  GenerateDockerfileParams,
  BaseImageRecommendation,
  DockerfileRequirement,
  DockerfileAnalysis,
  EnhancementGuidance,
} from '@/tools/generate-dockerfile/schema';

// ===== FIX-DOCKERFILE TYPES =====

export type {
  DockerfileFixPlan,
  FixDockerfileParams,
  ValidationIssue,
  FixRecommendation,
} from '@/tools/fix-dockerfile/schema';

// ===== BUILD-IMAGE TYPES =====

export type { BuildImageParams } from '@/tools/build-image/schema';
export type { BuildImageResult } from '@/tools/build-image/tool';

// ===== SCAN-IMAGE TYPES =====

export type { ScanImageParams } from '@/tools/scan-image/schema';
export type { ScanImageResult } from '@/tools/scan-image/tool';

// ===== TAG-IMAGE TYPES =====

export type { TagImageParams } from '@/tools/tag-image/schema';
export type { TagImageResult } from '@/tools/tag-image/tool';

// ===== PUSH-IMAGE TYPES =====

export type { PushImageResult } from '@/tools/push-image/tool';

// ===== GENERATE-K8S-MANIFESTS TYPES =====

export type {
  ManifestPlan,
  GenerateK8sManifestsParams,
  ManifestRequirement,
  RepositoryInfo,
} from '@/tools/generate-k8s-manifests/schema';

// ===== PREPARE-CLUSTER TYPES =====

export type { PrepareClusterParams } from '@/tools/prepare-cluster/schema';
export type { PrepareClusterResult } from '@/tools/prepare-cluster/tool';

// ===== VERIFY-DEPLOY TYPES =====

export type { VerifyDeployParams } from '@/tools/verify-deploy/schema';
export type { VerifyDeploymentResult } from '@/tools/verify-deploy/tool';

// ===== OPS TYPES =====

export type { OpsToolParams } from '@/tools/ops/schema';
export type { PingResult, ServerStatusResult, OpsResult } from '@/tools/ops/tool';
