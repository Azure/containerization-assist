import type { PromptFile } from '../../core/prompts/static-loader';

export const k8sManifestGeneration: PromptFile = {
  metadata: {
    name: 'k8s-manifest-generation',
    category: 'orchestration',
    description: 'Generate Kubernetes deployment manifests with best practices',
    version: '1.0',
    parameters: [
      {
        name: 'appName',
        type: 'string',
        required: true,
        description: 'Application name for the deployment',
      },
      {
        name: 'replicas',
        type: 'number',
        required: false,
        description: 'Number of replicas',
      },
      {
        name: 'environment',
        type: 'string',
        required: false,
        description: 'Target environment (development, staging, production)',
      },
      {
        name: 'resourceLimits',
        type: 'string',
        required: false,
        description: 'CPU and memory limits',
      },
      {
        name: 'highAvailability',
        type: 'boolean',
        required: false,
        description: 'Enable high availability features (anti-affinity, PDBs)',
      },
      {
        name: 'securityContext',
        type: 'boolean',
        required: false,
        description: 'Apply strict security contexts and policies',
      },
    ],
  },
  template: `Generate Kubernetes deployment manifests for application "{{appName}}".

Configuration:
- Environment: {{environment}}
- Replicas: {{replicas}}
{{#resourceLimits}}- Resource limits: {{resourceLimits}}{{/resourceLimits}}
{{#highAvailability}}- High availability: enabled{{/highAvailability}}
{{#securityContext}}- Security context: strict{{/securityContext}}

Include:
- Deployment
- Service
- ConfigMap (if needed)
- Ingress (if applicable)
- HorizontalPodAutoscaler (for scaling)
- PodDisruptionBudget (for availability)

Follow Kubernetes best practices and security guidelines.
`,
};
