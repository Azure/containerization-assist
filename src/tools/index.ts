/**
 * Tool Registry - Type-Safe Consolidation
 *
 * Explicit, type-safe imports for all MCP tools.
 * This replaces the distributed tool registration across multiple files.
 */

import type { Tool } from '@/types';

// Explicit tool imports - organized alphabetically
import { analyzeRepo } from './analyze-repo/tool';
import { analyzeRepoSchema } from './analyze-repo/schema';

import { buildImage } from './build-image/tool';
import { buildImageSchema } from './build-image/schema';

import { convertAcaToK8s } from './convert-aca-to-k8s/tool';
import { convertAcaToK8sSchema } from './convert-aca-to-k8s/schema';

import { deployApplication } from './deploy/tool';
import { deployApplicationSchema } from './deploy/schema';

import { fixDockerfile } from './fix-dockerfile/tool';
import { fixDockerfileSchema } from './fix-dockerfile/schema';

import { generateAcaManifests } from './generate-aca-manifests/tool';
import { generateAcaManifestsSchema } from './generate-aca-manifests/schema';

import { generateDockerfile } from './generate-dockerfile/tool';
import { generateDockerfileSchema } from './generate-dockerfile/schema';

import { generateHelmCharts } from './generate-helm-charts/tool';
import { generateHelmChartsSchema } from './generate-helm-charts/schema';

import { generateK8sManifests } from './generate-k8s-manifests/tool';
import { generateK8sManifestsSchema } from './generate-k8s-manifests/schema';

import { inspectSession } from './inspect-session/tool';
import { InspectSessionParamsSchema as inspectSessionSchema } from './inspect-session/schema';

import { opsTool } from './ops/tool';
import { opsToolSchema } from './ops/schema';

import { prepareCluster } from './prepare-cluster/tool';
import { prepareClusterSchema } from './prepare-cluster/schema';

import { pushImage } from './push-image/tool';
import { pushImageSchema } from './push-image/schema';

import { resolveBaseImages } from './resolve-base-images/tool';
import { resolveBaseImagesSchema } from './resolve-base-images/schema';

import { scanImage } from './scan/tool';
import { scanImageSchema } from './scan/schema';

import { tagImage } from './tag-image/tool';
import { tagImageSchema } from './tag-image/schema';

import { verifyDeployment } from './verify-deployment/tool';
import { verifyDeploymentSchema } from './verify-deployment/schema';

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

// Tool definitions with proper structure
const analyzeRepoTool: Tool = {
  name: TOOL_NAMES.ANALYZE_REPO,
  description: 'Analyze repository structure and detect technologies',
  schema: analyzeRepoSchema.shape,
  zodSchema: analyzeRepoSchema,
  execute: async (params, _logger, context) => {
    if (!context) {
      return { ok: false, error: 'Context is required for tool execution' };
    }
    return analyzeRepo(analyzeRepoSchema.parse(params), context);
  },
};

const buildImageTool: Tool = {
  name: TOOL_NAMES.BUILD_IMAGE,
  description: 'Build a Docker image',
  schema: buildImageSchema.shape,
  zodSchema: buildImageSchema,
  execute: async (params, _logger, context) => {
    if (!context) {
      return { ok: false, error: 'Context is required for tool execution' };
    }
    return buildImage(buildImageSchema.parse(params), context);
  },
};

const convertAcaToK8sTool: Tool = {
  name: TOOL_NAMES.CONVERT_ACA_TO_K8S,
  description: 'Convert Azure Container Apps manifests to Kubernetes',
  schema: convertAcaToK8sSchema.shape,
  zodSchema: convertAcaToK8sSchema,
  execute: async (params, _logger, context) => {
    if (!context) {
      return { ok: false, error: 'Context is required for tool execution' };
    }
    return convertAcaToK8s(convertAcaToK8sSchema.parse(params), context);
  },
};

const deployTool: Tool = {
  name: TOOL_NAMES.DEPLOY,
  description: 'Deploy application to Kubernetes',
  schema: deployApplicationSchema.shape,
  zodSchema: deployApplicationSchema,
  execute: async (params, _logger, context) => {
    if (!context) {
      return { ok: false, error: 'Context is required for tool execution' };
    }
    return deployApplication(deployApplicationSchema.parse(params), context);
  },
};

const fixDockerfileTool: Tool = {
  name: TOOL_NAMES.FIX_DOCKERFILE,
  description:
    'Validate and fix Dockerfile issues including syntax errors, [object Object] problems, and best practices',
  schema: fixDockerfileSchema.shape,
  zodSchema: fixDockerfileSchema,
  execute: async (params, _logger, context) => {
    if (!context) {
      return { ok: false, error: 'Context is required for tool execution' };
    }
    return fixDockerfile(fixDockerfileSchema.parse(params), context);
  },
};

const generateAcaManifestsTool: Tool = {
  name: TOOL_NAMES.GENERATE_ACA_MANIFESTS,
  description: 'Generate Azure Container Apps manifests',
  schema: generateAcaManifestsSchema.shape,
  zodSchema: generateAcaManifestsSchema,
  execute: async (params, _logger, context) => {
    if (!context) {
      return { ok: false, error: 'Context is required for tool execution' };
    }
    return generateAcaManifests(generateAcaManifestsSchema.parse(params), context);
  },
};

