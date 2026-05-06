import { TOOL_NAME, type IToolDefinition } from '@/tools/shared/toolDefinition';
import { validateSchema } from './schema';

export const validateToolDefinition = {
  name: TOOL_NAME.VALIDATE,
  description:
    'Validate a containerization artifact (Dockerfile, Kubernetes manifest, or docker-compose YAML) ' +
    'against organizational Rego policies. Returns structured violations (block), warnings, and ' +
    'suggestions with rule and category metadata. The `kind` field tells callers (and future evaluators) ' +
    'what the `content` payload represents. When no policy is loaded, returns a passing empty envelope ' +
    'so callers can degrade gracefully.',
  category: 'analysis' as const,
  version: '1.0.0',
  schema: validateSchema,
  metadata: { knowledgeEnhanced: false },
} satisfies IToolDefinition;
