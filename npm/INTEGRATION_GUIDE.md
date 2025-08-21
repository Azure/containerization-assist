# Container Kit Tools - Integration Guide

This guide shows how to import and use Container Kit tools in your own MCP server or application.

## Installation

```bash
npm install @thgamble/containerization-assist-mcp
```

## Basic Usage

### CommonJS (require)

```javascript
const containerKit = require('@thgamble/containerization-assist-mcp');

// Access individual tools
const analyzeRepo = containerKit.analyzeRepository;
const dockerfile = containerKit.generateDockerfile;

// Or destructure specific tools
const { 
  analyzeRepository, 
  generateDockerfile,
  buildImage 
} = require('@thgamble/containerization-assist-mcp');
```

### ES Modules (import)

```javascript
import * as containerKit from '@thgamble/containerization-assist-mcp';

// Or import specific tools
import { 
  analyzeRepository, 
  generateDockerfile,
  buildImage 
} from '@thgamble/containerization-assist-mcp';
```

## Tool Structure

Each tool exports three properties:

```javascript
{
  name: 'analyze_repository',        // Tool identifier
  metadata: {                        // Tool metadata
    title: 'Analyze Repository',
    description: 'Analyzes a repository...',
    inputSchema: {                   // Zod schema for validation
      repo_path: z.string(),
      session_id: z.string().optional()
    }
  },
  handler: async (params) => {...}   // Async function that executes the tool
}
```

## Integration Patterns

### 1. Register with Any MCP Server

```javascript
import { analyzeRepository } from '@thgamble/containerization-assist-mcp';

// Your MCP server instance (any implementation)
const server = /* your MCP server */;

// Register using the tool's properties
server.tool(
  analyzeRepository.name,                    // 'analyze_repository'
  analyzeRepository.metadata.description,    // Tool description
  analyzeRepository.metadata.inputSchema,    // Zod schema for validation
  analyzeRepository.handler                  // Async handler function
);
```

### 2. Register All Tools at Once

```javascript
const { registerAllTools } = require('@thgamble/containerization-assist-mcp');

// Your MCP server instance
const mcpServer = {
  addTool: function(config, handler) {
    // Your registration logic
  }
};

// Register all Container Kit tools
registerAllTools(mcpServer);

// Or with custom names
registerAllTools(mcpServer, {
  'analyze_repository': 'custom_analyze',
  'build_image': 'docker_build'
});
```

### 3. Custom Registration

```javascript
const { analyzeRepository } = require('@thgamble/containerization-assist-mcp');

// Register with your MCP server using custom name/description
myServer.tool(
  'custom_analyze',                          // Custom name (customizable)
  'My custom description',                   // Custom description (customizable)
  analyzeRepository.metadata.inputSchema,    // Original schema (fixed)
  analyzeRepository.handler                  // Original handler (fixed)
);

// Or use the original properties
myServer.tool(
  analyzeRepository.name,                    // 'analyze_repository'
  analyzeRepository.metadata.description,    // Original description
  analyzeRepository.metadata.inputSchema,    // Zod schema
  analyzeRepository.handler                  // Handler function
);
```

### 4. Direct Tool Execution

```javascript
const { analyzeRepository } = require('@thgamble/containerization-assist-mcp');

// Access tool properties
console.log(analyzeRepository.name);        // 'analyze_repository'
console.log(analyzeRepository.metadata);    // { title, description, inputSchema }

// Execute tool directly
const result = await analyzeRepository.handler({
  repo_path: '/path/to/repo',
  session_id: 'optional-session-id'
});

// Result format follows MCP spec:
// { content: [{ type: 'text', text: '...' }] }
```

### 5. With MCP SDK

```javascript
const { MCPServer } = require('@modelcontextprotocol/sdk');
const { getAllTools, convertZodToJsonSchema } = require('@thgamble/containerization-assist-mcp');

const server = new MCPServer();
const tools = getAllTools();

// Register each tool
Object.values(tools).forEach(tool => {
  server.addTool({
    name: tool.name,
    description: tool.metadata.description,
    inputSchema: convertZodToJsonSchema(tool.metadata.inputSchema)
  }, tool.handler);
});
```

## Available Tools

The package exports 13 tools:

