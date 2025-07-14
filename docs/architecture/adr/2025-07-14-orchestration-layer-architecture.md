# ADR-010: Orchestration Layer Architecture

Date: 2025-07-14
Status: Accepted
Context: Container Kit requires sophisticated orchestration capabilities for Docker container management, Kubernetes deployment, and workflow step execution across the complete containerization pipeline
Decision: Establish a unified orchestration layer that separates container operations, Kubernetes management, and workflow step implementations while maintaining clean integration with the domain workflow system
Consequences: Clear separation of orchestration concerns, improved testability of containerization operations, and better support for multiple deployment targets

## Context

Container Kit's containerization workflow requires extensive orchestration capabilities across multiple technologies:

1. **Container Management**: Docker image building, pushing, pulling, and container lifecycle management
2. **Kubernetes Operations**: Cluster management, manifest generation, deployment coordination, and resource monitoring
3. **Workflow Step Execution**: Implementation of the 10-step containerization workflow with proper error handling and progress tracking
4. **Multi-Target Deployment**: Support for different container runtimes and orchestration platforms

The system needed a coherent orchestration architecture that:
- Abstracts container and Kubernetes operations
- Provides consistent interfaces for workflow step implementations
- Enables testing without external dependencies
- Supports multiple deployment targets and container runtimes

## Decision

We establish a unified orchestration layer in `pkg/mcp/infrastructure/orchestration/` with three main subsystems:

### 1. Container Management (`container/`)
Handles Docker and container runtime operations:
- **Docker Manager**: `docker_manager.go` - Docker API integration for image operations
- **Container Lifecycle**: Build, tag, push, pull, and container management
- **Registry Integration**: Container registry authentication and operations
- **Image Optimization**: Layer caching and build optimization

### 2. Kubernetes Management (`kubernetes/`)
Manages Kubernetes cluster operations:
- **Deployment Manager**: `deployment_manager.go` - Kubernetes deployment coordination
- **Manifest Generation**: Dynamic K8s manifest creation based on application analysis
- **Cluster Operations**: Cluster setup, validation, and health monitoring
- **Resource Management**: Pod, service, and ingress management

### 3. Workflow Step Implementation (`steps/`)
Implements the 10-step containerization workflow:
- **Analysis Step**: `analyze.go` - Repository analysis and technology detection
- **Dockerfile Generation**: `dockerfile.go` - Intelligent Dockerfile creation
- **Build Operations**: `build.go` - Container image building with optimization
- **Security Scanning**: Integration with vulnerability scanners
- **Deployment Steps**: `k8s.go` - Kubernetes deployment execution
- **Registry Provider**: `registry_provider.go` - Step registry and factory management

### Unified Provider Pattern
All orchestration services managed through Wire dependency injection:

```go
var OrchestrationProviders = wire.NewSet(
    // Container management
    container.NewDockerContainerManager,
    
    // Kubernetes deployment
    kubernetes.NewKubernetesDeploymentManager,
    
    // Workflow steps
    steps.NewRegistryStepProvider,
)
```

## Architecture Principles

### 1. Clear Separation of Concerns
- **Container Operations**: Docker/container runtime specific logic
- **Kubernetes Operations**: K8s cluster and resource management
- **Step Implementation**: Workflow-specific business logic
- **Provider Coordination**: Unified service creation and lifecycle

### 2. Interface-Driven Design
Each subsystem exposes clear interfaces for:
- Container operations (`ContainerManager`)
- Kubernetes operations (`DeploymentManager`)
- Step execution (`WorkflowStep`)
- Registry management (`StepProvider`)

### 3. Technology Abstraction
- **Container Runtime**: Abstracts Docker API with potential for other runtimes
- **Orchestration Platform**: K8s abstraction with potential for other platforms
- **Step Execution**: Generic step interface for different implementation strategies

### 4. Error Handling Integration
All orchestration operations integrate with the progressive error context system:
- Structured error reporting with operation context
- AI-assisted error recovery for common containerization failures
- Error pattern recognition across orchestration operations

## Implementation Details

### Container Manager Architecture
Unified interface for container operations:
```go
type ContainerManager interface {
    BuildImage(ctx context.Context, opts BuildOptions) error
    PushImage(ctx context.Context, image string) error
    TagImage(ctx context.Context, source, target string) error
    ScanImage(ctx context.Context, image string) (*ScanResult, error)
}
```

### Kubernetes Deployment Manager
Manages complete K8s deployment lifecycle:
```go
type DeploymentManager interface {
    GenerateManifests(ctx context.Context, app *ApplicationInfo) (*Manifests, error)
    DeployApplication(ctx context.Context, manifests *Manifests) error
    VerifyDeployment(ctx context.Context, app string) (*DeploymentStatus, error)
}
```

### Workflow Step Registry
Centralized step management with factory pattern:
```go
type StepProvider interface {
    GetStep(stepType string) (WorkflowStep, error)
    RegisterStep(stepType string, factory StepFactory)
    ListAvailableSteps() []string
}
```

### Step Implementation Pattern
Consistent step interface with enhanced capabilities:
```go
type WorkflowStep interface {
    Execute(ctx context.Context, input StepInput) (*StepOutput, error)
    Validate(ctx context.Context, input StepInput) error
    GetMetadata() StepMetadata
}
```

