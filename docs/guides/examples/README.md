# Containerization Assist Examples

This directory contains code examples demonstrating how to use the Containerization Assist MCP Server in various scenarios.

## Examples

### Basic Usage

- **[minimal-server.js](./minimal-server.js)** - Minimal MCP server setup with Container Assist tools
- **[direct-usage.ts](./direct-usage.ts)** - Direct usage of tools without MCP server

### Integration Patterns

- **[mcp-integration.ts](./mcp-integration.ts)** - Full MCP server integration example
- **[custom-server.ts](./custom-server.ts)** - Custom MCP server with Container Assist tools
- **[clean-api-example.ts](./clean-api-example.ts)** - Clean API patterns using Result types

## Running the Examples

### Prerequisites

```bash
# Install the Container Assist package
npm install @thgamble/containerization-assist-mcp

# For TypeScript examples
npm install -D typescript tsx
```

### Running JavaScript Examples

```bash
node minimal-server.js
```

### Running TypeScript Examples

```bash
# Using tsx (recommended)
npx tsx mcp-integration.ts

# Or compile first
npx tsc mcp-integration.ts
node mcp-integration.js
```

## Testing with MCP Inspector

You can test any of these examples with the MCP Inspector:

```bash
# Test the minimal server
npx @modelcontextprotocol/inspector node minimal-server.js

# Test TypeScript examples
npx @modelcontextprotocol/inspector npx tsx mcp-integration.ts
```

## Key Concepts

### 1. Tool Configuration

Always configure tools with your server for AI features:

```typescript
import { configureTools } from '@thgamble/containerization-assist-mcp';

configureTools({ server });
```

### 2. Session Management

Use sessions to maintain context across tool calls:

```typescript
const sessionId = 'my-session-123';

await analyzeRepo.handler({ 
  repoPath: './my-app',
  sessionId 
});

await generateDockerfile.handler({ 
  sessionId // Uses analysis from previous call
});
```

### 3. Error Handling

All tools return Result types for safe error handling:

```typescript
const result = await buildImage.handler({ 
  dockerfilePath: './Dockerfile' 
});

if (!result.success) {
  console.error('Build failed:', result.error);
} else {
  console.log('Image built:', result.value);
}
```

## More Information

- [Getting Started Guide](../getting-started.md)
- [External Usage Guide](../external-usage.md)
- [Architecture Documentation](../../reference/architecture.md)
- [Main README](../../README.md)