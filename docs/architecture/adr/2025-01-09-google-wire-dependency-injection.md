# ADR-007: Google Wire for Compile-Time Dependency Injection

## Status
**Accepted** - January 9, 2025

## Context

### Current State
Container Kit currently uses manual dependency injection with large service container interfaces and scattered dependency management:

```go
// Current approach - manual wiring in multiple places
type ServiceContainer interface {
    SessionStore() SessionStore        // 8 methods
    SessionState() SessionState        // 6 methods
    BuildExecutor() BuildExecutor      // 12 methods
    ToolRegistry() ToolRegistry        // 7 methods
    // ... 4 more services with 32 total methods
}

// Manual wiring in constructors
func NewServer(
    sessionStore SessionStore,
    sessionState SessionState,
    buildExecutor BuildExecutor,
    toolRegistry ToolRegistry,
    workflowExecutor WorkflowExecutor,
    scanner Scanner,
    configValidator ConfigValidator,
    errorReporter ErrorReporter,
) *Server {
    return &Server{
        sessionStore: sessionStore,
        sessionState: sessionState,
        // ... manual field assignment
    }
}
```

### Problems with Current Approach
1. **Boilerplate Explosion**: 17+ manual dependencies in constructors
2. **Reflection Usage**: Current tool registry uses `reflect.TypeOf()` for registration
3. **Runtime Errors**: Dependency wiring failures discovered at runtime
4. **Testing Complexity**: Manual mock setup for 8 service interfaces
5. **Maintenance Burden**: Adding new dependencies requires updating multiple constructors

### Evaluation of DI Solutions

#### Option 1: Continue Manual DI
- **Pros**: Simple, no external dependencies, full control
- **Cons**: Boilerplate explosion, runtime errors, maintenance burden

#### Option 2: Uber Fx Framework
- **Pros**: Runtime DI, flexible, good Go adoption
- **Cons**: Runtime reflection, complex lifecycle management, large dependency

#### Option 3: Google Wire (Chosen)
- **Pros**: Compile-time generation, zero runtime overhead, type safety
- **Cons**: Learning curve, build-time dependency

## Decision

**We will adopt Google Wire for compile-time dependency injection** to replace manual dependency wiring while maintaining zero runtime overhead.

### Implementation Strategy

#### 1. Wire Provider Functions
```go
// pkg/mcp/application/di/providers.go
package di

import (
    "github.com/google/wire"
    "pkg/mcp/application/services"
    "pkg/mcp/infra/persistence"
    "pkg/mcp/infra/docker"
)

// ServiceSet provides all application services
var ServiceSet = wire.NewSet(
    // Storage providers
    persistence.NewBoltStore,
    wire.Bind(new(services.SessionStore), new(*persistence.BoltStore)),

    // Docker providers
    docker.NewClient,
    wire.Bind(new(services.BuildExecutor), new(*docker.Client)),

    // Registry providers
    registry.NewUnifiedRegistry,
    wire.Bind(new(services.ToolRegistry), new(*registry.UnifiedRegistry)),

    // Session providers
    session.NewManager,
    wire.Bind(new(services.SessionState), new(*session.Manager)),

    // Workflow providers
    workflow.NewExecutor,
    wire.Bind(new(services.WorkflowExecutor), new(*workflow.Executor)),

    // Security providers
    security.NewScanner,
    wire.Bind(new(services.Scanner), new(*security.Scanner)),

    // Config providers
    config.NewValidator,
    wire.Bind(new(services.ConfigValidator), new(*config.Validator)),

    // Error providers
    errors.NewReporter,
    wire.Bind(new(services.ErrorReporter), new(*errors.Reporter)),
)
```

#### 2. Wire Injector Functions
```go
// pkg/mcp/application/di/wire.go
//go:build wireinject
// +build wireinject

package di

import (
    "github.com/google/wire"
    "pkg/mcp/application/core"
    "pkg/mcp/application/services"
)

// InitializeServer creates a fully wired server
func InitializeServer(config *core.Config) (*core.Server, error) {
    wire.Build(
        ServiceSet,
        core.NewServer,
    )
    return nil, nil
}

// InitializeToolRegistry creates a wired tool registry
func InitializeToolRegistry() (services.ToolRegistry, error) {
    wire.Build(
        ServiceSet,
        registry.NewUnifiedRegistry,
    )
    return nil, nil
}

// InitializePipeline creates a wired pipeline
func InitializePipeline() (services.Pipeline, error) {
    wire.Build(
        ServiceSet,
        pipeline.NewOrchestrationPipeline,
    )
    return nil, nil
}
```

