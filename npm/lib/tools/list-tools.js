import { createTool, z } from './_tool-factory.js';

export default createTool({
  name: 'list_tools',
  title: 'List Tools',
  description: 'List all available MCP tools and their descriptions',
  inputSchema: {
    // No required parameters for list_tools
  }
});