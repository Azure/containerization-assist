const { createTool, z } = require('./_tool-factory');

module.exports = createTool({
  name: 'prepare_cluster',
  title: 'Prepare Kubernetes Cluster',
  description: 'Prepare the Kubernetes cluster for deployment',
  inputSchema: {
    session_id: z.string().describe('Session ID from workflow'),
    cluster_config: z.object({
      context: z.string().optional(),
      namespace: z.string().optional(),
      create_namespace: z.boolean().optional()
    }).optional().describe('Cluster configuration')
  }
});