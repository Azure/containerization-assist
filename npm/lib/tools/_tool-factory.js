import { z } from 'zod';
import { executeTool } from '../executor.js';

/**
 * Factory function to create standard tool definitions
 * @param {Object} config - Tool configuration
 * @returns {Object} Tool definition for MCP server registration with name, metadata, handler
 */
function createTool(config) {
  const handler = async (params) => {
    try {
      const result = await executeTool(config.name, params);
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
  };

  return {
    name: config.name,
    metadata: {
      title: config.title || config.name.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase()),
      description: config.description,
      inputSchema: config.inputSchema
    },
    handler
  };
}

export { createTool, z };