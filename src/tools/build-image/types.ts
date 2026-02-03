import { buildImageSchema } from './schema';
import { TOOL_NAME, type IToolDefinition } from '../shared/toolDefinition';

export const buildImageToolDefinition = {
  name: TOOL_NAME.BUILD_IMAGE,
  description: 'Build Docker images from Dockerfiles with security analysis',
  version: '2.0.0',
  schema: buildImageSchema,
  metadata: {
    knowledgeEnhanced: false,
  },
} satisfies IToolDefinition;
