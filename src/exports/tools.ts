/**
 * Tool collection and registry for external consumption
 */

// Import all tool implementations
import { analyzeRepo } from '@tools/analyze-repo/tool';
import { analyzeRepoSchema } from '@tools/analyze-repo/schema';
import { tool as generateDockerfileTool } from '@tools/generate-dockerfile/tool';
import { generateDockerfileSchema } from '@tools/generate-dockerfile/schema';
import { buildImage } from '@tools/build-image/tool';
import { buildImageSchema } from '@tools/build-image/schema';
import { tool as scanImageToolImport } from '@tools/scan/tool';
import { scanImageSchema } from '@tools/scan/schema';
import { createTagImageTool as tagImageToolImport } from '@tools/tag-image/tool';
import { tagImageSchema } from '@tools/tag-image/schema';
import { createPushImageTool as pushImageToolImport } from '@tools/push-image/tool';
import { pushImageSchema } from '@tools/push-image/schema';
import { tool as generateK8sManifestsTool } from '@tools/generate-k8s-manifests/tool';
import { generateK8sManifestsSchema } from '@tools/generate-k8s-manifests/schema';
import { prepareCluster } from '@tools/prepare-cluster/tool';
import { prepareClusterSchema } from '@tools/prepare-cluster/schema';
import { deployApplication } from '@tools/deploy/tool';
import { deployApplicationSchema } from '@tools/deploy/schema';
import { verifyDeploy } from '@tools/verify-deploy/tool';
import { verifyDeploymentSchema } from '@tools/verify-deploy/schema';
import { tool as fixDockerfile } from '@tools/fix-dockerfile/tool';
import { fixDockerfileSchema } from '@tools/fix-dockerfile/schema';
import { tool as resolveBaseImages } from '@tools/resolve-base-images/tool';
import { resolveBaseImagesSchema } from '@tools/resolve-base-images/schema';
import { createOpsTool as opsToolImport } from '@tools/ops/tool';
import { opsToolSchema } from '@tools/ops/schema';
import { tool as generateAcaManifestsTool } from '@tools/generate-aca-manifests/tool';
import { generateAcaManifestsSchema } from '@tools/generate-aca-manifests/schema';
import { convertAcaToK8s } from '@tools/convert-aca-to-k8s/tool';
import { convertAcaToK8sSchema } from '@tools/convert-aca-to-k8s/schema';
import { tool as generateHelmChartsTool } from '@tools/generate-helm-charts/tool';
import { generateHelmChartsSchema } from '@tools/generate-helm-charts/schema';
import { createInspectSessionTool as inspectSessionToolImport } from '@tools/inspect-session/tool';
import { InspectSessionParamsSchema as inspectSessionSchema } from '@tools/inspect-session/schema';
import type { Tool, Result, ToolContext } from '@types';
import type { ZodObject, ZodRawShape } from 'zod';
import type { Logger } from 'pino';

// Tool execution function type
type ToolExecuteFn = (
  params: unknown,
  logger: Logger,
  context?: ToolContext,
) => Promise<Result<unknown>>;

/**
 * Get all internal tool implementations
 * Used by ContainerAssistServer for registration
 */
