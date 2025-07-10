# Container Kit MCP Integration Design Document

## Executive Summary

This document provides a comprehensive analysis of how Container Kit's Model Context Protocol (MCP) implementation integrates with the gomcp library. Container Kit is a production-ready, enterprise-grade AI-powered containerization platform that uses MCP for tool registration, execution, and AI-driven workflow orchestration.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [gomcp Library Integration](#gomcp-library-integration)
3. [Transport Layer Implementation](#transport-layer-implementation)
4. [Server Interfaces and Lifecycle](#server-interfaces-and-lifecycle)
5. [Tool Registration and Execution Flow](#tool-registration-and-execution-flow)
6. [Error Handling and Recovery](#error-handling-and-recovery)
7. [Session Management Integration](#session-management-integration)
8. [Performance and Scalability](#performance-and-scalability)
9. [Security Considerations](#security-considerations)
10. [Future Enhancements](#future-enhancements)

## Architecture Overview

### Three-Layer Architecture

Container Kit follows a **three-context architecture** as defined in [ADR-001](architecture/adr/2025-07-07-three-context-architecture.md):

```
pkg/mcp/
├── application/          # Application layer - orchestration & coordination
│   ├── api/             # Canonical interface definitions (single source of truth)
│   ├── interfaces/      # Compatibility layer with type aliases
│   ├── core/           # Server lifecycle & registry management
│   ├── internal/       # Internal implementation details
│   ├── orchestration/ # Tool coordination & workflow execution
│   └── services/       # Service interfaces for dependency injection
├── domain/             # Domain layer - business logic
│   ├── containerization/ # Container operations (analyze, build, deploy, scan)
│   ├── errors/         # Rich error handling system
│   ├── session/        # Session management & persistence
│   └── types/          # Domain type definitions
└── infra/              # Infrastructure layer - external integrations
    ├── transport/      # MCP protocol transports (stdio, HTTP)
    ├── persistence/    # BoltDB storage layer
    └── templates/      # Kubernetes manifest templates
```

### Service-Oriented Architecture

The system implements **manual dependency injection** as defined in [ADR-006](architecture/adr/2025-01-07-manual-dependency-injection.md):

```go
type ServiceContainer interface {
    SessionStore() SessionStore        // Session CRUD operations
    SessionState() SessionState        // State & checkpoint management
    BuildExecutor() BuildExecutor      // Container build operations
    ToolRegistry() ToolRegistry        // Tool registration & discovery
    WorkflowExecutor() WorkflowExecutor // Multi-step workflows
    Scanner() Scanner                  // Security scanning
    ConfigValidator() ConfigValidator  // Configuration validation
    ErrorReporter() ErrorReporter      // Unified error handling
}
```

## gomcp Library Integration

### Core Integration Points

Container Kit integrates with the gomcp library (v1.6.5) at several key points:

1. **Primary Server**: `pkg/mcp/server/core.go` - Main MCP server implementation
2. **Transport Layer**: `pkg/mcp/transport/stdio.go` and `pkg/mcp/transport/http.go` - Protocol transports
3. **Tool Registration**: Server-side tool registration with gomcp
4. **Error Handling**: gomcp-specific error handling and adaptation
5. **Entry Point**: `cmd/mcp-server/main.go` - Application initialization

### gomcp Server Creation Pattern

```go
type simplifiedGomcpManager struct {
    server        server.Server
    isInitialized bool
    logger        *slog.Logger
    startTime     time.Time
}

func (s *simplifiedGomcpManager) Start(_ context.Context) error {
    s.server = server.NewServer("Container Kit MCP Server",
        server.WithLogger(s.logger),
        server.WithProtocolVersion("1.0.0"),
    ).AsStdio()

    if mcpServer, ok := s.server.(interface{ Run() error }); ok {
        return mcpServer.Run()
    }

    return errors.NewError().Messagef("server does not implement Run() method").Build()
}
```

### Tool Registration with gomcp

Container Kit uses a sophisticated tool registration system:

```go
func (s *simplifiedGomcpManager) RegisterTools(srv *Server) error {
    // Register analyze_repository tool
    s.server.Tool("analyze_repository", "Analyze repository structure...",
        func(ctx *server.Context, args *analyze.AtomicAnalyzeRepositoryArgs) (*analyze.AtomicAnalysisResult, error) {
            result, err := analyzeRepoTool.ExecuteRepositoryAnalysis(adaptMCPContext(ctx), *args)
            return result, err
        })

    // Register additional tools...
}
```

**Registered Tools:**
- `analyze_repository`: Repository analysis and Dockerfile generation
- `build_image`: Docker image building with error fixing
- `push_image`: Container registry operations
- `generate_manifests`: Kubernetes manifest generation
- `scan_image`: Security vulnerability scanning
- `detect_databases`: Database detection in repositories
- `list_sessions`: Session management utilities
- `ping/server_status`: Diagnostic tools

### Context Adaptation

The system bridges gomcp's server context with Go's standard context:

```go
func adaptMCPContext(mcpCtx *server.Context) context.Context {
    // Convert gomcp server.Context to Go context.Context
    return context.Background()
}
```

## Transport Layer Implementation

### Unified Transport Interface

Container Kit implements a unified transport interface that abstracts both stdio and HTTP transports:

```go
type Transport interface {
    Serve(ctx context.Context) error
    SetHandler(handler RequestHandler)
    Send(ctx context.Context, message interface{}) error
    Receive(ctx context.Context) (interface{}, error)
    Close() error
}
```

### stdio Transport Implementation

The stdio transport (`pkg/mcp/transport/stdio.go`) provides direct integration with gomcp:

```go
type StdioTransport struct {
    server       server.Server
    gomcpManager interface{} // GomcpManager interface for shutdown
    errorHandler *StdioErrorHandler
    logger       zerolog.Logger
    handler      core.RequestHandler
}

func (s *StdioTransport) Serve(ctx context.Context) error {
    // Delegate to gomcp manager
    mgr := s.gomcpManager.(interface{ StartServer() error })

    // Run server in goroutine
    serverDone := make(chan error, 1)
    go func() {
        if err := mgr.StartServer(); err != nil {
            serverDone <- err
        }
    }()

    // Wait for context cancellation or server error
    select {
    case <-ctx.Done():
        return s.Close()
    case err := <-serverDone:
        return err
    }
}
```

### HTTP Transport Implementation

The HTTP transport (`pkg/mcp/transport/http.go`) provides a REST API wrapper over MCP concepts:

```go
type HTTPTransport struct {
    tools          map[string]*ToolInfo
    router         *chi.Mux
    server         *http.Server
    logger         *slog.Logger
    handler        core.RequestHandler
    // ... additional fields
}

func (t *HTTPTransport) setupRouter() {
    t.router.Route("/api/v1", func(r chi.Router) {
        r.Get("/tools", t.handleListTools)
        r.Get("/tools/{tool}/schema", t.handleGetToolSchema)
        r.Post("/tools/{tool}", t.handleExecuteTool)
        r.Get("/health", t.handleHealth)
        r.Get("/sessions", t.handleListSessions)
        // ... additional routes
    })
}
```

### Transport Selection

The server automatically selects the appropriate transport based on configuration:

```go
switch config.TransportType {
case "stdio":
    mcpTransport = transport.NewStdioTransport()
case "http":
    mcpTransport = transport.NewHTTPTransport(transport.HTTPTransportConfig{
        Port:           config.HTTPPort,
        CORSOrigins:    config.CORSOrigins,
        APIKey:         config.APIKey,
        RateLimit:      config.RateLimit,
        Logger:         logger,
    })
}
```

## Server Interfaces and Lifecycle

### Server Configuration

The server supports comprehensive configuration:

```go
type ServerConfig struct {
    ServiceName      string
    TransportType    string // "stdio" or "http"
    LogLevel         string
    WorkspaceDir     string
    StorePath        string
    MaxSessions      int
    SessionTTL       time.Duration
    MaxWorkers       int
    JobTTL           time.Duration
    HTTPPort         int
    CORSOrigins      []string
    APIKey           string
    RateLimit        int
    // ... additional fields
}
```

### Server Lifecycle

The server follows a structured lifecycle:

1. **Initialization**: Configuration loading and validation
2. **Component Setup**: Session manager, job manager, transport creation
3. **Tool Registration**: Automatic tool discovery and registration
4. **Transport Start**: Protocol-specific server startup
5. **Graceful Shutdown**: Cleanup and resource release

```go
func (s *Server) Start(ctx context.Context) error {
    s.logger.Info("Starting Container Kit MCP Server")

    // Start cleanup routines
    s.sessionManager.StartCleanupRoutine()

    // Register tools with gomcp
    if err := s.gomcpManager.RegisterTools(s); err != nil {
        return err
    }

    // Start transport
    return s.gomcpManager.Start(ctx)
}
```

### Multi-Mode Operation

The server supports three operation modes:

- **Chat Mode**: Direct conversational tool interaction
- **Workflow Mode**: Multi-step atomic operations
- **Dual Mode**: Both chat and workflow capabilities

## Tool Registration and Execution Flow

### Tool Registration Architecture

Tools are registered through a centralized system in `pkg/mcp/server/core.go`:

```go
func (s *simplifiedGomcpManager) RegisterTools(srv *Server) error {
    // Create tool instances with dependency injection
    analyzeRepoTool := analyze.NewAtomicAnalyzeRepositoryTool(pipelineOps, unifiedSessionMgr, srv.logger)
    buildImageTool := build.NewAtomicBuildImageTool(pipelineOps, unifiedSessionMgr, srv.logger)
    // ... additional tools

    // Register with gomcp using lambda functions
    s.server.Tool("analyze_repository", "Description...",
        func(ctx *server.Context, args *analyze.AtomicAnalyzeRepositoryArgs) (*analyze.AtomicAnalysisResult, error) {
            return analyzeRepoTool.ExecuteRepositoryAnalysis(adaptMCPContext(ctx), *args)
        })
}
```

### Tool Execution Pipeline

The execution follows a multi-stage pipeline:

```
MCP Request → gomcp Server → Lambda Function → Tool Implementation → Session Management → Result Processing
```

**Stage 1: MCP Protocol Handling**
- gomcp library receives and parses MCP requests
- Protocol-level validation and context creation
- Tool lookup and routing

**Stage 2: Argument Processing**
- Type conversion from MCP JSON to Go structs
- Parameter validation and sanitization
- Session ID extraction and validation

**Stage 3: Tool Execution**
- Session retrieval and workspace setup
- Core operation execution via pipeline operations
- Error handling and recovery

**Stage 4: Result Serialization**
- Result packaging into standardized formats
- Error formatting and context preservation
- MCP response generation

### Type-Safe Parameter Handling

The system implements multiple layers of type safety:

```go
// MCP Protocol Level - gomcp handles JSON unmarshaling
func(ctx *server.Context, args *analyze.AtomicAnalyzeRepositoryArgs) (*analyze.AtomicAnalysisResult, error)

// Internal API Level
type ToolInput struct {
    SessionID string                 `json:"session_id"`
    Data      map[string]interface{} `json:"data"`
    Context   map[string]interface{} `json:"context,omitempty"`
}

// Tool-Specific Parameters
type AtomicAnalyzeRepositoryArgs struct {
    SessionID    string `json:"session_id"`
    RepoURL      string `json:"repo_url"`
    Branch       string `json:"branch,omitempty"`
    Context      string `json:"context,omitempty"`
    LanguageHint string `json:"language_hint,omitempty"`
    Shallow      bool   `json:"shallow,omitempty"`
}
```

## Error Handling and Recovery

### Rich Error System

Container Kit implements a comprehensive error handling system:

```go
type TypedMCPError struct {
    Category      ErrorCategory
    Module        string
    Operation     string
    Message       string
    Cause         error
    StringFields  map[string]string
    NumberFields  map[string]float64
    BooleanFields map[string]bool
    Retryable     bool
    Recoverable   bool
}
```

### Error Categories

- `CategoryValidation`: Input validation failures
- `CategoryNetwork`: Connection and network issues
- `CategoryInternal`: System-level failures
- `CategoryAuth`: Authentication/authorization issues
- `CategoryResource`: Resource-related problems
- `CategoryTimeout`: Operation timeouts

### Error Propagation

```go
return errors.NewError().
    Code(errors.CodeToolExecutionFailed).
    Message("Repository analysis execution failed").
    Type(errors.ErrTypeInternal).
    Severity(errors.SeverityHigh).
    Cause(err).
    Context("repository_path", analyzeParams.RepositoryPath).
    Context("session_id", analyzeParams.SessionID).
    Suggestion("Check repository accessibility and path validity").
    WithLocation().
    Build()
```

### Recovery Mechanisms

- **Retry Logic**: Configurable retry policies for transient failures
- **Circuit Breakers**: Prevent cascading failures (planned)
- **Graceful Degradation**: Fallback behaviors for non-critical failures
- **Session Recovery**: Automatic session state restoration

## Session Management Integration

### Session-Tool Integration

Every tool execution is tied to a session context:

```go
func (t *AtomicBuildImageTool) ExecuteWithContext(ctx context.Context, args *AtomicBuildImageArgs) (*AtomicBuildImageResult, error) {
    // Session retrieval
    sessionState, err := t.sessionManager.GetSession(ctx, args.SessionID)
    if err != nil {
        return nil, errors.NewError().Messagef("session not found: %s", args.SessionID).Build()
    }

    // Tool execution with session context
    buildParams := core.BuildImageParams{
        SessionID: sessionState.SessionID,
        WorkspaceDir: sessionState.WorkspaceDir,
        // ... other parameters
    }
}
```

### Session State Management

- **Session Creation**: Automatic session creation during tool execution
- **Isolated Workspaces**: Each session has dedicated workspace directories
- **State Persistence**: BoltDB-backed storage for session state
- **Metadata Tracking**: Label-based organization and tracking
- **Automatic Cleanup**: TTL-based session cleanup

## Performance and Scalability

### Execution Optimizations

- **Concurrent Tool Execution**: Multiple tools run simultaneously
- **Session Isolation**: Dedicated workspace per session
- **Resource Management**: Configurable limits and quotas
- **Connection Pooling**: Reused connections for external services

### Monitoring and Observability

- **Structured Logging**: Comprehensive logging with contextual information
- **Metrics Collection**: Prometheus-compatible metrics
- **Tracing Support**: OpenTelemetry integration (planned)
- **Error Tracking**: Rich error context and propagation

### Quality Standards

- **Error Budget**: 100 lint issues maximum
- **Performance Target**: <300μs P95 per request
- **Coverage Tracking**: Baseline enforcement
- **Pre-commit Hooks**: Code quality gates

## Security Considerations

### Input Validation

- **Tag-Based Validation DSL**: Declarative validation rules
- **Schema Validation**: JSON schema validation for tool parameters
- **Sanitization**: Input sanitization and normalization
- **Path Validation**: Secure path handling for file operations

### Authentication and Authorization

- **API Key Support**: HTTP transport authentication
- **Session Security**: Secure session token generation
- **Resource Limits**: Per-session resource quotas
- **Audit Logging**: Comprehensive audit trail

### Container Security

- **Image Scanning**: Trivy/Grype integration for vulnerability detection
- **Security Policies**: Configurable security policy enforcement
- **Least Privilege**: Minimal permission requirements
- **Sandbox Execution**: Isolated tool execution environment

## Future Enhancements

### Planned Features

1. **Advanced Orchestration**: Complex multi-tool workflows
2. **AI-Powered Error Recovery**: Intelligent error analysis and recovery
3. **Distributed Execution**: Multi-node tool execution
4. **Enhanced Monitoring**: Real-time metrics and alerting
5. **Plugin Architecture**: Extensible tool plugin system

### Technical Improvements

1. **Circuit Breaker Implementation**: Fault tolerance improvements
2. **Workspace Manager**: Enhanced workspace lifecycle management
3. **Conversation Mode**: Advanced conversational AI integration
4. **OpenTelemetry**: Distributed tracing implementation
5. **GraphQL API**: Alternative API interface

## Conclusion

Container Kit's MCP integration with gomcp demonstrates a production-ready architecture that successfully balances:

- **Simplicity**: Clean, understandable codebase with clear separation of concerns
- **Robustness**: Comprehensive error handling and recovery mechanisms
- **Scalability**: Efficient resource management and concurrent execution
- **Security**: Multi-layered security controls and validation
- **Maintainability**: Well-structured code with extensive documentation

The integration serves as an exemplary implementation of the Model Context Protocol for enterprise-grade AI-powered applications, providing a solid foundation for building complex containerization workflows while maintaining reliability and performance.

## References

- [ADR-001: Three-Context Architecture](architecture/adr/2025-07-07-three-context-architecture.md)
- [ADR-006: Manual Dependency Injection](architecture/adr/2025-01-07-manual-dependency-injection.md)
- [gomcp Library Documentation](https://github.com/localrivet/gomcp)
- [Model Context Protocol Specification](https://modelcontextprotocol.io/)
- [Container Kit README](../README.md)