## Workflow Integration

### 10-Step Containerization Process
Each step implemented as orchestration component:

1. **Analyze** (`analyze.go`): Repository scanning and technology detection
2. **Dockerfile** (`dockerfile.go`): Intelligent Dockerfile generation
3. **Build** (`build.go`): Optimized container image building
4. **Scan** (integrated): Security vulnerability scanning
5. **Tag** (container manager): Image tagging with version information
6. **Push** (container manager): Registry upload operations
7. **Manifest** (`k8s.go`): Kubernetes manifest generation
8. **Cluster** (deployment manager): Cluster validation and setup
9. **Deploy** (deployment manager): Application deployment
10. **Verify** (deployment manager): Health check and validation

### Enhanced Step Capabilities
Steps include optimization and AI-enhanced features:
- **Enhanced Build**: `enhanced_build.go` - AI-optimized build strategies
- **Optimized Steps**: `optimized/` - Performance-optimized implementations
- **Manifest Fix**: `manifest_fix.go` - Automatic manifest correction
- **Deployment Verification**: `deployment_verification.go` - Comprehensive health checks

## Error Handling and Recovery

### Container Operation Errors
- **Build Failures**: Dockerfile syntax, dependency issues, layer problems
- **Registry Errors**: Authentication, network, quota issues
- **Runtime Errors**: Container startup, resource constraints

### Kubernetes Operation Errors  
- **Manifest Errors**: YAML syntax, resource validation, policy violations
- **Deployment Errors**: Resource constraints, network policies, RBAC issues
- **Cluster Errors**: Connectivity, authentication, version compatibility

### AI-Assisted Recovery
Integration with AI/ML system for intelligent error recovery:
```go
func (s *BuildStep) ExecuteWithRecovery(ctx context.Context, input StepInput) (*StepOutput, error) {
    result, err := s.Execute(ctx, input)
    if err != nil {
        recoveryStrategy := s.aiRetry.AnalyzeError(err)
        return s.retryWithStrategy(ctx, input, recoveryStrategy)
    }
    return result, nil
}
```

## Consequences

### Positive
- **Clear Architecture**: Well-defined separation between container, K8s, and step concerns
- **Technology Flexibility**: Easy to swap container runtimes or orchestration platforms
- **Comprehensive Testing**: Isolated components enable thorough testing strategies
- **Error Recovery**: Intelligent error handling with AI-assisted recovery
- **Performance Optimization**: Specialized implementations for performance-critical operations

### Negative
- **Complexity**: Additional abstraction layers for orchestration operations
- **Dependencies**: External dependencies on Docker API and Kubernetes client libraries
- **Resource Usage**: Container and K8s operations are resource-intensive
- **Configuration**: Complex configuration for different deployment environments

### Performance Characteristics
- **Container Operations**: <30s P95 for image builds, <10s for push/pull
- **K8s Operations**: <20s P95 for deployments, <5s for manifest generation
- **Step Execution**: <300μs P95 for step coordination overhead
- **Error Recovery**: <5s P95 for AI-assisted error analysis

## Testing Strategy

### Unit Testing
- **Mock Implementations**: Docker and K8s client mocks for isolated testing
- **Step Testing**: Individual step validation with fixture data
- **Error Simulation**: Controlled error scenarios for recovery testing

### Integration Testing
- **Container Testing**: Real Docker operations with test images
- **K8s Testing**: Local cluster testing with kind/minikube
- **End-to-End**: Complete workflow testing with real orchestration

### Performance Testing
- **Build Performance**: Image build time optimization testing
- **Deployment Performance**: K8s deployment speed benchmarks
- **Concurrency Testing**: Multiple workflow execution scenarios

## Compliance

This architecture aligns with Container Kit's four-layer architecture:
- **Infrastructure Layer**: Orchestration implementations in `pkg/mcp/infrastructure/orchestration/`
- **Domain Layer**: Workflow interfaces in `pkg/mcp/domain/workflow/`
- **Application Layer**: Workflow coordination in `pkg/mcp/application/workflow/`
- **API Layer**: MCP tool interfaces for orchestration operations

## Configuration and Deployment

### Environment Configuration
- **Container Runtime**: Docker socket configuration, registry credentials
- **Kubernetes**: Kubeconfig, cluster endpoints, authentication
- **Step Configuration**: Step-specific settings and optimization parameters

### Security Considerations
- **Registry Security**: Secure credential management for container registries
- **K8s Security**: RBAC configuration, network policies, security contexts
- **Image Security**: Vulnerability scanning integration and policy enforcement

### Multi-Environment Support
- **Development**: Local Docker and kind cluster
- **Staging**: Managed container registry and K8s cluster
- **Production**: Enterprise registry and managed K8s service

## References
- Container manager implementation: `pkg/mcp/infrastructure/orchestration/container/docker_manager.go`
- Kubernetes deployment: `pkg/mcp/infrastructure/orchestration/kubernetes/deployment_manager.go`
- Step implementations: `pkg/mcp/infrastructure/orchestration/steps/`
- Integration tests: `pkg/mcp/infrastructure/orchestration/steps/integration_test.go`
- Performance benchmarks maintained at <300μs P95 for orchestration coordination