const { z } = require('zod');
const { executeTool } = require('../executor');

/**
 * Build Image tool definition for MCP server registration
 */
module.exports = {
  name: 'build_image',
  
  metadata: {
    title: 'Build Docker Image',
    description: 'Build Docker image from generated Dockerfile',
    inputSchema: {
      session_id: z.string().describe('Session ID from workflow'),
      dockerfile: z.string().optional().describe('Path to custom Dockerfile'),
      context: z.string().optional().describe('Build context directory'),
      target: z.string().optional().describe('Target stage for multi-stage builds'),
      build_args: z.record(z.string()).optional().describe('Build arguments'),
      tags: z.array(z.string()).optional().describe('Additional tags for the image'),
      no_cache: z.boolean().optional().describe('Build without using cache')
    }
  },
  
  handler: async (params) => {
    try {
      const result = await executeTool('build_image', params);
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