import { TOOL_NAME, type IToolDefinition } from '@/tools/shared/toolDefinition';
import { validateDockerfileSchema } from './schema';

export const validateDockerfileToolDefinition = {
  name: TOOL_NAME.VALIDATE_DOCKERFILE,
  description:
    'Validate a Dockerfile against organizational Rego policies. Returns ' +
    'structured violations (block), warnings, and suggestions with line/rule references. ' +
    'When no policy is loaded, returns a passing empty envelope so callers can degrade gracefully.',
  category: 'analysis' as const,
  version: '1.0.0',
  schema: validateDockerfileSchema,
  metadata: { knowledgeEnhanced: false },
} satisfies IToolDefinition;
