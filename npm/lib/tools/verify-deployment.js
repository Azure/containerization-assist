const { createTool, z } = require('./_tool-factory');

module.exports = createTool({
  name: 'verify_deployment',
  title: 'Verify Deployment',
  description: 'Verify deployment health with automatic port forwarding and application testing',
  inputSchema: {
    session_id: z.string().describe('Session ID from workflow'),
    health_check_path: z.string().optional().describe('Path for health check endpoint'),
    port_forward: z.boolean().optional().describe('Enable automatic port forwarding'),
    test_endpoints: z.array(z.string()).optional().describe('Endpoints to test')
  }
});