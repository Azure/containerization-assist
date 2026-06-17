import { prepareClusterSchema } from './schema';
import { TOOL_NAME, type IToolDefinition } from '../shared/toolDefinition';

export const prepareClusterToolDefinition = {
  name: TOOL_NAME.PREPARE_CLUSTER,
  description:
    'Analyze cluster state (read-only) and return a plan describing the exact commands and manifests needed to prepare a Kubernetes cluster for deployment. Does not execute anything; the calling agent runs the plan via its own terminal tools.',
  category: 'kubernetes' as const,
  version: '3.0.0',
  schema: prepareClusterSchema,
  metadata: {
    knowledgeEnhanced: false,
  },
  chainHints: {
    success:
      'Cluster plan generated. Next: run the setup commands from the plan via your terminal tools, apply the manifests (e.g. `kubectl apply -f -`), then call verify-deploy to confirm readiness.',
    failure:
      'Failed to generate a cluster plan. Review connectivity, namespace input, and platform configuration.',
  },
} satisfies IToolDefinition;
