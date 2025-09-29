/**
 * Example: Building a custom MCP server with Container Assist tools
 * Shows how to integrate tools into your own server implementation
 */

import {
  createApp,
  type Tool
} from '@thgamble/containerization-assist-mcp';

/**
 * Custom MCP server implementation
 */
class CustomMCPServer {
  private tools: Map<string, Tool>;
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
  registerContainerTool(tool: Tool, customName?: string): void {
    const name = customName || tool.name;
    const aliasedTool = customName ? { ...tool, name: customName } : tool;
    this.tools.set(name, aliasedTool);

    console.log(`Registered tool: ${name}`);
    console.log(`  Description: ${tool.description}`);
    console.log('');
  }

  /**
   * Execute a registered tool
   */
  async executeTool(name: string, params: any): Promise<any> {
    const tool = this.tools.get(name);
    if (!tool) {
      throw new Error(`Tool not found: ${name}`);
    }

    console.log(`Executing tool: ${name}`);
    // Note: In a real implementation, you'd need to create proper context
    // return await tool.run(params, context);
    return { success: true, message: `Would execute ${name}` };
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
  getToolInfo(name: string): Tool | undefined {
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

  // Get tools from Container Assist
  const app = createApp();
  const availableTools = app.listTools();

  // Register only containerization-related tools with custom names
  const containerTools = availableTools.filter(tool =>
    ['analyze-repo', 'generate-dockerfile', 'build-image', 'scan', 'tag-image', 'push-image'].includes(tool.name)
  );

  // Mock registration since we don't have the actual tool objects in this context
  server.registerContainerTool({ name: 'analyze-repo', description: 'Analyze repository structure' } as any);
  server.registerContainerTool({ name: 'generate-dockerfile', description: 'Generate Dockerfile' } as any, 'dockerfile_create');
  server.registerContainerTool({ name: 'build-image', description: 'Build Docker image' } as any, 'docker_build');
  server.registerContainerTool({ name: 'scan', description: 'Security scan' } as any, 'security_scan');
  server.registerContainerTool({ name: 'tag-image', description: 'Tag Docker image' } as any);
  server.registerContainerTool({ name: 'push-image', description: 'Push Docker image' } as any);

  await server.start();

  console.log('Available tools:', server.listTools().join(', '));
  console.log('');

  // Demonstrate tool execution
  try {
    const result = await server.executeTool('docker_build', {
      imageId: 'my-app:latest',
      context_path: '/app',
      dockerfile_path: '/app/Dockerfile'
    });

    console.log('Build result:', result.message);
  } catch (error) {
    console.log('Build would require proper implementation');
  }
}

/**
 * Example: Building a Kubernetes deployment server
 */
async function buildKubernetesServer() {
  console.log('\n=== Custom Kubernetes Server ===\n');

  const server = new CustomMCPServer('k8s-server', '1.0.0');

  // Mock register Kubernetes-related tools with custom names
  server.registerContainerTool({ name: 'generate-k8s-manifests', description: 'Generate Kubernetes manifests' } as any, 'create_manifests');
  server.registerContainerTool({ name: 'prepare-cluster', description: 'Prepare cluster' } as any, 'setup_cluster');
  server.registerContainerTool({ name: 'deploy', description: 'Deploy application' } as any, 'deploy_app');
  server.registerContainerTool({ name: 'verify-deploy', description: 'Verify deployment' } as any, 'verify_deploy');

  await server.start();

  console.log('Available tools:', server.listTools().join(', '));
}

/**
 * Example: Integration with Container Assist app
 */
async function useWithContainerAssist() {
  console.log('\n=== Using Container Assist Integration ===\n');

  const server = new CustomMCPServer('integrated-server', '1.0.0');

  // Create Container Assist app with custom aliases
  const app = createApp({
    toolAliases: {
      'analyze-repo': 'custom_analyze',
      'generate-dockerfile': 'create_dockerfile'
    }
  });

  // Get the aliased tools and register them
  const tools = app.listTools();
  console.log(`Available Container Assist tools: ${tools.length}`);

  // Mock registration of first few tools
  tools.slice(0, 2).forEach(tool => {
    server.registerContainerTool(tool as any);
  });

  await server.start();

  console.log('Tools registered via Container Assist:', server.listTools().join(', '));
}

// Run examples
if (import.meta.url === `file://${process.argv[1]}`) {
  await buildContainerizationServer();
  await buildKubernetesServer();
  await useWithContainerAssist();
}

export {
  CustomMCPServer,
  buildContainerizationServer,
  buildKubernetesServer,
  useWithContainerAssist
};