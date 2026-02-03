import { tagImageSchema } from './schema';
import type { IToolDefinition } from '../shared/toolDefinition';

export const tagImageToolDefinition = {
  name: 'tag-image' as const,
  description: 'Tag Docker images with version and registry information',
  category: 'docker' as const,
  version: '2.0.0',
  schema: tagImageSchema,
  metadata: {
    knowledgeEnhanced: false,
  },
} satisfies IToolDefinition<'tag-image'>;
