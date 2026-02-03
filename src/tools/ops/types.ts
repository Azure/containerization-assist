import { opsToolSchema } from './schema';
import { TOOL_NAME, type IToolDefinition } from '../shared/toolDefinition';

export const opsToolDefinition = {
  name: TOOL_NAME.OPS,
  description:
    'MCP server diagnostics: ping for connectivity testing, status for health metrics (memory, CPU, uptime). Use this for server monitoring, not application containerization.',
  category: 'utility' as const,
  version: '2.0.0',
  schema: opsToolSchema,
  metadata: {
    knowledgeEnhanced: false,
  },
} satisfies IToolDefinition;
