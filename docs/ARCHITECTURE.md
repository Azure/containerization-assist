# Container Kit Architecture

## Executive Summary

Container Kit is an AI-powered containerization platform with a **domain-driven architecture** optimized for MCP (Model Context Protocol) integration. The system provides atomic tools for precise containerization operations organized by functional domain.

### Key Achievements
- ✅ **Domain-driven organization** with clear separation of concerns
- ✅ **Atomic tool pattern** for composable operations
- ✅ **Unified session management** across all tools
- ✅ **Type-safe interfaces** with comprehensive validation
- ✅ **Extensible architecture** for easy addition of new domains and tools

## System Overview

### Core Architecture Pattern

Container Kit follows a **domain-driven design** with the following structure:

```
pkg/mcp/domain/
├── containerization/          # Core containerization operations
│   ├── analyze/              # Repository analysis & Dockerfile generation
│   ├── build/                # Docker build operations (placeholder)
│   ├── deploy/               # Kubernetes deployment & manifest generation
│   └── scan/                 # Security scanning & secret detection
├── session/                  # Session management & lifecycle
├── types/                    # Core type definitions & interfaces
├── validation/               # Input validation framework
└── common/                   # Shared utilities
```

### Tool Categories

#### Analyze Domain
- **Purpose**: Repository analysis and Dockerfile generation
- **Tools**: `analyze_repository_atomic`, `generate_dockerfile`, `validate_dockerfile_atomic`
- **Pattern**: Read-only operations with comprehensive metadata output

#### Deploy Domain
- **Purpose**: Kubernetes deployment and management
- **Tools**: `generate_manifests_atomic`, `deploy_kubernetes_atomic`, `check_health_atomic`
- **Pattern**: Declarative operations with health monitoring

#### Scan Domain
- **Purpose**: Security analysis and vulnerability detection
- **Tools**: `scan_image_security_atomic`, `scan_secrets_atomic`
- **Pattern**: Analysis operations with severity classification

#### Session Domain
- **Purpose**: Session lifecycle and state management
- **Tools**: `list_sessions`, `delete_session`, `manage_session_labels`
- **Pattern**: BoltDB-backed persistence with label-based organization

## Design Principles

### 1. Atomic Composability
Tools are designed as atomic operations that can be composed to create complex workflows:
- Each tool performs a single, well-defined task
- Clear input/output contracts
- Can be used independently or in combination
- Idempotent behavior where possible

### 2. Domain-Driven Organization
Functionality is organized by business domain:
- **Containerization**: Core Docker and Kubernetes operations
- **Session**: State and lifecycle management
- **Validation**: Input validation and sanitization
- **Security**: Threat assessment and vulnerability scanning

### 3. Type-Safe Interfaces
All tools implement consistent, type-safe interfaces:
- Compile-time validation of tool signatures
- Structured argument and response types
- Comprehensive validation frameworks
- Clear error handling patterns

### 4. Session-Centric Design
Persistent sessions maintain state across operations:
- BoltDB-backed persistence
- Cross-tool state sharing
- Workspace isolation
- Metadata tracking and labeling

## Core Components

### 1. Tool Interface System

All tools implement a standard interface defined in `pkg/mcp/domain/tools/interface.go`:

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() *json.RawMessage
    Execute(ctx context.Context, input json.RawMessage) (*ExecutionResult, error)
    Category() string
    Tags() []string
    Version() string
}
```

### 2. Session Management

Session management is handled by `pkg/mcp/domain/session/` with features:
- **Persistent Storage**: BoltDB for lightweight, embedded persistence
- **Label System**: Flexible metadata and organization
- **Workspace Isolation**: Each session gets isolated workspace directory
- **Lifecycle Management**: Automatic cleanup and expiration handling

### 3. Validation Framework

Located in `pkg/mcp/domain/validation/`, provides:
- **Type-safe validation**: Compile-time validation rules
- **Extensible rules**: Domain-specific validation logic
- **Error aggregation**: Comprehensive validation reports
- **Sanitization**: Input cleaning and normalization

### 4. Common Utilities

Shared functionality in `pkg/mcp/domain/common/`:
- **Error handling**: Rich error types with context
- **Path utilities**: Safe file system operations
- **Type assertions**: Safe type conversions
- **Validation mixins**: Reusable validation components

## Interface Design

### Unified Tool Interface

All tools use the `api.Tool` interface for consistency:

```go
type Tool interface {
    Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error)
    Name() string
    Description() string
    Schema() api.ToolSchema
}
```

### Standard Input/Output

Tools use standardized input and output structures:

```go
type ToolInput struct {
    SessionID string                 `json:"session_id,omitempty"`
    Data      map[string]interface{} `json:"data"`
}

type ToolOutput struct {
    Success bool                   `json:"success"`
    Data    map[string]interface{} `json:"data,omitempty"`
    Error   string                 `json:"error,omitempty"`
}
```

## Data Flow

### Tool Execution Flow

```
1. Input Validation
   ├── Parse and validate JSON input
   ├── Type checking and constraint validation
   └── Domain-specific validation rules

2. Session Management
   ├── Create or retrieve session
   ├── Set up workspace directory
   └── Load session state

3. Tool Execution
   ├── Execute domain-specific logic
   ├── Update session state
   └── Generate structured output

4. Response Processing
   ├── Serialize execution results
   ├── Update session metadata
   └── Return formatted response
```

## Extensibility

### Adding New Domains

1. Create domain package under `pkg/mcp/domain/`
2. Implement domain-specific tools
3. Follow standard interface patterns
4. Add domain-specific validation rules
5. Register tools in the tool registry

### Adding New Tools

1. Choose appropriate domain package
2. Implement `api.Tool` interface
3. Define input/output schemas
4. Add comprehensive validation
5. Include unit and integration tests

## Performance Considerations

### Session Management
- **BoltDB**: Efficient key-value storage for session data
- **Lazy Loading**: Sessions created only when needed
- **Background Cleanup**: Automatic cleanup of expired sessions
- **Memory Efficiency**: Bounded memory usage for large states

### Tool Registration
- **Build-time Registration**: Tools discovered at compile time
- **Direct Interface Calls**: No reflection overhead
- **Metadata Caching**: Fast tool lookup and schema access
- **Type Safety**: Compile-time validation eliminates runtime errors

## Security Model

### Input Validation
- All tool inputs validated before execution
- Type checking and constraint enforcement
- Injection attack prevention
- Resource limit enforcement

### Workspace Isolation
- Each session gets isolated workspace directory
- File system operations constrained to workspace
- Temporary file cleanup on session end
- Permission management for external tool execution

### Error Handling
- Structured error types with context
- No sensitive information in error messages
- Comprehensive logging for debugging
- Error sanitization and redaction

## Future Evolution

### Planned Enhancements
1. **Build Domain**: Complete Docker build operations implementation
2. **Registry Integration**: Enhanced container registry support
3. **Workflow Orchestration**: Complex multi-tool workflows
4. **Plugin System**: Dynamic tool loading and extension points

### Architectural Evolution
1. **Event Sourcing**: Full audit trail of tool executions
2. **Distributed Sessions**: Multi-node session sharing
3. **GraphQL API**: Rich query interface for tool metadata
4. **Microservices**: Optional split into separate services

## Conclusion

Container Kit's domain-driven architecture provides a solid foundation for AI-powered containerization that balances simplicity with extensibility. The atomic tool pattern, unified session management, and type-safe interfaces enable reliable, maintainable operations while preserving the flexibility needed for diverse containerization scenarios.

The architecture successfully supports current needs while providing clear evolution paths for future requirements, making it an ideal platform for both immediate use and long-term development.
