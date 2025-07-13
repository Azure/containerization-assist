# Container Kit Design Document

**Version**: 3.0
**Date**: 2025-07-12
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

Container Kit is an advanced, AI-powered containerization platform that automates the complete Docker and Kubernetes workflow through a unified Model Context Protocol (MCP) server. The system follows a clean 4-layer architecture with workflow-driven design that balances simplicity with maintainability.

### Key Capabilities
- **Unified Workflow**: Single `containerize_and_deploy` tool handles complete process
- **Progress Tracking**: Built-in progress indicators for all 10 workflow steps
- **AI-Powered Process**: Intelligent automation with error recovery
- **Enterprise Security**: Comprehensive vulnerability scanning with Trivy/Grype
- **Session Management**: Persistent state with BoltDB storage
- **Simplified Architecture**: Essential functionality without over-engineering

### Technology Stack
- **Core**: Go 1.24.4 with 4-layer clean architecture
- **Protocol**: Model Context Protocol (MCP) via mcp-go v0.33.0 library
- **AI Integration**: Azure OpenAI SDK for guided workflows and error recovery
- **Storage**: BoltDB for session persistence
- **Container Runtime**: Docker with full lifecycle management
- **Orchestration**: Kubernetes client with manifest generation
- **Security**: Trivy/Grype vulnerability scanning

## System Overview

### Vision Statement
Container Kit provides a simple, unified workflow that guides users through the complete containerization process from analysis to deployment with AI-powered assistance and built-in progress tracking.

### Core Principles
1. **Workflow-First Design**: Single unified process instead of atomic tools
2. **Simplicity**: Eliminate over-engineering while maintaining functionality
3. **Progress Transparency**: Visual feedback for every step
4. **AI Integration**: Intelligent automation with error recovery
5. **Developer Experience**: Intuitive interface with clear documentation

### System Boundaries
- **Input**: Source code repositories, configuration parameters, user interactions
- **Processing**: Complete containerization workflow with 10 steps
- **Output**: Built containers, security reports, Kubernetes manifests, deployment status
- **External Systems**: Docker Engine, Kubernetes clusters, container registries, security scanners

## Architecture

### Four-Layer Clean Architecture

Container Kit follows a clean 4-layer Domain-Driven Design architecture:

```
pkg/mcp/
├── api/                    # Interface definitions and contracts
│   └── interfaces.go       # Essential MCP tool interfaces
├── application/            # Application services and orchestration
│   ├── server.go          # MCP server implementation
│   ├── bootstrap.go       # Dependency injection setup
│   ├── commands/          # CQRS command handlers
│   ├── queries/           # CQRS query handlers
│   ├── config/            # Application configuration
│   └── session/           # Session management
├── domain/                # Business logic and workflows
│   ├── workflow/          # Core containerization workflow
│   ├── events/            # Domain events and handlers
│   ├── progress/          # Progress tracking (business concept)
│   ├── saga/              # Saga pattern coordination
│   └── sampling/          # Domain sampling contracts
└── infrastructure/        # Technical implementations
    ├── steps/             # Workflow step implementations
    ├── ml/                # Machine learning integrations
    ├── sampling/          # LLM integration
    ├── progress/          # Progress tracking implementations
    ├── prompts/           # MCP prompt management
    ├── resources/         # MCP resource providers
    ├── tracing/           # Observability integration
    ├── utilities/         # Infrastructure utilities
    └── validation/        # Validation implementations
```

### Key Architecture Benefits
- **Clean Dependencies**: Infrastructure → Application → Domain → API
- **Single Workflow**: `containerize_and_deploy` handles complete process
- **CQRS Implementation**: Separate command and query handling for scalability
- **Event-Driven Design**: Domain events for workflow coordination and observability
- **Saga Orchestration**: Distributed transaction coordination for complex workflows
- **ML Integration**: Machine learning for build optimization and pattern recognition
- **Domain-Driven**: Core business logic isolated in domain layer
- **Separation of Concerns**: Each layer has clear responsibilities
- **AI-Enhanced**: Built-in AI error recovery and analysis capabilities

