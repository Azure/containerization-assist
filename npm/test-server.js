#!/usr/bin/env node

/**
 * Container Kit MCP Tools - Development Test Server
 * 
 * This test server is used for development and testing of the Container Kit
 * npm package. It can run in two modes:
 * 
 * 1. Mock Mode (default) - Tests without MCP SDK dependency
 * 2. Real Mode - Uses actual MCP SDK if installed
 * 
 * Usage:
 *   # Run in mock mode (no dependencies needed)
 *   node test-server.js
 * 
 *   # Run with real MCP SDK (requires @modelcontextprotocol/sdk)
 *   node test-server.js --real
 * 
 *   # Test specific tools
 *   node test-server.js --tools ping,list_tools
 * 
 *   # Test with custom names
 *   node test-server.js --custom-names
 * 
 * Development Testing:
 *   # After making changes to tools, rebuild and test:
 *   npm run build:current
 *   node test-server.js
 */

import containerKit from './lib/index.js';

const args = process.argv.slice(2);
const useRealSDK = args.includes('--real');
const useCustomNames = args.includes('--custom-names');
const toolsArg = args.find(arg => arg.startsWith('--tools='));
const selectedTools = toolsArg ? toolsArg.split('=')[1].split(',') : null;

// Mock MCP Server for testing without SDK dependency
class MockMcpServer {
  constructor(config) {
    this.name = config.name;
    this.version = config.version;
    this.tools = new Map();
    console.log(`\n📦 Creating MCP Server: ${this.name} v${this.version}`);
  }
  
  // Mimics MCP SDK's addTool method
  addTool(definition, handler) {
    console.log(`  ➕ Adding tool: ${definition.name}`);
    this.tools.set(definition.name, { definition, handler });
  }
  
  // Mock registerTool for testing our helper
  registerTool(name, metadata, handler) {
    console.log(`  ✅ Registering tool: ${name}`);
    this.tools.set(name, { 
      definition: { name, ...metadata }, 
      handler 
    });
  }
  
  async callTool(name, params = {}) {
    const tool = this.tools.get(name);
    if (!tool) {
      throw new Error(`Tool not found: ${name}`);
    }
    
    console.log(`\n🔧 Calling tool: ${name}`);
    if (Object.keys(params).length > 0) {
      console.log(`   Parameters:`, params);
    }
    
    try {
      const result = await tool.handler(params);
      console.log(`   ✅ Success`);
      if (result?.content?.[0]?.text) {
        const text = result.content[0].text;
        try {
          const parsed = JSON.parse(text);
          console.log(`   Result:`, JSON.stringify(parsed, null, 2).split('\n').slice(0, 10).join('\n'));
        } catch {
          console.log(`   Result:`, text.substring(0, 200));
        }
      }
      return result;
    } catch (error) {
      console.log(`   ❌ Error:`, error.message);
      throw error;
    }
  }
  
  listTools() {
    console.log(`\n📋 Registered tools (${this.tools.size}):`);
    const toolList = Array.from(this.tools.entries());
    toolList.forEach(([name, tool]) => {
      console.log(`   • ${name}: ${tool.definition.description || tool.definition.title || 'No description'}`);
    });
  }
  
  getStats() {
    const stats = {
      totalTools: this.tools.size,
      categories: {
        workflow: 0,
        utility: 0,
        custom: 0
      }
    };
    
    this.tools.forEach((tool, name) => {
      if (name.includes('repository') || name.includes('docker') || 
          name.includes('image') || name.includes('k8s') || 
          name.includes('deploy') || name.includes('cluster')) {
        stats.categories.workflow++;
      } else if (name === 'ping' || name === 'list_tools' || name === 'server_status') {
        stats.categories.utility++;
      } else {
        stats.categories.custom++;
      }
    });
    
    return stats;
  }
}

