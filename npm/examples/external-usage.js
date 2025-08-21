#!/usr/bin/env node

/**
 * Example showing how external programs can import and use Container Kit tools
 * with any MCP server implementation.
 */

// Method 1: Import all tools at once
import * as containerKit from '../lib/index.js';

// Method 2: Import specific tools
import { analyzeRepository, generateDockerfile, buildImage } from '../lib/index.js';

// Method 3: Import helper functions
import { registerTool, registerAllTools, getAllTools } from '../lib/index.js';

// Example 1: Using with a generic MCP server
function example1_genericMcpServer() {
  console.log('\n=== Example 1: Generic MCP Server ===\n');
  
  // Simulate any MCP server with addTool method
  const mcpServer = {
    tools: {},
    addTool: function(config, handler) {
      this.tools[config.name] = { config, handler };
      console.log(`Registered tool: ${config.name}`);
    }
  };

  // Register all Container Kit tools
  registerAllTools(mcpServer);
  
  // Or register individual tools with custom names
  registerTool(mcpServer, analyzeRepository, 'custom_analyze');
  
  console.log(`\nTotal tools registered: ${Object.keys(mcpServer.tools).length}`);
}

// Example 2: Direct tool usage without MCP server
async function example2_directUsage() {
  console.log('\n=== Example 2: Direct Tool Usage ===\n');
  
  // Access tool properties directly
  console.log('Tool name:', analyzeRepository.name);
  console.log('Tool description:', analyzeRepository.metadata.description);
  console.log('Tool input schema:', analyzeRepository.metadata.inputSchema);
  
  // Call tool handler directly
  try {
    const result = await analyzeRepository.handler({
      repo_path: '/tmp/test-repo',
      session_id: 'test-session-123'
    });
    console.log('\nTool result:', result);
  } catch (error) {
    console.log('\nTool execution (expected to fail without valid repo):', error.message);
  }
}

// Example 3: Building custom MCP server with Container Kit tools
function example3_customMcpImplementation() {
  console.log('\n=== Example 3: Custom MCP Implementation ===\n');
  
  class CustomMCPServer {
    constructor() {
      this.tools = new Map();
    }
    
    registerContainerKitTool(tool, customName = null) {
      const name = customName || tool.name;
      
      // Store the tool with its metadata and handler
      this.tools.set(name, {
        name: name,
        metadata: tool.metadata,
        handler: tool.handler
      });
      
      console.log(`Registered: ${name}`);
      console.log(`  Title: ${tool.metadata.title}`);
      console.log(`  Description: ${tool.metadata.description}`);
    }
    
    async executeTool(name, params) {
      const tool = this.tools.get(name);
      if (!tool) {
        throw new Error(`Tool not found: ${name}`);
      }
      return await tool.handler(params);
    }
  }
  
  const server = new CustomMCPServer();
  
  // Register specific tools
  server.registerContainerKitTool(analyzeRepository);
  server.registerContainerKitTool(generateDockerfile);
  server.registerContainerKitTool(buildImage, 'docker_build'); // with custom name
  
  console.log(`\nTotal tools: ${server.tools.size}`);
}

// Example 4: Accessing all available tools
function example4_allTools() {
  console.log('\n=== Example 4: All Available Tools ===\n');
  
  const allTools = getAllTools();
  
  console.log(`Container Kit provides ${Object.keys(allTools).length} tools:\n`);
  
  Object.entries(allTools).forEach(([key, tool]) => {
    console.log(`- ${tool.name}`);
    console.log(`  Export name: ${key}`);
    console.log(`  Title: ${tool.metadata.title}`);
    
    // Show input parameters
    if (tool.metadata.inputSchema) {
      const params = Object.keys(tool.metadata.inputSchema.shape || tool.metadata.inputSchema);
      console.log(`  Parameters: ${params.join(', ')}`);
    }
    console.log();
  });
}

// Example 5: Integration with real MCP SDK
function example5_mcpSdkIntegration() {
  console.log('\n=== Example 5: MCP SDK Integration Pattern ===\n');
  
  console.log('To use with @modelcontextprotocol/sdk:\n');
  console.log(`
const { MCPServer } = require('@modelcontextprotocol/sdk');
const { registerAllTools } = require('containerization-assist');

const server = new MCPServer();

// Register all Container Kit tools
registerAllTools(server);

// Or register individual tools
const { analyzeRepository } = require('containerization-assist');
server.addTool({
  name: analyzeRepository.name,
  description: analyzeRepository.metadata.description,
  inputSchema: analyzeRepository.metadata.inputSchema
}, analyzeRepository.handler);

server.start();
  `);
}

// Run examples
async function main() {
  console.log('Container Kit NPM Package - External Usage Examples');
  console.log('=' .repeat(50));
  
  example1_genericMcpServer();
  await example2_directUsage();
  example3_customMcpImplementation();
  example4_allTools();
  example5_mcpSdkIntegration();
  
  console.log('\n' + '='.repeat(50));
  console.log('Examples complete!');
}

// Run if executed directly
main().catch(console.error);

export { main };