### Dependency Rules
- **API Layer**: Essential interfaces only, avoid over-abstraction
- **Application Layer**: Coordinate domain logic, handle MCP protocol, CQRS orchestration
- **Domain Layer**: Pure business logic, domain events, saga coordination, no infrastructure dependencies
- **Infrastructure Layer**: Technical implementations, external integrations, event handlers

## Core Components

### 1. Workflow Server (`pkg/mcp/server/`)

**Single Workflow Tool**: `containerize_and_deploy`

The unified workflow tool handles the complete containerization process:

```go
// Complete 10-step workflow with AI orchestration
steps := []WorkflowStep{
    {Name: "analyze_repository", Message: "Analyzing repository structure and detecting language/framework"},
    {Name: "generate_dockerfile", Message: "Generating optimized Dockerfile for detected language/framework"},
    {Name: "build_image", Message: "Building Docker image with AI-powered error fixing"},
    {Name: "setup_kind_cluster", Message: "Setting up local Kubernetes cluster with registry"},
    {Name: "load_image", Message: "Loading Docker image into Kubernetes cluster"},
    {Name: "generate_k8s_manifests", Message: "Generating Kubernetes deployment manifests"},
    {Name: "deploy_to_k8s", Message: "Deploying application to Kubernetes cluster"},
    {Name: "health_probe", Message: "Performing application health checks and endpoint discovery"},
    {Name: "vulnerability_scan", Message: "Scanning Docker image for security vulnerabilities (optional)"},
    {Name: "finalize_result", Message: "Finalizing workflow results and cleanup"},
}
```

**Features**:
- Progress tracking with visual indicators
- Error recovery with actionable messages
- AI-powered automation throughout
- Session management with BoltDB persistence
- Event-driven workflow coordination
- CQRS command and query separation
- Saga orchestration for distributed transactions
- ML-powered build optimization
- Comprehensive observability with tracing

### 2. Step Implementations (`pkg/mcp/infrastructure/steps/`)

#### Analyze Step (`pkg/mcp/infrastructure/steps/analyze.go`)
- **Repository Analysis**: Language detection, dependency analysis, framework identification
- **AI Enhancement**: Optional AI-powered analysis for better recommendations (`analyze_enhance.go`)
- **Technology Detection**: Automated technology stack identification
- **Port Detection**: Automatic application port discovery

#### Build Steps
- **Docker Build** (`pkg/mcp/infrastructure/steps/build.go`): Standard Docker operations with error handling
- **Optimized Build** (`pkg/mcp/infrastructure/steps/optimized_build.go`): ML-enhanced build optimization
- **AI-Powered Fixing**: Automatic Dockerfile error detection and resolution
- **Registry Integration**: Multi-registry support with health monitoring

#### Deployment Steps
- **Dockerfile Generation** (`pkg/mcp/infrastructure/steps/dockerfile.go`): Automated Dockerfile creation
- **Kubernetes Manifests** (`pkg/mcp/infrastructure/steps/k8s.go`): YAML generation with customization
- **Manifest Fixing** (`pkg/mcp/infrastructure/steps/manifest_fix.go`): AI-powered manifest error resolution
- **Deployment Verification** (`pkg/mcp/infrastructure/steps/deployment_verification.go`): Health checks and validation

### 3. Error Handling (`pkg/common/errors/`)

**Rich Error System**:
- **Structured Error Context**: Comprehensive error information with Rich type
- **Actionable Messages**: Clear guidance for resolution with user-facing flags
- **Core Infrastructure**: Used across the entire codebase
- **Error Classification**: Severity levels (Unknown, Low, Medium, High, Critical)
- **Retry Support**: Built-in retryable flag for automated error recovery
- **AI Integration**: Error context for AI-powered retry logic
- **Code Generation**: Error codes generated from YAML configuration

### 4. Server Core (`pkg/mcp/application/`)

**MCP Server Implementation**:
- **Protocol Handling**: mcp-go v0.33.0 integration
- **Session Management**: BoltDB-based persistence with TTL
- **Tool Registration**: Single workflow tool with progress tracking
- **Transport Layer**: stdio transport with proper shutdown handling
- **AI Integration**: Built-in chat mode support for Copilot integration
- **CQRS Architecture**: Separate command and query handlers
- **Configuration Management**: Environment-based configuration with validation

