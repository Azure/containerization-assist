export const K8S_GENERATION_TEMPLATE = {
    id: "k8s-generation",
    name: "Universal Kubernetes Manifests Generation",
    description: "Generate production-ready Kubernetes manifests for any application",
    version: "2.0.0",
    system: `You are a Kubernetes expert specializing in application deployments for ANY technology stack.
Generate production-ready Kubernetes manifests with proper resource limits, probes, and configurations.

Follow these principles:
1. Use appropriate resource requests and limits
2. Include liveness and readiness probes
3. Configure security contexts
4. Use ConfigMaps for configuration
5. Implement proper labeling and selectors
6. Consider scalability requirements`,
    user: `Generate Kubernetes manifests for:

**Application Details:**
- Image: {{image}}
- Port: {{port}}
- Language: {{language}}
- Framework: {{framework}}
- Replicas: {{replicas}}

**Configuration:**
- Environment: {{environment}}
- Resources: {{resources}}
- Health Check Path: {{healthCheckPath}}

**Requirements:**
1. Deployment with appropriate resource limits for {{language}}
2. Service (ClusterIP type)
3. ConfigMap for environment variables
4. Liveness and readiness probes using {{healthCheckPath}}
5. Security context with non-root user
6. Horizontal Pod Autoscaler if replicas > 3

Generate ONLY the YAML manifests without explanation.`,
    outputFormat: "kubernetes",
    variables: [
        {
            name: "image",
            description: "Container image name and tag",
            required: true
        },
        {
            name: "port",
            description: "Application port",
            required: true,
            default: "8080"
        },
        {
            name: "language",
            description: "Programming language for resource optimization",
            required: true
        },
        {
            name: "framework",
            description: "Application framework",
            required: false
        },
        {
            name: "replicas",
            description: "Number of pod replicas",
            required: false,
            default: "3"
        },
        {
            name: "environment",
            description: "Deployment environment (dev, staging, prod)",
            required: false,
            default: "production"
        },
        {
            name: "resources",
            description: "Resource requirements level (small, medium, large)",
            required: false,
            default: "medium"
        },
        {
            name: "healthCheckPath",
            description: "Health check endpoint path",
            required: false,
            default: "/health"
        }
    ],
    tags: [
        "kubernetes",
        "k8s",
        "deployment",
        "universal",
        "multi-language"
    ]
} as const;