import { analyzeRepoSchema } from './schema';
import type { IToolDefinition } from '../shared/toolDefinition';

export const analyzeRepoToolDefinition = {
  name: 'analyze-repo' as const,
  description:
    'Analyze repository structure and detect technologies by parsing config files to prepare for containerization and generating container artifacts like Dockerfiles and Kubernetes manifests',
  category: 'analysis' as const,
  version: '4.0.0',
  schema: analyzeRepoSchema,
  metadata: {
    knowledgeEnhanced: false,
  },
  chainHints: {
    success:
      'Repository analysis completed successfully. Continue by calling the generate-dockerfile or fix-dockerfile tools to create or fix your Dockerfile.',
    failure: 'Repository analysis failed. Please check the logs for details.',
  },
} satisfies IToolDefinition<'analyze-repo'>;
