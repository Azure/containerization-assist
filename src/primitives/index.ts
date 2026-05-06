import queryKnowledge from './query-knowledge';
import validate from './validate';
import type { ToolName } from '@/tools';

export { queryKnowledge, validate };
export * from './types';

export type Primitive = (typeof queryKnowledge | typeof validate) & { name: ToolName };

export const ALL_PRIMITIVES: readonly Primitive[] = [queryKnowledge, validate] as const;
