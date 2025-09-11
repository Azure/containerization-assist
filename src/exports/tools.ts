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
import type { Tool } from '../types.js';

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
  ];
}

// Helper to create tool wrapper
const createToolWrapper = (
  name: string,
  description: string,
  schema: any,
  executeFn: (params: any, context: any) => Promise<any>,
  zodSchema?: any, // Add original Zod schema for McpServer compatibility
): Tool => ({
  name,
  description,
  schema,
  zodSchema, // Store the original Zod schema
  execute: async (params, _logger, context) => {
    // Context must be provided by the calling code (ContainerAssistServer)
    if (!context) {
      throw new Error(
        `Context is required for ${name} tool execution. Use ContainerAssistServer for proper integration.`,
      );
    }
    return executeFn(params as any, context);
  },
});

// Create Tool wrappers for all functions
const analyzeRepoTool = createToolWrapper(
  'analyze_repo',
  'Analyze repository structure and detect technologies',
  analyzeRepoSchema.shape,
  analyzeRepo,
  analyzeRepoSchema.shape,
);

const generateDockerfileTool = createToolWrapper(
  'generate_dockerfile',
  'Generate a Dockerfile for the analyzed repository',
  generateDockerfileSchema.shape,
  generateDockerfile,
  generateDockerfileSchema.shape,
);

const buildImageTool = createToolWrapper(
  'build_image',
  'Build a Docker image',
  buildImageSchema.shape,
  buildImage,
  buildImageSchema.shape,
);

const scanImageTool = createToolWrapper(
  'scan_image',
  'Scan a Docker image for vulnerabilities',
  scanImageSchema.shape,
  scanImage,
  scanImageSchema.shape,
);

const tagImageTool = createToolWrapper(
  'tag_image',
  'Tag a Docker image',
  tagImageSchema.shape,
  tagImage,
  tagImageSchema.shape,
);

const pushImageTool = createToolWrapper(
  'push_image',
  'Push a Docker image to a registry',
  pushImageSchema.shape,
  pushImage,
  pushImageSchema.shape,
);

const generateK8sManifestsTool = createToolWrapper(
  'generate_k8s_manifests',
  'Generate Kubernetes manifests',
  generateK8sManifestsSchema.shape,
  generateK8sManifests,
  generateK8sManifestsSchema.shape,
);

const prepareClusterTool = createToolWrapper(
  'prepare_cluster',
  'Prepare Kubernetes cluster for deployment',
  prepareClusterSchema.shape,
  prepareCluster,
  prepareClusterSchema.shape,
);

const deployApplicationTool = createToolWrapper(
  'deploy_application',
  'Deploy application to Kubernetes',
  deployApplicationSchema.shape,
  deployApplication,
  deployApplicationSchema.shape,
);

const verifyDeploymentTool = createToolWrapper(
  'verify_deployment',
  'Verify deployment status',
  verifyDeploymentSchema.shape,
  verifyDeployment,
  verifyDeploymentSchema.shape,
);

const fixDockerfileTool = createToolWrapper(
  'fix_dockerfile',
  'Fix issues in a Dockerfile',
  fixDockerfileSchema.shape,
  fixDockerfile,
  fixDockerfileSchema.shape,
);

const resolveBaseImagesTool = createToolWrapper(
  'resolve_base_images',
  'Resolve and recommend base images',
  resolveBaseImagesSchema.shape,
  resolveBaseImages,
  resolveBaseImagesSchema.shape,
);

const opsToolWrapper = createToolWrapper(
  'ops',
  'Operational utilities',
  opsToolSchema.shape,
  opsTool,
  opsToolSchema.shape,
);

const workflowTool = createToolWrapper(
  'workflow',
  'Execute containerization workflows',
  workflowSchema.shape,
  workflow,
  workflowSchema.shape,
);
