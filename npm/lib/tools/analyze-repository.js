import { createTool, z } from './_tool-factory.js';

/**
 * Analyze Repository tool definition for MCP server registration
 */
export default createTool({
  name: 'analyze_repository',
  title: 'Analyze Repository',
  description: 'Analyze repository to detect language, framework, and build requirements',
  inputSchema: {
    repo_path: z.string().describe('Path to the repository to analyze'),
    session_id: z.string().optional().describe('Session ID for workflow tracking')
  }
});
