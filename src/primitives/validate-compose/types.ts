import { TOOL_NAME, type IToolDefinition } from '@/tools/shared/toolDefinition';
import { validateComposeSchema } from './schema';

export const validateComposeToolDefinition = {
  name: TOOL_NAME.VALIDATE_COMPOSE,
  description:
    'Validate a docker-compose file against organizational Rego policies. ' +
    'Returns structured violations, warnings, and suggestions. ' +
    'When no policy is loaded, returns a passing empty envelope so callers can degrade gracefully.',
  category: 'analysis' as const,
  version: '1.0.0',
  schema: validateComposeSchema,
  metadata: { knowledgeEnhanced: false },
} satisfies IToolDefinition;
