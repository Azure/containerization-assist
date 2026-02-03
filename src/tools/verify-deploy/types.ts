import { verifyDeploySchema } from './schema';
import type { IToolDefinition } from '../shared/toolDefinition';

export const verifyDeployToolDefinition = {
  name: 'verify-deploy' as const,
  description: 'Verify Kubernetes deployment status',
  category: 'kubernetes' as const,
  version: '2.0.0',
  schema: verifyDeploySchema,
  metadata: {
    knowledgeEnhanced: false,
  },
} satisfies IToolDefinition<'verify-deploy'>;