export function getAllInternalTools(): Tool[] {
  return [
    analyzeRepoTool,
    generateDockerfileToolWrapper,
    buildImageTool,
    scanImageTool,
    tagImageTool,
    pushImageTool,
    generateK8sManifestsToolWrapper,
    prepareClusterTool,
    deployApplicationTool,
    verifyDeploymentTool,
    fixDockerfileTool,
    resolveBaseImagesTool,
    opsToolWrapper,
    generateAcaManifestsToolWrapper,
    convertAcaToK8sTool,
    generateHelmChartsToolWrapper,
    inspectSessionTool,
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
 * Using kebab-case naming convention as standard
 */
export const TOOLS = {
  ANALYZE_REPO: 'analyze-repo',
  GENERATE_DOCKERFILE: 'generate-dockerfile',
  BUILD_IMAGE: 'build-image',
  SCAN_IMAGE: 'scan',
  TAG_IMAGE: 'tag-image',
  PUSH_IMAGE: 'push-image',
  GENERATE_K8S_MANIFESTS: 'generate-k8s-manifests',
  PREPARE_CLUSTER: 'prepare-cluster',
  DEPLOY: 'deploy',
  VERIFY_DEPLOYMENT: 'verify-deploy',
  FIX_DOCKERFILE: 'fix-dockerfile',
  RESOLVE_BASE_IMAGES: 'resolve-base-images',
  OPS: 'ops',
  GENERATE_ACA_MANIFESTS: 'generate-aca-manifests',
  CONVERT_ACA_TO_K8S: 'convert-aca-to-k8s',
  GENERATE_HELM_CHARTS: 'generate-helm-charts',
  INSPECT_SESSION: 'inspect-session',
} as const;

/**
 * Type for valid tool names
 */
export type ToolName = (typeof TOOLS)[keyof typeof TOOLS];

// Helper to create tool wrapper
const createToolWrapper = (
  name: ToolName,
  description: string,
  zodSchema: ZodObject<ZodRawShape>, // Pass the Zod object schema
  executeFn: ToolExecuteFn,
): Tool => ({
  name,
  description,
  schema: zodSchema.shape, // Extract .shape for JSON schema
  zodSchema: zodSchema.shape, // Extract .shape for McpServer (ZodRawShape)
  execute: async (params, logger, context) => {
    // Context must be provided by the calling code (ContainerAssistServer)
    if (!context) {
      return {
        ok: false,
        error: `Context is required for ${name} tool execution. Use ContainerAssistServer for proper integration.`,
      };
    }
    return executeFn(params, logger, context);
  },
});

// Create Tool wrappers for all functions
const analyzeRepoTool = createToolWrapper(
  TOOLS.ANALYZE_REPO,
  'Analyze repository structure and detect technologies',
  analyzeRepoSchema,
  async (params: unknown, _logger: Logger, context?: ToolContext): Promise<Result<unknown>> => {
    if (!context) {
      return { ok: false, error: 'Context required' };
    }
    const validationResult = analyzeRepoSchema.safeParse(params);
    if (!validationResult.success) {
      return { ok: false, error: `Invalid parameters: ${validationResult.error.message}` };
    }
    return analyzeRepo.execute(validationResult.data, context);
  },
);

const generateDockerfileToolWrapper = createToolWrapper(
  TOOLS.GENERATE_DOCKERFILE,
  'Generate a Dockerfile for the analyzed repository',
  generateDockerfileSchema,
  async (params: unknown, _logger: Logger, context?: ToolContext): Promise<Result<unknown>> => {
    if (!context) {
      return { ok: false, error: 'Context required' };
    }
    const validationResult = generateDockerfileSchema.safeParse(params);
    if (!validationResult.success) {
      return { ok: false, error: `Invalid parameters: ${validationResult.error.message}` };
    }
    return generateDockerfileTool.execute(validationResult.data, context);
  },
);

const buildImageTool = createToolWrapper(
  TOOLS.BUILD_IMAGE,
  'Build a Docker image',
  buildImageSchema,
  async (params: unknown, _logger: Logger, context?: ToolContext): Promise<Result<unknown>> => {
    if (!context) {
      return { ok: false, error: 'Context required' };
    }
    const validationResult = buildImageSchema.safeParse(params);
    if (!validationResult.success) {
      return { ok: false, error: `Invalid parameters: ${validationResult.error.message}` };
    }
    return buildImage.execute(validationResult.data, context);
  },
);

const scanImageTool = createToolWrapper(
  TOOLS.SCAN_IMAGE,
  'Scan a Docker image for vulnerabilities',
  scanImageSchema,
  async (params: unknown, _logger: Logger, context?: ToolContext): Promise<Result<unknown>> => {
    if (!context) {
      return { ok: false, error: 'Context required' };
    }
    const validationResult = scanImageSchema.safeParse(params);
    if (!validationResult.success) {
      return { ok: false, error: `Invalid parameters: ${validationResult.error.message}` };
    }
    return scanImageToolImport.execute(validationResult.data, context);
  },
);

const tagImageTool = createToolWrapper(
  TOOLS.TAG_IMAGE,
  'Tag a Docker image',
  tagImageSchema,
  async (params: unknown, _logger: Logger, context?: ToolContext): Promise<Result<unknown>> => {
    if (!context) {
      return { ok: false, error: 'Context required' };
    }
    const validationResult = tagImageSchema.safeParse(params);
    if (!validationResult.success) {
      return { ok: false, error: `Invalid parameters: ${validationResult.error.message}` };
    }
    const { createDockerClient } = await import('@services/docker-client');
    const deps = {
      docker: createDockerClient(_logger),
      logger: _logger,
    };
    const tool = tagImageToolImport(deps);
    return tool(validationResult.data, context);
  },
);

const pushImageTool = createToolWrapper(
  TOOLS.PUSH_IMAGE,
  'Push a Docker image to a registry',
  pushImageSchema,
  async (params: unknown, _logger: Logger, context?: ToolContext): Promise<Result<unknown>> => {
    if (!context) {
      return { ok: false, error: 'Context required' };
    }
    const validationResult = pushImageSchema.safeParse(params);
    if (!validationResult.success) {
      return { ok: false, error: `Invalid parameters: ${validationResult.error.message}` };
    }
    const { createDockerClient } = await import('@services/docker-client');
    const deps = {
      docker: createDockerClient(_logger),
      logger: _logger,
    };
    const tool = pushImageToolImport(deps);
    return tool(validationResult.data, context);
  },
);

const generateK8sManifestsToolWrapper = createToolWrapper(
  TOOLS.GENERATE_K8S_MANIFESTS,
  'Generate Kubernetes manifests',
  generateK8sManifestsSchema,
  async (params: unknown, _logger: Logger, context?: ToolContext): Promise<Result<unknown>> => {
    if (!context) {
      return { ok: false, error: 'Context required' };
    }
    const validationResult = generateK8sManifestsSchema.safeParse(params);
    if (!validationResult.success) {
      return { ok: false, error: `Invalid parameters: ${validationResult.error.message}` };
    }
    return generateK8sManifestsTool.execute(validationResult.data, context);
  },
);

const prepareClusterTool = createToolWrapper(
  TOOLS.PREPARE_CLUSTER,
  'Prepare Kubernetes cluster for deployment',
  prepareClusterSchema,
  async (params: unknown, _logger: Logger, context?: ToolContext): Promise<Result<unknown>> => {
    if (!context) {
      return { ok: false, error: 'Context required' };
    }
    const validationResult = prepareClusterSchema.safeParse(params);
    if (!validationResult.success) {
      return { ok: false, error: `Invalid parameters: ${validationResult.error.message}` };
    }
    return prepareCluster.execute(validationResult.data, context);
  },
);

const deployApplicationTool = createToolWrapper(
  TOOLS.DEPLOY,
  'Deploy application to Kubernetes',
  deployApplicationSchema,
  async (params: unknown, _logger: Logger, context?: ToolContext): Promise<Result<unknown>> => {
    if (!context) {
      return { ok: false, error: 'Context required' };
    }
    const validationResult = deployApplicationSchema.safeParse(params);
    if (!validationResult.success) {
      return { ok: false, error: `Invalid parameters: ${validationResult.error.message}` };
    }
    return deployApplication.execute(validationResult.data, context);
  },
);

const verifyDeploymentTool = createToolWrapper(
  TOOLS.VERIFY_DEPLOYMENT,
  'Verify deployment status',
  verifyDeploymentSchema,
  async (params: unknown, _logger: Logger, context?: ToolContext): Promise<Result<unknown>> => {
    if (!context) {
      return { ok: false, error: 'Context required' };
    }
    const validationResult = verifyDeploymentSchema.safeParse(params);
    if (!validationResult.success) {
      return { ok: false, error: `Invalid parameters: ${validationResult.error.message}` };
    }
    return verifyDeploy.execute(validationResult.data, context);
  },
);

const fixDockerfileTool = createToolWrapper(
  TOOLS.FIX_DOCKERFILE,
  'Fix issues in a Dockerfile',
  fixDockerfileSchema,
  async (params: unknown, _logger: Logger, context?: ToolContext): Promise<Result<unknown>> => {
    if (!context) {
      return { ok: false, error: 'Context required' };
    }
    const validationResult = fixDockerfileSchema.safeParse(params);
    if (!validationResult.success) {
      return { ok: false, error: `Invalid parameters: ${validationResult.error.message}` };
    }
    return fixDockerfile.execute(validationResult.data, context);
  },
);

const resolveBaseImagesTool = createToolWrapper(
  TOOLS.RESOLVE_BASE_IMAGES,
  'Resolve and recommend base images',
  resolveBaseImagesSchema,
  async (params: unknown, _logger: Logger, context?: ToolContext): Promise<Result<unknown>> => {
    if (!context) {
      return { ok: false, error: 'Context required' };
    }
    const validationResult = resolveBaseImagesSchema.safeParse(params);
    if (!validationResult.success) {
      return { ok: false, error: `Invalid parameters: ${validationResult.error.message}` };
    }
    return resolveBaseImages.execute(validationResult.data, context);
  },
);

const opsToolWrapper = createToolWrapper(
  TOOLS.OPS,
  'Operational utilities',
  opsToolSchema,
  async (params: unknown, _logger: Logger, context?: ToolContext): Promise<Result<unknown>> => {
    if (!context) {
      return { ok: false, error: 'Context required' };
    }
    const validationResult = opsToolSchema.safeParse(params);
    if (!validationResult.success) {
      return { ok: false, error: `Invalid parameters: ${validationResult.error.message}` };
    }
    const deps = { logger: _logger };
    const tool = opsToolImport(deps);
    return tool(validationResult.data, context);
  },
);

const generateAcaManifestsToolWrapper = createToolWrapper(
  TOOLS.GENERATE_ACA_MANIFESTS,
  'Generate Azure Container Apps manifests',
  generateAcaManifestsSchema,
  async (params: unknown, _logger: Logger, context?: ToolContext): Promise<Result<unknown>> => {
    if (!context) {
      return { ok: false, error: 'Context required' };
    }
    const validationResult = generateAcaManifestsSchema.safeParse(params);
    if (!validationResult.success) {
      return { ok: false, error: `Invalid parameters: ${validationResult.error.message}` };
    }
    return generateAcaManifestsTool.execute(validationResult.data, context);
  },
);

const convertAcaToK8sTool = createToolWrapper(
  TOOLS.CONVERT_ACA_TO_K8S,
  'Convert Azure Container Apps manifests to Kubernetes',
  convertAcaToK8sSchema,
  async (params: unknown, _logger: Logger, context?: ToolContext): Promise<Result<unknown>> => {
    if (!context) {
      return { ok: false, error: 'Context required' };
    }
    const validationResult = convertAcaToK8sSchema.safeParse(params);
    if (!validationResult.success) {
      return { ok: false, error: `Invalid parameters: ${validationResult.error.message}` };
    }
    return convertAcaToK8s.execute(validationResult.data, context);
  },
);

const generateHelmChartsToolWrapper = createToolWrapper(
  TOOLS.GENERATE_HELM_CHARTS,
  'Generate Helm charts for Kubernetes deployments',
  generateHelmChartsSchema,
  async (params: unknown, _logger: Logger, context?: ToolContext): Promise<Result<unknown>> => {
    if (!context) {
      return { ok: false, error: 'Context required' };
    }
    const validationResult = generateHelmChartsSchema.safeParse(params);
    if (!validationResult.success) {
      return { ok: false, error: `Invalid parameters: ${validationResult.error.message}` };
    }
    return generateHelmChartsTool.execute(validationResult.data, context);
  },
);

const inspectSessionTool = createToolWrapper(
  TOOLS.INSPECT_SESSION,
  'Inspect session data for debugging',
  inspectSessionSchema,
  async (params: unknown, _logger: Logger, context?: ToolContext): Promise<Result<unknown>> => {
    if (!context) {
      return { ok: false, error: 'Context required' };
    }
    const validationResult = inspectSessionSchema.safeParse(params);
    if (!validationResult.success) {
      return { ok: false, error: `Invalid parameters: ${validationResult.error.message}` };
    }
    const deps = { logger: _logger };
    const tool = inspectSessionToolImport(deps);
    return tool(validationResult.data, context);
  },
);
