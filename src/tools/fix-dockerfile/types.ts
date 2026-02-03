import { fixDockerfileSchema } from './schema';
import { TOOL_NAME, type IToolDefinition } from '../shared/toolDefinition';

export const fixDockerfileToolDefinition = {
  name: TOOL_NAME.FIX_DOCKERFILE,
  description: 'Analyze Dockerfile for issues and return knowledge-based fix recommendations',
  category: 'docker' as const,
  version: '2.0.0',
  schema: fixDockerfileSchema,
  metadata: {
    knowledgeEnhanced: true,
  },
  chainHints: {
    success:
      'Dockerfile validation and analysis complete (includes built-in best practices + organizational policy validation if configured). Next: Apply recommended fixes, then call build-image to test the Dockerfile.',
    failure:
      'Dockerfile validation failed. Review validation errors, policy violations (if any), and apply recommended fixes.',
  },
} satisfies IToolDefinition;
