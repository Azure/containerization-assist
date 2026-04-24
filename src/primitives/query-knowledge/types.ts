import { TOOL_NAME, type IToolDefinition } from '@/tools/shared/toolDefinition';
import { queryKnowledgeSchema } from './schema';

export const queryKnowledgeToolDefinition = {
  name: TOOL_NAME.QUERY_KNOWLEDGE,
  description:
    'Query the containerization knowledge base by tags. Returns ranked, actionable guidance snippets for Dockerfile generation, K8s manifests, fix recommendations, and scan remediation.',
  category: 'analysis' as const,
  version: '1.0.0',
  schema: queryKnowledgeSchema,
  metadata: {
    knowledgeEnhanced: false,
  },
} satisfies IToolDefinition;
