import analyzeRepoTool from './analyze-repo/tool';
import buildImageTool from './build-image/tool';
import convertAcaToK8sTool from './convert-aca-to-k8s/tool';
import deployTool from './deploy/tool';
import fixDockerfileTool from './fix-dockerfile/tool';
import generateDockerfileTool from './generate-dockerfile/tool';
import generateK8sManifestsTool from './generate-k8s-manifests/tool';
import opsTool from './ops/tool';
import prepareClusterTool from './prepare-cluster/tool';
import pushImageTool from './push-image/tool';
import resolveBaseImagesTool from './resolve-base-images/tool';
import scanTool from './scan/tool';
import tagImageTool from './tag-image/tool';
import validateDockerfileTool from './validate-dockerfile/tool';
import verifyDeployTool from './verify-deployment/tool';

export const TOOL_NAME = {
  ANALYZE_REPO: 'analyze-repo',
  BUILD_IMAGE: 'build-image',
  CONVERT_ACA_TO_K8S: 'convert-aca-to-k8s',
  DEPLOY: 'deploy',
  FIX_DOCKERFILE: 'fix-dockerfile',
  GENERATE_DOCKERFILE: 'generate-dockerfile',
  GENERATE_K8S_MANIFESTS: 'generate-k8s-manifests',
  OPS: 'ops',
  PREPARE_CLUSTER: 'prepare-cluster',
  PUSH_IMAGE: 'push-image',
  RESOLVE_BASE_IMAGES: 'resolve-base-images',
  SCAN: 'scan',
  TAG_IMAGE: 'tag-image',
  VALIDATE_DOCKERFILE: 'validate-dockerfile',
  VERIFY_DEPLOY: 'verify-deploy',
} as const;

export type ToolName = (typeof TOOL_NAME)[keyof typeof TOOL_NAME];

// Ensure proper names on all tools
analyzeRepoTool.name = TOOL_NAME.ANALYZE_REPO;
buildImageTool.name = TOOL_NAME.BUILD_IMAGE;
convertAcaToK8sTool.name = TOOL_NAME.CONVERT_ACA_TO_K8S;
deployTool.name = TOOL_NAME.DEPLOY;
fixDockerfileTool.name = TOOL_NAME.FIX_DOCKERFILE;
generateDockerfileTool.name = TOOL_NAME.GENERATE_DOCKERFILE;
generateK8sManifestsTool.name = TOOL_NAME.GENERATE_K8S_MANIFESTS;
opsTool.name = TOOL_NAME.OPS;
prepareClusterTool.name = TOOL_NAME.PREPARE_CLUSTER;
pushImageTool.name = TOOL_NAME.PUSH_IMAGE;
resolveBaseImagesTool.name = TOOL_NAME.RESOLVE_BASE_IMAGES;
scanTool.name = TOOL_NAME.SCAN;
tagImageTool.name = TOOL_NAME.TAG_IMAGE;
validateDockerfileTool.name = TOOL_NAME.VALIDATE_DOCKERFILE;
verifyDeployTool.name = TOOL_NAME.VERIFY_DEPLOY;

// Create a union type of all tool types for better type safety
export type Tool = (
  | typeof analyzeRepoTool
  | typeof buildImageTool
  | typeof convertAcaToK8sTool
  | typeof deployTool
  | typeof fixDockerfileTool
  | typeof generateDockerfileTool
  | typeof generateK8sManifestsTool
  | typeof opsTool
  | typeof prepareClusterTool
  | typeof pushImageTool
  | typeof resolveBaseImagesTool
  | typeof scanTool
  | typeof tagImageTool
  | typeof validateDockerfileTool
  | typeof verifyDeployTool
) & { name: string };

// Type-safe tool array using the union type
// All tools are now deterministic plan-based or hybrid tools (as of Phase 4 completion)
export const ALL_TOOLS: readonly Tool[] = [
  // Plan-based generation tools
  analyzeRepoTool,
  fixDockerfileTool,
  generateDockerfileTool,
  generateK8sManifestsTool,
  resolveBaseImagesTool,
  validateDockerfileTool,

  // Operational/deterministic tools
  convertAcaToK8sTool,
  opsTool,
  prepareClusterTool,
  pushImageTool,
  tagImageTool,
  verifyDeployTool,

  // Hybrid tools (deterministic core + optional AI insights)
  buildImageTool,
  deployTool,
  scanTool,
] as const;

export {
  analyzeRepoTool,
  buildImageTool,
  convertAcaToK8sTool,
  deployTool,
  fixDockerfileTool,
  generateDockerfileTool,
  generateK8sManifestsTool,
  opsTool,
  prepareClusterTool,
  pushImageTool,
  resolveBaseImagesTool,
  scanTool,
  tagImageTool,
  validateDockerfileTool,
  verifyDeployTool,
};
