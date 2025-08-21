import { createTool, z } from './_tool-factory.js';

export default createTool({
  name: 'deploy_application',
  title: 'Deploy Application',
  description: 'Deploy the application to Kubernetes',
  inputSchema: {
    session_id: z.string().describe('Session ID from workflow'),
    namespace: z.string().optional().describe('Target namespace'),
    wait: z.boolean().optional().describe('Wait for deployment to be ready'),
    timeout: z.number().optional().describe('Deployment timeout in seconds'),
    dry_run: z.boolean().optional().describe('Perform a dry run')
  }
});