### 5. Machine Learning Integration (`pkg/mcp/infrastructure/ml/`)

**ML-Powered Features**:
- **Build Optimization** (`build_optimizer.go`): ML-enhanced build performance
- **Pattern Recognition** (`pattern_recognizer.go`): Build pattern analysis and optimization
- **Error History** (`error_history.go`): Learning from previous build failures
- **Build History** (`build_history.go`): Historical build data analysis
- **Workflow Integration** (`workflow_integration.go`): ML integration with workflow steps
- **Resource Prediction** (`resource_predictor.go`): Predictive resource allocation

### 6. Event-Driven Architecture (`pkg/mcp/domain/events/`)

**Domain Events**:
- **Event System** (`events.go`): Core event definitions and types
- **Event Handlers** (`handlers.go`): Event processing and routing
- **Event Publisher** (`publisher.go`): Event publication and distribution
- **Workflow Coordination**: Events for step completion and state changes
- **Observability**: Event-driven monitoring and tracing

## Design Patterns

### 1. Unified Workflow Pattern

A single workflow tool with AI orchestrator handles the complete containerization process:

```go
// RegisterWorkflowTools registers the comprehensive containerization workflow
func RegisterWorkflowTools(mcpServer *server.MCPServer, logger *slog.Logger) error {
	tool := mcp.Tool{
		Name:        "containerize_and_deploy",
		Description: "Complete containerization workflow from analysis to deployment",
	}

	mcpServer.RegisterTool(tool, func(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
		// Use new orchestrator-based workflow
		orchestrator := NewOrchestrator(logger)
		result, err := orchestrator.Execute(ctx, &req, &args)
		return result, err
	})
}
```

### 2. Progress Tracking Pattern

Every step provides detailed progress feedback with AI integration:

```go
type WorkflowStep struct {
    Name     string `json:"name"`
    Status   string `json:"status"`     // "running", "completed", "failed"
    Duration string `json:"duration"`
    Error    string `json:"error,omitempty"`
    Progress string `json:"progress"`    // "3/9"
    Message  string `json:"message"`     // Human-readable with percentage
}

// Unified progress tracker integrates with MCP protocol
progressTracker := progress.NewProgressTracker(ctx, req, totalSteps, logger)
progressTracker.Update(currentStep, message, metadata)
```

### 3. Rich Error System

Unified error handling with structured context and AI integration:

```go
// Builder pattern for structured errors
return errors.NewError().
    Code(errors.CodeValidationFailed).
    Type(errors.ErrTypeValidation).
    Severity(errors.SeverityMedium).
    Message("validation failed").
    Context("field", fieldName).
    Suggestion("Check field format").
    WithLocation().
    Build()

// AI-powered error analysis integration
errorAnalysis, err := samplingClient.AnalyzeError(ctx, buildError, contextInfo)
```

### 4. Direct Implementation Pattern

Straightforward, clear implementations:

```go
// Instead of complex service containers, use direct implementation
func (s *AnalyzeStep) Execute(ctx context.Context, args AnalyzeArgs) error {
    // Direct implementation without abstraction layers
    return s.analyzeRepository(ctx, args.RepoPath)
}
```

## Data Flow

### 1. Unified Workflow Flow

```
User Request → MCP Server → Workflow Tool → Step Execution →
Progress Updates → Error Recovery → Completion Response
```

### 2. Step Execution Flow

```
Step Start → Progress Update → Implementation → Success/Error →
Next Step / Error Recovery → Progress Update → Continue
```

### 3. Error Handling Flow

```
Error Detection → Rich Error Creation → Context Enrichment →
Recovery Strategy → User Notification → Retry/Abort
```

### 4. Progress Tracking Flow

```
Step Start → Progress Indicator → Message Update → Status Update →
Duration Tracking → Completion Notification
```

## Security Architecture

### 1. Input Validation
- **Parameter Validation**: Type and format checking
- **Path Validation**: Protection against traversal attacks
- **Session Isolation**: Scoped operations within session boundaries

