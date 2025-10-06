/**
 * Tool Registry - Type-Safe Consolidation
 *
 * Explicit, type-safe imports for all MCP tools.
 * This replaces the distributed tool registration across multiple files.
 */

// Direct tool imports - all tools now use the unified Tool interface
import analyzeRepoTool from './analyze-repo/tool';
import buildImageTool from './build-image/tool';
import convertAcaToK8sTool from './convert-aca-to-k8s/tool';
import deployTool from './deploy/tool';
import fixDockerfileTool from './fix-dockerfile/tool';
import generateAcaManifestsTool from './generate-aca-manifests/tool';
import generateDockerfileTool from './generate-dockerfile/tool';
import generateHelmChartsTool from './generate-helm-charts/tool';
import generateK8sManifestsTool from './generate-k8s-manifests/tool';
import generateKustomizeTool from './generate-kustomize/tool';
import inspectSessionTool from './inspect-session/tool';
import opsTool from './ops/tool';
import planDockerfileGenerationTool from './plan-dockerfile-generation/tool';
import planManifestGenerationTool from './plan-manifest-generation/tool';
import prepareClusterTool from './prepare-cluster/tool';
import pushImageTool from './push-image/tool';
import resolveBaseImagesTool from './resolve-base-images/tool';
import scanTool from './scan/tool';
import tagImageTool from './tag-image/tool';
import verifyDeployTool from './verify-deployment/tool';

// Tool name constants for type safety
export const TOOL_NAMES = {
  ANALYZE_REPO: 'analyze-repo',
  BUILD_IMAGE: 'build-image',
  CONVERT_ACA_TO_K8S: 'convert-aca-to-k8s',
  DEPLOY: 'deploy',
  FIX_DOCKERFILE: 'fix-dockerfile',
  GENERATE_ACA_MANIFESTS: 'generate-aca-manifests',
  GENERATE_DOCKERFILE: 'generate-dockerfile',
  GENERATE_HELM_CHARTS: 'generate-helm-charts',
  GENERATE_K8S_MANIFESTS: 'generate-k8s-manifests',
  GENERATE_KUSTOMIZE: 'generate-kustomize',
  INSPECT_SESSION: 'inspect-session',
  OPS: 'ops',
  PLAN_DOCKERFILE_GENERATION: 'plan-dockerfile-generation',
  PLAN_MANIFEST_GENERATION: 'plan-manifest-generation',
  PREPARE_CLUSTER: 'prepare-cluster',
  PUSH_IMAGE: 'push-image',
  RESOLVE_BASE_IMAGES: 'resolve-base-images',
  SCAN: 'scan',
  TAG_IMAGE: 'tag-image',
  VERIFY_DEPLOY: 'verify-deploy',
} as const;

// Type for valid tool names
export type ToolName = (typeof TOOL_NAMES)[keyof typeof TOOL_NAMES];

// Ensure proper names on all tools
analyzeRepoTool.name = TOOL_NAMES.ANALYZE_REPO;
buildImageTool.name = TOOL_NAMES.BUILD_IMAGE;
convertAcaToK8sTool.name = TOOL_NAMES.CONVERT_ACA_TO_K8S;
deployTool.name = TOOL_NAMES.DEPLOY;
fixDockerfileTool.name = TOOL_NAMES.FIX_DOCKERFILE;
generateAcaManifestsTool.name = TOOL_NAMES.GENERATE_ACA_MANIFESTS;
generateDockerfileTool.name = TOOL_NAMES.GENERATE_DOCKERFILE;
generateHelmChartsTool.name = TOOL_NAMES.GENERATE_HELM_CHARTS;
generateK8sManifestsTool.name = TOOL_NAMES.GENERATE_K8S_MANIFESTS;
generateKustomizeTool.name = TOOL_NAMES.GENERATE_KUSTOMIZE;
inspectSessionTool.name = TOOL_NAMES.INSPECT_SESSION;
opsTool.name = TOOL_NAMES.OPS;
planDockerfileGenerationTool.name = TOOL_NAMES.PLAN_DOCKERFILE_GENERATION;
planManifestGenerationTool.name = TOOL_NAMES.PLAN_MANIFEST_GENERATION;
prepareClusterTool.name = TOOL_NAMES.PREPARE_CLUSTER;
pushImageTool.name = TOOL_NAMES.PUSH_IMAGE;
resolveBaseImagesTool.name = TOOL_NAMES.RESOLVE_BASE_IMAGES;
scanTool.name = TOOL_NAMES.SCAN;
tagImageTool.name = TOOL_NAMES.TAG_IMAGE;
verifyDeployTool.name = TOOL_NAMES.VERIFY_DEPLOY;

// Create a union type of all tool types for better type safety
export type AllToolTypes =
  | typeof analyzeRepoTool
  | typeof buildImageTool
  | typeof convertAcaToK8sTool
  | typeof deployTool
  | typeof fixDockerfileTool
  | typeof generateAcaManifestsTool
  | typeof generateDockerfileTool
  | typeof generateHelmChartsTool
  | typeof generateK8sManifestsTool
  | typeof generateKustomizeTool
  | typeof inspectSessionTool
  | typeof opsTool
  | typeof planDockerfileGenerationTool
  | typeof planManifestGenerationTool
  | typeof prepareClusterTool
  | typeof pushImageTool
  | typeof resolveBaseImagesTool
  | typeof scanTool
  | typeof tagImageTool
  | typeof verifyDeployTool;

// Type-safe tool array using the union type
export const ALL_TOOLS: readonly AllToolTypes[] = [
  analyzeRepoTool,
  buildImageTool,
  convertAcaToK8sTool,
  deployTool,
  fixDockerfileTool,
  generateAcaManifestsTool,
  generateDockerfileTool,
  generateHelmChartsTool,
  generateK8sManifestsTool,
  generateKustomizeTool,
  inspectSessionTool,
  opsTool,
  planDockerfileGenerationTool,
  planManifestGenerationTool,
  prepareClusterTool,
  pushImageTool,
  resolveBaseImagesTool,
  scanTool,
  tagImageTool,
  verifyDeployTool,
] as const;

// Get all tools
export function getAllInternalTools(): readonly AllToolTypes[] {
  return ALL_TOOLS;
}

// Export a type-safe version of "any tool" that's actually the union of all tools
export type InternalTool = AllToolTypes;
