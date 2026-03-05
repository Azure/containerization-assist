# SDK Usage Guide

Use `containerization-assist-mcp` as a TypeScript library to execute containerization tools directly or integrate them into your own MCP server.

## Installation

```bash
npm install containerization-assist-mcp
```

**Requirements:** Node.js 22+, Docker (for container operations)

## Quick Start: Direct Tool Execution

Execute tools programmatically without an MCP server:

```typescript
import { createApp } from 'containerization-assist-mcp';

const app = createApp();

// Analyze a repository
const result = await app.execute('analyze-repo', { path: './my-app' });

if (result.ok) {
  console.log('Language:', result.value.language);
  console.log('Framework:', result.value.framework);
} else {
  console.error('Failed:', result.error);
}
```

## Quick Start: MCP Server Integration

Embed Container Assist tools in your own MCP server:

```typescript
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import { createApp, analyzeRepoTool, generateDockerfileTool } from 'containerization-assist-mcp';

const server = new McpServer({ name: 'my-server', version: '1.0.0' });

const app = createApp({
  tools: [analyzeRepoTool, generateDockerfileTool],
  outputFormat: 'natural-language',
});

app.bindToMCP(server);

const transport = new StdioServerTransport();
await server.connect(transport);
```

See [examples/mcp-integration.ts](../examples/mcp-integration.ts) for a complete working example.

## API Reference

### `createApp(config?)`

Creates an `AppRuntime` instance. This is the primary entry point.

**Parameters** (`AppRuntimeConfig`):

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `tools` | `Tool[]` | All 11 tools | Subset of tools to register |
| `toolAliases` | `Record<string, string>` | `{}` | Map tool names to custom names |
| `outputFormat` | `OutputFormat` | `'json'` | Response format: `'json'`, `'text'`, `'markdown'`, `'natural-language'` |
| `chainHintsMode` | `ChainHintsMode` | `'enabled'` | Show next-step suggestions in responses |
| `logger` | `Logger` | Built-in | Custom Pino logger instance |

**Returns:** `AppRuntime`

### `AppRuntime`

The main application interface.

| Method | Description |
|--------|-------------|
| `execute(toolName, params, metadata?)` | Execute a tool with type-safe params and results |
| `bindToMCP(server, transportLabel?)` | Register all configured tools with an MCP server |
| `startServer(transport)` | Start a standalone MCP server with the given transport config |
| `listTools()` | List registered tools with name, description, version, and category |
| `healthCheck()` | Check runtime health including Docker and Kubernetes availability |
| `stop()` | Clean up resources |
| `getLogFilePath()` | Get current log file path (if tool logging is enabled) |

### `execute(toolName, params, metadata?)`

Type-safe tool execution. TypeScript infers param and result types from the tool name.

```typescript
const app = createApp();

// TypeScript knows the exact param and result types for each tool
const analysis = await app.execute('analyze-repo', { path: './my-app' });
const dockerfile = await app.execute('generate-dockerfile', { path: './my-app' });
const build = await app.execute('build-image', { imageName: 'myapp', tag: 'latest', path: '.' });
```

**Metadata** (`ExecutionMetadata`):

| Field | Type | Description |
|-------|------|-------------|
| `transport` | `string` | Transport type label (e.g., `'stdio'`, `'http'`) |
| `requestId` | `string` | Request ID for tracing |
| `signal` | `AbortSignal` | Cancellation support |
| `progress` | `unknown` | Progress reporter |
| `sendNotification` | `function` | MCP notification callback |

## Available Tools

All 11 tools organized by workflow stage:

### Analysis
- **`analyze-repo`** — Detect language, framework, and dependencies

### Dockerfile
- **`generate-dockerfile`** — AI-powered Dockerfile generation with knowledge base
- **`fix-dockerfile`** — Analyze and fix Dockerfile issues with policy validation

### Image Operations
- **`build-image`** — Build Docker images with security analysis
- **`scan-image`** — Vulnerability scanning with remediation guidance (requires Trivy)
- **`tag-image`** — Tag images with version and registry info
- **`push-image`** — Push images to a registry

### Kubernetes
- **`generate-k8s-manifests`** — Generate K8s, Helm, ACA, or Kustomize manifests
- **`prepare-cluster`** — Prepare cluster for deployment
- **`verify-deploy`** — Verify deployment status

### Utilities
- **`ops`** — Ping and server status

## Tool Registration with Telemetry

For production integrations requiring observability, use `createToolHandler()` instead of `bindToMCP()`:

```typescript
import { createApp, ALL_TOOLS, createToolHandler, createSafeTelemetryEvent } from 'containerization-assist-mcp';

const app = createApp();
const server = new McpServer({ name: 'my-server', version: '1.0.0' });

for (const tool of ALL_TOOLS) {
  server.tool(
    tool.name,
    tool.description,
    tool.inputSchema,
    createToolHandler(app, tool.name, {
      transport: 'my-integration',
      onSuccess: (result, toolName, params) => {
        const event = createSafeTelemetryEvent(
          toolName,
          params as Record<string, unknown>,
          { ok: true, value: result as Record<string, unknown> },
        );
        myTelemetry.track(event);
      },
      onError: (error, toolName) => {
        console.error(`${toolName} failed:`, error);
      },
    }),
  );
}
```

See [examples/mcp-integration-with-telemetry.ts](../examples/mcp-integration-with-telemetry.ts) for a complete example with type-safe callbacks and safe telemetry practices.

## Package Exports

The package provides sub-path exports for granular imports:

| Import Path | Contents |
|-------------|----------|
| `containerization-assist-mcp` | Full API: `createApp`, tools, types, utilities |
| `containerization-assist-mcp/server` | MCP server creation |
| `containerization-assist-mcp/tools` | Tool definitions and `ALL_TOOLS` |
| `containerization-assist-mcp/types` | TypeScript types (`AppRuntime`, `Tool`, `Result`, etc.) |
| `containerization-assist-mcp/config` | Configuration utilities |

## Result Type

All tool executions return `Result<T>` — a discriminated union for explicit error handling:

```typescript
import { type Result, Success, Failure } from 'containerization-assist-mcp';

const result = await app.execute('analyze-repo', { path: '.' });

if (result.ok) {
  // result.value is the typed success value
  console.log(result.value);
} else {
  // result.error is the error message
  // result.errorGuidance has optional hint/resolution
  console.error(result.error);
}
```

## Parameter Defaults

Built-in defaults for common parameters:

```typescript
import { withDefaults, K8S_DEFAULTS, CONTAINER_DEFAULTS, BUILD_DEFAULTS } from 'containerization-assist-mcp';

// Merge user params with tool defaults
const params = withDefaults('build-image', { imageName: 'myapp' });

// Or access defaults directly
console.log(K8S_DEFAULTS);         // { namespace: 'default', replicas: 1, ... }
console.log(CONTAINER_DEFAULTS);   // { registry, platform, ... }
console.log(BUILD_DEFAULTS);       // { context, dockerfilePath, ... }
```

## More Information

- [MCP Integration Examples](../examples/) — Working code examples
- [Main README](../../README.md) — Installation and MCP client setup
- [Policy Authoring Guide](./policy-authoring.md) — Custom policy configuration
- [Contributing](../../CONTRIBUTING.md) — Development guidelines
