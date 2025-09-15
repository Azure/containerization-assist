/**
 * Example: Integration with MCP SDK
 * Shows how to register Container Assist tools with an MCP server
 */

import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import {
  createContainerAssistServer,
  registerAllTools,
  registerTool,
  tools
} from '@thgamble/containerization-assist-mcp';

/**
 * Example 1: Register all tools with default names
 */
async function registerAllToolsExample() {
  const server = new McpServer({
    name: 'my-mcp-server',
    version: '1.0.0'
  });
  
  const caServer = createContainerAssistServer();
  caServer.bindAll({ server });
  
  // Start the server
  // McpServer uses connect() instead of start()
  // await server.connect(transport);
}

/**
 * Example 2: Register specific tools with custom names
 */
async function registerCustomToolsExample() {
  
  const server = new McpServer({
    name: 'my-custom-server',
    version: '1.0.0'
  });
  
  // Create Container Assist instance and register specific tools
  const caServer = createContainerAssistServer();
  caServer.bindSampling({ server });
  caServer.registerTools(
    { server },
    {
      tools: ['analyze_repo', 'build_image', 'deploy_application'],
      nameMapping: {
        'analyze_repo': 'analyze_repository',
        'build_image': 'docker_build',
        'deploy_application': 'k8s_deploy'
      }
    }
  );
  
  console.log('Custom tools registered:');
  console.log('- analyze_repository (was: analyze_repo)');
  console.log('- docker_build (was: build_image)');
  console.log('- k8s_deploy (was: deploy_application)\n');
  
  // McpServer uses connect() instead of start()
  // await server.connect(transport);
}

/**
 * Example 3: Register tools with name mapping
 */
async function registerWithMappingExample() {
  console.log('=== Name Mapping Example ===\n');
  
  const server = new McpServer({
    name: 'mapped-server',
    version: '1.0.0'
  });
  
  // Create Container Assist instance
  const caServer = createContainerAssistServer();
  
  // Define custom names for all tools
  const nameMapping = {
    analyze_repo: 'project_analyze',
    generate_dockerfile: 'dockerfile_create',
    build_image: 'image_build',
    scan_image: 'security_scan',
    deploy_application: 'app_deploy',
    verify_deployment: 'deployment_check'
  };
  
  // Register all tools with custom names
  caServer.bindSampling({ server });
  caServer.registerTools({ server }, { nameMapping });
  
  console.log('Tools registered with custom names:');
  Object.entries(nameMapping).forEach(([original, custom]) => {
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