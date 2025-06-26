# MCP Architecture Documentation

## Overview

Container Kit is an AI-powered tool that automates application containerization and Kubernetes manifest generation. The MCP (Model Context Protocol) system has been reorganized around a unified interface pattern that provides both public and internal interfaces to handle import cycles while maintaining a clean, single source of truth for interface definitions.

## Two-Mode Architecture

Container Kit provides two distinct modes of operation:

### 1. MCP Server (Primary) - Atomic + Conversational
The MCP server is the modern, recommended approach with enhanced AI integration:

```
┌─────────────────────────────────────────────────────────────┐
│                    MCP Server                               │
├─────────────────────────────────────────────────────────────┤
│  Transport Layer (stdio/http)                              │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────────────────────┐   │
│  │  Unified Tools  │  │    Conversation Mode           │   │
│  │  (Auto-Reg)    │  │                                 │   │
│  │                 │  │ • Chat Tool                     │   │
│  │ Tool Interface  │  │ • Prompt Manager                │   │
│  │ ├─analyze       │  │ • Session State                 │   │
│  │ ├─build         │  │ • Observability                 │   │
│  │ ├─deploy        │  │                                 │   │
│  │ ├─scan          │  └─────────────────────────────────┘   │
│  │ └─validate      │                                        │
│  └─────────────────┘                                        │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐                  │
│  │ Session Manager │  │ Workflow Orch   │                  │
│  │ (Unified)       │  │ (Simplified)    │                  │
│  └─────────────────┘  └─────────────────┘                  │
└─────────────────────────────────────────────────────────────┘
```

### 2. CLI Tool - Pipeline-Based (Legacy)
The original CLI uses a three-stage iterative pipeline for direct execution.

## Unified Interface System

### Core Interfaces

All core interfaces are defined in `pkg/mcp/interfaces.go` as the single source of truth:

```go
// Unified Tool interface for all tools
type Tool interface {
    Execute(ctx context.Context, args interface{}) (interface{}, error)
    GetMetadata() ToolMetadata
    Validate(ctx context.Context, args interface{}) error
}

// Session management interface
type Session interface {
    ID() string
    GetWorkspace() string
    UpdateState(func(*SessionState))
}

// Transport abstraction for MCP communication
type Transport interface {
    Serve(ctx context.Context) error
    Stop() error
    Name() string
    SetHandler(handler RequestHandler)
}

// Tool orchestration and execution
type Orchestrator interface {
    ExecuteTool(ctx context.Context, name string, args interface{}) (interface{}, error)
    RegisterTool(name string, tool Tool) error
    GetToolMetadata(toolName string) (interface{}, error)
    ValidateToolArgs(toolName string, args interface{}) error
}
```

### Internal Interface Strategy

To prevent import cycles between `pkg/mcp` and internal packages, lightweight "Internal" prefixed interfaces are maintained in `pkg/mcp/types/interfaces.go`:

```go
// Internal lightweight interfaces for avoiding import cycles
type InternalTool interface {
    Execute(ctx context.Context, args interface{}) (interface{}, error)
    GetMetadata() ToolMetadata
    Validate(ctx context.Context, args interface{}) error
}

type InternalTransport interface {
    Serve(ctx context.Context) error
    Stop() error
    Name() string
    SetHandler(handler InternalRequestHandler)
}
```

This dual-interface strategy ensures:
- **Single source of truth**: Main interfaces in `pkg/mcp/interfaces.go`
- **No import cycles**: Internal packages use lightweight versions
- **Type compatibility**: Both interfaces have identical method signatures
- **Build success**: Clean compilation without circular dependencies

## System Architecture

### Component Hierarchy

```
pkg/mcp/
├── interfaces.go           # Unified public interfaces
├── server.go              # Main MCP server
├── internal/
│   ├── core/              # Server lifecycle & management
│   ├── transport/         # Transport implementations (stdio, http)
│   ├── session/           # Session management
│   ├── orchestration/     # Tool orchestration & dispatch
│   ├── build/             # Build domain tools
│   ├── deploy/            # Deployment domain tools
│   ├── scan/              # Security scanning tools
│   ├── analyze/           # Analysis tools
│   └── types/
│       └── interfaces.go  # Internal lightweight interfaces
```

### Interface Implementation Flow

