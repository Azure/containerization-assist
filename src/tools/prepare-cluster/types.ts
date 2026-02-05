import { prepareClusterSchema } from './schema';
import { TOOL_NAME, type IToolDefinition } from '../shared/toolDefinition';

export const prepareClusterToolDefinition = {
  name: TOOL_NAME.PREPARE_CLUSTER,
  description: 'Prepare Kubernetes cluster for deployment',
  category: 'kubernetes' as const,
  version: '2.0.0',
  schema: prepareClusterSchema,
  metadata: {
    knowledgeEnhanced: false,
  },
  chainHints: {
    success:
      'Cluster preparation successful. Next: Use `kubectl apply -f <manifest-folder>` to deploy your manifests to the cluster, then call verify-deploy to check deployment status.',
    failure:
      'Cluster preparation found issues. Check connectivity, permissions, and namespace configuration.',
  },
} satisfies IToolDefinition;
