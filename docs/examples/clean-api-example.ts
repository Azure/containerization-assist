/**
 * Example: Clean API for Container Assist integration
 * Shows the new instance-based approach without global state
 */

import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { ContainerAssistServer } from '@thgamble/containerization-assist-mcp';

/**
 * Example 1: Simple integration - register all tools
 */
async function simpleIntegration() {
  // Create your MCP server
  const mcpServer = new McpServer({
    name: 'my-mcp-server',
    version: '1.0.0'
  });

  // Create Container Assist instance (no globals!)
  const caServer = new ContainerAssistServer();
  
  // Bind everything at once (sampling + tools)
  caServer.bindAll({ server: mcpServer });
  
  console.log('✅ All Container Assist tools registered with AI sampling support');
  
  // McpServer uses connect() instead of start()
  // await mcpServer.connect(transport);
}

/**
 * Example 2: Selective tool registration
 */
async function selectiveRegistration() {
  const mcpServer = new McpServer({
    name: 'my-selective-server',
    version: '1.0.0'
  });

  const caServer = new ContainerAssistServer();
  
  // Configure sampling separately
  caServer.bindSampling({ server: mcpServer });
  
  // Register only specific tools
  caServer.registerTools(
    { server: mcpServer },
    { 
      tools: ['analyze_repo', 'generate_dockerfile', 'build_image']
    }
  );
  
  console.log('✅ Selected tools registered');
  
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

  const caServer = new ContainerAssistServer();
  
  // Bind with custom names
  caServer.bindSampling({ server: mcpServer });
  caServer.registerTools(
    { server: mcpServer },
    {
      nameMapping: {
        'analyze_repo': 'project_analyze',
        'generate_dockerfile': 'dockerfile_create',
        'build_image': 'docker_build'
      }
    }
  );
  
  console.log('✅ Tools registered with custom names');
  
  // McpServer uses connect() instead of start()
  // await mcpServer.connect(transport);
}

/**
 * Example 4: Multiple independent instances (no global conflicts!)
 */
async function multipleInstances() {
  // Server 1: Development tools
  const devServer = new McpServer({
    name: 'dev-server',
    version: '1.0.0'
  });
  
  const devCA = new ContainerAssistServer({ 
    logger: createCustomLogger('dev') 
  });
  devCA.bindAll({ server: devServer });
  
  // Server 2: Production tools
  const prodServer = new McpServer({
    name: 'prod-server',
    version: '1.0.0'
  });
  
  const prodCA = new ContainerAssistServer({ 
    logger: createCustomLogger('prod') 
  });
  prodCA.bindAll({ server: prodServer });
  
  console.log('✅ Multiple independent Container Assist instances created');
  console.log('   Each has its own session manager and configuration');
  console.log('   No global state conflicts!');
  
  // McpServer uses connect() instead of start()
  // await Promise.all([
  //   devServer.connect(devTransport),
  //   prodServer.connect(prodTransport)
  // ]);
}

/**
 * Example 5: Advanced usage with tool access
 */
async function advancedUsage() {
  const mcpServer = new McpServer({
    name: 'advanced-server',
    version: '1.0.0'
  });

  const caServer = new ContainerAssistServer();
  caServer.bindAll({ server: mcpServer });
  
  // Access specific tools after registration
  const analyzeRepoTool = caServer.getTool('analyze_repo');
  if (analyzeRepoTool) {
    console.log('Tool metadata:', analyzeRepoTool.metadata);
    
    // You can even call the tool directly
    const result = await analyzeRepoTool.handler({
      path: '/path/to/repo'
    });
    console.log('Direct tool call result:', result);
  }
  
  // Get all registered tools
  const allTools = caServer.getAllTools();
  console.log(`Total tools registered: ${allTools.length}`);
  
  // McpServer uses connect() instead of start()
  // await mcpServer.connect(transport);
}

// Helper function for custom logger
function createCustomLogger(name: string) {
  return {
    info: (msg: any, ...args: any[]) => console.log(`[${name}]`, msg, ...args),
    warn: (msg: any, ...args: any[]) => console.warn(`[${name}]`, msg, ...args),
    error: (msg: any, ...args: any[]) => console.error(`[${name}]`, msg, ...args),
    debug: (msg: any, ...args: any[]) => console.debug(`[${name}]`, msg, ...args),
    child: () => createCustomLogger(name)
  } as any;
}

// Run examples
if (require.main === module) {
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