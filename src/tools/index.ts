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
import opsTool from './ops/tool';
import generateDockerfilePlanTool from './generate-dockerfile-plan/tool';
import generateManifestPlanTool from './generate-manifest-plan/tool';
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
  GENERATE_ACA_MANIFESTS: 'generate-aca-manifests',
  GENERATE_DOCKERFILE: 'generate-dockerfile',
  GENERATE_HELM_CHARTS: 'generate-helm-charts',
  GENERATE_K8S_MANIFESTS: 'generate-k8s-manifests',
  GENERATE_KUSTOMIZE: 'generate-kustomize',
  OPS: 'ops',
  GENERATE_DOCKERFILE_PLAN: 'generate-dockerfile-plan',
  GENERATE_MANIFEST_PLAN: 'generate-manifest-plan',
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
generateAcaManifestsTool.name = TOOL_NAME.GENERATE_ACA_MANIFESTS;
generateDockerfileTool.name = TOOL_NAME.GENERATE_DOCKERFILE;
generateHelmChartsTool.name = TOOL_NAME.GENERATE_HELM_CHARTS;
generateK8sManifestsTool.name = TOOL_NAME.GENERATE_K8S_MANIFESTS;
generateKustomizeTool.name = TOOL_NAME.GENERATE_KUSTOMIZE;
opsTool.name = TOOL_NAME.OPS;
generateDockerfilePlanTool.name = TOOL_NAME.GENERATE_DOCKERFILE_PLAN;
generateManifestPlanTool.name = TOOL_NAME.GENERATE_MANIFEST_PLAN;
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
  | typeof generateAcaManifestsTool
  | typeof generateDockerfileTool
  | typeof generateHelmChartsTool
  | typeof generateK8sManifestsTool
  | typeof generateKustomizeTool
  | typeof opsTool
  | typeof generateDockerfilePlanTool
  | typeof generateManifestPlanTool
  | typeof prepareClusterTool
  | typeof pushImageTool
  | typeof resolveBaseImagesTool
  | typeof scanTool
  | typeof tagImageTool
  | typeof validateDockerfileTool
  | typeof verifyDeployTool
) & { name: string };

// Type-safe tool array using the union type
export const ALL_TOOLS: readonly Tool[] = [
  analyzeRepoTool,
  generateDockerfilePlanTool,
  generateManifestPlanTool,
  validateDockerfileTool,
  // ----- COMING SOON TOOLS ---
  // buildImageTool,
  // convertAcaToK8sTool,
  // deployTool,
  // fixDockerfileTool,
  // generateAcaManifestsTool,
  // generateDockerfileTool,
  // generateHelmChartsTool,
  // generateK8sManifestsTool,
  // generateKustomizeTool,
  // opsTool,
  // prepareClusterTool,
  // pushImageTool,
  // resolveBaseImagesTool,
  // scanTool,
  // tagImageTool,
  // verifyDeployTool,
] as const;

export {
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
  opsTool,
  generateDockerfilePlanTool,
  generateManifestPlanTool,
  prepareClusterTool,
  pushImageTool,
  resolveBaseImagesTool,
  scanTool,
  tagImageTool,
  validateDockerfileTool,
  verifyDeployTool,
};