async function runTests(server) {
  console.log('\n' + '='.repeat(60));
  console.log('🧪 Running Tool Tests');
  console.log('='.repeat(60));
  
  const testsToRun = selectedTools || ['ping', 'list_tools', 'server_status'];
  
  for (const toolName of testsToRun) {
    try {
      if (toolName === 'ping') {
        await server.callTool('ping', { message: 'test from dev server' });
      } else if (toolName === 'list_tools') {
        await server.callTool('list_tools', {});
      } else if (toolName === 'server_status') {
        await server.callTool('server_status', { details: true });
      } else if (toolName === 'analyze_repository' || toolName === 'analyze') {
        const actualName = server.tools.has('analyze') ? 'analyze' : 'analyze_repository';
        await server.callTool(actualName, {
          repo_path: '.',
          session_id: containerKit.createSession()
        });
      } else {
        console.log(`\n⚠️  Skipping ${toolName} (requires full server context)`);
      }
    } catch (error) {
      // Error already logged in callTool
    }
  }
}

async function main() {
  console.log('='.repeat(60));
  console.log('🚀 Container Kit MCP Tools - Development Test Server');
  console.log('='.repeat(60));
  
  let server;
  
  if (useRealSDK) {
    console.log('\n📌 Mode: Real MCP SDK\n');
    try {
      // Try to load real MCP SDK using dynamic import
      const { Server } = await import('@modelcontextprotocol/sdk/server/index.js');
      server = new Server({
        name: 'container-kit-test',
        version: '1.0.0'
      });
      console.log('✅ Using real MCP SDK Server');
    } catch (error) {
      console.log('❌ MCP SDK not found, falling back to mock server');
      console.log('   Install with: npm install @modelcontextprotocol/sdk\n');
      server = new MockMcpServer({
        name: 'container-kit-test',
        version: '1.0.0'
      });
    }
  } else {
    console.log('\n📌 Mode: Mock Server (no dependencies)\n');
    server = new MockMcpServer({
      name: 'container-kit-test',
      version: '1.0.0'
    });
  }
  
  // Register all tools
  console.log('='.repeat(60));
  console.log('📚 Registering Container Kit Tools');
  console.log('='.repeat(60));
  
  if (useCustomNames) {
    console.log('\n📝 Using custom tool names\n');
    
    // Test custom naming
    containerKit.registerTool(server, containerKit.analyzeRepository, 'analyze');
    containerKit.registerTool(server, containerKit.generateDockerfile, 'dockerfile');
    containerKit.registerTool(server, containerKit.buildImage, 'build');
    containerKit.registerTool(server, containerKit.ping, 'test-ping');
    containerKit.registerTool(server, containerKit.listTools, 'tools');
    containerKit.registerTool(server, containerKit.serverStatus, 'status');
  } else {
    console.log('\n📝 Using default tool names\n');
    
    // Register all tools with default names
    containerKit.registerAllTools(server);
  }
  
  // Show registered tools
  server.listTools();
  
  // Show stats
  const stats = server.getStats();
  console.log('\n📊 Registration Summary:');
  console.log(`   Total tools: ${stats.totalTools}`);
  console.log(`   Workflow tools: ${stats.categories.workflow}`);
  console.log(`   Utility tools: ${stats.categories.utility}`);
  if (stats.categories.custom > 0) {
    console.log(`   Custom named tools: ${stats.categories.custom}`);
  }
  
  // Run tests
  await runTests(server);
  
  // Final status
  console.log('\n' + '='.repeat(60));
  console.log('✅ Test Server Ready');
  console.log('='.repeat(60));
  
  console.log('\n📖 Instructions:');
  console.log('   • This server tests the npm package functionality');
  console.log('   • Workflow tools require the Go binary to be built');
  console.log('   • Utility tools (ping, list_tools, server_status) work standalone');
  
  console.log('\n💡 Tips:');
  console.log('   • Run "npm run build:current" to rebuild the Go binary');
  console.log('   • Use --tools=ping,list_tools to test specific tools');
  console.log('   • Use --custom-names to test custom naming');
  console.log('   • Use --real to test with actual MCP SDK (if installed)');
  
  console.log('\n🔄 Next steps:');
  console.log('   1. Make changes to tool definitions in lib/tools/');
  console.log('   2. Run: node test-server.js');
  console.log('   3. Verify tools register correctly');
  console.log('   4. Test execution with utility tools\n');
}

// Run the main function
main().catch(console.error);