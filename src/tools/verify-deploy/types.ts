import { verifyDeploySchema } from './schema';
import { TOOL_NAME, type IToolDefinition } from '../shared/toolDefinition';

export const verifyDeployToolDefinition = {
  name: TOOL_NAME.VERIFY_DEPLOY,
  description: 'Verify Kubernetes deployment status',
  category: 'kubernetes' as const,
  version: '2.0.0',
  schema: verifyDeploySchema,
  metadata: {
    knowledgeEnhanced: false,
  },
} satisfies IToolDefinition;
