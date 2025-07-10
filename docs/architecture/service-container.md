# Service Container Pattern

Container Kit implements a **manual dependency injection** pattern through a service container, as defined in [ADR-006](adr/2025-01-07-manual-dependency-injection.md). This pattern replaced 4 large Manager interfaces (65+ methods) with 21 focused service interfaces (total of 32 core methods).

## Architecture Overview

The service container provides a unified interface for accessing all system services, enabling clean dependency injection and service discovery.

```go
type ServiceContainer interface {
    // Core Services (8 services, 32 methods)
    SessionStore() SessionStore        // Session CRUD operations (4 methods)
    SessionState() SessionState        // State & checkpoint management (4 methods)
    BuildExecutor() BuildExecutor      // Container build operations (5 methods)
    ToolRegistry() ToolRegistry        // Tool registration & discovery (5 methods)
    WorkflowExecutor() WorkflowExecutor // Multi-step workflows (4 methods)
    Scanner() Scanner                  // Security scanning (3 methods)
    ConfigValidator() ConfigValidator  // Configuration validation (4 methods)
    ErrorReporter() ErrorReporter      // Unified error handling (3 methods)
    
    // Specialized Services (13 additional services)
    FileAccessService() FileAccessService // Secure file operations (6 methods)
    StateManager() StateManager        // Application state management
    KnowledgeBase() KnowledgeBase      // Pattern storage and retrieval
    ConversationService() ConversationService // Chat-based interactions
    // ... and 9 more specialized services
}
```

## Core Services

### 1. SessionStore
**Purpose**: Session CRUD operations and metadata management
```go
type SessionStore interface {
    Create(ctx context.Context, session *Session) error
    Get(ctx context.Context, sessionID string) (*Session, error)
    Update(ctx context.Context, session *Session) error
    Delete(ctx context.Context, sessionID string) error
}
```

### 2. SessionState
**Purpose**: State and checkpoint management
```go
type SessionState interface {
    GetWorkspaceDir(ctx context.Context, sessionID string) (string, error)
    SaveCheckpoint(ctx context.Context, sessionID string, checkpoint *Checkpoint) error
    LoadCheckpoint(ctx context.Context, sessionID string) (*Checkpoint, error)
    ClearState(ctx context.Context, sessionID string) error
}
```

### 3. BuildExecutor
**Purpose**: Container build operations
```go
type BuildExecutor interface {
    Build(ctx context.Context, request *BuildRequest) (*BuildResponse, error)
    Push(ctx context.Context, request *PushRequest) (*PushResponse, error)
    Pull(ctx context.Context, request *PullRequest) (*PullResponse, error)
    Tag(ctx context.Context, request *TagRequest) (*TagResponse, error)
    GetBuildHistory(ctx context.Context, sessionID string) ([]*BuildRecord, error)
}
```

### 4. ToolRegistry
**Purpose**: Tool registration and discovery
```go
type ToolRegistry interface {
    Register(tool Tool) error
    Get(name string) (Tool, error)
    List() []Tool
    GetByCategory(category string) []Tool
    GetMetrics(toolName string) (*ToolMetrics, error)
}
```

### 5. WorkflowExecutor
**Purpose**: Multi-step workflow execution
```go
type WorkflowExecutor interface {
    Execute(ctx context.Context, workflow *Workflow) (*WorkflowResult, error)
    GetStatus(ctx context.Context, workflowID string) (*WorkflowStatus, error)
    Cancel(ctx context.Context, workflowID string) error
    ListActive(ctx context.Context) ([]*WorkflowStatus, error)
}
```

### 6. Scanner
**Purpose**: Security scanning operations
```go
type Scanner interface {
    ScanImage(ctx context.Context, imageRef string) (*ScanResult, error)
    GetScanHistory(ctx context.Context, sessionID string) ([]*ScanResult, error)
    UpdatePolicies(ctx context.Context, policies *SecurityPolicies) error
}
```

### 7. ConfigValidator
**Purpose**: Configuration validation
```go
type ConfigValidator interface {
    ValidateDockerfile(ctx context.Context, content string) (*ValidationResult, error)
    ValidateManifest(ctx context.Context, content string) (*ValidationResult, error)
    ValidateConfig(ctx context.Context, config interface{}) (*ValidationResult, error)
    GetValidationRules(ctx context.Context) (*ValidationRules, error)
}
```

### 8. ErrorReporter
**Purpose**: Unified error handling and reporting
```go
type ErrorReporter interface {
    Report(ctx context.Context, err error, context map[string]interface{}) error
    GetErrorStats(ctx context.Context, sessionID string) (*ErrorStats, error)
    ClassifyError(err error) *ErrorClassification
}
```

## Specialized Services

