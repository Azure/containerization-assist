import { generateK8sManifestsSchema } from './schema';
import type { IToolDefinition } from '../shared/toolDefinition';

export const generateK8sManifestsToolDefinition = {
  name: 'generate-k8s-manifests' as const,
  description:
    'Gather insights from knowledgebase and return requirements for Kubernetes/Helm/ACA/Kustomize manifest creation. Supports repository analysis or ACA manifest conversion.',
  category: 'kubernetes' as const,
  version: '2.0.0',
  schema: generateK8sManifestsSchema,
  metadata: {
    knowledgeEnhanced: true,
  },
  chainHints: {
    success:
      'Manifest plan generated successfully and passed policy validation. Next: Call prepare-cluster to create a kind cluster to deploy to.',
    failure:
      'Manifest generation failed or plan violates policies. Review manifest requirements and policy violations.',
  },
} satisfies IToolDefinition<'generate-k8s-manifests'>;
