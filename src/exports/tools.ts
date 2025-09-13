/**
 * Tool collection and registry for external consumption
 */

// Import all tool implementations
import { analyzeRepo } from '../tools/analyze-repo/tool.js';
import { analyzeRepoSchema } from '../tools/analyze-repo/schema.js';
import { generateDockerfile } from '../tools/generate-dockerfile/tool.js';
import { generateDockerfileSchema } from '../tools/generate-dockerfile/schema.js';
import { buildImage } from '../tools/build-image/tool.js';
import { buildImageSchema } from '../tools/build-image/schema.js';
import { scanImage } from '../tools/scan/tool.js';
import { scanImageSchema } from '../tools/scan/schema.js';
import { tagImage } from '../tools/tag-image/tool.js';
import { tagImageSchema } from '../tools/tag-image/schema.js';
import { pushImage } from '../tools/push-image/index.js';
import { pushImageSchema } from '../tools/push-image/schema.js';
import { generateK8sManifests } from '../tools/generate-k8s-manifests/tool.js';
import { generateK8sManifestsSchema } from '../tools/generate-k8s-manifests/schema.js';
import { prepareCluster } from '../tools/prepare-cluster/tool.js';
import { prepareClusterSchema } from '../tools/prepare-cluster/schema.js';
import { deployApplication } from '../tools/deploy/tool.js';
import { deployApplicationSchema } from '../tools/deploy/schema.js';
import { verifyDeployment } from '../tools/verify-deployment/tool.js';
import { verifyDeploymentSchema } from '../tools/verify-deployment/schema.js';
import { fixDockerfile } from '../tools/fix-dockerfile/tool.js';
import { fixDockerfileSchema } from '../tools/fix-dockerfile/schema.js';
import { resolveBaseImages } from '../tools/resolve-base-images/tool.js';
import { resolveBaseImagesSchema } from '../tools/resolve-base-images/schema.js';
import { opsTool } from '../tools/ops/tool.js';
import { opsToolSchema } from '../tools/ops/schema.js';
import { workflow } from '../tools/workflow/tool.js';
import { workflowSchema } from '../tools/workflow/schema.js';
import { generateAcaManifests } from '../tools/generate-aca-manifests/tool.js';
import { generateAcaManifestsSchema } from '../tools/generate-aca-manifests/schema.js';
import { convertAcaToK8s } from '../tools/convert-aca-to-k8s/tool.js';
import { convertAcaToK8sSchema } from '../tools/convert-aca-to-k8s/schema.js';
import { generateHelmCharts } from '../tools/generate-helm-charts/tool.js';
import { generateHelmChartsSchema } from '../tools/generate-helm-charts/schema.js';
import type { Tool, Result } from '../types';
import type { ZodObject, ZodRawShape } from 'zod';

/**
 * Get all internal tool implementations
 * Used by ContainerAssistServer for registration
 */
export function getAllInternalTools(): Tool[] {
  return [
    analyzeRepoTool,
    generateDockerfileTool,
    buildImageTool,
    scanImageTool,
    tagImageTool,
    pushImageTool,
    generateK8sManifestsTool,
    prepareClusterTool,
    deployApplicationTool,
    verifyDeploymentTool,
    fixDockerfileTool,
    resolveBaseImagesTool,
    opsToolWrapper,
    workflowTool,
    generateAcaManifestsTool,
    convertAcaToK8sTool,
    generateHelmChartsTool,
  ];
}

/**
 * Get all available tool names
 * Useful for selective tool registration
 */
export function getAllToolNames(): string[] {
  return getAllInternalTools().map((tool) => tool.name);
}

/**
 * Tool names as constants for type-safe registration
 * Use these instead of raw strings when registering specific tools
 */
export const TOOL_NAMES = {
  ANALYZE_REPO: 'analyze_repo',
  GENERATE_DOCKERFILE: 'generate_dockerfile',
  BUILD_IMAGE: 'build_image',
  SCAN_IMAGE: 'scan_image',
  TAG_IMAGE: 'tag_image',
  PUSH_IMAGE: 'push_image',
  GENERATE_K8S_MANIFESTS: 'generate_k8s_manifests',
  PREPARE_CLUSTER: 'prepare_cluster',
  DEPLOY_APPLICATION: 'deploy_application',
  VERIFY_DEPLOYMENT: 'verify_deployment',
  FIX_DOCKERFILE: 'fix_dockerfile',
  RESOLVE_BASE_IMAGES: 'resolve_base_images',
  OPS: 'ops',
  WORKFLOW: 'workflow',
  GENERATE_ACA_MANIFESTS: 'generate_aca_manifests',
  CONVERT_ACA_TO_K8S: 'convert_aca_to_k8s',
  GENERATE_HELM_CHARTS: 'generate_helm_charts',
} as const;

/**
 * Type for valid tool names
 */
export type ToolName = (typeof TOOL_NAMES)[keyof typeof TOOL_NAMES];

