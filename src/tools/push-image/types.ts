import { pushImageSchema } from './schema';
import { TOOL_NAME, type IToolDefinition } from '../shared/toolDefinition';

export const pushImageToolDefinition = {
  name: TOOL_NAME.PUSH_IMAGE,
  description: 'Push a Docker image to a registry',
  category: 'docker' as const,
  version: '2.0.0',
  schema: pushImageSchema,
  metadata: {
    knowledgeEnhanced: false,
  },
  chainHints: {
    success: 'Image pushed successfully. Review AI optimization insights for push improvements.',
    failure:
      'Image push failed. Check registry credentials, network connectivity, and image tag format.',
  },
} satisfies IToolDefinition;
