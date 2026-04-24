import queryKnowledge from './query-knowledge';
import validateDockerfile from './validate-dockerfile';
import validateK8sManifest from './validate-k8s-manifest';
import validateCompose from './validate-compose';
import type { ToolName } from '@/tools';

export { queryKnowledge, validateDockerfile, validateK8sManifest, validateCompose };
export * from './types';

export type Primitive = (
  | typeof queryKnowledge
  | typeof validateDockerfile
  | typeof validateK8sManifest
  | typeof validateCompose
) & { name: ToolName };

export const ALL_PRIMITIVES: readonly Primitive[] = [
  queryKnowledge,
  validateDockerfile,
  validateK8sManifest,
  validateCompose,
] as const;
