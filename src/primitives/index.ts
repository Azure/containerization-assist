import queryKnowledge from './query-knowledge';
import validateDockerfile from './validate-dockerfile';
import validateK8sManifest from './validate-k8s-manifest';
import validateCompose from './validate-compose';
import type { Tool } from '@/types/tool';

export { queryKnowledge, validateDockerfile, validateK8sManifest, validateCompose };
export * from './types';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export const ALL_PRIMITIVES: readonly Tool<any, any>[] = [
  queryKnowledge,
  validateDockerfile,
  validateK8sManifest,
  validateCompose,
] as const;
