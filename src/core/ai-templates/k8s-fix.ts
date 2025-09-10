import type { AITemplate } from './types';

export const K8S_FIX: AITemplate = {
  id: 'k8s-fix',
  name: 'Kubernetes Manifest Fix',
  description: 'Fix Kubernetes deployment issues based on error messages',
  version: '1.0.0',
  system:
    'You are a Kubernetes expert specializing in debugging and fixing deployment issues.\nAnalyze the error message and provide corrected Kubernetes manifests.\nFocus on common issues like resource limits, image pull policies, and RBAC.\nEnsure manifests follow best practices and are production-ready.\n',
  user: 'Fix the Kubernetes manifest based on this error:\n\nCurrent Manifest:\n```yaml\n{{manifest}}\n```\n\nError Message:\n{{error}}\n\n{{#if context}}\nAdditional Context:\n{{context}}\n{{/if}}\n\n{{#if clusterInfo}}\nCluster Information:\n{{clusterInfo}}\n{{/if}}\n\nProvide the complete corrected manifest with the issue resolved.\nInclude comments explaining what was fixed.\n',
  variables: [
    {
      name: 'manifest',
      description: 'Current Kubernetes manifest with issues',
      required: true,
    },
    {
      name: 'error',
      description: 'Error message from kubectl or deployment',
      required: true,
    },
    {
      name: 'context',
      description: 'Additional context about the deployment',
      required: false,
    },
    {
      name: 'clusterInfo',
      description: 'Cluster version and configuration',
      required: false,
    },
  ],
  outputFormat: 'yaml',
  examples: [
    {
      input: {
        manifest:
          'apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: app\nspec:\n  selector:\n    matchLabels:\n      app: app\n  template:\n    spec:\n      containers:\n      - name: app\n        image: myapp:latest\n',
        error:
          "error validating data: ValidationError(Deployment.spec.template.metadata): missing required field 'labels'",
      },
      output:
        'apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: app\nspec:\n  replicas: 1  # Added explicit replica count\n  selector:\n    matchLabels:\n      app: app\n  template:\n    metadata:\n      labels:  # Fixed: Added required labels to pod template\n        app: app\n    spec:\n      containers:\n      - name: app\n        image: myapp:v1.0.0  # Fixed: Use specific version instead of latest\n        resources:  # Added: Resource limits for better cluster management\n          requests:\n            memory: "128Mi"\n            cpu: "100m"\n          limits:\n            memory: "256Mi"\n            cpu: "200m"\n',
    },
  ],
  tags: ['kubernetes', 'debugging', 'yaml', 'deployment'],
} as const;
