# Container Kit Design Document

**Version**: 2.0
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

Container Kit is a streamlined, AI-powered containerization platform that automates the complete Docker and Kubernetes workflow through a unified Model Context Protocol (MCP) server. The system focuses on simplicity and effectiveness with just 25 core files delivering all essential functionality.

### Key Capabilities
- **Unified Workflow**: Single `containerize_and_deploy` tool handles complete process
- **Progress Tracking**: Built-in progress indicators for all 10 workflow steps
- **AI-Powered Process**: Intelligent automation with error recovery
- **Enterprise Security**: Comprehensive vulnerability scanning with Trivy/Grype
- **Session Management**: Persistent state with BoltDB storage
- **Simplified Architecture**: Essential functionality without over-engineering

### Technology Stack
- **Core**: Go 1.24.1 with simplified architecture
- **Protocol**: Model Context Protocol (MCP) via gomcp library
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

### Workflow-Driven Architecture

Container Kit uses a focused, workflow-driven architecture with modular organization:

```
pkg/
├── mcp/             # Model Context Protocol server & workflow
│   ├── application/     # Server implementation & session management
│   ├── domain/          # Business logic (workflows, types)
│   └── infrastructure/  # Workflow steps, analysis, retry
├── core/            # Core containerization services
│   ├── docker/          # Docker operations & services
│   ├── kubernetes/      # Kubernetes operations & manifests
│   ├── kind/            # Kind cluster management
│   └── security/        # Security scanning & validation
├── common/          # Shared utilities
│   ├── errors/          # Rich error handling system
│   ├── filesystem/      # File operations
│   ├── logger/          # Logging utilities
│   └── runner/          # Command execution
├── ai/              # AI integration and analysis
└── pipeline/        # Legacy pipeline stages
```

### Key Architecture Benefits
- **Focused Design**: Only 25 core files to maintain
- **Single Workflow**: Unified process without coordination complexity
- **Direct Implementation**: Clear, straightforward code paths
- **Clear Structure**: Easy to understand and modify
- **Essential Functionality**: Everything needed, nothing more

### Dependency Rules
- **Workflow Steps**: Can use domain/errors and core packages
- **Server Core**: Handles MCP protocol and session management
- **Error System**: Provides structured error handling across all components

## Core Components

### 1. Workflow Server (`pkg/mcp/server/`)

**Single Workflow Tool**: `containerize_and_deploy`

The unified workflow tool handles the complete containerization process:

```go
// Complete 10-step workflow
steps := []string{
    "analyze",      // 1/10: Repository analysis
    "dockerfile",   // 2/10: Dockerfile generation
    "build",        // 3/10: Docker build
    "scan",         // 4/10: Security scanning
    "tag",          // 5/10: Image tagging
    "push",         // 6/10: Registry push
    "manifest",     // 7/10: K8s manifest generation
    "cluster",      // 8/10: Cluster setup
    "deploy",       // 9/10: Deployment
    "verify",       // 10/10: Health verification
}
```

**Features**:
- Progress tracking with visual indicators
- Error recovery with actionable messages
- AI-powered automation throughout
- Session management with BoltDB persistence

### 2. Step Implementations (`pkg/mcp/internal/steps/`)

#### Analyze Step (`analyze.go`)
- **Repository Analysis**: Language detection, dependency analysis, framework identification
- **Context Gathering**: Project structure understanding
- **Technology Detection**: Automated technology stack identification
- **Best Practices**: Framework-specific optimization recommendations

#### Build Step (`build.go`)
- **Docker Operations**: Build, push, pull, tag with comprehensive error handling
- **AI-Powered Fixing**: Automatic build error detection and resolution
- **Registry Integration**: Multi-registry support with health monitoring
- **Build Optimization**: Layer caching and multi-stage build optimization

#### Kubernetes Step (`k8s.go`)
- **Manifest Generation**: Automated YAML generation with customization
- **Health Checks**: Application readiness and liveness probe configuration
- **Secret Management**: Secure secret generation and injection
- **Deployment Orchestration**: Rolling updates with rollback capabilities

### 3. Error Handling (`pkg/common/errors/`)

**Rich Error System**:
- **Structured Error Context**: Comprehensive error information
- **Actionable Messages**: Clear guidance for resolution
- **Core Infrastructure**: Essential component used throughout codebase
- **Error Classification**: Severity and category-based handling

### 4. Server Core (`pkg/mcp/application/core/`)

**MCP Server Implementation**:
- **Protocol Handling**: MCP protocol implementation
- **Session Management**: Simple session state management
- **Tool Registration**: Workflow tool registration
- **Transport Layer**: stdio and HTTP support

## Design Patterns

### 1. Unified Workflow Pattern

A single workflow tool handles the complete containerization process:

```go
type ContainerizeAndDeployTool struct {
    workspaceDir string
    logger       *slog.Logger
}

func (t *ContainerizeAndDeployTool) Execute(ctx context.Context, args ContainerizeAndDeployArgs) (interface{}, error) {
    for i, step := range steps {
        progress := fmt.Sprintf("%d/%d", i+1, len(steps))
        message := fmt.Sprintf("Step %d: %s", i+1, getStepDescription(step))
        
        // Execute step with progress tracking
        if err := t.executeStep(ctx, step, progress, message); err != nil {
            return nil, err
        }
    }
    return result, nil
}
```

### 2. Progress Tracking Pattern

Every step provides progress feedback:

```go
type WorkflowStep struct {
    Name     string `json:"name"`
    Status   string `json:"status"`
    Duration string `json:"duration"`
    Error    string `json:"error,omitempty"`
    Progress string `json:"progress"`    // "3/10"
    Message  string `json:"message"`     // Human-readable
}
```

### 3. Rich Error System

Unified error handling with structured context:

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
- **25 core files** delivering complete functionality
- **Workflow-focused architecture** with clear design
- **Single unified workflow** for the entire process
- **Essential components** only

**Performance Targets**:
- Response Time: <300μs P95 per request
- Optimized Memory: Efficient memory usage
- Fast Builds: Quick compilation time

### B. Technology Dependencies

- **Go**: 1.24.1 (core language)
- **gomcp**: Model Context Protocol implementation
- **BoltDB**: Embedded key-value storage
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