### Workflow Tools (10)
- `analyzeRepository` - Analyze repository structure and languages
- `generateDockerfile` - Generate optimized Dockerfile
- `buildImage` - Build container image
- `scanImage` - Security vulnerability scanning
- `tagImage` - Tag container images
- `pushImage` - Push to registry
- `generateK8sManifests` - Generate Kubernetes manifests
- `prepareCluster` - Prepare K8s cluster
- `deployApplication` - Deploy to Kubernetes
- `verifyDeployment` - Verify deployment health

### Utility Tools (3)
- `listTools` - List all available tools
- `ping` - Test connectivity
- `serverStatus` - Get server status

**Note:** The orchestration tools (`startWorkflow` and `workflowStatus`) are available through the Go binary but not yet exposed as JavaScript imports.

## Helper Functions

```javascript
// CommonJS
const {
  registerTool,           // Register single tool with MCP server
  registerAllTools,       // Register all tools at once
  getAllTools,            // Get dictionary of all tools
  createSession,          // Create a new session ID
  convertZodToJsonSchema  // Convert Zod schema to JSON Schema
} = require('@thgamble/containerization-assist-mcp');

// ES Modules
import {
  registerTool,
  registerAllTools,
  getAllTools,
  createSession,
  convertZodToJsonSchema
} from '@thgamble/containerization-assist-mcp';
```

### Usage Examples

```javascript
// Create a session for workflow tracking
const sessionId = createSession();

// Register a single tool with custom name
registerTool(server, analyzeRepository, 'my_analyzer');

// Get all tools for iteration
const tools = getAllTools();
Object.values(tools).forEach(tool => {
  console.log(`${tool.name}: ${tool.metadata.title}`);
});
```

## Complete Example

```javascript
import { 
  analyzeRepository, 
  generateDockerfile,
  buildImage,
  createSession 
} from '@thgamble/containerization-assist-mcp';

// Example: Custom MCP Server Implementation
class MyMCPServer {
  constructor() {
    this.tools = new Map();
  }
  
  // Register a tool with the standard interface
  tool(name, description, inputSchema, handler) {
    this.tools.set(name, {
      name,
      description,
      inputSchema,
      handler
    });
    console.log(`Registered tool: ${name}`);
  }
  
  async executeTool(toolName, params) {
    const tool = this.tools.get(toolName);
    if (!tool) throw new Error(`Tool not found: ${toolName}`);
    return await tool.handler(params);
  }
}

// Create server and register Container Kit tools
const server = new MyMCPServer();

// Register tools - you control the names and descriptions
server.tool(
  analyzeRepository.name,                    // Use original name
  analyzeRepository.metadata.description,    // Use original description
  analyzeRepository.metadata.inputSchema,    // Must use original schema
  analyzeRepository.handler                  // Must use original handler
);

server.tool(
  'custom_dockerfile',                       // Custom name
  'Generate optimized container config',     // Custom description
  generateDockerfile.metadata.inputSchema,   // Must use original schema
  generateDockerfile.handler                 // Must use original handler
);

// Execute a workflow
async function containerizeApp(repoPath) {
  const sessionId = createSession();
  
  // Step 1: Analyze
  const analysis = await server.executeTool('analyze_repository', {
    repo_path: repoPath,
    session_id: sessionId
  });
  
  // Step 2: Generate Dockerfile
  const dockerfile = await server.executeTool('custom_dockerfile', {
    session_id: sessionId
  });
  
  return { analysis, dockerfile };
}
```

## Running Examples

```bash
# Run the integration examples
node examples/external-usage.js

# Test with mock server
npm run test:server
```

## Requirements

- Node.js 14+
- Platform-specific binary is included in npm/bin/{platform}/
- Supported platforms: darwin-x64, darwin-arm64, linux-x64, linux-arm64, win32-x64, win32-arm64

## Debugging

Enable debug output:
```bash
DEBUG_MCP=true node your-app.js
```

## Key Points

1. **Flexibility**: You can customize tool names and descriptions when registering
2. **Fixed Contract**: The inputSchema and handler must remain unchanged to ensure proper functionality
3. **Subprocess Bridge**: Tools execute via subprocess to the Go binary, enabling cross-language integration
4. **MCP Compliance**: All tools follow MCP specification for input/output format
5. **Session Management**: Use `createSession()` to track workflows across multiple tool invocations