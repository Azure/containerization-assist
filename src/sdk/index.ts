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
import type {
  SDKOptions,
  // Types needed for ToolRegistry interface
  RepositoryAnalysis,
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
} from './types.js';
import { executeTool } from './executor.js';

// ===== TOOL AND SCHEMA IMPORTS =====
// Consolidated imports - tools and schemas from each module

// analyze-repo
import analyzeRepoTool from '@/tools/analyze-repo/tool';
import { analyzeRepoSchema } from '@/tools/analyze-repo/schema';

// generate-dockerfile
import generateDockerfileTool from '@/tools/generate-dockerfile/tool';
import { generateDockerfileSchema } from '@/tools/generate-dockerfile/schema';

// fix-dockerfile
import fixDockerfileTool from '@/tools/fix-dockerfile/tool';
import { fixDockerfileSchema } from '@/tools/fix-dockerfile/schema';

// build-image
import buildImageTool from '@/tools/build-image/tool';
import { buildImageSchema } from '@/tools/build-image/schema';

// scan-image
import scanImageTool from '@/tools/scan-image/tool';
import { scanImageSchema } from '@/tools/scan-image/schema';

// tag-image
import tagImageTool from '@/tools/tag-image/tool';
import { tagImageSchema } from '@/tools/tag-image/schema';

// push-image
import pushImageTool from '@/tools/push-image/tool';
import { pushImageSchema } from '@/tools/push-image/schema';

// generate-k8s-manifests
import generateK8sManifestsTool from '@/tools/generate-k8s-manifests/tool';
import { generateK8sManifestsSchema } from '@/tools/generate-k8s-manifests/schema';

// prepare-cluster
import prepareClusterTool from '@/tools/prepare-cluster/tool';
import { prepareClusterSchema } from '@/tools/prepare-cluster/schema';

// verify-deploy
import verifyDeployTool from '@/tools/verify-deploy/tool';
import { verifyDeploySchema } from '@/tools/verify-deploy/schema';

// ops
import opsTool from '@/tools/ops/tool';
import { opsToolSchema } from '@/tools/ops/schema';

// ===== TYPE RE-EXPORTS =====
// All types are consolidated in ./types.ts - re-export from there for convenience

// Core result types and value constructors
export type { Result, ErrorGuidance } from '@/types/core';
export { Success, Failure } from '@/types/core';

// SDK-specific types and tool result types
// Re-exported from types.ts to provide single import location
export type {
  // SDK options
  SDKOptions,
  // Tool result types
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
} from './types.js';

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

// ===== TOOL REGISTRY TYPE =====

/**
 * Type-safe tool registry mapping tool names to their Tool instances.
 *
 * This interface ensures:
 * - All expected tools are present
 * - Each tool has the correct schema and output type
 * - TypeScript catches typos in tool names
 */
interface ToolRegistry {
  analyzeRepo: Tool<typeof analyzeRepoSchema, RepositoryAnalysis>;
  generateDockerfile: Tool<typeof generateDockerfileSchema, DockerfilePlan>;
  fixDockerfile: Tool<typeof fixDockerfileSchema, DockerfileFixPlan>;
  buildImage: Tool<typeof buildImageSchema, BuildImageResult>;
  scanImage: Tool<typeof scanImageSchema, ScanImageResult>;
  tagImage: Tool<typeof tagImageSchema, TagImageResult>;
  pushImage: Tool<typeof pushImageSchema, PushImageResult>;
  generateK8sManifests: Tool<typeof generateK8sManifestsSchema, ManifestPlan>;
  prepareCluster: Tool<typeof prepareClusterSchema, PrepareClusterResult>;
  verifyDeploy: Tool<typeof verifyDeploySchema, VerifyDeploymentResult>;
  ops: Tool<typeof opsToolSchema, OpsResult>;
}

// ===== ADVANCED: DIRECT TOOL ACCESS =====

/**
 * Direct access to all 11 tool objects for advanced use cases.
 *
 * Use this when you need:
 * - Access to tool schemas for validation
 * - Tool metadata and descriptions
 * - Custom execution patterns
 *
 * Uses `satisfies` to ensure type safety:
 * - TypeScript will error if a tool is missing
 * - TypeScript will error if a tool has the wrong type
 * - TypeScript will error if a key is misspelled
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
} as const satisfies ToolRegistry;

/**
 * Type for the tools object keys - useful for generic functions.
 */
export type SDKToolName = keyof ToolRegistry;

/**
 * Export the registry type for consumers who need to extend or type-check tools.
 */
export type { ToolRegistry };

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

// ===== JSON SCHEMA EXPORTS (for VS Code extension) =====

export {
  jsonSchemas,
  type ToolSchemaName,
  // Individual schemas for selective imports
  analyzeRepoJsonSchema,
  generateDockerfileJsonSchema,
  fixDockerfileJsonSchema,
  buildImageJsonSchema,
  scanImageJsonSchema,
  tagImageJsonSchema,
  pushImageJsonSchema,
  generateK8sManifestsJsonSchema,
  prepareClusterJsonSchema,
  verifyDeployJsonSchema,
  opsJsonSchema,
} from './schemas.js';

// ===== TOOL METADATA EXPORTS (for VS Code extension) =====

export {
  toolMetadata,
  type ToolMetadata,
  type ToolMetadataName,
  type ConfirmationConfig,
  standardWorkflow,
  getToolsByCategory,
  // External dependency types and helpers
  ExternalDeps,
  type ExternalDepId,
  type ExternalDependency,
  getRequiredDepsString,
  requiresDependency,
  // Individual metadata for selective imports
  analyzeRepoMetadata,
  generateDockerfileMetadata,
  fixDockerfileMetadata,
  buildImageMetadata,
  scanImageMetadata,
  tagImageMetadata,
  pushImageMetadata,
  generateK8sManifestsMetadata,
  prepareClusterMetadata,
  verifyDeployMetadata,
  opsMetadata,
} from './metadata.js';

// ===== RESULT FORMATTERS (for VS Code extension) =====

export {
  resultFormatters,
  type FormatterOptions,
  type FormatterName,
  type FormatterRegistry,
  // Individual formatters for selective imports
  formatAnalyzeRepoResult,
  formatGenerateDockerfileResult,
  formatFixDockerfileResult,
  formatBuildImageResult,
  formatScanImageResult,
  formatTagImageResult,
  formatPushImageResult,
  formatGenerateK8sManifestsResult,
  formatPrepareClusterResult,
  formatVerifyDeployResult,
  formatOpsResult,
} from './formatters.js';

// ===== VS CODE UTILITIES =====

export {
  createAbortSignalFromToken,
  formatErrorForLLM,
  resolveWorkspacePath,
  validateRequiredFields,
  sanitizeForMarkdown,
  type CancellationTokenLike,
  type AbortSignalResult,
  type ValidationResult,
} from './vscode-utils.js';
