/**
 * Tool definitions (types only) aggregator.
 *
 * This file exports all tool definitions without their handler implementations,
 * providing a lightweight way to import tool metadata without pulling in
 * heavy dependencies like Dockerode, Kubernetes clients, etc.
 *
 * Use this when you only need:
 * - Tool names
 * - Tool descriptions
 * - Tool schemas
 * - Tool metadata
 *
 * For full tool implementations with handlers, import from './index' instead.
 */

export { analyzeRepoToolDefinition } from './analyze-repo/types';
export { buildImageToolDefinition } from './build-image/types';
export { fixDockerfileToolDefinition } from './fix-dockerfile/types';
export { generateDockerfileToolDefinition } from './generate-dockerfile/types';
export { generateK8sManifestsToolDefinition } from './generate-k8s-manifests/types';
export { opsToolDefinition } from './ops/types';
export { prepareClusterToolDefinition } from './prepare-cluster/types';
export { pushImageToolDefinition } from './push-image/types';
export { scanImageToolDefinition } from './scan-image/types';
export { tagImageToolDefinition } from './tag-image/types';
export { verifyDeployToolDefinition } from './verify-deploy/types';

// Re-export the shared type
export type { IToolDefinition } from './shared/toolDefinition';

// Aggregate all tool definitions into a single array
import { analyzeRepoToolDefinition } from './analyze-repo/types';
import { buildImageToolDefinition } from './build-image/types';
import { fixDockerfileToolDefinition } from './fix-dockerfile/types';
import { generateDockerfileToolDefinition } from './generate-dockerfile/types';
import { generateK8sManifestsToolDefinition } from './generate-k8s-manifests/types';
import { opsToolDefinition } from './ops/types';
import { prepareClusterToolDefinition } from './prepare-cluster/types';
import { pushImageToolDefinition } from './push-image/types';
import { scanImageToolDefinition } from './scan-image/types';
import { tagImageToolDefinition } from './tag-image/types';
import { verifyDeployToolDefinition } from './verify-deploy/types';

export const ALL_TOOL_DEFINITIONS = [
  analyzeRepoToolDefinition,
  buildImageToolDefinition,
  fixDockerfileToolDefinition,
  generateDockerfileToolDefinition,
  generateK8sManifestsToolDefinition,
  opsToolDefinition,
  prepareClusterToolDefinition,
  pushImageToolDefinition,
  scanImageToolDefinition,
  tagImageToolDefinition,
  verifyDeployToolDefinition,
] as const;

// Export tool name constants (re-exported from index for convenience)
export const TOOL_NAME = {
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