### FileAccessService
**Purpose**: Secure file operations with session isolation
```go
type FileAccessService interface {
    ReadFile(ctx context.Context, sessionID, relativePath string) (string, error)
    ListDirectory(ctx context.Context, sessionID, relativePath string) ([]FileInfo, error)
    FileExists(ctx context.Context, sessionID, relativePath string) (bool, error)
    GetFileTree(ctx context.Context, sessionID, rootPath string) (*FileTree, error)
    ReadFileWithMetadata(ctx context.Context, sessionID, relativePath string) (*FileContent, error)
    SearchFiles(ctx context.Context, sessionID, pattern string) ([]string, error)
}
```

**Security Features**:
- Session-based workspace isolation
- Path traversal protection
- File type and size validation
- Blocked path configuration
- Structured error handling

### StateManager
**Purpose**: Application state management
- Global state coordination
- Configuration management
- Runtime state tracking
- Event broadcasting

### KnowledgeBase
**Purpose**: Pattern storage and retrieval
- Best practice patterns
- Template management
- Knowledge graph storage
- Pattern matching

### ConversationService
**Purpose**: Chat-based interactions
- Conversation context management
- Message history tracking
- AI interaction coordination
- Response formatting

## Implementation Pattern

### Service Registration
Services are registered through the container implementation:

```go
type serviceContainer struct {
    sessionStore     SessionStore
    sessionState     SessionState
    buildExecutor    BuildExecutor
    toolRegistry     ToolRegistry
    workflowExecutor WorkflowExecutor
    scanner          Scanner
    configValidator  ConfigValidator
    errorReporter    ErrorReporter
    fileAccessService FileAccessService
    // ... other services
}

func (c *serviceContainer) SessionStore() SessionStore {
    return c.sessionStore
}

func (c *serviceContainer) FileAccessService() FileAccessService {
    return c.fileAccessService
}
```

### Service Usage in Tools
Tools access services through the container:

```go
func LazyAnalyzeTool() LazyTool {
    return func(serviceContainer services.ServiceContainer) (string, string, interface{}) {
        // Access required services
        fileAccess := serviceContainer.FileAccessService()
        sessionState := serviceContainer.SessionState()
        configValidator := serviceContainer.ConfigValidator()
        
        toolFunc := func(ctx *server.Context, args *AnalyzeArgs) (*AnalyzeResult, error) {
            return executeAnalysis(ctx, args, fileAccess, sessionState, configValidator)
        }
        
        return "analyze_repository", "Repository analysis with FileAccessService", toolFunc
    }
}
```

## Benefits

### 1. Focused Interfaces
- **Single Responsibility**: Each service has a clear purpose
- **Method Reduction**: 51% reduction in total methods (65 → 32 core methods)
- **Interface Segregation**: Clients depend only on methods they use
- **Testability**: Easy to mock individual services

### 2. Dependency Injection
- **Loose Coupling**: Services depend on abstractions
- **Testability**: Easy to inject test doubles
- **Configurability**: Services can be swapped at runtime
- **Maintainability**: Changes isolated to service implementations

### 3. Service Discovery
- **Unified Access**: Single point of access for all services
- **Type Safety**: Compile-time service resolution
- **Documentation**: Clear service contracts
- **Monitoring**: Service health and metrics tracking

### 4. Error Handling
- **Consistent Patterns**: All services use unified error system
- **Context Preservation**: Rich error context across service boundaries
- **Failure Isolation**: Service failures don't cascade
- **Recovery**: Automatic retry and fallback mechanisms

## Quality Standards

### Performance
- **Lazy Loading**: Services initialized on first access
- **Connection Pooling**: Efficient resource utilization
- **Caching**: Smart caching strategies per service
- **Monitoring**: Performance metrics and alerting

### Security
- **Service Isolation**: Services operate in isolated contexts
- **Authentication**: Service-level authentication
- **Authorization**: Fine-grained access control
- **Audit**: Complete audit trail of service interactions

### Reliability
- **Health Checks**: Service health monitoring
- **Circuit Breakers**: Failure protection
- **Retry Logic**: Automatic retry with backoff
- **Graceful Degradation**: Fallback mechanisms

## Related Documentation

- [ADR-006: Manual Dependency Injection](adr/2025-01-07-manual-dependency-injection.md)
- [Three-Layer Architecture](three-layer-architecture.md)
- [Tool Development Guide](../guides/developer/adding-new-tools.md)
- [Error Handling Guide](../guides/developer/error-handling.md)
- [API Reference](../reference/api/interfaces.md)

## Next Steps

1. **Understand Services**: Review individual service interfaces
2. **Tool Development**: Use services in [Adding New Tools](../guides/developer/adding-new-tools.md)
3. **Error Handling**: Implement [Error Handling Patterns](../guides/developer/error-handling.md)
4. **Testing**: Create service mocks for testing
5. **Monitoring**: Set up service health monitoring

The service container pattern provides a clean, maintainable foundation for Container Kit's sophisticated architecture while maintaining high performance and reliability standards.