const generateDockerfileTool: Tool = {
  name: TOOL_NAMES.GENERATE_DOCKERFILE,
  description: 'Generate a Dockerfile for the analyzed repository',
  schema: generateDockerfileSchema.shape,
  zodSchema: generateDockerfileSchema,
  execute: async (params, _logger, context) => {
    if (!context) {
      return { ok: false, error: 'Context is required for tool execution' };
    }
    return generateDockerfile(generateDockerfileSchema.parse(params), context);
  },
};

const generateHelmChartsTool: Tool = {
  name: TOOL_NAMES.GENERATE_HELM_CHARTS,
  description: 'Generate Helm charts for Kubernetes deployments',
  schema: generateHelmChartsSchema.shape,
  zodSchema: generateHelmChartsSchema,
  execute: async (params, _logger, context) => {
    if (!context) {
      return { ok: false, error: 'Context is required for tool execution' };
    }
    return generateHelmCharts(generateHelmChartsSchema.parse(params), context);
  },
};

const generateK8sManifestsTool: Tool = {
  name: TOOL_NAMES.GENERATE_K8S_MANIFESTS,
  description: 'Generate Kubernetes manifests',
  schema: generateK8sManifestsSchema.shape,
  zodSchema: generateK8sManifestsSchema,
  execute: async (params, _logger, context) => {
    if (!context) {
      return { ok: false, error: 'Context is required for tool execution' };
    }
    return generateK8sManifests(generateK8sManifestsSchema.parse(params), context);
  },
};

const inspectSessionTool: Tool = {
  name: TOOL_NAMES.INSPECT_SESSION,
  description: 'Inspect session data for debugging',
  schema: inspectSessionSchema.shape,
  zodSchema: inspectSessionSchema,
  execute: async (params, _logger, context) => {
    if (!context) {
      return { ok: false, error: 'Context is required for tool execution' };
    }
    return inspectSession(inspectSessionSchema.parse(params), context);
  },
};

const opsToolInstance: Tool = {
  name: TOOL_NAMES.OPS,
  description: 'Operational utilities',
  schema: opsToolSchema.shape,
  zodSchema: opsToolSchema,
  execute: async (params, _logger, context) => {
    if (!context) {
      return { ok: false, error: 'Context is required for tool execution' };
    }
    return opsTool(opsToolSchema.parse(params), context);
  },
};

const prepareClusterTool: Tool = {
  name: TOOL_NAMES.PREPARE_CLUSTER,
  description: 'Prepare Kubernetes cluster for deployment',
  schema: prepareClusterSchema.shape,
  zodSchema: prepareClusterSchema,
  execute: async (params, _logger, context) => {
    if (!context) {
      return { ok: false, error: 'Context is required for tool execution' };
    }
    return prepareCluster(prepareClusterSchema.parse(params), context);
  },
};

const pushImageTool: Tool = {
  name: TOOL_NAMES.PUSH_IMAGE,
  description: 'Push a Docker image to a registry',
  schema: pushImageSchema.shape,
  zodSchema: pushImageSchema,
  execute: async (params, _logger, context) => {
    if (!context) {
      return { ok: false, error: 'Context is required for tool execution' };
    }
    return pushImage(pushImageSchema.parse(params), context);
  },
};

const resolveBaseImagesTool: Tool = {
  name: TOOL_NAMES.RESOLVE_BASE_IMAGES,
  description: 'Resolve and recommend base images',
  schema: resolveBaseImagesSchema.shape,
  zodSchema: resolveBaseImagesSchema,
  execute: async (params, _logger, context) => {
    if (!context) {
      return { ok: false, error: 'Context is required for tool execution' };
    }
    return resolveBaseImages(resolveBaseImagesSchema.parse(params), context);
  },
};

const scanTool: Tool = {
  name: TOOL_NAMES.SCAN,
  description: 'Scan a Docker image for vulnerabilities',
  schema: scanImageSchema.shape,
  zodSchema: scanImageSchema,
  execute: async (params, _logger, context) => {
    if (!context) {
      return { ok: false, error: 'Context is required for tool execution' };
    }
    return scanImage(scanImageSchema.parse(params), context);
  },
};

const tagImageTool: Tool = {
  name: TOOL_NAMES.TAG_IMAGE,
  description: 'Tag a Docker image',
  schema: tagImageSchema.shape,
  zodSchema: tagImageSchema,
  execute: async (params, _logger, context) => {
    if (!context) {
      return { ok: false, error: 'Context is required for tool execution' };
    }
    return tagImage(tagImageSchema.parse(params), context);
  },
};

const verifyDeployTool: Tool = {
  name: TOOL_NAMES.VERIFY_DEPLOY,
  description: 'Verify deployment status',
  schema: verifyDeploymentSchema.shape,
  zodSchema: verifyDeploymentSchema,
  execute: async (params, _logger, context) => {
    if (!context) {
      return { ok: false, error: 'Context is required for tool execution' };
    }
    return verifyDeployment(verifyDeploymentSchema.parse(params), context);
  },
};

// Type-safe tool array - maintains order for consistency
export const ALL_TOOLS = [
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
  opsToolInstance,
  prepareClusterTool,
  pushImageTool,
  resolveBaseImagesTool,
  scanTool,
  tagImageTool,
  verifyDeployTool,
] as const;

// Get all tools (function for consistency with loader pattern)
export function getAllInternalTools(): readonly Tool[] {
  return ALL_TOOLS;
}
