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
npm install containerization-assist-mcp

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
import { configureTools } from 'containerization-assist-mcp';

configureTools({ server });
```

**AI Determinism:**
All AI-powered tools use deterministic sampling (`count: 1`) to ensure reproducible outputs. Each generation includes scoring metadata for quality validation.

**Progress Notifications:**
Long-running operations (build, deploy, scan) emit MCP notifications that clients can subscribe to for real-time progress updates.

### 2. Tool Orchestration

Tools are stateless functions that can be called independently or chained together:

```typescript
// Tools are stateless - each call is independent
// The MCP client (Claude) orchestrates the workflow

const analysisResult = await analyzeRepo.handler({
  repoPath: './my-app'
});

// Pass relevant information from previous tool to the next
await generateDockerfile.handler({
  projectPath: './my-app',
  language: analysisResult.language,
  framework: analysisResult.frameworks?.[0]?.name
});
```

**Orchestration Notes:**
- Tools are pure functions with no shared state
- The MCP client maintains conversation context
- Results from previous tools can inform subsequent calls
- Each tool execution is independent and can be tested in isolation

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

- [Main README](../../README.md)