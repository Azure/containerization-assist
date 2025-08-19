const { createTool, z } = require('./_tool-factory');

module.exports = createTool({
  name: 'server_status',
  title: 'Server Status',
  description: 'Get basic server status information',
  inputSchema: {
    details: z.boolean().optional().describe('Include detailed status information')
  }
});