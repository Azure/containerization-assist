const { z } = require('zod');
const { executeTool } = require('../executor');

/**
 * Factory function to create standard tool definitions
 * @param {Object} config - Tool configuration
 * @returns {Object} Tool definition for MCP server registration
 */
function createTool(config) {
  return {
    name: config.name,
    
    metadata: {
      title: config.title,
      description: config.description,
      inputSchema: config.inputSchema
    },
    
    handler: async (params) => {
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
    }
  };
}

module.exports = { createTool, z };