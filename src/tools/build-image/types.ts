import { buildImageSchema } from './schema';
import type { IToolDefinition } from '../shared/toolDefinition';

export const buildImageToolDefinition = {
  name: 'build-image' as const,
  description: 'Build Docker images from Dockerfiles with security analysis',
  version: '2.0.0',
  schema: buildImageSchema,
  metadata: {
    knowledgeEnhanced: false,
  },
} satisfies IToolDefinition<'build-image'>;
