# ADR-003: Manual Dependency Injection Pattern (Wire Infrastructure Prepared)

Date: 2025-07-11
Status: Partially Implemented
Context: Container Kit originally had complex service interfaces with dependency injection frameworks and large manager objects containing 65+ methods across 4 different interfaces. This created unnecessary complexity, harder testing, and difficult-to-understand code paths. The system needed a simpler approach to dependency management that maintained clarity and testability.

Decision: Manual dependency injection is currently used with focused service interfaces. Wire infrastructure is prepared but not active due to import cycle issues. Large manager interfaces have been replaced with focused services, reducing complexity from 65+ methods to streamlined interfaces.

## Architecture Details

### Current Dependency Construction
- **Manual Injection**: Dependencies passed directly through constructors and options pattern
- **Wire Infrastructure Ready**: Complete Wire setup exists in `pkg/mcp/infrastructure/wire/`
- **Import Cycle Prevention**: Wire temporarily disabled to prevent circular dependencies
- **Functional Options**: Clean API using WithLogger, WithConfig patterns
- **Explicit Composition**: Service dependencies visible in constructor signatures

### Service Interface Design
```go
// Before: Large manager with 65+ methods
type ContainerManager interface {
    BuildImage(...) error
    PushImage(...) error
    PullImage(...) error
    TagImage(...) error
    ScanImage(...) error
    GetBuildLogs(...) error
    // ... 60+ more methods
}

// After: Focused services with clear responsibilities
type DockerBuilder interface {
    Build(ctx context.Context, args BuildArgs) error
    GetBuildLogs(ctx context.Context, buildID string) ([]string, error)
}

type ImageRegistry interface {
    Push(ctx context.Context, args PushArgs) error
    Pull(ctx context.Context, args PullArgs) error
    Tag(ctx context.Context, args TagArgs) error
}

type SecurityScanner interface {
    Scan(ctx context.Context, args ScanArgs) (*ScanResult, error)
    GetVulnerabilities(ctx context.Context, imageID string) ([]Vulnerability, error)
}
```

### Current Implementation Status

#### Wire Infrastructure (Ready but Disabled)
```go
// pkg/mcp/infrastructure/wire/wire.go - EXISTS AND READY
//go:generate wire

var ConfigurationSet = wire.NewSet(
    ProvideDefaultServerConfig,
)

var ApplicationSet = wire.NewSet(
    ProvideSessionManager,
    ProvideResourceStore,
    ProvideProgressFactory,
)

var InfrastructureSet = wire.NewSet(
    ProvideSamplingClient,
    ProvidePromptManager,
    provideDomainSampler,
    wire.Bind(new(domainsampling.UnifiedSampler), new(*sampling.DomainAdapter)),
)

// Wire provider sets fully defined but not used
func InitializeServer(logger *slog.Logger) (api.MCPServer, error) {
    wire.Build(ProviderSet)
    return nil, nil
}
```

#### Manual Dependency Injection (Currently Active)
```go
// pkg/mcp/application/server.go - ACTUAL IMPLEMENTATION
func NewMCPServer(ctx context.Context, logger *slog.Logger, opts ...Option) (api.MCPServer, error) {
    // Wire initialization attempted but falls back to manual
    server, err := initializeServerFromEnv(logger)
    if err != nil {
        logger.Warn("Falling back to manual dependency injection")
        fallbackServer := NewServer(append([]Option{WithLogger(logger), WithConfig(config)}, opts...)...)
        return fallbackServer, nil
    }
    return server, nil
}

// Wire functions return error to trigger fallback
func initializeServer(logger *slog.Logger) (api.MCPServer, error) {
    return nil, fmt.Errorf("Wire injection temporarily disabled - see Phase 1b of implementation plan")
}
```

## Previous Architecture Problems

### Before: Complex IoC System
- **Large Manager Interfaces**: 4 managers with 65+ methods total
- **Complex Dependencies**: Circular dependencies and unclear relationships
- **Hard to Test**: Mocking large interfaces was cumbersome
- **Framework Overhead**: Additional complexity from DI frameworks
- **Unclear Composition**: Service assembly hidden in framework

