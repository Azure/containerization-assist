import { generateDockerfileSchema } from './schema';
import { TOOL_NAME, type IToolDefinition } from '../shared/toolDefinition';

export const generateDockerfileToolDefinition = {
  name: TOOL_NAME.GENERATE_DOCKERFILE,
  description:
    'Gather insights from knowledgebase and return requirements for Dockerfile creation or enhancement. Automatically detects existing Dockerfiles and provides detailed analysis and guidance.',
  category: 'docker' as const,
  version: '2.0.0',
  schema: generateDockerfileSchema,
  metadata: {
    knowledgeEnhanced: true,
  },
  chainHints: {
    success:
      'Dockerfile plan generated successfully and passed policy validation. Next: Use fix-dockerfile to validate the actual Dockerfile content before building.',
    failure:
      'Failed to generate Dockerfile plan or plan violates policies. Review repository analysis and policy violations.',
  },
} satisfies IToolDefinition;
