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

export {
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
};

export { TOOL_NAME, type IToolDefinition } from './shared/toolDefinition';
export type { ToolName } from './shared/toolDefinition';

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
