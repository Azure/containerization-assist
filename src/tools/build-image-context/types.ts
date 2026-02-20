import { buildImageSchema } from './schema';
import { TOOL_NAME, type IToolDefinition } from '../shared/toolDefinition';

export const buildImageContextToolDefinition = {
  name: TOOL_NAME.BUILD_IMAGE_CONTEXT,
  description:
    'Prepare Docker build context with security analysis and optimized build commands. Returns structured guidance for executing builds.',
  version: '3.0.0',
  schema: buildImageSchema,
  metadata: {
    knowledgeEnhanced: false,
  },
} satisfies IToolDefinition;
