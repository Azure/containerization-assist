import { tagImageSchema } from './schema';
import { TOOL_NAME, type IToolDefinition } from '../shared/toolDefinition';

export const tagImageToolDefinition = {
  name: TOOL_NAME.TAG_IMAGE,
  description: 'Tag Docker images with version and registry information',
  category: 'docker' as const,
  version: '2.0.0',
  schema: tagImageSchema,
  metadata: {
    knowledgeEnhanced: false,
  },
} satisfies IToolDefinition;