1. **Tool Implementation**: Tools implement either the main `Tool` interface or `InternalTool` interface depending on their location
2. **Registration**: Tools register with the orchestrator using the unified registration system
3. **Execution**: Tool execution flows through the orchestrator which handles the interface conversions
4. **Transport**: Communication flows through transport adapters that bridge internal and public interfaces

### Key Components

#### Server (`pkg/mcp/internal/core/server.go`)
- Main entry point for the MCP server
- Manages component lifecycle (session, transport, orchestration)
- Handles graceful shutdown with proper timeout handling
- Coordinates between gomcp manager and internal components

#### Transport Layer (`pkg/mcp/internal/transport/`)
- **stdio**: Standard input/output transport for CLI interaction
- **http**: HTTP REST transport for web-based interaction
- Implements `InternalTransport` interface internally
- Provides adapter to `Transport` interface for public API

#### Tool Orchestration (`pkg/mcp/internal/orchestration/`)
- **Dispatcher**: Type-safe tool dispatch without reflection
- **Orchestrator**: Main orchestration logic
- **Registry**: Tool registration and metadata management
- Handles conversion between internal and public interfaces

#### Session Management (`pkg/mcp/internal/session/`)
- Session lifecycle management
- State persistence and recovery
- Session cleanup and garbage collection
- Workspace management

### Domain-Specific Interfaces

Some packages define specialized interfaces for their domain:

#### Build Domain (`pkg/mcp/internal/build/`)
```go
type DockerfileValidator interface {
    Validate(content string, options ValidationOptions) (*ValidationResult, error)
}

type DockerfileAnalyzer interface {
    Analyze(lines []string, context ValidationContext) interface{}
}
```

#### Runtime Domain (`pkg/mcp/internal/runtime/`)
```go
type RuntimeValidator interface {
    Validate(ctx context.Context, input interface{}, options ValidationOptions) (*ValidationResult, error)
    GetName() string
}

type RuntimeAnalyzer interface {
    Analyze(ctx context.Context, input interface{}, options AnalysisOptions) (*AnalysisResult, error)
    GetName() string
    GetCapabilities() AnalyzerCapabilities
}
```

## Interface Evolution Strategy

### Versioning Approach
- Interfaces maintain backward compatibility through careful method additions
- Breaking changes require new interface versions with migration paths
- Deprecation warnings provide transition periods for interface changes

### Extension Points
- Tool registration supports factory patterns for dynamic tool creation
- Transport layer supports pluggable transport implementations
- Orchestration layer supports middleware and interceptors

### Compatibility Guidelines
- Internal interfaces must maintain method signature compatibility with public interfaces
- New methods should be added with sensible defaults or optional parameters
- Interface contracts should be thoroughly documented with examples

## Benefits of This Architecture

### Development Experience
1. **Clear Separation**: Public vs internal interfaces provide clear API boundaries
2. **No Import Cycles**: Internal packages can work without circular dependencies
3. **Type Safety**: Compile-time interface checking prevents runtime errors
4. **Modularity**: Domain-specific tools can be developed independently

### Maintainability
1. **Single Source of Truth**: All core interfaces defined in one location
2. **Consistent Patterns**: All tools follow the same interface contract
3. **Easy Testing**: Interface-based design enables comprehensive mocking
4. **Clear Dependencies**: Interface boundaries make system dependencies explicit

### Performance
1. **No Reflection**: Type-safe dispatch avoids runtime reflection overhead
2. **Efficient Registration**: Direct interface-based tool registration
3. **Optimized Transport**: Minimal conversion overhead between interface types
4. **Fast Validation**: Interface validation happens at compile time

## Integration Points

### External Tools
External tools can integrate by implementing the public `Tool` interface:

```go
type MyCustomTool struct {
    // tool implementation
}

func (t *MyCustomTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    // implementation
}

func (t *MyCustomTool) GetMetadata() ToolMetadata {
    // metadata implementation
}

func (t *MyCustomTool) Validate(ctx context.Context, args interface{}) error {
    // validation implementation
}
```

### Transport Extensions
New transports can be added by implementing the `Transport` interface:

```go
type CustomTransport struct {
    // transport implementation
}

func (t *CustomTransport) Serve(ctx context.Context) error {
    // serving logic
}

func (t *CustomTransport) Stop() error {
    // cleanup logic
}
```

This architecture provides a robust, scalable foundation for the MCP system while maintaining clean interfaces and preventing common Go architectural pitfalls like import cycles.