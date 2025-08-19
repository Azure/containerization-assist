const { createTool, z } = require('./_tool-factory');

module.exports = createTool({
  name: 'generate_k8s_manifests',
  title: 'Generate Kubernetes Manifests',
  description: 'Generate Kubernetes manifests for the application',
  inputSchema: {
    session_id: z.string().describe('Session ID from workflow'),
    namespace: z.string().optional().describe('Kubernetes namespace'),
    replicas: z.number().optional().describe('Number of replicas'),
    port: z.number().optional().describe('Container port'),
    service_type: z.enum(['ClusterIP', 'NodePort', 'LoadBalancer']).optional()
      .describe('Kubernetes service type'),
    ingress: z.boolean().optional().describe('Generate ingress manifest'),
    config_maps: z.record(z.string()).optional().describe('ConfigMap data'),
    secrets: z.record(z.string()).optional().describe('Secret data')
  }
});