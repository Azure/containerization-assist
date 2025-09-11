# Using Container Assist Tools in External MCP Servers

This guide explains how to import and use Container Assist tools in your own MCP server implementation.

## Quick Start

```typescript
import { 
  configureTools, 
  analyzeRepo, 
  buildImage,
  registerTool 
} from '@thgamble/containerization-assist-mcp';
import { Server } from '@modelcontextprotocol/sdk';

// Create your MCP server
const server = new Server({
  name: 'my-server',
  version: '1.0.0'
});

// IMPORTANT: Configure tools with your server for AI sampling
configureTools({ server });

// Register individual tools
registerTool(server, analyzeRepo);
registerTool(server, buildImage);

// Start your server
await server.start();
```

## Why Configure Tools?

The `configureTools()` function is **essential** for proper tool operation. It provides:

1. **AI Sampling**: Tools like `analyzeRepo` and `generateDockerfile` use AI to analyze code and generate content. Without configuration, these AI features won't work.

2. **Session Management**: Automatically handles session creation and retrieval when tools pass `sessionId` in params.

3. **Progress Reporting**: Enables proper progress tracking through your MCP server.

## Import Patterns

### Individual Tool Imports

```typescript
// Import specific tools you need
import { 
  analyzeRepo,
  generateDockerfile,
  buildImage,
  scanImage,
  deployApplication
} from '@thgamble/containerization-assist-mcp';
```

### Batch Registration

```typescript
import { 
  configureTools,
  registerAllTools 
} from '@thgamble/containerization-assist-mcp';

// Configure once
configureTools({ server });

// Register all tools at once
registerAllTools(server);
```

### Custom Naming

```typescript
// Register with custom names
registerTool(server, analyzeRepo, 'custom_analyze');
registerTool(server, buildImage, 'docker_build');

// Or use name mapping for batch registration
registerAllTools(server, {
  analyzeRepo: 'analyze_repository',
  buildImage: 'docker_build',
  scanImage: 'security_scan'
});
```

## Available Tools

All tools are exported as MCPTool objects ready for registration:

- `analyzeRepo` / `analyzeRepository` - Analyze repository structure
- `generateDockerfile` - Generate Dockerfile
- `buildImage` - Build Docker images
- `scanImage` - Scan for vulnerabilities
- `tagImage` - Tag Docker images
- `pushImage` - Push to registries
- `generateK8sManifests` - Generate Kubernetes manifests
- `prepareCluster` - Prepare K8s cluster
- `deployApplication` - Deploy to Kubernetes
- `verifyDeployment` - Verify deployment status
- `fixDockerfile` - Fix Dockerfile issues
- `resolveBaseImages` - Recommend base images
- `workflow` - Execute workflows
- `ping` / `serverStatus` - Operational tools

## Session Management

Tools automatically handle sessions when a `sessionId` is provided:

```typescript
// The tool will create/retrieve the session automatically
const result = await analyzeRepo.handler({
  repo_path: '/path/to/repo',
  sessionId: 'my-session-123'  // Optional
});
```

## Complete Example

```typescript
import { 
  configureTools,
  analyzeRepo,
  generateDockerfile,
  buildImage,
  registerTool
} from '@thgamble/containerization-assist-mcp';
import { Server } from '@modelcontextprotocol/sdk';

async function setupServer() {
  // Create server
  const server = new Server({
    name: 'containerization-server',
    version: '1.0.0'
  });

  // Configure tools with server
  configureTools({ server });

  // Register the tools you need
  registerTool(server, analyzeRepo);
  registerTool(server, generateDockerfile);
  registerTool(server, buildImage);

  // Add your own custom tools if needed
  server.addTool({
    name: 'my_custom_tool',
    description: 'My custom tool',
    inputSchema: { type: 'object', properties: {} }
  }, async (params) => {
    // Your custom logic
    return { content: [{ type: 'text', text: 'Result' }] };
  });

  // Start server
  await server.start();
  console.log('Server running with Container Assist tools');
}

setupServer().catch(console.error);
```

## Troubleshooting

### "Cannot read properties of undefined"

If you see errors about undefined properties (like 'progress'), make sure you've called `configureTools({ server })` before using the tools.

### AI Sampling Not Working

Ensure your MCP server has proper AI sampling support and that you've configured the tools with your server instance.

### Session Issues

Sessions are created automatically when a `sessionId` is provided in params. If you need to manage sessions manually, you can access the session manager through the tool context.