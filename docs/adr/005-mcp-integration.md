# ADR-005: MCP Protocol Integration

**Date:** 2025-10-17
**Status:** Accepted
**Deciders:** Development Team
**Context:** We needed to decide on the primary interface for exposing containerization capabilities to AI agents across multiple IDEs, requiring standardization, tool composability, progress tracking, and clean protocol separation.

## Decision

We will use the **Model Context Protocol (MCP)** as the primary interface for the Containerization Assist server, implementing it through the official `@modelcontextprotocol/sdk` (version 1.17.3).

The implementation follows these principles:

1. **MCP-First Architecture**: All containerization tools are exposed through MCP's standardized tool interface
2. **Stdio Transport**: Use stdio as the primary transport mechanism for maximum IDE compatibility
3. **Tool Composability**: Each tool is independently discoverable and executable through MCP
4. **Schema-Driven**: All tool inputs/outputs are defined with Zod schemas, automatically converted to JSON Schema for MCP
5. **Progress Notifications**: Leverage MCP's notification system for real-time progress updates
6. **Resource Exposure**: Expose server status and capabilities through MCP resources

## Rationale

### Why MCP?

1. **Industry Standard Protocol**
   - MCP is an emerging standard for AI-tool integration backed by Anthropic
   - Provides a well-defined specification for tool discovery, execution, and progress tracking
   - Growing ecosystem of MCP-compatible clients and tools

2. **Multi-Client Support**
   - Works natively with Claude Desktop, VS Code (via GitHub Copilot), Cline, and other MCP clients
   - Single implementation serves multiple AI development environments
   - No need to maintain separate integrations for each IDE/client

3. **Tool Composability**
   - MCP's tool model allows AI agents to discover and combine tools dynamically
   - Each tool is independently documented with JSON Schema
   - Natural language requests are automatically routed to appropriate tools

4. **Built-in Progress Tracking**
   - MCP notification system supports real-time progress updates
   - AI agents can show build/deployment progress to users
   - Better user experience during long-running operations (builds, scans, deployments)

5. **Clean Architecture**
   - MCP provides clear protocol boundaries (src/mcp/)
   - Business logic (tools, infrastructure) remains independent of protocol
   - Easy to add additional interfaces (REST, gRPC) in the future without changing core logic

### Implementation Details

**MCP Server Structure (src/mcp/mcp-server.ts:116-146)**:
```typescript
export function createMCPServer<TTool extends Tool>(
  tools: Array<TTool>,
  options: ServerOptions = {},
  execute: ToolExecutor,
): MCPServer {
  const server = new McpServer({
    name: 'containerization-assist',
    version: '1.0.0',
  });

  registerToolsWithServer({
    outputFormat,
    server,
    tools,
    logger,
    transport: 'stdio',
    execute,
  });

  // Server manages lifecycle and transport
}
```

**Tool Registration**:
- 17 containerization tools automatically registered with MCP server
- Zod schemas converted to JSON Schema for MCP tool definitions
- Tool execution delegated to orchestrator for dependency resolution

**Transport**:
- Stdio transport for IDE integration (VS Code, Claude Desktop)
- Started via CLI: `containerization-assist-mcp start`
- Configuration via `.vscode/mcp.json` or Claude Desktop config

## Consequences

### Positive

1. **Broad IDE Compatibility**
   - Works with Claude Desktop, VS Code (GitHub Copilot), Cline, and any MCP-compatible client
   - Single implementation serves all clients
   - No client-specific code needed

2. **Developer Experience**
   - Natural language interface: "Build and scan my Java application"
   - AI automatically routes to correct tools (analyze-repo → generate-dockerfile → build-image → scan-image)
   - Real-time progress updates during long operations

3. **Discoverability**
   - Tools are self-documenting through JSON Schema
   - AI agents can understand tool capabilities without hardcoding
   - Users can discover features through conversation

4. **Maintainability**
   - Clean separation: MCP protocol (src/mcp/) vs business logic (src/tools/, src/infra/)
   - Easy to add new tools - just implement Tool interface and register
   - Protocol layer isolated from Docker/Kubernetes logic

5. **Ecosystem Integration**
   - Can be composed with other MCP servers (filesystem, git, etc.)
   - AI agents can orchestrate across multiple MCP servers
   - Follows community standards and best practices

6. **Progress Transparency**
   - MCP notifications show build progress, scan results, deployment status
   - Better than CLI where output is hidden from AI context
   - Users see what's happening in real-time

### Negative

1. **MCP Dependency**
   - Tied to MCP SDK and protocol evolution
   - Breaking changes in MCP SDK require updates
   - Limited to clients that support MCP (though this is growing)