### Specific Issues Addressed
- **Interface Bloat**: Single interfaces handling multiple responsibilities
- **Dependency Confusion**: Unclear what services needed what dependencies
- **Testing Complexity**: Large mocks with many irrelevant methods
- **Framework Learning Curve**: Additional concepts for developers to learn
- **Debug Difficulty**: Hard to trace dependency construction and errors

## Current Architecture Benefits

### Simplified Service Design
- **8 Focused Services**: Clear single responsibilities
- **32 Total Methods**: Down from 65+ in manager interfaces
- **Clear Boundaries**: Well-defined service boundaries
- **Easy Testing**: Small, focused interfaces easy to mock

### Explicit Dependency Management
- **Visible Construction**: Dependency creation visible in code
- **Clear Relationships**: Direct dependency relationships
- **No Magic**: No hidden framework behavior
- **Simple Debugging**: Easy to trace service construction

### Service Categories
1. **Docker Services**: Building, registry, image management
2. **Kubernetes Services**: Deployment, manifest generation, cluster management
3. **Security Services**: Vulnerability scanning, policy enforcement
4. **Analysis Services**: Repository analysis, technology detection
5. **Workflow Services**: Step orchestration, progress tracking
6. **Session Services**: State management, workspace handling
7. **Error Services**: Rich error handling, retry logic
8. **Storage Services**: File operations, configuration management

## Implementation Examples

### Service Interface Definition
```go
// pkg/core/docker/interfaces.go
type ImageBuilder interface {
    Build(ctx context.Context, dockerfile string, tags []string) error
    GetBuildContext(ctx context.Context, path string) (*BuildContext, error)
}

type RegistryClient interface {
    Push(ctx context.Context, image string) error
    Pull(ctx context.Context, image string) error
    Authenticate(ctx context.Context, registry string) error
}

// pkg/core/kubernetes/interfaces.go
type ManifestGenerator interface {
    Generate(ctx context.Context, spec AppSpec) (*ManifestSet, error)
    Customize(ctx context.Context, manifests *ManifestSet, config CustomConfig) error
}

type DeploymentManager interface {
    Deploy(ctx context.Context, manifests *ManifestSet) error
    GetStatus(ctx context.Context, deployment string) (*DeploymentStatus, error)
}
```

### Wire-Generated Dependency Injection
```go
// pkg/mcp/infrastructure/wire/wire.go
//go:build wireinject

package wire

import (
    "github.com/google/wire"
    "github.com/your-org/container-kit/pkg/mcp/application"
    "github.com/your-org/container-kit/pkg/mcp/domain/workflow"
    "github.com/your-org/container-kit/pkg/mcp/infrastructure/steps"
)

// InitializeServer creates a fully-wired MCP server
func InitializeServer(config *workflow.ServerConfig, logger *slog.Logger) (*application.Server, error) {
    wire.Build(
        ConfigSet,
        CoreSet,
        ApplicationSet,
        InfrastructureSet,
    )
    return nil, nil
}

// The generated wire_gen.go file contains the actual implementation
// with all dependencies properly constructed and validated at compile time
```

### Testing Benefits
```go
// Simple, focused mocks
type mockImageBuilder struct {
    buildFunc func(ctx context.Context, dockerfile string, tags []string) error
}

func (m *mockImageBuilder) Build(ctx context.Context, dockerfile string, tags []string) error {
    if m.buildFunc != nil {
        return m.buildFunc(ctx, dockerfile, tags)
    }
    return nil
}

// Easy test setup
func TestBuildStep(t *testing.T) {
    builder := &mockImageBuilder{
        buildFunc: func(ctx context.Context, dockerfile string, tags []string) error {
            return nil // or specific test behavior
        },
    }
    
    step := steps.NewBuildStep(builder, nil, logger)
    err := step.Execute(ctx, args)
    assert.NoError(t, err)
}
```

## Consequences

### Current Benefits (Manual DI)
- **Clear Dependencies**: Explicit dependency relationships in constructors
- **Easy Testing**: Small, focused interfaces are easy to mock
- **No Magic**: Direct construction without code generation complexity
- **Reduced Coupling**: Services depend only on what they need
- **Type Safety**: Full Go type checking on all dependencies
- **Simple Debugging**: Direct call stack without generated code

