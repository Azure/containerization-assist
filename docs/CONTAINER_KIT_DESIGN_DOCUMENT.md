# Container Kit Design Document

**Version**: 1.0
**Date**: 2025-07-10
**Status**: Current

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [System Overview](#system-overview)
3. [Architecture](#architecture)
4. [Core Components](#core-components)
5. [Design Patterns](#design-patterns)
6. [Data Flow](#data-flow)
7. [Security Architecture](#security-architecture)
8. [Quality Assurance](#quality-assurance)
9. [Deployment & Operations](#deployment--operations)
10. [Development Guidelines](#development-guidelines)
11. [Appendices](#appendices)

## Executive Summary

Container Kit is a production-ready, enterprise-grade AI-powered containerization platform that automates the complete Docker and Kubernetes workflow through an intelligent Model Context Protocol (MCP) server architecture. The system transforms traditional container operations from manual processes into AI-guided, automated workflows with comprehensive error recovery, security scanning, and deployment orchestration.

### Key Capabilities
- **AI-Powered Analysis**: Intelligent repository analysis and Dockerfile generation
- **Automated Container Operations**: Build, scan, deploy with automated error fixing
- **Multi-Mode Architecture**: Chat, workflow, and dual-mode operations
- **Enterprise Security**: Comprehensive vulnerability scanning with Trivy/Grype
- **Session Management**: Persistent state with BoltDB storage
- **Production Ready**: 159,570 lines of code across 606 files with comprehensive testing

### Technology Stack
- **Core**: Go 1.24.1 with clean three-layer architecture
- **Protocol**: Model Context Protocol (MCP) via gomcp library
- **Storage**: BoltDB for session persistence
- **Container Runtime**: Docker with full lifecycle management
- **Orchestration**: Kubernetes client with manifest generation
- **Monitoring**: Prometheus metrics, OpenTelemetry tracing
- **Security**: Trivy/Grype vulnerability scanning

## System Overview

### Vision Statement
Container Kit transforms containerization from a complex, error-prone manual process into an intelligent, automated workflow that guides users through analysis, building, scanning, and deployment with AI-powered assistance and comprehensive error recovery.

### Core Principles
1. **AI-First Design**: Every operation enhanced with intelligent automation
2. **Clean Architecture**: Strict three-layer separation with dependency injection
3. **Production Ready**: Enterprise-grade error handling, monitoring, and security
4. **Developer Experience**: Intuitive interfaces with comprehensive tooling
5. **Extensibility**: Plugin architecture for custom tools and workflows

### System Boundaries
- **Input**: Source code repositories, configuration parameters, user interactions
- **Processing**: Repository analysis, container operations, security scanning, deployment
- **Output**: Built containers, security reports, Kubernetes manifests, deployment status
- **External Systems**: Docker Engine, Kubernetes clusters, container registries, security scanners

## Architecture

### Three-Layer Architecture

Container Kit implements a clean three-layer architecture pattern ensuring proper separation of concerns, maintainability, and testability.

```
pkg/mcp/
├── domain/              # Domain Layer (101 files)
│   ├── config/         # Configuration entities and validation
│   ├── containerization/ # Container operations domain logic
│   ├── errors/         # Rich error handling system
│   ├── security/       # Security policies and validation
│   ├── session/        # Session entities and rules
│   ├── types/          # Core domain types
│   └── internal/       # Shared utilities
├── application/         # Application Layer (153 files)
│   ├── api/            # Canonical interface definitions
│   ├── commands/       # Command implementations
│   ├── core/           # Server lifecycle & registry
│   ├── orchestration/  # Tool coordination
│   ├── services/       # Service interfaces
│   ├── state/          # Application state management
│   └── workflows/      # Workflow management
└── infra/              # Infrastructure Layer (38 files)
    ├── persistence/    # BoltDB storage layer
    ├── transport/      # MCP protocol transports
    ├── telemetry/      # Monitoring and observability
    └── templates/      # YAML templates
```

#### Dependency Rules
- **Domain Layer**: No external dependencies (pure business logic)
- **Application Layer**: Depends on Domain only
- **Infrastructure Layer**: Depends on Domain and Application

### Service-Oriented Architecture

#### Manual Dependency Injection
The system implements manual dependency injection as defined in ADR-006, replacing 4 large Manager interfaces (65+ methods) with 8 focused service interfaces (32 methods total):

```go
type ServiceContainer interface {
    SessionStore() SessionStore        // Session CRUD operations (4 methods)
    SessionState() SessionState        // State & checkpoint management (4 methods)
    BuildExecutor() BuildExecutor      // Container build operations (5 methods)
    ToolRegistry() ToolRegistry        // Tool registration & discovery (5 methods)
    WorkflowExecutor() WorkflowExecutor // Multi-step workflows (4 methods)
    Scanner() Scanner                  // Security scanning (3 methods)
    ConfigValidator() ConfigValidator  // Configuration validation (4 methods)
    ErrorReporter() ErrorReporter      // Unified error handling (3 methods)
}
```

#### Unified Interface System
**Single Source of Truth**: `pkg/mcp/application/api/interfaces.go` (831 lines)
- Contains ALL canonical interface definitions
- Comprehensive Tool, Registry, Session, Workflow, and Server interfaces
- Rich metadata, retry policies, and configuration options

### Multi-Mode Server Architecture

The unified MCP server supports three operation modes:

1. **Chat Mode**: Direct conversational tool interaction
2. **Workflow Mode**: Multi-step atomic operations
3. **Dual Mode**: Both chat and workflow capabilities

#### Key Server Features
- Service container integration with dependency injection
- Session management with BoltDB persistence
- Tool registry with metrics and lifecycle management
- AI-powered automation and error recovery
- Graceful shutdown and signal handling

## Core Components

### 1. Containerization Domain

#### Analyze Tools
- **Repository Analysis**: Language detection, dependency analysis, framework identification
- **Dockerfile Generation**: AI-powered Dockerfile creation with best practices
- **Template Management**: Language-specific templates with go:embed integration
- **Validation**: Dockerfile syntax and security validation

#### Build Tools
- **Docker Operations**: Build, push, pull, tag with comprehensive error handling
- **AI-Powered Fixing**: Automatic build error detection and resolution
- **Registry Integration**: Multi-registry support with health monitoring
- **Build Optimization**: Layer caching and multi-stage build optimization

#### Deploy Tools
- **Kubernetes Manifest Generation**: Automated YAML generation with customization
- **Health Checks**: Application readiness and liveness probe configuration
- **Secret Management**: Secure secret generation and injection
- **Deployment Orchestration**: Rolling updates with rollback capabilities

#### Scan Tools
- **Security Scanning**: Trivy/Grype integration for vulnerability detection
- **SBOM Generation**: Software Bill of Materials with CycloneDX format
- **Policy Engine**: Configurable security policies and compliance checking
- **Report Generation**: Comprehensive security reports with remediation guidance

### 2. Session Management

#### Persistence Layer
- **BoltDB Storage**: Efficient key-value storage for session state
- **Session Lifecycle**: Creation, management, and cleanup
- **Workspace Isolation**: Dedicated workspace directories per session
- **Metadata Management**: Label-based organization and tracking

#### State Management
- **Context Preservation**: Maintains conversation and workflow context
- **Checkpoint System**: Rollback capabilities for failed operations
- **State Synchronization**: Consistent state across distributed operations
- **Event Sourcing**: Audit trail for all state changes

### 3. AI Integration

#### Analysis Service
- **Code Intelligence**: Repository structure and pattern analysis
- **Recommendation Engine**: Best practice suggestions and optimizations
- **Error Classification**: Intelligent error categorization and resolution
- **Context Enrichment**: Enhanced AI context for better decision making

#### Conversation System
- **Auto-Fix Helpers**: Intelligent error resolution with strategy chaining
- **Prompt Engineering**: Optimized prompts for different scenarios
- **Workflow Detection**: Automatic detection of multi-step workflows
- **Progress Tracking**: Real-time progress updates with AI insights

## Design Patterns

### 1. Rich Error System (ADR-004)

Unified error handling with structured context and metadata:

```go
return errors.NewError().
    Code(errors.CodeValidationFailed).
    Type(errors.ErrTypeValidation).
    Severity(errors.SeverityMedium).
    Message("validation failed").
    Context("field", fieldName).
    Suggestion("Check field format").
    WithLocation().
    Build()
```

**Features**:
- Structured error codes for unique identification
- Error categorization (Validation, Network, Internal, etc.)
- Severity levels (Low, Medium, High, Critical)
- Rich context with key-value pairs
- Automatic source location capture
- Error chaining with proper cause tracking
- Human-readable resolution guidance

### 2. Tag-Based Validation DSL (ADR-005)

Declarative validation using struct tags:

```go
type BuildConfig struct {
    Repository string `validate:"required,git_url" security:"sanitize"`
    Tag        string `validate:"required,docker_tag" security:"validate"`
    Push       bool   `validate:"omitempty" security:"safe"`
}
```

**Benefits**:
- Declarative validation rules
- Automatic code generation
- Consistent validation patterns
- Security integration
- Reduced boilerplate

### 3. Template Management with Go Embed

YAML templates embedded at compile time:

```go
//go:embed templates/*.yaml
var templateFS embed.FS

func LoadTemplate(name string) (string, error) {
    return templateFS.ReadFile(fmt.Sprintf("templates/%s.yaml", name))
}
```

**Advantages**:
- No external file dependencies
- Version control integration
- Atomic deployments
- Better security

### 4. Interface Segregation

Small, focused interfaces following Single Responsibility Principle:

```go
type SessionStore interface {
    Create(ctx context.Context, session *Session) error
    Get(ctx context.Context, id string) (*Session, error)
    Update(ctx context.Context, session *Session) error
    Delete(ctx context.Context, id string) error
}
```

## Data Flow

### 1. Tool Execution Flow

```
User Request → MCP Server → Tool Registry → Command Router →
Tool Implementation → Service Container → Domain Logic →
Infrastructure Services → External Systems → Response
```

### 2. Session Lifecycle

```
Session Creation → Workspace Setup → Tool Registration →
Execution Context → State Persistence → Cleanup
```

### 3. Error Handling Flow

```
Error Detection → Classification → Context Enrichment →
Recovery Strategy → Auto-Fix Attempt → User Notification →
Audit Logging
```

### 4. AI Integration Flow

```
User Input → Context Analysis → AI Service → Response Generation →
Context Update → Action Execution → Result Validation
```

## Security Architecture

### 1. Input Validation
- **Struct Tag Validation**: Declarative validation rules
- **Security Sanitization**: Automatic input cleaning
- **Parameter Validation**: Type and format checking
- **Path Traversal Protection**: Safe file system operations

### 2. Vulnerability Scanning
- **Multi-Scanner Support**: Trivy and Grype integration
- **Policy Engine**: Configurable security policies
- **SBOM Generation**: Complete dependency tracking
- **Continuous Monitoring**: Automated security updates

### 3. Secret Management
- **Secure Generation**: Cryptographically secure secrets
- **Environment Integration**: Kubernetes secret injection
- **Rotation Support**: Automated secret rotation
- **Access Control**: Role-based secret access

### 4. Container Security
- **Minimal Base Images**: Distroless and minimal containers
- **Non-Root Execution**: Security context enforcement
- **Resource Limits**: CPU and memory constraints
- **Network Policies**: Traffic isolation and control

## Quality Assurance

### 1. Testing Strategy
- **Unit Tests**: Comprehensive test coverage with baselines
- **Integration Tests**: End-to-end workflow validation
- **Performance Tests**: Benchmark testing with regression detection
- **Security Tests**: Vulnerability and penetration testing

### 2. Quality Gates
- **Error Budget**: Maximum 100 lint issues
- **Performance Target**: <300μs P95 per request
- **Coverage Baseline**: Tracked coverage with enforcement
- **Pre-commit Hooks**: Automated quality checks

### 3. Code Quality Tools
- **Linting**: golangci-lint with custom rules
- **Formatting**: gofmt and goimports
- **Complexity Analysis**: Cyclomatic complexity limits
- **Dependency Analysis**: Import cycle detection

### 4. Monitoring & Observability
- **Metrics**: Prometheus metrics for all operations
- **Tracing**: OpenTelemetry distributed tracing
- **Logging**: Structured logging with slog
- **Health Checks**: Comprehensive health endpoints

## Deployment & Operations

### 1. Container Deployment
- **Multi-Architecture**: Support for AMD64 and ARM64
- **Minimal Images**: Distroless base images for security
- **Health Checks**: Readiness and liveness probes
- **Resource Management**: CPU and memory limits

### 2. Kubernetes Integration
- **Manifest Generation**: Automated YAML creation
- **Service Discovery**: DNS-based service resolution
- **Load Balancing**: Kubernetes native load balancing
- **Rolling Updates**: Zero-downtime deployments

### 3. Configuration Management
- **Environment Variables**: 12-factor app configuration
- **ConfigMaps**: Kubernetes-native configuration
- **Secrets**: Secure credential management
- **Hot Reloading**: Runtime configuration updates

### 4. Monitoring Setup
- **Prometheus**: Metrics collection and alerting
- **Grafana**: Visualization and dashboards
- **Jaeger**: Distributed tracing analysis
- **ELK Stack**: Log aggregation and analysis

## Development Guidelines

### 1. Adding New Tools
1. **Interface Definition**: Add to `pkg/mcp/application/api/interfaces.go`
2. **Implementation**: Create in appropriate layer following three-layer architecture
3. **Registration**: Auto-registration via unified interface system
4. **Testing**: Unit and integration tests with mocks
5. **Documentation**: Update tool inventory and guides

### 2. Error Handling
- Use unified RichError system from `pkg/mcp/domain/errors/rich.go`
- Include structured context and actionable suggestions
- Implement proper error classification and severity
- Capture source location for debugging

### 3. Validation
- Use struct tag-based validation DSL
- Generate validation code with `go generate`
- Implement security validation alongside business validation
- Follow declarative validation patterns

### 4. Service Integration
- Use service container pattern for dependency injection
- Define focused interfaces following Single Responsibility Principle
- Implement proper lifecycle management
- Include comprehensive testing with mocks


## Appendices

### A. Architecture Decision Records (ADRs)

1. **ADR-001**: Three-Context Architecture - Simplified 30+ packages to 3 bounded contexts
2. **ADR-002**: Go Embed for YAML Templates - Improved template management
3. **ADR-003**: Zerolog to Slog Migration - Unified logging interface
4. **ADR-004**: Unified Rich Error System - Consolidated error handling
5. **ADR-005**: Tag-Based Validation DSL - Declarative validation
6. **ADR-006**: Manual Dependency Injection - Focused service interfaces

### B. Key Metrics

- **Codebase Size**: 606 Go files, 159,570 lines of code, 126MB total
- **Architecture**: 3 layers (domain: 101 files, application: 153 files, infra: 38 files)
- **Service Interfaces**: 8 focused services with 32 total methods
- **Error Reduction**: 51% method reduction from manager refactoring
- **Quality Gates**: <100 lint issues, <300μs P95 performance

### C. Technology Dependencies

- **Go**: 1.24.1 (core language)
- **gomcp**: Model Context Protocol implementation
- **BoltDB**: Embedded key-value storage
- **Docker Client**: Container operations
- **Kubernetes Client**: Orchestration integration
- **Trivy/Grype**: Security scanning
- **Prometheus**: Metrics collection
- **OpenTelemetry**: Distributed tracing

### D. Development Commands

```bash
# Build and test
make mcp                # Build MCP server
make test               # Run tests
make test-all          # All packages
make bench             # Performance benchmarks

# Code quality
make fmt               # Format code
make lint              # Lint with error budget
make lint-strict       # Strict linting
make pre-commit        # Pre-commit hooks

# Coverage and performance
make coverage-html     # Generate coverage report
make bench-baseline    # Set performance baseline
```

---

**Document Maintenance**: This design document should be updated when significant architectural changes are made. See ADRs for detailed decision rationale and implementation guidance.
