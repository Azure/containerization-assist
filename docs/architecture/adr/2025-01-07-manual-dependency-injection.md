# ADR-006: Manual Dependency Injection for Service Architecture

**Date**: 2025-01-07
**Status**: Accepted
**Context**: EPSILON Workstream - DI Implementation & Integration

## Context

The Container-Kit architecture currently uses 4 large Manager interfaces (SessionManager, BuildManager, RegistryManager, WorkflowManager) with 65+ total methods. This creates several problems:

1. **Testing Complexity**: Large manager interfaces are difficult to mock comprehensively
2. **Interface Bloat**: Managers have 13-19 methods each, violating single responsibility
3. **Hidden Dependencies**: Unclear relationships between managers and their dependencies
4. **Wrapper Anti-patterns**: Manager → Service → Implementation chains add overhead

## Decision

We will implement **manual dependency injection** to replace the 4 large Manager interfaces with 8 focused service interfaces:

### Service Breakdown
1. **SessionStore** (4 methods) - Session CRUD operations
2. **SessionState** (4 methods) - State and checkpoint management
3. **BuildExecutor** (5 methods) - Container build operations
4. **ToolRegistry** (5 methods) - Tool registration and discovery
5. **WorkflowExecutor** (4 methods) - Multi-step workflow orchestration
6. **Scanner** (3 methods) - Security scanning capabilities
7. **ConfigValidator** (4 methods) - Unified configuration validation
8. **ErrorReporter** (3 methods) - Unified error handling

### DI Implementation Approach
- **Manual DI Container**: Single struct containing all service instances
- **Constructor injection**: Services receive dependencies via constructors
- **Explicit wiring**: Clear dependency relationships in container setup
- **Lifecycle management**: Container handles service start/stop ordering

## Alternatives Considered

### 1. Keep Manager Interfaces
- **Pros**: No migration effort, existing patterns
- **Cons**: Testing complexity, interface bloat, unclear dependencies
- **Rejected**: Does not solve core architectural problems

### 2. DI Framework (e.g., uber/dig, google/wire)
- **Pros**: Automatic dependency resolution, reflection-based injection
- **Cons**: Added complexity, runtime reflection overhead, learning curve
- **Rejected**: Adds unnecessary complexity for our focused service set

### 3. Service Locator Pattern
- **Pros**: Centralized service access, runtime service resolution
- **Cons**: Hidden dependencies, service locator anti-pattern, harder testing
- **Rejected**: Does not improve testability or dependency clarity

## Consequences

### Positive
- **Focused interfaces**: 3-5 methods per service vs 13-19 per manager
- **Direct testability**: Mock individual services without manager complexity
- **Clear dependencies**: Explicit service injection makes relationships obvious
- **Performance**: No manager/adapter overhead, direct service calls
- **Maintainability**: Single responsibility services are easier to understand
- **Testing speed**: Faster tests due to focused mocking and no wrapper overhead

### Negative
- **Migration effort**: Need to update all tools and workflows to use services
- **Manual wiring**: Need to maintain dependency relationships in container
- **Service proliferation**: 8 services vs 4 managers (though more focused)

### Neutral
- **Container maintenance**: Need to update DI container when adding services
- **Dependency ordering**: Must handle service startup/shutdown dependencies

## Implementation Details

### Service Container Interface
```go
type ServiceContainer interface {
    // Session services
    SessionStore() SessionStore
    SessionState() SessionState

    // Build services
    BuildExecutor() BuildExecutor

    // Registry services
    ToolRegistry() ToolRegistry

    // Workflow services
    WorkflowExecutor() WorkflowExecutor

    // Security services
    Scanner() Scanner

    // Cross-cutting services
    ConfigValidator() ConfigValidator
    ErrorReporter() ErrorReporter

    // Lifecycle
    Close() error
}
```

### Tool Integration Example
```go
// Before: Tool depends on large manager
type BuildTool struct {
    buildManager interfaces.BuildManager  // 18 methods
}

// After: Tool depends on focused services
type BuildTool struct {
    buildExecutor   services.BuildExecutor    // 5 methods
    configValidator services.ConfigValidator  // 4 methods
    errorReporter   services.ErrorReporter   // 3 methods
}
```

## Success Metrics

- ✅ **Service count**: 8 focused interfaces (target: 4-8)
- ✅ **Method reduction**: 32 service methods vs 65 manager methods (51% reduction)
- ✅ **Interface focus**: 3-5 methods per service vs 13-19 per manager
- ✅ **Testing simplicity**: Direct service mocking without manager complexity
- ✅ **Performance**: No degradation from manager approach
- ✅ **Integration success**: All components working together seamlessly

## References

- [Service Interface Design](/pkg/mcp/services/interfaces.go)
- [DI Implementation Strategy](/docs/DI_IMPLEMENTATION_STRATEGY.md)
- [EPSILON Workstream Plan](/docs/WORKSTREAM_EPSILON_PROMPT.md)
- [BETA Validation Framework](/pkg/mcp/domain/validation/)
- [BETA Error System](/pkg/mcp/domain/errors/)
- [GAMMA Tool Interface](/pkg/mcp/application/api/interfaces.go)

## Related ADRs

- ADR-003: Three-Context Architecture (ALPHA)
- ADR-004: Unified Validation Framework (BETA)
- ADR-005: Canonical Tool Interface (GAMMA)
