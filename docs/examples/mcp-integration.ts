/**
 * Example: Integration with MCP SDK
 * Shows how to use Container Assist with an MCP server
 */

import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { createApp } from 'containerization-assist-mcp';

/**
 * Example 1: Register all tools with default names
 */
async function registerAllToolsExample() {
  const server = new McpServer({
    name: 'my-mcp-server',
    version: '1.0.0'
  });

  const app = createApp();
  app.bindToMCP(server);

  console.log('✅ All Container Assist tools registered with default names');

  // List tools
  const tools = app.listTools();
  console.log(`Registered ${tools.length} tools:`);
  tools.forEach(tool => console.log(`- ${tool.name}`));

  // McpServer uses connect() instead of start()
  // await server.connect(transport);
}

/**
 * Example 2: Register tools with custom names
 */
async function registerCustomToolsExample() {
  const server = new McpServer({
    name: 'my-custom-server',
    version: '1.0.0'
  });

  // Create app with custom tool names
  const app = createApp({
    toolAliases: {
      'analyze-repo': 'analyze_repository',
      'build-image': 'docker_build',
      'deploy': 'k8s_deploy'
    }
  });

  app.bindToMCP(server);

  console.log('Custom tools registered:');
  console.log('- analyze_repository (was: analyze-repo)');
  console.log('- docker_build (was: build-image)');
  console.log('- k8s_deploy (was: deploy)\n');

  // McpServer uses connect() instead of start()
  // await server.connect(transport);
}

/**
 * Example 3: Register tools with comprehensive name mapping
 */
async function registerWithMappingExample() {
  console.log('=== Name Mapping Example ===\n');

  const server = new McpServer({
    name: 'mapped-server',
    version: '1.0.0'
  });

  // Define custom names for tools
  const app = createApp({
    toolAliases: {
      'analyze-repo': 'project_analyze',
      'generate-dockerfile': 'dockerfile_create',
      'build-image': 'image_build',
      'scan': 'security_scan',
      'deploy': 'app_deploy',
      'verify-deploy': 'deployment_check'
    }
  });

  app.bindToMCP(server);

  console.log('Tools registered with custom names:');
  const aliases = {
    'analyze-repo': 'project_analyze',
    'generate-dockerfile': 'dockerfile_create',
    'build-image': 'image_build',
    'scan': 'security_scan',
    'deploy': 'app_deploy',
    'verify-deploy': 'deployment_check'
  };

  Object.entries(aliases).forEach(([original, custom]) => {
    console.log(`- ${custom} (was: ${original})`);
  });
  console.log('');

  // McpServer uses connect() instead of start()
  // await server.connect(transport);
}

// Run examples (choose one)
if (import.meta.url === `file://${process.argv[1]}`) {
  const example = process.argv[2] || 'all';

  switch (example) {
    case 'all':
      await registerAllToolsExample();
      break;
    case 'custom':
      await registerCustomToolsExample();
      break;
    case 'mapping':
      await registerWithMappingExample();
      break;
    default:
      console.log('Usage: tsx mcp-integration.ts [all|custom|mapping]');
  }
}

export {
  registerAllToolsExample,
  registerCustomToolsExample,
  registerWithMappingExample
};