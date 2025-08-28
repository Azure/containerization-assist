import { createTool, z } from './_tool-factory.js';

/**
 * Verify Dockerfile tool definition for MCP server registration
 */
export default createTool({
  name: 'verify_dockerfile',
  title: 'Verify Dockerfile',
  description: 'Verify that an AI agent has generated an optimized Dockerfile based on repository analysis',
  inputSchema: {
    session_id: z.string().describe('Session ID from repository analysis'),
    base_image: z.string().optional().describe('Custom base image to use'),
    optimization_level: z.enum(['minimal', 'standard', 'aggressive']).optional()
      .describe('Level of optimization to apply')
  }
});
