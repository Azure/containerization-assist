/**
 * Containerization Assist SDK
 *
 * Provides direct access to all 11 containerization tools without requiring
 * the MCP (Model Context Protocol) server infrastructure.
 *
 * This SDK is designed for:
 * - VS Code extension developers integrating with Copilot
 * - Direct programmatic usage without MCP overhead
 * - Lightweight tool execution in Node.js applications
 *
 * @example
 * ```typescript
 * import { analyzeRepo, buildImage, scanImage } from 'containerization-assist-mcp/sdk';
 *
 * // Full containerization workflow
 * const analysis = await analyzeRepo({ repositoryPath: './my-app' });
 * const build = await buildImage({ path: './my-app', imageName: 'myapp:v1' });
 * const scan = await scanImage({ imageId: 'myapp:v1' });
 * ```
 *
 * @packageDocumentation
 */

import type { z } from 'zod';
import type { Result } from '@/types/core';
import type { Tool } from '@/types/tool';
import type { SDKOptions } from './types.js';
import { executeTool } from './executor.js';

// ===== TOOL AND SCHEMA IMPORTS =====
// Consolidated imports - tools, schemas, and types from each module

// analyze-repo
import analyzeRepoTool from '@/tools/analyze-repo/tool';
import {
  analyzeRepoSchema,
  type RepositoryAnalysis,
  type ModuleInfo,
} from '@/tools/analyze-repo/schema';

// generate-dockerfile
import generateDockerfileTool from '@/tools/generate-dockerfile/tool';
import {
  generateDockerfileSchema,
  type DockerfilePlan,
} from '@/tools/generate-dockerfile/schema';

// fix-dockerfile
import fixDockerfileTool from '@/tools/fix-dockerfile/tool';
import {
  fixDockerfileSchema,
  type DockerfileFixPlan,
} from '@/tools/fix-dockerfile/schema';

// build-image
import buildImageTool, { type BuildImageResult } from '@/tools/build-image/tool';
import { buildImageSchema } from '@/tools/build-image/schema';

// scan-image
import scanImageTool, { type ScanImageResult } from '@/tools/scan-image/tool';
import { scanImageSchema } from '@/tools/scan-image/schema';

// tag-image
import tagImageTool, { type TagImageResult } from '@/tools/tag-image/tool';
import { tagImageSchema } from '@/tools/tag-image/schema';

// push-image
import pushImageTool, { type PushImageResult } from '@/tools/push-image/tool';
import { pushImageSchema } from '@/tools/push-image/schema';

// generate-k8s-manifests
import generateK8sManifestsTool from '@/tools/generate-k8s-manifests/tool';
import {
  generateK8sManifestsSchema,
  type ManifestPlan,
} from '@/tools/generate-k8s-manifests/schema';

// prepare-cluster
import prepareClusterTool, { type PrepareClusterResult } from '@/tools/prepare-cluster/tool';
import { prepareClusterSchema } from '@/tools/prepare-cluster/schema';

// verify-deploy
import verifyDeployTool, { type VerifyDeploymentResult } from '@/tools/verify-deploy/tool';
import { verifyDeploySchema } from '@/tools/verify-deploy/schema';

// ops
import opsTool, { type OpsResult } from '@/tools/ops/tool';
import { opsToolSchema } from '@/tools/ops/schema';

// ===== TYPE RE-EXPORTS =====

// Result types
export type { Result, ErrorGuidance } from '@/types/core';
export { Success, Failure } from '@/types/core';

// SDK options
export type { SDKOptions } from './types.js';

// Re-export result types for SDK consumers
export type {
  RepositoryAnalysis,
  ModuleInfo,
  DockerfilePlan,
  DockerfileFixPlan,
  BuildImageResult,
  ScanImageResult,
  TagImageResult,
  PushImageResult,
  ManifestPlan,
  PrepareClusterResult,
  VerifyDeploymentResult,
  OpsResult,
};

// ===== INPUT TYPES (derived from Zod schemas) =====

/** Input type for analyzeRepo - derived from Zod schema */
export type AnalyzeRepoInput = z.input<typeof analyzeRepoSchema>;
/** Input type for generateDockerfile - derived from Zod schema */
export type GenerateDockerfileInput = z.input<typeof generateDockerfileSchema>;
/** Input type for fixDockerfile - derived from Zod schema */
export type FixDockerfileInput = z.input<typeof fixDockerfileSchema>;
/** Input type for buildImage - derived from Zod schema */
export type BuildImageInput = z.input<typeof buildImageSchema>;
/** Input type for scanImage - derived from Zod schema */
export type ScanImageInput = z.input<typeof scanImageSchema>;
/** Input type for tagImage - derived from Zod schema */
export type TagImageInput = z.input<typeof tagImageSchema>;
/** Input type for pushImage - derived from Zod schema */
export type PushImageInput = z.input<typeof pushImageSchema>;
/** Input type for generateK8sManifests - derived from Zod schema */
export type GenerateK8sManifestsInput = z.input<typeof generateK8sManifestsSchema>;
/** Input type for prepareCluster - derived from Zod schema */
export type PrepareClusterInput = z.input<typeof prepareClusterSchema>;
/** Input type for verifyDeploy - derived from Zod schema */
export type VerifyDeployInput = z.input<typeof verifyDeploySchema>;
/** Input type for ops - derived from Zod schema */
export type OpsInput = z.input<typeof opsToolSchema>;

