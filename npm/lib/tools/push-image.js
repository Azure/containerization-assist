const { createTool, z } = require('./_tool-factory');

module.exports = createTool({
  name: 'push_image',
  title: 'Push Docker Image',
  description: 'Push the Docker image to a container registry',
  inputSchema: {
    session_id: z.string().describe('Session ID from workflow'),
    registry: z.string().describe('Registry URL to push to'),
    username: z.string().optional().describe('Registry username'),
    password: z.string().optional().describe('Registry password'),
    insecure: z.boolean().optional().describe('Allow insecure registry connections')
  }
});