# Container Kit Architecture Overview

Container Kit is built on a **three-layer clean architecture** with sophisticated service-oriented design patterns, providing a production-ready, enterprise-grade AI-powered containerization platform.

## High-Level Architecture

```
Container Kit (606 files, 159k+ lines)
├── Domain Layer (101 files)      # Pure business logic
├── Application Layer (153 files)  # Orchestration & coordination
└── Infrastructure Layer (42 files) # External integrations
```

## Core Design Principles

### 1. Three-Layer Architecture (ADR-001)
- **Domain Layer**: Pure business logic with zero external dependencies
- **Application Layer**: Orchestration and coordination of domain services
- **Infrastructure Layer**: External integrations and technical concerns

### 2. Service Container Pattern (ADR-006)
- **21 Services**: Including FileAccessService for secure file operations
- **Manual Dependency Injection**: Focused service interfaces
- **Service Boundaries**: Clear separation of concerns

### 3. Unified Interface System
- **Single Source of Truth**: All interfaces in `application/api/interfaces.go`
- **Consistent Patterns**: Standardized tool and service interfaces
- **Auto-Registration**: Zero-configuration tool discovery

## Key Components

### Service Container (21 Services)
```go
type ServiceContainer interface {
    // Core Services (8)
    SessionStore() SessionStore
    SessionState() SessionState
    BuildExecutor() BuildExecutor
    ToolRegistry() ToolRegistry
    WorkflowExecutor() WorkflowExecutor
    Scanner() Scanner
    ConfigValidator() ConfigValidator
    ErrorReporter() ErrorReporter
    
    // Specialized Services (13)
    FileAccessService() FileAccessService  // Secure file operations
    StateManager() StateManager
    KnowledgeBase() KnowledgeBase
    ConversationService() ConversationService
    // ... and 9 more services
}
```

### Tool Architecture (12 Production Tools)
- **Containerization Tools (6)**: analyze, generate, build, push, deploy, scan
- **File Access Tools (3)**: read_file, list_directory, file_exists
- **Session Tools (1)**: list_sessions
- **Diagnostic Tools (2)**: ping, server_status

### Security Architecture
- **FileAccessService**: Session-based workspace isolation
- **Path Validation**: Protection against traversal attacks
- **Vulnerability Scanning**: Trivy/Grype integration
- **Session Management**: BoltDB-backed persistence

## Technology Stack

### Core Technologies
- **Language**: Go 1.24.1
- **Protocol**: Model Context Protocol (MCP)
- **Storage**: BoltDB for session persistence
- **Containers**: Docker with full lifecycle management
- **Orchestration**: Kubernetes with manifest generation

### Observability
- **Metrics**: Prometheus integration
- **Tracing**: OpenTelemetry support
- **Logging**: Structured logging with slog
- **Monitoring**: Comprehensive service health checks

## Architectural Patterns

### Domain-Driven Design
- **Bounded Contexts**: Clear domain boundaries
- **Aggregates**: Consistent business entities
- **Domain Services**: Business logic encapsulation
- **Event Sourcing**: Audit trail for state changes

### Error Handling (ADR-004)
- **Unified Rich Error System**: Structured error context
- **Error Classification**: Severity and category-based
- **Actionable Suggestions**: Resolution guidance
- **Source Location**: Debugging information

### Validation (ADR-005)
- **Tag-Based DSL**: Declarative validation rules
- **Code Generation**: Automated validation logic
- **Type Safety**: Compile-time validation
- **Security Validation**: Input sanitization

## Quality Standards

### Performance Targets
- **Response Time**: <300μs P95 latency
- **Throughput**: Concurrent session handling
- **Resource Usage**: Efficient memory and CPU usage
- **Scalability**: Horizontal scaling capabilities

### Code Quality
- **Lint Budget**: <100 issues maximum
- **Test Coverage**: Comprehensive unit and integration tests
- **Documentation**: Living documentation with examples
- **Security**: Vulnerability scanning and validation

## Deployment Architecture

### Multi-Mode Operations
- **Chat Mode**: Direct conversational tool interaction
- **Workflow Mode**: Multi-step atomic operations
- **Dual Mode**: Combined chat and workflow capabilities

### Session Management
- **Workspace Isolation**: Session-scoped file operations
- **State Persistence**: BoltDB-backed session storage
- **Lifecycle Management**: Automatic cleanup and recovery
- **Context Preservation**: Conversation and workflow continuity

## Related Documentation

- [Three-Layer Architecture Details](three-layer-architecture.md)
- [Service Container Pattern](service-container.md)
- [Architecture Decision Records](adr/)
- [Developer Guide](../guides/developer/)
- [Tool Inventory](../reference/tools/inventory.md)

## Next Steps

1. **Understand Layers**: Review [Three-Layer Architecture](three-layer-architecture.md)
2. **Explore Services**: Study [Service Container Pattern](service-container.md)
3. **Review ADRs**: Read [Architecture Decision Records](adr/)
4. **See Examples**: Check [Examples](../examples/)
5. **Development**: Follow [Developer Guides](../guides/developer/)

Container Kit's architecture balances simplicity with extensibility, providing a solid foundation for enterprise containerization needs while maintaining high performance and security standards.