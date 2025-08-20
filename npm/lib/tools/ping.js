import { createTool, z } from './_tool-factory.js';

export default createTool({
  name: 'ping',
  title: 'Ping',
  description: 'Simple ping tool to test MCP connectivity',
  inputSchema: {
    message: z.string().optional().describe('Optional message to echo back')
  }
});