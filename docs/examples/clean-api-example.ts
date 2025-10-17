/**
 * Example: Clean API for Container Assist integration
 * Shows the new instance-based approach
 */

import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { createApp, ALL_TOOLS } from 'containerization-assist-mcp';

/**
 * Example 1: Simple integration - register all tools
 */
async function simpleIntegration() {
  // Create your MCP server
  const mcpServer = new McpServer({
    name: 'my-mcp-server',
    version: '1.0.0'
  });

  // Create Container Assist app
  const app = createApp();

  // Bind all tools to the MCP server
  app.bindToMCP(mcpServer);

  console.log('✅ All Container Assist tools registered');
  console.log(`Total tools: ${app.listTools().length}`);

  // McpServer uses connect() instead of start()
  // await mcpServer.connect(transport);
}

/**
 * Example 2: Selective tool registration with aliases
 */
async function selectiveRegistration() {
  const mcpServer = new McpServer({
    name: 'my-selective-server',
    version: '1.0.0'
  });

  // Create app with only specific tools and custom names
  const selectedTools = ['analyze-repo', 'generate-dockerfile', 'build-image'];

  const app = createApp({
    tools: ALL_TOOLS.filter(tool => selectedTools.includes(tool.name)),
    toolAliases: {
      'analyze-repo': 'project_analyze',
      'generate-dockerfile': 'dockerfile_create',
      'build-image': 'image_build'
    },
    chainHintsMode: 'enabled'
  });

  app.bindToMCP(mcpServer);

  console.log('✅ Selected tools registered with custom names');
  app.listTools().forEach(tool => {
    console.log(`- ${tool.name}`);
  });

  // McpServer uses connect() instead of start()
  // await mcpServer.connect(transport);
}

/**
 * Example 3: Custom tool names
 */
async function customToolNames() {
  const mcpServer = new McpServer({
    name: 'custom-names-server',
    version: '1.0.0'
  });

  const app = createApp({
    toolAliases: {
      'analyze-repo': 'project_analyze',
      'generate-dockerfile': 'dockerfile_create',
      'build-image': 'docker_build'
    }
  });

  app.bindToMCP(mcpServer);

  console.log('✅ Tools registered with custom names:');
  const customNames = ['project_analyze', 'dockerfile_create', 'docker_build'];
  app.listTools()
    .filter(tool => customNames.includes(tool.name))
    .forEach(tool => console.log(`- ${tool.name}`));

  // McpServer uses connect() instead of start()
  // await mcpServer.connect(transport);
}

/**
 * Example 4: Multiple independent instances
 */
async function multipleInstances() {
  // Server 1: Development tools
  const devServer = new McpServer({
    name: 'dev-server',
    version: '1.0.0'
  });

  const devApp = createApp({
    toolAliases: {
      'analyze-repo': 'dev_analyze',
      'build-image': 'dev_build'
    }
  });
  devApp.bindToMCP(devServer);

  // Server 2: Production tools
  const prodServer = new McpServer({
    name: 'prod-server',
    version: '1.0.0'
  });

  const prodApp = createApp({
    toolAliases: {
      'deploy': 'prod_deploy',
      'verify-deploy': 'prod_verify'
    }
  });
  prodApp.bindToMCP(prodServer);

  console.log('✅ Multiple independent Container Assist instances created');
  console.log('   Each has its own configuration and tool names');
  console.log(`   Dev tools: ${devApp.listTools().length}`);
  console.log(`   Prod tools: ${prodApp.listTools().length}`);

  // McpServer uses connect() instead of start()
  // await Promise.all([
  //   devServer.connect(devTransport),
  //   prodServer.connect(prodTransport)
  // ]);
}

/**
 * Example 5: Advanced usage with direct tool access
 */
async function advancedUsage() {
  const mcpServer = new McpServer({
    name: 'advanced-server',
    version: '1.0.0'
  });

  const app = createApp({
    toolAliases: {
      'analyze-repo': 'advanced_analyze'
    }
  });

  app.bindToMCP(mcpServer);

  // Access app functionality
  console.log('✅ Advanced Container Assist setup:');

  // Health check
  const health = app.healthCheck();
  console.log(`Health: ${health.status} - ${health.message}`);

  // List all tools
  const allTools = app.listTools();
  console.log(`Total tools registered: ${allTools.length}`);

  // Show some example tools with their descriptions
  console.log('\nExample tools:');
  allTools.slice(0, 3).forEach(tool => {
    console.log(`- ${tool.name}: ${tool.description}`);
  });

  // Direct tool execution (if needed)
  try {
    const result = await app.execute('advanced_analyze', {
      path: '/example/repo'
    });
    if (result.ok) {
      console.log('\n✅ Direct tool execution successful');
    }
  } catch (error) {
    console.log('\n⚠️ Direct execution would require proper setup');
  }

  // McpServer uses connect() instead of start()
  // await mcpServer.connect(transport);
}

// Run examples
if (import.meta.url === `file://${process.argv[1]}`) {
  console.log('Container Assist - Clean API Examples\n');
  console.log('Choose an example to run:');
  console.log('1. Simple integration');
  console.log('2. Selective registration');
  console.log('3. Custom tool names');
  console.log('4. Multiple instances');
  console.log('5. Advanced usage');

  const example = process.argv[2] || '1';

  switch (example) {
    case '1':
      simpleIntegration().catch(console.error);
      break;
    case '2':
      selectiveRegistration().catch(console.error);
      break;
    case '3':
      customToolNames().catch(console.error);
      break;
    case '4':
      multipleInstances().catch(console.error);
      break;
    case '5':
      advancedUsage().catch(console.error);
      break;
    default:
      console.log('Invalid example number');
  }
}