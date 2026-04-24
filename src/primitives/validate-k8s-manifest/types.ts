import { TOOL_NAME, type IToolDefinition } from '@/tools/shared/toolDefinition';
import { validateK8sManifestSchema } from './schema';

export const validateK8sManifestToolDefinition = {
  name: TOOL_NAME.VALIDATE_K8S_MANIFEST,
  description:
    'Validate a Kubernetes manifest YAML against organizational Rego policies. ' +
    'Returns structured violations, warnings, and suggestions. ' +
    'When no policy is loaded, returns a passing empty envelope so callers can degrade gracefully.',
  category: 'analysis' as const,
  version: '1.0.0',
  schema: validateK8sManifestSchema,
  metadata: { knowledgeEnhanced: false },
} satisfies IToolDefinition;
