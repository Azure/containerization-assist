import analyzeRepoTool from './analyze-repo/tool';
import buildImageContextTool from './build-image-context/tool';
import fixDockerfileTool from './fix-dockerfile/tool';
import generateDockerfileTool from './generate-dockerfile/tool';
import generateGithubWorkflowTool from './generate-github-workflow/tool';
import generateK8sManifestsTool from './generate-k8s-manifests/tool';
import opsTool from './ops/tool';
import prepareClusterTool from './prepare-cluster/tool';
import pushImageTool from './push-image/tool';
import scanImageTool from './scan-image/tool';
import tagImageTool from './tag-image/tool';
import verifyDeployTool from './verify-deploy/tool';
import { TOOL_NAME, WORKFLOW_TOOL_NAME, ToolName } from './shared/toolDefinition';

export type { ToolName };

// Ensure proper names on all tools
analyzeRepoTool.name = TOOL_NAME.ANALYZE_REPO;
buildImageContextTool.name = TOOL_NAME.BUILD_IMAGE_CONTEXT;
fixDockerfileTool.name = TOOL_NAME.FIX_DOCKERFILE;
generateDockerfileTool.name = TOOL_NAME.GENERATE_DOCKERFILE;
generateGithubWorkflowTool.name = TOOL_NAME.GENERATE_GITHUB_WORKFLOW;
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
  | typeof buildImageContextTool
  | typeof fixDockerfileTool
  | typeof generateDockerfileTool
  | typeof generateGithubWorkflowTool
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
  generateGithubWorkflowTool,
  generateK8sManifestsTool,

  // Operational/deterministic tools
  buildImageContextTool,
  opsTool,
  prepareClusterTool,
  pushImageTool,
  scanImageTool,
  tagImageTool,
  verifyDeployTool,
] as const;

export {
  TOOL_NAME,
  WORKFLOW_TOOL_NAME,
  analyzeRepoTool,
  buildImageContextTool,
  fixDockerfileTool,
  generateDockerfileTool,
  generateGithubWorkflowTool,
  generateK8sManifestsTool,
  opsTool,
  prepareClusterTool,
  pushImageTool,
  scanImageTool,
  tagImageTool,
  verifyDeployTool,
};
