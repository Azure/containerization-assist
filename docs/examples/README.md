# Containerization Assist - MCP Integration Examples

This directory contains integration patterns for using Container Assist tools in your own MCP server.

## Examples

### 1. Standard Integration (Recommended)

**[mcp-integration.ts](./mcp-integration.ts)** - Standard integration using `bindToMCP()`:
- Use `createApp()` to configure Container Assist tools
- Select specific tools (not all 12)
- Use custom tool names (aliases)
- Configure output format and behavior
- Use `bindToMCP()` to automatically register tools (handles context creation)
- Create custom orchestration tools that reference Container Assist tools

**Best for:** Simple integrations where you don't need custom telemetry or hooks.

### 2. Integration with Telemetry (Advanced)

**[mcp-integration-with-telemetry.ts](./mcp-integration-with-telemetry.ts)** - Advanced integration with telemetry hooks:
- Use `createToolHandler()` for fine-grained control over tool registration
- **✨ NEW: Full TypeScript type safety** - Strongly-typed params and results in callbacks
- Add custom telemetry tracking for tool executions
- Implement error reporting and monitoring
- Track performance metrics and success rates
- Use `onSuccess` and `onError` hooks for observability
- Access typed tool parameters and results for better error handling

**Best for:** Production integrations requiring telemetry, monitoring, or custom error handling.

**Type Safety:** When using literal tool names with `createToolHandler()`, TypeScript automatically infers the specific parameter and result types for your callbacks, giving you full IntelliSense support and compile-time safety.

## Choosing the Right Pattern

### Use `bindToMCP()` (Standard) when:
- ✅ You want the simplest integration
- ✅ You don't need custom telemetry or error tracking
- ✅ You're building a proof-of-concept or simple application
- ✅ You want Container Assist to handle all context creation automatically

### Use `createToolHandler()` (Telemetry) when:
- ✅ You need to track tool execution metrics
- ✅ You want to integrate with observability platforms (DataDog, AppInsights, etc.)
- ✅ You need custom error reporting or alerting
- ✅ You want per-tool or per-category telemetry configurations
- ✅ You need lifecycle hooks (onSuccess, onError) for audit logging
- ✅ You want type-safe access to tool parameters and results

**Type Safety Example:**

```typescript
import { createToolHandler } from 'containerization-assist';

// Using a literal tool name gives you fully-typed callbacks!
server.tool(
  'build-image',
  buildImageTool.description,
  buildImageTool.inputSchema,
  createToolHandler(app, 'build-image', {
    onSuccess: (result, toolName, params) => {
      // ✅ result is typed as BuildImageResult
      console.log(`Built: ${result.imageId}, size: ${result.size} bytes`);

      // ✅ params is typed as BuildImageInput
      console.log(`Image name: ${params.imageName}`);

      // ✅ Full IntelliSense support!
      telemetry.track({
        tool: toolName,
        imageSize: result.size,
        buildTime: result.buildTime,
        tags: result.tags
      });
    },
    onError: (error, toolName, params) => {
      // ✅ params is also typed in error handler
      console.error(`Failed to build ${params.imageName}`);
    }
  })
);
```

**Note:** Both patterns let Container Assist handle all ToolContext creation (logger, policy, etc.) automatically. The difference is only in control over telemetry and lifecycle hooks.

## Running the Examples

### Prerequisites

```bash
# Install dependencies
npm install containerization-assist-mcp
npm install @modelcontextprotocol/sdk
npm install zod

# For TypeScript
npm install -D typescript tsx
```

### Run the Server

```bash
npx tsx mcp-integration.ts
```

### Test with MCP Inspector

```bash
npx @modelcontextprotocol/inspector npx tsx mcp-integration.ts
```

## Key Integration Pattern

### 1. Import Tools and Utilities

```typescript
import { createApp, analyzeRepoTool, generateDockerfileTool } from 'containerization-assist-mcp';
```

### 2. Configure and Bind Tools

```typescript
const server = new McpServer({ name: "my-server", version: "1.0.0" });

// Create Container Assist app with configuration
const app = createApp({
  // Select only the tools you need
  tools: [analyzeRepoTool, generateDockerfileTool],

  // Use custom tool names
  toolAliases: {
    'analyze-repo': 'my-analyze-repository',
    'generate-dockerfile': 'my-generate-dockerfile'
  },

  // Configure output format
  outputFormat: "natural-language"

  // Note: chainHintsMode defaults to 'enabled' for automatic next-step suggestions
  // Set chainHintsMode: 'disabled' only if you want to suppress these suggestions
});

// Bind tools to your MCP server
// This handles all context creation and tool registration automatically
app.bindToMCP(server);
```

### 3. Configuration Options

**Tool Selection:**
- Import only the tool objects you need
- Pass them in the `tools` array

**Tool Aliases:**
- Use tool names as strings (e.g., `'analyze-repo'`)
- Map them to your custom names

**Output Formats:**
- `"json"` - Full structured JSON (default)
- `"text"` - Summary text only
- `"markdown"` - Summary + collapsible JSON details
- `"natural-language"` - Rich narrative output (recommended for user-facing UIs)

**Chain Hints:**
- `"enabled"` - Show next-step suggestions after tool execution
- `"disabled"` - Disable automatic suggestions

### 4. Create Orchestration Tools

Build your own tools that reference the Container Assist tools:

```typescript
server.tool(
  "get-containerization-plan",
  "Generate a containerization plan",
  schema,
  async (args) => {
    // Your orchestration logic that references Container Assist tools
    // by their custom names
    return {
      content: [{
        type: "text",
        text: `Use tool 'my-analyze-repository' to analyze the repository...`
      }]
    };
  }
);
```

## Available Container Assist Tools

Import only the tools you need:

```typescript
import {
  analyzeRepoTool,           // Repository analysis and framework detection
  generateDockerfileTool,    // AI-powered Dockerfile generation
  fixDockerfileTool,         // Fix and optimize existing Dockerfiles
  buildImageTool,            // Docker image building with progress
  scanImageTool,             // Security vulnerability scanning
  tagImageTool,              // Docker image tagging
  pushImageTool,             // Push images to registry
  generateK8sManifestsTool,  // Kubernetes manifest generation
  prepareClusterTool,        // Kubernetes cluster preparation
  verifyDeployTool,          // Verify deployment status
  opsTool,                   // Operational utilities
} from 'containerization-assist-mcp';
```

## Tool Names

Use these string names when defining aliases:

- `'analyze-repo'` - Repository analysis
- `'generate-dockerfile'` - Dockerfile generation
- `'fix-dockerfile'` - Dockerfile fixes
- `'build-image'` - Docker build
- `'scan-image'` - Security scanning
- `'tag-image'` - Image tagging
- `'push-image'` - Registry push
- `'generate-k8s-manifests'` - K8s manifest generation
- `'prepare-cluster'` - Cluster setup
- `'verify-deploy'` - Deployment verification
- `'ops'` - Operational utilities

## Build Validation

The example must compile successfully with TypeScript:

```bash
# From the project root
npx tsc --noEmit docs/examples/*.ts
```

## More Information

- [Main README](../../README.md) - Installation and usage guide
- [CLAUDE.md](../../CLAUDE.md) - Development guidelines
