/**
 * Tool Registry - Type-Safe Consolidation
 *
 * Explicit, type-safe imports for all MCP tools.
 * This replaces the distributed tool registration across multiple files.
 */

import type { Tool } from '@/types/tool';

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
import inspectSessionTool from './inspect-session/tool';
import opsTool from './ops/tool';
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
  INSPECT_SESSION: 'inspect-session',
  OPS: 'ops',
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
inspectSessionTool.name = TOOL_NAMES.INSPECT_SESSION;
opsTool.name = TOOL_NAMES.OPS;
prepareClusterTool.name = TOOL_NAMES.PREPARE_CLUSTER;
pushImageTool.name = TOOL_NAMES.PUSH_IMAGE;
resolveBaseImagesTool.name = TOOL_NAMES.RESOLVE_BASE_IMAGES;
scanTool.name = TOOL_NAMES.SCAN;
tagImageTool.name = TOOL_NAMES.TAG_IMAGE;
verifyDeployTool.name = TOOL_NAMES.VERIFY_DEPLOY;

// Type-safe tool array - all tools use the unified Tool interface
export const ALL_TOOLS: readonly Tool<any, any>[] = [
  analyzeRepoTool,
  buildImageTool,
  convertAcaToK8sTool,
  deployTool,
  fixDockerfileTool,
  generateAcaManifestsTool,
  generateDockerfileTool,
  generateHelmChartsTool,
  generateK8sManifestsTool,
  inspectSessionTool,
  opsTool,
  prepareClusterTool,
  pushImageTool,
  resolveBaseImagesTool,
  scanTool,
  tagImageTool,
  verifyDeployTool,
] as const;

// Get all tools (function for consistency with loader pattern)
export function getAllInternalTools(): readonly Tool<any, any>[] {
  return ALL_TOOLS;
}
