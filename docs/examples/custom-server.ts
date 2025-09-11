/**
 * Example: Building a custom MCP server with Container Assist tools
 * Shows how to integrate tools into your own server implementation
 */

import { 
  tools, 
  registerTool,
  type MCPTool,
  type MCPToolResult 
} from '@thgamble/containerization-assist-mcp';

/**
 * Custom MCP server implementation
 */
class CustomMCPServer {
  private tools: Map<string, MCPTool>;
  private name: string;
  private version: string;
  
  constructor(name: string, version: string) {
    this.name = name;
    this.version = version;
    this.tools = new Map();
  }
  
  /**
   * Register a Container Assist tool
   */
  registerContainerTool(tool: MCPTool, customName?: string): void {
    const name = customName || tool.name;
    this.tools.set(name, tool);
    
    console.log(`Registered tool: ${name}`);
    console.log(`  Title: ${tool.metadata.title}`);
    console.log(`  Description: ${tool.metadata.description}`);
    console.log('');
  }
  
  /**
   * Execute a registered tool
   */
  async executeTool(name: string, params: any): Promise<MCPToolResult> {
    const tool = this.tools.get(name);
    if (!tool) {
      throw new Error(`Tool not found: ${name}`);
    }
    
    console.log(`Executing tool: ${name}`);
    return await tool.handler(params);
  }
  
  /**
   * List all registered tools
   */
  listTools(): string[] {
    return Array.from(this.tools.keys());
  }
  
  /**
   * Get tool metadata
   */
  getToolInfo(name: string): MCPTool | undefined {
    return this.tools.get(name);
  }
  
  /**
   * Start the server (mock implementation)
   */
  async start(): Promise<void> {
    console.log(`Starting ${this.name} v${this.version}`);
    console.log(`Registered tools: ${this.tools.size}`);
    console.log('Server ready!\n');
  }
}

/**
 * Example: Building a containerization-focused server
 */
async function buildContainerizationServer() {
  console.log('=== Custom Containerization Server ===\n');
  
  const server = new CustomMCPServer('container-server', '1.0.0');
  
  // Register only containerization-related tools
  server.registerContainerTool(tools.analyzeRepo);
  server.registerContainerTool(tools.generateDockerfile);
  server.registerContainerTool(tools.buildImage, 'docker_build');
  server.registerContainerTool(tools.scanImage, 'security_scan');
  server.registerContainerTool(tools.tagImage);
  server.registerContainerTool(tools.pushImage);
  
  await server.start();
  
  // Demonstrate tool execution
  console.log('Available tools:', server.listTools().join(', '));
  console.log('');
  
  // Execute a tool
  try {
    const result = await server.executeTool('docker_build', {
      imageId: 'my-app:latest',
      context_path: '/app',
      dockerfile_path: '/app/Dockerfile'
    });
    
    console.log('Build result:', result.content[0]?.text);
  } catch (error) {
    console.log('Build would fail without valid Docker environment');
  }
}

/**
 * Example: Building a Kubernetes deployment server
 */
async function buildKubernetesServer() {
  console.log('\n=== Custom Kubernetes Server ===\n');
  
  const server = new CustomMCPServer('k8s-server', '1.0.0');
  
  // Register only Kubernetes-related tools
  server.registerContainerTool(tools.generateK8sManifests, 'create_manifests');
  server.registerContainerTool(tools.prepareCluster, 'setup_cluster');
  server.registerContainerTool(tools.deployApplication, 'deploy_app');
  server.registerContainerTool(tools.verifyDeployment, 'verify_deploy');
  
  await server.start();
  
  console.log('Available tools:', server.listTools().join(', '));
}

/**
 * Example: Using the custom server with the helper function
 */
async function useWithHelper() {
  console.log('\n=== Using Helper with Custom Server ===\n');
  
  const server = new CustomMCPServer('helper-server', '1.0.0');
  
  // The helper function can work with custom servers too
  // It will use the tools Map directly
  registerTool(server, tools.analyzeRepo);
  registerTool(server, tools.generateDockerfile, 'create_dockerfile');
  
  await server.start();
  
  console.log('Tools registered via helper:', server.listTools().join(', '));
}

// Run examples
if (import.meta.url === `file://${process.argv[1]}`) {
  await buildContainerizationServer();
  await buildKubernetesServer();
  await useWithHelper();
}

export { 
  CustomMCPServer,
  buildContainerizationServer,
  buildKubernetesServer,
  useWithHelper
};