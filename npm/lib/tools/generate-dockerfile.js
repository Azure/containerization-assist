const { z } = require('zod');
const { executeTool } = require('../executor');

/**
 * Generate Dockerfile tool definition for MCP server registration
 */
module.exports = {
  name: 'generate_dockerfile',
  
  metadata: {
    title: 'Generate Dockerfile',
    description: 'Generate an optimized Dockerfile based on repository analysis',
    inputSchema: {
      session_id: z.string().describe('Session ID from repository analysis'),
      base_image: z.string().optional().describe('Custom base image to use'),
      optimization_level: z.enum(['minimal', 'standard', 'aggressive']).optional()
        .describe('Level of optimization to apply')
    }
  },
  
  handler: async (params) => {
    try {
      const result = await executeTool('generate_dockerfile', params);
      return {
        content: [{
          type: 'text',
          text: typeof result === 'string' ? result : JSON.stringify(result)
        }]
      };
    } catch (error) {
      return {
        content: [{
          type: 'text',
          text: JSON.stringify({
            success: false,
            error: error.message
          })
        }]
      };
    }
  }
};