// Helper to create tool wrapper
const createToolWrapper = (
  name: string,
  description: string,
  zodSchema: ZodObject<ZodRawShape>, // Pass the Zod object schema
  executeFn: (params: unknown, context: unknown) => Promise<Result<unknown>>,
): Tool => ({
  name,
  description,
  schema: zodSchema.shape, // Extract .shape for JSON schema
  zodSchema: zodSchema.shape, // Extract .shape for McpServer (ZodRawShape)
  execute: async (params, _logger, context) => {
    // Context must be provided by the calling code (ContainerAssistServer)
    if (!context) {
      return {
        ok: false,
        error: `Context is required for ${name} tool execution. Use ContainerAssistServer for proper integration.`,
      };
    }
    return executeFn(params, context);
  },
});

// Create Tool wrappers for all functions
const analyzeRepoTool = createToolWrapper(
  TOOL_NAMES.ANALYZE_REPO,
  'Analyze repository structure and detect technologies',
  analyzeRepoSchema,
  analyzeRepo as (params: unknown, context: unknown) => Promise<Result<unknown>>,
);

const generateDockerfileTool = createToolWrapper(
  TOOL_NAMES.GENERATE_DOCKERFILE,
  'Generate a Dockerfile for the analyzed repository',
  generateDockerfileSchema,
  generateDockerfile as (params: unknown, context: unknown) => Promise<Result<unknown>>,
);

const buildImageTool = createToolWrapper(
  TOOL_NAMES.BUILD_IMAGE,
  'Build a Docker image',
  buildImageSchema,
  buildImage as (params: unknown, context: unknown) => Promise<Result<unknown>>,
);

const scanImageTool = createToolWrapper(
  TOOL_NAMES.SCAN_IMAGE,
  'Scan a Docker image for vulnerabilities',
  scanImageSchema,
  scanImage as (params: unknown, context: unknown) => Promise<Result<unknown>>,
);

const tagImageTool = createToolWrapper(
  TOOL_NAMES.TAG_IMAGE,
  'Tag a Docker image',
  tagImageSchema,
  tagImage as (params: unknown, context: unknown) => Promise<Result<unknown>>,
);

const pushImageTool = createToolWrapper(
  TOOL_NAMES.PUSH_IMAGE,
  'Push a Docker image to a registry',
  pushImageSchema,
  pushImage as (params: unknown, context: unknown) => Promise<Result<unknown>>,
);

const generateK8sManifestsTool = createToolWrapper(
  TOOL_NAMES.GENERATE_K8S_MANIFESTS,
  'Generate Kubernetes manifests',
  generateK8sManifestsSchema,
  generateK8sManifests as (params: unknown, context: unknown) => Promise<Result<unknown>>,
);

const prepareClusterTool = createToolWrapper(
  TOOL_NAMES.PREPARE_CLUSTER,
  'Prepare Kubernetes cluster for deployment',
  prepareClusterSchema,
  prepareCluster as (params: unknown, context: unknown) => Promise<Result<unknown>>,
);

const deployApplicationTool = createToolWrapper(
  TOOL_NAMES.DEPLOY_APPLICATION,
  'Deploy application to Kubernetes',
  deployApplicationSchema,
  deployApplication as (params: unknown, context: unknown) => Promise<Result<unknown>>,
);

const verifyDeploymentTool = createToolWrapper(
  TOOL_NAMES.VERIFY_DEPLOYMENT,
  'Verify deployment status',
  verifyDeploymentSchema,
  verifyDeployment as (params: unknown, context: unknown) => Promise<Result<unknown>>,
);

const fixDockerfileTool = createToolWrapper(
  TOOL_NAMES.FIX_DOCKERFILE,
  'Fix issues in a Dockerfile',
  fixDockerfileSchema,
  fixDockerfile as (params: unknown, context: unknown) => Promise<Result<unknown>>,
);

const resolveBaseImagesTool = createToolWrapper(
  TOOL_NAMES.RESOLVE_BASE_IMAGES,
  'Resolve and recommend base images',
  resolveBaseImagesSchema,
  resolveBaseImages as (params: unknown, context: unknown) => Promise<Result<unknown>>,
);

const opsToolWrapper = createToolWrapper(
  TOOL_NAMES.OPS,
  'Operational utilities',
  opsToolSchema,
  opsTool as (params: unknown, context: unknown) => Promise<Result<unknown>>,
);

const workflowTool = createToolWrapper(
  TOOL_NAMES.WORKFLOW,
  'Execute containerization workflows',
  workflowSchema,
  workflow as (params: unknown, context: unknown) => Promise<Result<unknown>>,
);

const generateAcaManifestsTool = createToolWrapper(
  TOOL_NAMES.GENERATE_ACA_MANIFESTS,
  'Generate Azure Container Apps manifests',
  generateAcaManifestsSchema,
  generateAcaManifests as (params: unknown, context: unknown) => Promise<Result<unknown>>,
);

const convertAcaToK8sTool = createToolWrapper(
  TOOL_NAMES.CONVERT_ACA_TO_K8S,
  'Convert Azure Container Apps manifests to Kubernetes',
  convertAcaToK8sSchema,
  convertAcaToK8s as (params: unknown, context: unknown) => Promise<Result<unknown>>,
);

const generateHelmChartsTool = createToolWrapper(
  TOOL_NAMES.GENERATE_HELM_CHARTS,
  'Generate Helm charts for Kubernetes deployments',
  generateHelmChartsSchema,
  generateHelmCharts as (params: unknown, context: unknown) => Promise<Result<unknown>>,
);