### Future Benefits (When Wire Activated)
- **Compile-Time Safety**: All dependency issues caught at build time
- **Reduced Boilerplate**: Wire generates optimal dependency construction
- **Better Scaling**: Easier to add new dependencies without manual wiring
- **Performance**: No runtime overhead, optimal generated code
- **Consistency**: Standardized dependency injection patterns

### Current Trade-offs
- **Manual Wiring**: More boilerplate code for dependency construction
- **Human Error**: Possibility of missing dependencies in manual wiring
- **Refactoring Impact**: Changes require manual updates to all constructors
- **Import Cycles**: Current architecture prevents Wire activation

### Future Trade-offs (Wire)
- **Build Step**: Will require `wire` generation during development
- **Learning Curve**: Developers will need to understand Wire provider sets
- **Generated Code**: Must run `make wire-gen` when dependencies change
- **Compile-Time Only**: No runtime dependency resolution

### Maintenance Impact
- **Refactoring**: Easier to refactor with explicit dependencies
- **Service Addition**: Clear pattern for adding new services
- **Interface Evolution**: Small interfaces easier to evolve
- **Testing**: Focused tests with minimal setup

## Service Boundaries

### Core Services (8 total)
1. **DockerBuilder** (4 methods): Image building operations
2. **RegistryClient** (3 methods): Container registry operations  
3. **SecurityScanner** (4 methods): Vulnerability scanning
4. **ManifestGenerator** (3 methods): Kubernetes manifest creation
5. **DeploymentManager** (4 methods): Kubernetes deployment operations
6. **AnalysisEngine** (5 methods): Repository and technology analysis
7. **WorkflowOrchestrator** (4 methods): Step coordination and progress
8. **SessionManager** (5 methods): State and workspace management

### Interface Characteristics
- **Single Responsibility**: Each service has one clear purpose
- **Minimal Methods**: 3-5 methods per service interface
- **Clear Contracts**: Well-defined inputs and outputs
- **Error Handling**: Consistent error patterns using rich error system

## Implementation Status
- 🔄 **Wire-based DI**: Infrastructure complete but not active due to import cycles
- ✅ Focused service interfaces defined and simplified
- ✅ **Wire construction**: Provider sets defined in `pkg/mcp/infrastructure/wire/`
- ✅ Wire code generation works (`wire_gen.go` successfully generated)
- ✅ Testing patterns simplified with focused mocks
- ✅ Service boundaries well-defined and documented
- 🔄 Manual dependency injection currently used as fallback
- ⏳ **Wire activation**: Pending resolution of import cycle issues

### Why Wire is Temporarily Disabled

1. **Import Cycle Issue**: 
   - `application` package imports `infrastructure/wire` for DI
   - `infrastructure/wire` imports `application` to construct server
   - Creates circular dependency at compile time

2. **Current Workaround**:
   - Manual dependency injection using functional options
   - Wire infrastructure ready for future activation
   - No loss of functionality with manual approach

3. **Resolution Path**:
   - Move Wire initialization to a separate `cmd` package
   - Or restructure to break the import cycle
   - Activate Wire once architectural pattern is resolved

## Guidelines
1. **Keep Interfaces Small**: 3-5 methods maximum per interface
2. **Single Responsibility**: One clear purpose per service
3. **Use Wire Providers**: Define providers for all service constructors
4. **Clear Naming**: Service and method names should be self-explanatory
5. **Consistent Patterns**: Follow established Wire patterns for new services
6. **Run Generation**: Always run `make wire-gen` after dependency changes

## Wire Usage Patterns

### Creating New Services
1. Define the interface in the appropriate layer (api, domain, infrastructure)
2. Implement the service with a constructor function
3. Add the constructor to the appropriate Wire provider set
4. Run `make wire-gen` to update generated code
5. Wire will automatically handle dependency injection

### Testing with Wire
- Use Wire's testing support for creating test fixtures
- Mock interfaces can be provided through test-specific provider sets
- Wire ensures all test dependencies are properly satisfied

## Related ADRs
- ADR-001: Single Workflow Tool Architecture (workflow orchestration context)
- ADR-004: Unified Rich Error System (error handling across services)
- ADR-005: AI-Assisted Error Recovery (retry logic integration)
- ADR-006: Four-Layer MCP Architecture (service layer organization)
- ADR-007: CQRS, Saga, and Wire Patterns (advanced Wire usage)