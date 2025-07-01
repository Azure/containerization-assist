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

All core interfaces are defined in `pkg/mcp/core/interfaces.go` as the single source of truth:

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

### Unified Interface Strategy

The codebase uses a unified interface approach where all interfaces are centralized in `pkg/mcp/core/interfaces.go`. This eliminates the need for separate internal interfaces:

```go
// All interfaces are defined in pkg/mcp/core/interfaces.go
// No separate internal interfaces needed
```

This unified strategy ensures:
- **Single source of truth**: All interfaces in `pkg/mcp/core/interfaces.go`
- **No import cycles**: Clean import hierarchy with core package at the center
- **Simplified maintenance**: No interface duplication
- **Build success**: Clean compilation without circular dependencies

## System Architecture

### Component Hierarchy

```
pkg/mcp/
├── core/
│   └── interfaces.go      # All unified interfaces
├── internal/
│   ├── core/              # Server lifecycle & management
│   ├── transport/         # Transport implementations (stdio, http)
│   ├── session/           # Session management with labels
│   ├── orchestration/     # Tool orchestration & dispatch
│   ├── analyze/           # Repository analysis & Dockerfile generation
│   ├── build/             # Docker operations (build, push, pull, tag)
│   ├── deploy/            # Kubernetes deployment & management
│   ├── scan/              # Security scanning (Trivy/Grype)
│   ├── conversation/      # Chat tool & guided workflows
│   ├── workflow/          # Multi-tool workflow orchestration
│   ├── observability/     # Prometheus metrics & OpenTelemetry
│   ├── context/           # AI context aggregation & caching
│   ├── config/            # Configuration management
│   ├── customizer/        # Docker/K8s customization helpers
│   ├── errors/            # Structured error handling
│   ├── monitoring/        # Health checks & metrics
│   ├── pipeline/          # Pipeline operations
│   ├── registry/          # Container registry providers (AWS ECR, Azure)
│   ├── retry/             # Circuit breakers & retry coordination
│   ├── runtime/           # Tool registration & validation
│   ├── server/            # Unified server implementation
│   ├── state/             # State management & synchronization
│   ├── types/             # Common types & constants
│   └── utils/             # Security validation & utilities
```

### Interface Implementation Flow

1. **Tool Implementation**: All tools implement the unified `Tool` interface from `pkg/mcp/core/interfaces.go`
2. **Auto-Registration**: Tools automatically register via naming convention (e.g., `AnalyzeRepositoryAtomicTool` → `analyze_repository_atomic`)
3. **Execution**: Tool execution flows through the orchestrator with direct interface calls
4. **Transport**: Communication flows through transport adapters implementing the unified interfaces

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
- **Orchestrator**: Main orchestration logic with workflow support
- **Registry**: Automatic tool registration via code generation
- **Workflow Engine**: Multi-tool workflow coordination
- Direct interface calls without conversion overhead

#### Session Management (`pkg/mcp/internal/session/`)
- Session lifecycle management with BoltDB persistence
- State persistence and recovery
- Session cleanup and garbage collection
- Workspace management
- Label-based session organization
- Session metadata and filtering

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
- All interfaces use the unified definitions in `pkg/mcp/core/interfaces.go`
- New methods should be added with sensible defaults or optional parameters
- Interface contracts should be thoroughly documented with examples
- Auto-registration ensures new tools are discovered automatically

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

## Key Architectural Patterns

### Auto-Registration System
Tools are automatically discovered and registered at build time:
- Naming convention: `StructNameTool` → `struct_name`
- Code generation creates registration in `pkg/mcp/internal/registry/generated.go`
- Zero manual registration required
- Compile-time validation of tool implementations

### Unified Server Architecture
Single server implementation supporting multiple modes:
- **Chat Mode**: Conversational workflows via `chat` tool
- **Workflow Mode**: Multi-tool orchestration
- **Atomic Mode**: Direct tool execution
- Seamless mode switching based on tool selection

### Observability Integration
Built-in production-grade monitoring:
- **Prometheus Metrics**: Tool execution, latency, errors
- **OpenTelemetry Tracing**: Distributed request tracing
- **Structured Logging**: Contextual log aggregation
- **Health Endpoints**: Liveness and readiness probes

### AI Context Management
Intelligent context handling for AI assistants:
- **Context Aggregation**: Combines relevant information
- **Caching Layer**: Reduces redundant operations
- **Token Optimization**: Manages context window efficiently
- **Semantic Pruning**: Removes irrelevant context

### Resilience Patterns
Production-ready error handling:
- **Circuit Breakers**: Prevent cascading failures
- **Retry Coordination**: Intelligent retry with backoff
- **Fix Providers**: AI-driven error remediation
- **Graceful Degradation**: Partial functionality on failures

This architecture provides a robust, scalable foundation for the MCP system while maintaining clean interfaces and preventing common Go architectural pitfalls like import cycles.
