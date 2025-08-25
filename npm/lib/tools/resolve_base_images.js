import { createTool, z } from './_tool-factory.js';

/**
 * Resolve Base Images tool definition for MCP server registration
 */
export default createTool({
  name: 'resolve_base_images',
  title: 'Resolve Base Images',
  description: 'Resolve recommended base images for creating Dockerfiles based on repository analysis',
  inputSchema: {
    session_id: z.string().describe('Session ID from repository analysis'),
  }
});