### 2. Vulnerability Scanning
- **Multi-Scanner Support**: Trivy and Grype integration
- **Comprehensive Scanning**: Full container vulnerability analysis
- **Report Generation**: Detailed security reports with remediation guidance

### 3. Container Security
- **Minimal Base Images**: Distroless and minimal containers
- **Non-Root Execution**: Security context enforcement
- **Resource Limits**: CPU and memory constraints
- **Network Policies**: Traffic isolation and control

### 4. Session Security
- **Workspace Isolation**: Session-scoped operations
- **State Protection**: Secure session state management
- **Access Control**: Session-based access restrictions

## Quality Assurance

### 1. Testing Strategy
- **Unit Tests**: Focus on individual workflow steps
- **Integration Tests**: End-to-end workflow validation
- **Performance Tests**: Workflow execution benchmarks

### 2. Quality Gates
- **Performance Target**: <300μs P95 per request
- **CI Pipeline**: Single workflow validation
- **Code Quality**: Automated linting and formatting

### 3. Code Quality Tools
- **Linting**: golangci-lint with essential rules
- **Formatting**: gofmt and goimports
- **Simplicity**: Focus on maintainable code

### 4. Monitoring & Observability
- **Progress Tracking**: Built-in workflow progress monitoring
- **Error Reporting**: Comprehensive error context
- **Health Checks**: Simple health endpoints

## Deployment & Operations

### 1. Deployment Model
- **Single Binary**: container-kit-mcp executable
- **Minimal Dependencies**: Reduced external dependencies
- **Easy Configuration**: Environment-based configuration

### 2. Kubernetes Integration
- **Manifest Generation**: Automated YAML creation
- **Health Checks**: Readiness and liveness probes
- **Rolling Updates**: Zero-downtime deployments

### 3. Configuration Management
- **Environment Variables**: 12-factor app configuration
- **Session Storage**: BoltDB for state persistence
- **Workspace Management**: Automatic workspace creation

### 4. Operations
- **Single Process**: Simplified operational model
- **Progress Monitoring**: Built-in progress tracking
- **Error Recovery**: Automated error handling

## Development Guidelines

### 1. Adding New Workflow Steps
1. **Step Implementation**: Create in `pkg/mcp/internal/steps/`
2. **Error Handling**: Use unified Rich error system
3. **Progress Tracking**: Include progress indicators
4. **Testing**: Unit and integration tests

### 2. Error Handling
- Use unified Rich error system from `pkg/common/errors/`
- Include structured context and actionable suggestions
- Implement proper error classification and severity

### 3. Workflow Development
- Follow the 10-step workflow pattern
- Include progress tracking for all steps
- Implement error recovery where possible
- Maintain session state consistency

### 4. Quality Standards
- Focus on simplicity and maintainability
- Comprehensive testing for workflow steps
- Clear documentation and progress messages

## Appendices

### A. Key Metrics

**Codebase Scale**:
- **4-layer clean architecture** with Domain-Driven Design
- **Single workflow approach** that simplifies the entire process
- **AI-powered automation** with error recovery throughout
- **Comprehensive test coverage** with integration tests

**Performance Targets**:
- Response Time: <300μs P95 per request
- Optimized Memory: Efficient memory usage
- Fast Builds: Quick compilation time

### B. Technology Dependencies

- **Go**: 1.24.4 (core language)
- **mcp-go**: v0.33.0 Model Context Protocol implementation
- **Azure OpenAI SDK**: AI integration for error recovery
- **BoltDB**: Embedded key-value storage for sessions
- **Docker Client**: Container operations
- **Kubernetes Client**: Orchestration integration
- **Trivy/Grype**: Security scanning

### C. Development Commands

```bash
# Build and test
make build              # Build MCP server
make test               # Unit tests
make test-integration   # Integration tests

# Code quality
make fmt                # Format code
make lint               # Lint code
make clean              # Clean build artifacts

# Utility
make version            # Show version
```

---

**Document Maintenance**: This design document reflects the current architecture. The system provides all essential functionality through a clean, unified workflow design focused on developer experience and reliability.