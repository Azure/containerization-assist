const { createTool, z } = require('./_tool-factory');

module.exports = createTool({
  name: 'list_tools',
  title: 'List Tools',
  description: 'List all available MCP tools and their descriptions',
  inputSchema: {
    // No required parameters for list_tools
  }
});