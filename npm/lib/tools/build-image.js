import { createTool, z } from './_tool-factory.js';

/**
 * Build Image tool definition for MCP server registration
 */
export default createTool({
  name: 'build_image',
  title: 'Build Image',
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
});
