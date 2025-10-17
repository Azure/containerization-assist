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
- **[app-mod-telemetry.ts](./app-mod-telemetry.ts)** - External telemetry integration pattern for wrapping tools with custom tracking
- **[minimal-tool-context.ts](./minimal-tool-context.ts)** - Minimal ToolContext implementation for quick setup

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

## Build Requirements

All examples must successfully compile with TypeScript before merging PRs:

```bash
# From the project root
npx tsc --noEmit docs/examples/*.ts
```

This ensures examples remain valid and functional. The CI pipeline validates example compilation as part of the test suite.

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

**AI Features:**
Tools provide deterministic outputs through built-in prompt engineering.

**Progress Notifications:**
Long-running operations (build, deploy, scan-image) emit MCP notifications that clients can subscribe to for real-time progress updates.

### 2. Tool Context Requirements

All Container Assist tools require a ToolContext with these properties:

```typescript
interface ToolContext {
  logger: Logger;                    // Pino-compatible logger (required)
  signal?: AbortSignal;              // Optional cancellation signal
  progress?: ProgressReporter;       // Optional progress reporting
}
```

**Quick Setup Options:**

1. **Minimal Implementation** - See [minimal-tool-context.ts](./minimal-tool-context.ts)
   ```typescript
   const context: ToolContext = {
     logger: pino({ level: 'info' }),
     signal: undefined,
     progress: async (msg) => console.log(msg),
   };
   ```

2. **With Real Pino Logger** - Production setup
   ```typescript
   import pino from 'pino';

   const context: ToolContext = {
     logger: pino({
       name: 'containerization-assist',
       level: 'info',
       transport: {
         target: 'pino-pretty',
         options: { colorize: true }
       }
     }),
     signal: undefined,
     progress: async (message, current, total) => {
       console.log(`Progress: ${message} (${current}/${total})`);
     },
   };
   ```

**Logger Requirements:**
The logger must implement Pino's interface with methods: debug, info, warn, error, fatal, trace, silent, child, and a level property.

### 3. Tool Orchestration

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

### 4. Error Handling

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