#### 3. Generated Code Integration
```go
// pkg/mcp/application/core/server.go
package core

import (
    "pkg/mcp/application/di"
    "pkg/mcp/application/services"
)

// Server with Wire-injected dependencies
type Server struct {
    sessionStore     services.SessionStore
    sessionState     services.SessionState
    buildExecutor    services.BuildExecutor
    toolRegistry     services.ToolRegistry
    workflowExecutor services.WorkflowExecutor
    scanner          services.Scanner
    configValidator  services.ConfigValidator
    errorReporter    services.ErrorReporter
}

// NewServer constructor for Wire
func NewServer(
    sessionStore services.SessionStore,
    sessionState services.SessionState,
    buildExecutor services.BuildExecutor,
    toolRegistry services.ToolRegistry,
    workflowExecutor services.WorkflowExecutor,
    scanner services.Scanner,
    configValidator services.ConfigValidator,
    errorReporter services.ErrorReporter,
) *Server {
    return &Server{
        sessionStore:     sessionStore,
        sessionState:     sessionState,
        buildExecutor:    buildExecutor,
        toolRegistry:     toolRegistry,
        workflowExecutor: workflowExecutor,
        scanner:          scanner,
        configValidator:  configValidator,
        errorReporter:    errorReporter,
    }
}

// Factory function using Wire
func CreateServer(config *Config) (*Server, error) {
    return di.InitializeServer(config)
}
```

#### 4. Build Integration
```makefile
# Add to Makefile
.PHONY: wire-generate
wire-generate:
    @echo "Generating Wire code..."
    wire gen ./pkg/mcp/application/di/...
    @echo "✅ Wire code generated"

.PHONY: wire-check
wire-check:
    @echo "Checking Wire code is up to date..."
    wire check ./pkg/mcp/application/di/...
    @echo "✅ Wire code is up to date"

# Update build target
.PHONY: build
build: wire-generate mcp

# Update pre-commit
.PHONY: pre-commit
pre-commit: wire-check
    @pre-commit run --all-files
    @$(MAKE) validate-architecture
```

## Consequences

### Positive
1. **Compile-Time Safety**: Dependency errors caught at build time, not runtime
2. **Zero Runtime Overhead**: No reflection, no runtime DI container
3. **Type Safety**: Full Go type checking for all dependency relationships
4. **Reduced Boilerplate**: ~80% reduction in manual dependency wiring code
5. **Better Testing**: Automated mock generation through Wire providers
6. **Maintainability**: Adding dependencies only requires provider updates

### Negative
1. **Build Complexity**: Requires `wire` tool in build process
2. **Learning Curve**: Team needs to understand Wire concepts and patterns
3. **Generated Code**: Build artifacts need to be committed or generated in CI
4. **Debugging**: Stack traces may include generated code paths

### Migration Impact

#### Performance Impact
- **Positive**: Eliminates reflection-based tool registration
- **Positive**: Zero runtime dependency resolution overhead
- **Neutral**: Compile-time generation adds ~2-3 seconds to build

#### Development Impact
- **Breaking Change**: All service constructors need Wire provider functions
- **Testing**: Existing tests need migration to Wire-based setup
- **Documentation**: Architecture docs need Wire patterns and examples

#### Deployment Impact
- **CI/CD**: Build pipeline needs `wire` tool installation
- **Artifacts**: Generated code in `wire_gen.go` files
- **Rollback**: Can revert to manual DI if needed (low risk)

## Implementation Plan

### Phase 1: Foundation (Week 2, Days 1-3)
- Install Wire and integrate into build system
- Create basic provider functions for core services
- Generate initial Wire code for server construction

### Phase 2: Service Migration (Week 2-3, Days 4-10)
- Migrate all 8 service interfaces to Wire providers
- Update constructors to use Wire-generated factories
- Remove manual dependency wiring code

### Phase 3: Tool Integration (Week 3-4, Days 11-15)
- Integrate Wire with unified tool registry
- Remove reflection-based tool registration
- Add Wire providers for all tool types

### Phase 4: Testing & Validation (Week 4-5, Days 16-20)
- Migrate all tests to Wire-based setup
- Validate performance improvements
- Complete integration testing

## Alternatives Considered

### Runtime DI Frameworks
- **Uber Fx**: Mature ecosystem but runtime overhead
- **Container**: Simple but limited functionality
- **Dig**: Flexible but complex setup

### Code Generation Tools
- **Wire (Chosen)**: Compile-time safety, zero runtime cost
- **Custom Generator**: High maintenance, reinventing wheel

### Manual Approaches
- **Service Locator**: Anti-pattern, runtime dependencies
- **Factory Pattern**: Still requires manual wiring

## Success Metrics

### Technical Metrics
- **Build Time**: <5 seconds additional for Wire generation
- **Runtime Performance**: 0% overhead (compile-time only)
- **Code Reduction**: 80% less manual dependency wiring
- **Type Safety**: 100% compile-time dependency validation

### Quality Metrics
- **Test Coverage**: Maintain >55% with Wire-based tests
- **Bug Reports**: <5 dependency-related issues post-migration
- **Developer Velocity**: Faster service addition after learning curve

## References

- [Google Wire Documentation](https://github.com/google/wire)
- [Wire User Guide](https://github.com/google/wire/blob/main/docs/guide.md)
- [Dependency Injection in Go](https://blog.golang.org/wire)
- [Container Kit Service Architecture](../THREE_LAYER_ARCHITECTURE.md)

## Revision History

| Date | Author | Changes |
|------|--------|---------|
| 2025-01-09 | Claude | Initial ADR creation |

---

**Note**: This ADR represents a significant architectural decision that will impact all Container Kit components. The Wire-based approach provides compile-time safety and zero runtime overhead, aligning with Go's performance and type safety principles.
