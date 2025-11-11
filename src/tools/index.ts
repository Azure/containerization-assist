import analyzeRepoTool from './analyze-repo/tool';
import buildImageTool from './build-image/tool';
import fixDockerfileTool from './fix-dockerfile/tool';
import generateDockerfileTool from './generate-dockerfile/tool';
import generateK8sManifestsTool from './generate-k8s-manifests/tool';
import opsTool from './ops/tool';
import prepareClusterTool from './prepare-cluster/tool';
import pushImageTool from './push-image/tool';
import scanImageTool from './scan-image/tool';
import tagImageTool from './tag-image/tool';
import verifyDeployTool from './verify-deploy/tool';

const TOOL_NAME = {
  ANALYZE_REPO: 'analyze-repo',
  BUILD_IMAGE: 'build-image',
  FIX_DOCKERFILE: 'fix-dockerfile',
  GENERATE_DOCKERFILE: 'generate-dockerfile',
  GENERATE_K8S_MANIFESTS: 'generate-k8s-manifests',
  OPS: 'ops',
  PREPARE_CLUSTER: 'prepare-cluster',
  PUSH_IMAGE: 'push-image',
  SCAN_IMAGE: 'scan-image',
  TAG_IMAGE: 'tag-image',
  VERIFY_DEPLOY: 'verify-deploy',
} as const;

export type ToolName = (typeof TOOL_NAME)[keyof typeof TOOL_NAME];

// Ensure proper names on all tools
analyzeRepoTool.name = TOOL_NAME.ANALYZE_REPO;
buildImageTool.name = TOOL_NAME.BUILD_IMAGE;
fixDockerfileTool.name = TOOL_NAME.FIX_DOCKERFILE;
generateDockerfileTool.name = TOOL_NAME.GENERATE_DOCKERFILE;
generateK8sManifestsTool.name = TOOL_NAME.GENERATE_K8S_MANIFESTS;
opsTool.name = TOOL_NAME.OPS;
prepareClusterTool.name = TOOL_NAME.PREPARE_CLUSTER;
pushImageTool.name = TOOL_NAME.PUSH_IMAGE;
scanImageTool.name = TOOL_NAME.SCAN_IMAGE;
tagImageTool.name = TOOL_NAME.TAG_IMAGE;
verifyDeployTool.name = TOOL_NAME.VERIFY_DEPLOY;

// Create a union type of all tool types for better type safety
export type Tool = (
  | typeof analyzeRepoTool
  | typeof buildImageTool
  | typeof fixDockerfileTool
  | typeof generateDockerfileTool
  | typeof generateK8sManifestsTool
  | typeof opsTool
  | typeof prepareClusterTool
  | typeof pushImageTool
  | typeof scanImageTool
  | typeof tagImageTool
  | typeof verifyDeployTool
) & { name: ToolName };

// Type-safe tool array using the union type
// All tools are now deterministic plan-based or operational tools
export const ALL_TOOLS: readonly Tool[] = [
  // Plan-based generation tools (use knowledge to create plans)
  analyzeRepoTool,
  fixDockerfileTool,
  generateDockerfileTool,
  generateK8sManifestsTool,

  // Operational/deterministic tools
  buildImageTool,
  opsTool,
  prepareClusterTool,
  pushImageTool,
  scanImageTool,
  tagImageTool,
  verifyDeployTool,
] as const;

export {
  TOOL_NAME,
  analyzeRepoTool,
  buildImageTool,
  fixDockerfileTool,
  generateDockerfileTool,
  generateK8sManifestsTool,
  opsTool,
  prepareClusterTool,
  pushImageTool,
  scanImageTool,
  tagImageTool,
  verifyDeployTool,
};