// Full type exports available via sdk/types
// import type { ... } from 'containerization-assist-mcp/sdk/types';

// ===== SDK FUNCTION FACTORY =====

/**
 * Create a typed SDK function from a tool.
 *
 * This factory ensures consistent behavior across all SDK functions
 * while preserving full type inference for inputs and outputs.
 *
 * @param tool - The tool to wrap
 * @returns A typed function that executes the tool
 */
function createSDKFunction<TSchema extends z.ZodTypeAny, TOutput>(
  tool: Tool<TSchema, TOutput>,
): (input: z.input<TSchema>, options?: SDKOptions) => Promise<Result<TOutput>> {
  return (input, options) => executeTool(tool, input, options);
}

// ===== SDK FUNCTION EXPORTS =====

// ----- Analysis Tools -----

/**
 * Analyze a repository to detect language, framework, and dependencies.
 */
export const analyzeRepo = createSDKFunction(analyzeRepoTool);

// ----- Dockerfile Tools -----

/**
 * Generate Dockerfile recommendations for a repository.
 */
export const generateDockerfile = createSDKFunction(generateDockerfileTool);

/**
 * Fix and optimize an existing Dockerfile.
 */
export const fixDockerfile = createSDKFunction(fixDockerfileTool);

// ----- Image Tools -----

/**
 * Build a Docker image from a Dockerfile.
 * Requires Docker daemon to be running.
 */
export const buildImage = createSDKFunction(buildImageTool);

/**
 * Scan a Docker image for security vulnerabilities.
 * Requires Trivy to be installed for full functionality.
 */
export const scanImage = createSDKFunction(scanImageTool);

/**
 * Tag a Docker image with additional tags.
 * Requires Docker daemon to be running.
 */
export const tagImage = createSDKFunction(tagImageTool);

/**
 * Push a Docker image to a registry.
 * Requires Docker daemon and registry authentication.
 */
export const pushImage = createSDKFunction(pushImageTool);

// ----- Kubernetes Tools -----

/**
 * Generate Kubernetes manifests for deployment.
 */
export const generateK8sManifests = createSDKFunction(generateK8sManifestsTool);

/**
 * Prepare a Kubernetes cluster for deployment.
 * Requires kubectl configured with cluster access.
 */
export const prepareCluster = createSDKFunction(prepareClusterTool);

/**
 * Verify a Kubernetes deployment status.
 * Requires kubectl configured with cluster access.
 */
export const verifyDeploy = createSDKFunction(verifyDeployTool);

// ----- Operational Tools -----

/**
 * Operational utilities (ping, status).
 */
export const ops = createSDKFunction(opsTool);

// ===== ADVANCED: DIRECT TOOL ACCESS =====

/**
 * Direct access to all 11 tool objects for advanced use cases.
 *
 * Use this when you need:
 * - Access to tool schemas for validation
 * - Tool metadata and descriptions
 * - Custom execution patterns
 *
 * @example
 * ```typescript
 * import { tools } from 'containerization-assist-mcp/sdk';
 *
 * // Access tool schema
 * const schema = tools.analyzeRepo.schema;
 *
 * // Access tool metadata
 * console.log(tools.buildImage.description);
 * ```
 */
export const tools = {
  // ===== Analysis =====
  /** Analyze repository structure, detect languages, frameworks, and dependencies */
  analyzeRepo: analyzeRepoTool,

  // ===== Dockerfile =====
  /** Generate optimized Dockerfile with security best practices */
  generateDockerfile: generateDockerfileTool,
  /** Fix and optimize existing Dockerfile issues */
  fixDockerfile: fixDockerfileTool,

  // ===== Image Operations =====
  /** Build Docker image from Dockerfile (requires Docker daemon) */
  buildImage: buildImageTool,
  /** Scan image for security vulnerabilities (requires Trivy) */
  scanImage: scanImageTool,
  /** Tag Docker image with additional tags */
  tagImage: tagImageTool,
  /** Push image to container registry */
  pushImage: pushImageTool,

  // ===== Kubernetes =====
  /** Generate Kubernetes deployment manifests */
  generateK8sManifests: generateK8sManifestsTool,
  /** Prepare Kubernetes cluster namespace and prerequisites */
  prepareCluster: prepareClusterTool,
  /** Verify Kubernetes deployment status and health */
  verifyDeploy: verifyDeployTool,

  // ===== Operations =====
  /** Operational utilities (ping, status checks) */
  ops: opsTool,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
} as const satisfies Record<string, Tool<any, any>>;

/**
 * Execute any tool directly with full control.
 *
 * Use this when you need to:
 * - Execute tools not exposed via simplified functions
 * - Pass custom options to the executor
 * - Handle tool objects dynamically
 *
 * @example
 * ```typescript
 * import { executeTool, tools } from 'containerization-assist-mcp/sdk';
 *
 * const result = await executeTool(
 *   tools.analyzeRepo,
 *   { repositoryPath: '.' },
 *   { signal: controller.signal }
 * );
 * ```
 */
export { executeTool } from './executor.js';
