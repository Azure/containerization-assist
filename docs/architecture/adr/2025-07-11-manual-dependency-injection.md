# ADR-011: Manual Dependency Injection Pattern

Date: 2025-07-11
Status: Accepted
Context: Container Kit originally had complex service interfaces with dependency injection frameworks and large manager objects containing 65+ methods across 4 different interfaces. This created unnecessary complexity, harder testing, and difficult-to-understand code paths. The system needed a simpler approach to dependency management that maintained clarity and testability.

Decision: Adopt manual dependency injection with focused service interfaces, direct instantiation patterns, and clear dependency graphs. Replace large manager interfaces with 8 focused services containing 32 methods total, eliminating dependency injection frameworks in favor of explicit dependency construction.

## Architecture Details

### Manual Dependency Construction
- **Direct Instantiation**: Services created explicitly in main/setup functions
- **Clear Dependencies**: Dependencies passed directly to constructors
- **No IoC Containers**: No dependency injection frameworks or containers
- **Explicit Composition**: Service composition visible in code

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

### Dependency Structure
```go
// Explicit service construction in main
func NewMCPServer(workspaceDir string, logger *slog.Logger) *Server {
    // Direct dependency construction
    dockerClient := docker.NewClient()
    kubeClient := kubernetes.NewClient()
    scanner := security.NewScanner(dockerClient)
    
    // Explicit dependency injection
    analyzeStep := steps.NewAnalyzeStep(logger)
    buildStep := steps.NewBuildStep(dockerClient, logger)
    deployStep := steps.NewDeployStep(kubeClient, logger)
    
    // Clear service composition
    workflow := workflow.NewContainerizeWorkflow(
        analyzeStep,
        buildStep,
        deployStep,
        logger,
    )
    
    return &Server{
        workflow: workflow,
        logger:   logger,
    }
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

### Direct Dependency Injection
```go
// pkg/mcp/application/server.go
func (s *Server) initializeServices() error {
    // Direct service construction with clear dependencies
    s.dockerClient = docker.NewClient(s.config.Docker)
    s.kubeClient = kubernetes.NewClient(s.config.Kubernetes)
    
    // Explicit dependency passing
    s.imageBuilder = docker.NewImageBuilder(s.dockerClient, s.logger)
    s.registryClient = docker.NewRegistryClient(s.dockerClient, s.logger)
    s.scanner = security.NewScanner(s.dockerClient, s.logger)
    
    s.manifestGenerator = kubernetes.NewManifestGenerator(s.logger)
    s.deploymentManager = kubernetes.NewDeploymentManager(s.kubeClient, s.logger)
    
    // Workflow step composition
    s.analyzeStep = steps.NewAnalyzeStep(s.logger)
    s.buildStep = steps.NewBuildStep(s.imageBuilder, s.registryClient, s.logger)
    s.deployStep = steps.NewDeployStep(s.manifestGenerator, s.deploymentManager, s.logger)
    
    return nil
}
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

### Benefits
- **Simple Architecture**: No framework complexity or magic behavior
- **Clear Dependencies**: Explicit dependency relationships visible in code
- **Easy Testing**: Small, focused interfaces are easy to mock
- **Better Debugging**: Clear service construction and dependency paths
- **Reduced Coupling**: Services depend only on what they need
- **Performance**: No framework overhead or reflection
- **Learning Curve**: Easier for new developers to understand

### Trade-offs
- **Manual Work**: Need to manually wire dependencies
- **Duplication**: Some dependency setup code may be repeated
- **No Auto-wiring**: No automatic dependency resolution
- **Verbosity**: More explicit code for service construction

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
- ✅ Manual dependency injection pattern implemented
- ✅ 8 focused service interfaces defined (32 methods total)
- ✅ Direct service construction in server initialization
- ✅ Clear dependency graphs established
- ✅ Testing patterns simplified with focused mocks
- ✅ Service boundaries well-defined and documented
- ✅ Integration with workflow architecture complete

## Guidelines
1. **Keep Interfaces Small**: 3-5 methods maximum per interface
2. **Single Responsibility**: One clear purpose per service
3. **Explicit Dependencies**: Pass dependencies directly to constructors
4. **Clear Naming**: Service and method names should be self-explanatory
5. **Consistent Patterns**: Follow established patterns for new services

## Related ADRs
- ADR-008: Single Workflow Tool Architecture (workflow orchestration context)
- ADR-009: Unified Rich Error System (error handling across services)
- ADR-010: AI-Assisted Error Recovery (retry logic integration)