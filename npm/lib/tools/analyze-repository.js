import { z } from 'zod';
import { executeTool } from '../executor.js';

/**
 * Analyze Repository tool definition for MCP server registration
 */
export const name = 'analyze_repository';
export const metadata = {
  title: 'Analyze Repository',
  description: 'Analyze repository to detect language, framework, and build requirements',
  inputSchema: {
    repo_path: z.string().describe('Path to the repository to analyze'),
    session_id: z.string().optional().describe('Session ID for workflow tracking')
  }
};
export async function handler(params) {
  try {
    const result = await executeTool('analyze_repository', params);

    // Wrap result in MCP format
    return {
      content: [{
        type: 'text',
        text: typeof result === 'string' ? result : JSON.stringify(result)
      }]
    };
  } catch (error) {
    // Return error in MCP format
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
}export default { name, metadata, handler };
