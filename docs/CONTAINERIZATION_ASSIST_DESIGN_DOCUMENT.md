# Containerization Assist Design Document

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

Containerization Assist is an advanced, AI-powered containerization platform that automates the complete Docker and Kubernetes workflow through individual, chainable tools exposed via Model Context Protocol (MCP). The system follows a simplified 4-layer architecture with tool-driven design that balances focused functionality with maintainability.

### Key Capabilities
- **15 Individual Tools**: Focused tools (10 workflow, 2 orchestration, 3 utility) with intelligent chaining
- **Tool Chaining System**: Each tool provides hints for next steps with pre-populated parameters
- **Session State Persistence**: WorkflowState shared seamlessly across tool calls via BoltDB
- **AI-Powered Process**: Intelligent automation with error recovery
- **Enterprise Security**: Comprehensive vulnerability scanning with Trivy/Grype
- **Non-Interactive Mode**: Test mode simulation prevents interactive prompts
- **Simplified Architecture**: Direct dependency injection with consolidated infrastructure

### Technology Stack
- **Core**: Go 1.24.4 with simplified 4-layer architecture
- **Protocol**: Model Context Protocol (MCP) via mcp-go library
- **AI Integration**: Azure OpenAI SDK for guided workflows and error recovery
- **Storage**: BoltDB for session persistence
- **Container Runtime**: Docker with full lifecycle management
- **Orchestration**: Kubernetes client with manifest generation
- **Security**: Trivy/Grype vulnerability scanning
- **Architecture**: Direct dependency injection with unified infrastructure packages

## System Overview

### Vision Statement
Containerization Assist provides individual, focused tools that can be chained together to guide users through the complete containerization process from analysis to deployment with AI-powered assistance and built-in progress tracking.

### Core Principles
1. **Tool-First Design**: 15 individual focused tools with intelligent chaining capabilities
2. **State Persistence**: Workflow state shared seamlessly across tool boundaries via BoltDB
3. **Simplicity**: Direct dependency injection eliminates complexity while maintaining functionality
4. **Progress Transparency**: Visual feedback for every step with unified messaging
5. **AI Integration**: Intelligent automation with error recovery
6. **Developer Experience**: Table-driven tool configuration for easy extension

### System Boundaries
- **Input**: Source code repositories, configuration parameters, user interactions
- **Processing**: Complete containerization workflow with 10 steps
- **Output**: Built containers, security reports, Kubernetes manifests, deployment status
- **External Systems**: Docker Engine, Kubernetes clusters, container registries, security scanners

## Architecture

### Four-Layer Clean Architecture

Containerization Assist follows a simplified 4-layer Domain-Driven Design architecture:

```
pkg/mcp/
├── api/                   # Interface definitions and contracts
│   └── interfaces.go      # Essential MCP tool interfaces
├── service/               # Unified service layer (simplified from application)
│   ├── bootstrap/         # Application bootstrapping
│   ├── commands/          # CQRS command handlers
│   ├── config/            # Configuration management
│   ├── dependencies.go    # Simple direct dependency injection
│   ├── lifecycle/         # Application lifecycle management
│   ├── queries/           # CQRS query handlers  
│   ├── registrar/         # MCP tool/resource registration
│   ├── registry/          # Service registry
│   ├── server.go          # MCP server implementation with direct DI
│   ├── session/           # Session management
│   ├── tools/             # Tool registration and handlers
│   ├── transport/         # HTTP and stdio transport
│   └── workflow/          # Workflow orchestration
├── domain/                # Business logic and workflows
│   ├── events/            # Domain events
│   ├── health/            # Health check interfaces
│   ├── progress/          # Progress tracking (business concept)
│   ├── prompts/           # Prompt interfaces
│   ├── resources/         # Resource interfaces
│   ├── sampling/          # LLM sampling domain logic
│   ├── session/           # Session domain objects
│   └── workflow/          # Core containerization workflow
│       ├── workflow_error.go # Simple workflow error handling
│       └── utils.go       # Workflow utility functions
└── infrastructure/        # Technical implementations (consolidated)
    ├── ai_ml/             # AI/ML implementations
    │   ├── prompts/       # Prompt management
    │   │   └── templates/ # Embedded prompt templates
    │   └── sampling/      # LLM integration
    ├── core/              # Core infrastructure
    │   ├── resources/     # Resource providers and stores
    │   ├── testutil/      # Testing utilities
    │   ├── util/          # General utilities
    │   ├── utilities/     # Advanced utilities
    │   └── validation/    # Validation logic
    ├── messaging/         # UNIFIED: Event publishing and progress reporting
    │   ├── cli_direct.go        # CLI progress reporting
    │   ├── emitter.go           # Progress emitters
    │   ├── event_publisher.go   # Domain event publishing
    │   ├── factory_direct.go    # Progress factory
    │   └── mcp_direct.go        # MCP progress reporting
    ├── observability/     # UNIFIED: Monitoring, tracing, and health
    │   ├── monitor.go           # Health monitoring
    │   ├── tracing_config.go    # OpenTelemetry configuration
    │   ├── tracing_helpers.go   # Tracing utilities
    │   └── tracing_integration.go # Tracing middleware
    ├── orchestration/     # Container and K8s orchestration
    │   └── steps/         # Focused workflow step implementations
    └── persistence/       # Data persistence
        └── session/       # Session storage (BoltDB)
```

