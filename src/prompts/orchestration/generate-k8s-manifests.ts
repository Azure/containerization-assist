import type { PromptFile } from '../../core/prompts/static-loader';

export const generateK8sManifests: PromptFile = {
  metadata: {
    name: 'generate-k8s-manifests',
    category: 'orchestration',
    description: 'Generate Kubernetes manifests for containerized applications',
    version: '1.0',
    parameters: [
      {
        name: 'appName',
        type: 'string',
        required: true,
        description: 'Application name for the deployment',
      },
      {
        name: 'imageId',
        type: 'string',
        required: true,
        description: 'Docker image to deploy',
      },
      {
        name: 'namespace',
        type: 'string',
        required: false,
        description: 'Kubernetes namespace (defaults to default)',
      },
      {
        name: 'replicas',
        type: 'number',
        required: false,
        description: 'Number of replicas to deploy',
      },
      {
        name: 'ports',
        type: 'string',
        required: false,
        description: 'Comma-separated port numbers to expose',
      },
      {
        name: 'environment',
        type: 'string',
        required: false,
        description: 'Target environment (development, staging, production)',
      },
      {
        name: 'resources',
        type: 'string',
        required: false,
        description: 'Resource limits and requests specification',
      },
      {
        name: 'securityLevel',
        type: 'string',
        required: false,
        description: 'Security level (standard, strict)',
      },
      {
        name: 'highAvailability',
        type: 'boolean',
        required: false,
        description: 'Enable high availability features',
      },
    ],
  },
  template: `Generate production-ready Kubernetes manifests for containerized application.

Application Details:
- Name: {{appName}}
- Image: {{imageId}}
{{#namespace}}
- Namespace: {{namespace}}
{{/namespace}}
{{#replicas}}
- Replicas: {{replicas}}
{{/replicas}}
{{#ports}}
- Ports: {{ports}}
{{/ports}}
{{#environment}}
- Environment: {{environment}}
{{/environment}}

{{#resources}}
Resource Requirements:
{{resources}}
{{/resources}}

Configuration:
{{#securityLevel}}
- Security Level: {{securityLevel}}
{{/securityLevel}}
{{#highAvailability}}
- High Availability: enabled
{{/highAvailability}}

Generate complete YAML manifests with the following requirements:

1. **Deployment Manifest:**
   - Use appropriate resource limits and requests
   - Include health checks (readiness/liveness probes)
   - Set security contexts (non-root user when possible)
   - Use proper labeling and selectors
   - Include restart policies

2. **Service Manifest:**
   - Appropriate service type for the environment
   - Proper port configuration
   - Correct selectors matching deployment labels

3. **Additional Manifests (if requested):**
   - ConfigMap for configuration (if needed)
   - Ingress for external access (if production environment)
   - HorizontalPodAutoscaler for scaling (if production)
   - PodDisruptionBudget for high availability
   - NetworkPolicy for security (if strict security level)

Best Practices:
- Use specific image tags (avoid :latest in production)
- Set resource limits to prevent resource starvation
- Include proper labels for monitoring and management
- Use namespaces for environment isolation
- Enable security contexts for better security posture
- Include annotations for better observability

Return only the YAML manifests separated by "---" without explanation or code fences.
`,
};