2. **Stdio Limitations**
   - Stdio transport requires process spawning per session
   - Not suitable for high-concurrency scenarios (but not our use case)
   - More complex debugging than HTTP-based protocols

3. **Limited Direct Access**
   - No native REST API for non-MCP clients
   - Programmatic access requires MCP client library
   - Web integrations would need MCP client or separate API layer

4. **Learning Curve**
   - Developers need to understand MCP protocol and SDK
   - More complex than simple CLI or REST API
   - Debugging requires MCP Inspector tool

5. **Single-User Design**
   - MCP stdio model is single-user, single-session
   - Not designed for multi-tenant or web-scale scenarios
   - Each user needs separate process instance

## Alternatives Considered

### Alternative 1: REST API

**Approach**: HTTP/REST API with endpoints for each tool operation

**Pros**:
- Familiar HTTP protocol, well-understood by developers
- Easy to test with curl, Postman, or HTTP clients
- Language-agnostic - any language can consume REST
- Simple stateless request/response model
- Web-native - easy to build web UIs
- Scalable - can handle many concurrent requests
- Easy debugging with browser DevTools

**Cons**:
- No standardized tool discovery mechanism
- AI agents need custom integration per API
- Progress updates require WebSockets/SSE
- Manual routing - AI must know which endpoint to call
- Requires API documentation, versioning, auth
- More infrastructure - need web server, load balancer
- Less composable with other AI tools

**Rejected because**:
- Lacks tool discoverability standard that AI agents expect
- Requires custom integration code for each AI client (Claude, Copilot, etc.)
- Progress tracking requires additional complexity (WebSockets/SSE)
- MCP provides better native AI integration experience

### Alternative 2: gRPC Service

**Approach**: gRPC service with protobuf definitions for each tool

**Pros**:
- Type-safe with protobuf schemas
- Efficient binary protocol
- Streaming support for progress updates
- Generated clients in multiple languages
- Strong contract with .proto files
- Good for microservice architectures

**Cons**:
- No AI-specific tool discovery
- Requires gRPC client library (not as universal as HTTP)
- More complex than REST for simple use cases
- AI agents would need gRPC support
- Higher complexity for simple single-app workflows
- Debugging harder than REST
- Not web-browser friendly

**Rejected because**:
- Over-engineered for single-user, single-app workflow
- AI integration would require custom client code
- MCP is more suited to AI agent interaction patterns
- gRPC is better for service-to-service, not AI-to-tool

### Alternative 3: CLI-Only Interface

**Approach**: Pure command-line interface with no server component

**Pros**:
- Simplest architecture - no server/transport needed
- Familiar to developers - just a CLI tool
- Easy to script in bash/shell
- No process overhead - runs and exits
- Simple debugging - just run commands
- Works everywhere Node.js runs

**Cons**:
- AI agents can't easily execute CLI commands (security restrictions)
- No progress updates - output is opaque to AI
- Each invocation requires new process startup
- State management difficult across commands
- AI can't compose tools dynamically
- Limited integration with IDEs

**Rejected because**:
- AI agents have limited/no ability to execute arbitrary shell commands
- No standardized way for AI to discover CLI capabilities
- Poor user experience - AI can't show real-time progress
- MCP provides much better AI integration

### Alternative 4: Custom Protocol

**Approach**: Design a custom JSON-RPC or message-based protocol

**Pros**:
- Full control over protocol design
- Optimized for our specific use case
- No external dependencies
- Can evolve independently

**Cons**:
- Requires custom client implementations for each AI tool
- No existing ecosystem or tooling
- Reinventing the wheel
- Maintenance burden for protocol evolution
- Documentation and SDK development needed
- No community support or examples

**Rejected because**:
- MCP already provides everything we need
- Building custom protocol is significant engineering investment
- Would fragment the AI tooling ecosystem
- No benefit over adopting MCP standard

## Related Decisions

- **ADR-002: Unified Tool Interface** - Tool interface design enables seamless MCP integration
- **ADR-006: Infrastructure Layer Organization** - MCP server sits in src/mcp/ adapter layer

## References

- MCP Specification: https://modelcontextprotocol.io/
- MCP SDK: https://github.com/modelcontextprotocol/sdk
- Implementation: src/mcp/mcp-server.ts
- CLI Entry Point: src/cli/cli.ts
- Tool Registration: src/mcp/mcp-server.ts:139-146
- VS Code Setup: README.md:30-49
- Package Configuration: package.json:87 (MCP SDK dependency)