### Key Architecture Benefits
- **Clean Dependencies**: Infrastructure → Service → Domain → API
- **Individual Tools**: 15 focused tools with intelligent chaining capabilities
- **Direct Dependency Injection**: Simple Dependencies struct eliminates complex Wire patterns
- **Unified Infrastructure**: Consolidated messaging, observability, and orchestration packages
- **Event-Driven Design**: Domain events for workflow coordination and observability
- **Domain-Driven**: Core business logic isolated in domain layer
- **Separation of Concerns**: Each layer has clear responsibilities
- **Simple Error Handling**: Workflow errors with step and attempt tracking

### Dependency Rules
- **API Layer**: Essential interfaces only, avoid over-abstraction
- **Service Layer**: Coordinate domain logic, handle MCP protocol, direct dependency injection
- **Domain Layer**: Pure business logic, no infrastructure dependencies  
- **Infrastructure Layer**: Technical implementations, external integrations, consolidated packages

## Core Components

### 1. Tool Registry (`pkg/mcp/service/tools/`)

**15 Individual Tools**: Table-driven tool registration

The tool registry provides focused, chainable tools:

```go
// Table-driven tool configuration
var toolConfigs = []ToolConfig{
    // 10 Workflow Step Tools
    {
        Name:           "analyze_repository",
        Description:    "Analyze repository to detect language and framework",
        Category:       CategoryWorkflow,
        NextTool:       "generate_dockerfile",
        ChainReason:    "Repository analyzed successfully. Ready to generate Dockerfile",
    },
    // ... 9 more workflow tools ...
    
    // 2 Orchestration Tools
    {
        Name:        "start_workflow",
        Description: "Start a complete containerization workflow",
        Category:    CategoryOrchestration,
    },
    {
        Name:        "workflow_status", 
        Description: "Check the status of a running workflow",
        Category:    CategoryOrchestration,
    },
    
    // 3 Utility Tools
    {
        Name:        "list_tools",
        Description: "List all available MCP tools",
        Category:    CategoryUtility,
    },
    // ... ping, server_status ...
}
```

**Features**:
- **Tool Chaining**: Each tool suggests the next step with chain hints
- **Session Persistence**: Workflow state shared across tool boundaries via BoltDB
- **Progress tracking**: Visual indicators with unified messaging
- **Simple Error Handling**: Workflow errors with step and attempt tracking
- **AI-powered automation**: Error recovery throughout workflow steps
- **Table-Driven Design**: Easy addition of new tools with configuration
- **Direct Dependency Injection**: Simple Dependencies struct for clarity
- **Unified Infrastructure**: Consolidated messaging and observability packages

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

### 3. Error Handling (`pkg/mcp/domain/workflow/`)

**Simple Workflow Error System**:
- **Step Context**: Clear identification of which workflow step failed
- **Attempt Tracking**: Number of retry attempts for debugging
- **Error Wrapping**: Standard Go error wrapping with fmt.Errorf
- **Lightweight Design**: No complex accumulation or escalation logic
- **Standard Error Interface**: Compatible with Go's error handling patterns
- **AI Integration**: Error context for AI-powered retry logic in workflow steps

### 4. Server Core (`pkg/mcp/service/`)

**MCP Server Implementation**:
- **Protocol Handling**: mcp-go library integration
- **Session Management**: BoltDB-based persistence with TTL
- **Tool Registration**: 15 individual tools with chaining capabilities
- **Transport Layer**: stdio and HTTP transports with proper shutdown handling
- **AI Integration**: Built-in chat mode support for Copilot integration
- **Direct Dependency Injection**: Simple Dependencies struct for clear DI
- **Configuration Management**: Environment-based configuration with validation

### 5. AI/ML Integration (`pkg/mcp/infrastructure/ai_ml/`)

**AI-Powered Features**:
- **Sampling Client** (`sampling/`): LLM integration for error recovery
- **Prompt Management** (`prompts/`): Template-based AI prompts
- **Error Analysis**: AI-powered analysis of build and deployment failures
- **Dockerfile Generation**: AI assistance for Dockerfile creation
- **Manifest Fixing**: AI-powered Kubernetes manifest error resolution
- **Repository Analysis**: AI-enhanced technology detection and recommendations

### 6. Unified Infrastructure (`pkg/mcp/infrastructure/`)

**Consolidated Packages**:
- **Messaging** (`messaging/`): Unified event publishing and progress reporting
- **Observability** (`observability/`): Consolidated monitoring, tracing, and health
- **Orchestration** (`orchestration/steps/`): Focused workflow step implementations
- **Core** (`core/`): Resource providers, utilities, and validation
- **Persistence** (`persistence/session/`): BoltDB session storage

## Design Patterns

### 1. Tool-Driven Pattern

Individual tools with intelligent chaining handle the containerization process:

```go
// RegisterTools registers all 15 individual tools
func RegisterTools(server MCPServer, deps ToolDependencies) error {
    for _, config := range toolConfigs {
        tool := mcp.Tool{
            Name:        config.Name,
            Description: config.Description,
            InputSchema: BuildToolSchema(config),
        }
        
        handler := BuildHandler(config, deps)
        server.AddTool(tool, handler)
    }
    return nil
}

// Each tool provides chain hints for next steps
type ChainHint struct {
    NextTool string `json:"next_tool"`
    Reason   string `json:"reason"`
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

### 3. Simple Workflow Error System

Lightweight error handling with step context:

```go
// Simple workflow error with step context
type WorkflowError struct {
    Step    string // Which workflow step failed
    Attempt int    // Which attempt number
    Err     error  // The underlying error
}

// Usage in workflow steps
return workflow.NewWorkflowError(
    "build",    // Step name
    attempt,    // Current attempt
    fmt.Errorf("Docker build failed: %w", err),
)

// AI-powered error analysis integration
errorAnalysis, err := samplingClient.AnalyzeError(ctx, workflowError, contextInfo)
```

### 4. Direct Dependency Injection Pattern

Simple, clear dependency management:

```go
// Simple Dependencies struct replaces complex Wire patterns
type Dependencies struct {
    Logger         *slog.Logger
    Config         workflow.ServerConfig
    SessionManager session.OptimizedSessionManager
    // ... other dependencies
}

// Direct initialization in buildDependencies()
func (f *ServerFactory) buildDependencies(ctx context.Context) (*Dependencies, error) {
    deps := &Dependencies{
        Logger: f.logger,
        Config: f.config,
        // ... initialize other dependencies in order
    }
    return deps, nil
}
```

## Data Flow

### 1. Individual Tool Flow

```
User Request → MCP Server → Tool Registry → Tool Handler →
Step Execution → Progress Updates → Chain Hint → Next Tool Suggestion
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
- **Single Binary**: containerization-assist-mcp executable
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

### 1. Adding New Tools
1. **Tool Configuration**: Add to `toolConfigs` in `pkg/mcp/service/tools/registry.go`
2. **Step Implementation**: Create corresponding step method in StepProvider
3. **Error Handling**: Use simple workflow error system from `pkg/mcp/domain/workflow/`
4. **Chain Hints**: Define NextTool and ChainReason for user guidance
5. **Testing**: Unit and integration tests with tool validation

### 2. Error Handling
- Use simple workflow error system from `pkg/mcp/domain/workflow/workflow_error.go`
- Include step name and attempt number for clear error tracking
- Wrap original errors with context using fmt.Errorf
- Keep error handling lightweight and focused on workflow needs

### 3. Tool Development
- Add tools to the table-driven registry in `pkg/mcp/service/tools/registry.go`
- Follow the ToolConfig pattern for consistent tool definitions
- Include chain hints to guide users to the next tool
- Use session state for cross-tool data sharing via BoltDB

### 4. Quality Standards
- Focus on tool simplicity and composability
- Table-driven design for easy tool addition and maintenance
- Clear chain hints to guide users through workflows
- Simple error handling with step context for debugging
- Performance targets: <300μs P95 per request

## Appendices

### A. Key Metrics

**Codebase Scale**:
- **Simplified 4-layer architecture** with Domain-Driven Design
- **15 individual tools** that can be used independently or chained
- **Direct dependency injection** eliminates complex Wire patterns
- **Unified infrastructure** packages reduce complexity
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