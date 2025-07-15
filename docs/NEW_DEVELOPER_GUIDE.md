# Container Kit MCP Server - New Developer Guide

Welcome to Container Kit! This comprehensive guide will help you understand our MCP (Model Context Protocol) server architecture, development workflow, and best practices for contributing to the project.

## Table of Contents

1. [Quick Start](#quick-start)
2. [Architecture Overview](#architecture-overview)
3. [Key Design Patterns](#key-design-patterns)
4. [MCP Server and Protocol](#mcp-server-and-protocol)
5. [Workflow Implementation](#workflow-implementation)
6. [Development Guide](#development-guide)
7. [Testing Strategy](#testing-strategy)
8. [Common Tasks](#common-tasks)
9. [Debugging and Troubleshooting](#debugging-and-troubleshooting)
10. [Reference](#reference)

## Quick Start

### Prerequisites
- Go 1.24.4+ (configured via `.tool-versions`)
- Docker (for container operations)
- Make (for build commands)
- Kind (for local Kubernetes clusters)
- Azure OpenAI credentials (for AI features)

### Initial Setup

```bash
# Clone the repository
git clone https://github.com/Azure/container-kit.git
cd container-kit

# Build the MCP server
make build

# Run tests
make test

# Start MCP server (stdio mode)
./container-kit-mcp

# Start MCP server (HTTP mode)
./container-kit-mcp --transport=http --http-port=8080
```

### Development Environment

The project includes a VS Code Dev Container with all dependencies pre-installed:
1. Open repository in VS Code
2. Choose "Reopen in Container" when prompted
3. Use provided aliases: `run-mcp` (stdio) or `run-mcp-http` (HTTP mode)

## Architecture Overview

Container Kit follows a **4-layer clean architecture** implementing Domain-Driven Design (DDD) principles:

```
pkg/mcp/
├── api/                    # Interface Layer - Contracts and DTOs
│   └── interfaces.go       # Essential MCP tool interfaces
├── application/            # Application Layer - Use cases and orchestration
│   ├── server.go          # MCP server implementation
│   ├── bootstrap.go       # Dependency injection setup
│   ├── commands/          # CQRS command handlers
│   ├── queries/           # CQRS query handlers
│   └── session/           # Session management
├── domain/                # Domain Layer - Business logic
│   ├── workflow/          # Core containerization workflow
│   ├── events/            # Domain events system
│   ├── errors/            # Rich error handling
│   ├── progress/          # Progress tracking
│   └── saga/              # Saga pattern implementation
└── infrastructure/        # Infrastructure Layer - External integrations
    ├── steps/             # Workflow step implementations
    ├── ml/                # AI/ML integrations
    ├── sampling/          # LLM client implementation
    ├── prompts/           # Prompt templates
    ├── resources/         # MCP resource providers
    └── wire/              # Dependency injection setup
```

### Layer Responsibilities

#### API Layer
- **Purpose**: Define stable interfaces and contracts
- **Contents**: Interface definitions, DTOs, no implementation
- **Dependencies**: None (innermost layer)
- **Example**: `MCPServer` interface, tool/prompt interfaces

#### Application Layer
- **Purpose**: Orchestrate use cases and application flow
- **Contents**: Command/query handlers, session management, server setup
- **Dependencies**: Domain layer interfaces
- **Example**: `ContainerizeCommandHandler`, `WorkflowStatusQueryHandler`

#### Domain Layer
- **Purpose**: Core business logic and rules
- **Contents**: Workflow definitions, domain events, error handling
- **Dependencies**: API layer interfaces only
- **Example**: `Orchestrator`, `SagaCoordinator`, `RichError`

#### Infrastructure Layer
- **Purpose**: Technical implementations and external integrations
- **Contents**: Docker/K8s clients, AI services, file operations
- **Dependencies**: All other layers (outermost)
- **Example**: Step implementations, Azure OpenAI client

### Dependency Flow
```
Infrastructure → Application → Domain → API
```

## Key Design Patterns

### 1. Dependency Injection (Google Wire)

Container Kit uses compile-time dependency injection via Google Wire:

```go
// pkg/mcp/infrastructure/wire/wire.go
var ProviderSet = wire.NewSet(
    ConfigurationSet,
    ApplicationSet,
    DomainSet,
    InfrastructureSet,
)

// Generated InitializeServer assembles all dependencies
func InitializeServer(ctx context.Context, logger *slog.Logger, config *ServerConfig) (*MCPServer, error) {
    wire.Build(ProviderSet)
    return nil, nil
}
```

**Adding new dependencies:**
1. Create provider function in `wire.go`
2. Add to appropriate provider set
3. Run `go generate ./pkg/mcp/infrastructure/wire`
4. Wire ensures compile-time safety

### 2. CQRS (Command Query Responsibility Segregation)

Separates write operations (commands) from read operations (queries):

**Commands** - Change state:
```go
type ContainerizeCommand struct {
    ID        string
    SessionID string
    Args      ContainerizeArgs
}

func (h *ContainerizeCommandHandler) Handle(ctx context.Context, cmd Command) error {
    // Start workflow, update session, publish events
    result, err := h.orchestrator.Execute(ctx, cmd)
    h.sessionManager.UpdateSession(cmd.SessionID, result)
    return err
}
```

**Queries** - Read state:
```go
type WorkflowStatusQuery struct {
    SessionID  string
    WorkflowID string
}

func (h *WorkflowStatusQueryHandler) Handle(ctx context.Context, q Query) (interface{}, error) {
    // Read from session, build view, return data
    session := h.sessionManager.GetSession(q.SessionID)
    return buildWorkflowStatusView(session), nil
}
```

### 3. Saga Pattern for Distributed Transactions

Manages long-running workflows with compensating actions:

```go
// Each saga step has forward and compensating operations
type SagaStep interface {
    Name() string
    Execute(ctx context.Context) error
    Compensate(ctx context.Context) error
    CanCompensate() bool
}

// Coordinator manages saga lifecycle
saga := coordinator.StartSaga(sagaID, steps)
// If step 7 fails, automatically compensates steps 6→1
```

### 4. Domain Events

Event-driven architecture for loose coupling:

```go
type WorkflowStepCompletedEvent struct {
    WorkflowID string
    StepName   string
    Success    bool
    Duration   time.Duration
}

// Publishers emit events
publisher.PublishAsync(WorkflowStepCompletedEvent{...})

// Subscribers react to events
subscriber.On("WorkflowStepCompleted", handleStepCompleted)
```

### 5. Rich Error System

Structured error handling with context:

```go
return errors.NewError().
    Code(errors.CodeBuildFailed).
    Type(errors.ErrTypeBuild).
    Severity(errors.SeverityHigh).
    Message("Docker build failed").
    Context("image", imageName).
    Suggestion("Check Dockerfile syntax").
    UserFacing(true).
    Retryable(true).
    WithLocation().
    Build()
```

## MCP Server and Protocol

### What is MCP?

The Model Context Protocol (MCP) is a standardized protocol for AI assistants to interact with external tools. Container Kit implements an MCP server that exposes containerization capabilities to AI systems.

### Server Architecture

```go
// MCP server registration
func RegisterWorkflowTools(mcpServer MCPServer, logger *slog.Logger) error {
    tool := mcp.Tool{
        Name:        "containerize_and_deploy",
        Description: "Complete containerization workflow",
        InputSchema: mcp.ToolInputSchema{
            Type: "object",
            Properties: map[string]interface{}{
                "repo_url": map[string]interface{}{
                    "type":        "string",
                    "description": "Repository URL",
                },
            },
            Required: []string{"repo_url"},
        },
    }
    
    mcpServer.AddTool(tool, handleContainerizeAndDeploy)
    return nil
}
```

### Transport Modes

1. **STDIO Mode** (Default)
   - Reads JSON-RPC from stdin, writes to stdout
   - Ideal for subprocess integration
   - Usage: `./container-kit-mcp`

2. **HTTP Mode**
   - REST API with Server-Sent Events
   - For remote clients and web UIs
   - Usage: `./container-kit-mcp --transport=http --port=8080`

### Components Registration

The MCP server registers three types of components:

1. **Tools**: Actions AI can invoke (e.g., `containerize_and_deploy`)
2. **Resources**: Data endpoints (e.g., `progress://workflow-id`)
3. **Prompts**: AI guidance templates (e.g., dockerfile generation)

## Workflow Implementation

### The 10-Step Containerization Process

Container Kit implements a unified workflow with 10 sequential steps:

1. **Analyze Repository** - Detect language, framework, dependencies
2. **Generate Dockerfile** - AI-powered Dockerfile creation
3. **Build Image** - Docker build with AI error recovery
4. **Setup Kind Cluster** - Local Kubernetes environment
5. **Load Image** - Transfer image to cluster
6. **Generate Manifests** - Kubernetes YAML generation
7. **Deploy Application** - Apply manifests with error handling
8. **Health Probe** - Validate deployment health
9. **Vulnerability Scan** - Security analysis (optional)
10. **Finalize Result** - Cleanup and summary

### Step Implementation Pattern

Each step follows a consistent pattern:

```go
// pkg/mcp/infrastructure/steps/analyze.go
func AnalyzeRepository(ctx context.Context, args AnalyzeArgs) (*AnalyzeResult, error) {
    // 1. Core analysis logic
    result := performAnalysis(args.RepoPath)
    
    // 2. Optional AI enhancement
    if aiEnabled {
        enhanced := enhanceWithAI(ctx, result)
        result.merge(enhanced)
    }
    
    // 3. Rich error handling
    if err != nil {
        return nil, errors.NewError().
            Code(errors.CodeAnalysisFailed).
            Message("Failed to analyze repository").
            Context("path", args.RepoPath).
            Build()
    }
    
    return result, nil
}
```

### AI-Powered Error Recovery

Key innovation: Automatic error recovery using AI:

```go
func executeDockerBuildWithAIFix(ctx context.Context, args BuildArgs) error {
    for attempt := 1; attempt <= maxRetries; attempt++ {
        err := dockerBuild(args)
        if err == nil {
            return nil // Success
        }
        
        // Capture error and ask AI for fix
        fix := aiClient.AnalyzeBuildError(err, args.Dockerfile)
        applyDockerfileFix(args.DockerfilePath, fix)
        
        logger.Info("Retrying build with AI fix", "attempt", attempt)
    }
    return fmt.Errorf("build failed after %d attempts", maxRetries)
}
```

### Progress Tracking

Real-time progress updates throughout workflow:

```go
tracker := progress.NewTracker(totalSteps)

// During execution
tracker.Begin("Starting containerization")
tracker.Update(3, 10, "Building Docker image...")
tracker.Complete("Workflow completed successfully")

// Accessible via MCP resources
// GET progress://workflow-123 returns current status
```

## Development Guide

### Setting Up Your Environment

1. **Clone and Configure**
   ```bash
   git clone https://github.com/Azure/container-kit.git
   cd container-kit
   
   # Set up environment
   export AZURE_OPENAI_API_KEY="your-key"
   export AZURE_OPENAI_ENDPOINT="https://your-instance.openai.azure.com"
   export CONTAINER_KIT_WORKSPACE="/tmp/container-kit"
   ```

2. **Build and Test**
   ```bash
   make build
   make test
   make lint
   ```

### Adding a New Workflow Step

1. **Create step implementation**:
   ```go
   // pkg/mcp/infrastructure/steps/notify.go
   type NotifyStep struct {
       notifier Notifier
   }
   
   func (s *NotifyStep) Name() string { return "notify" }
   func (s *NotifyStep) MaxRetries() int { return 1 }
   
   func (s *NotifyStep) Execute(ctx context.Context, state *WorkflowState) error {
       state.Logger.Info("Sending notification")
       
       return s.notifier.Send(NotificationRequest{
           Message: fmt.Sprintf("Workflow %s completed", state.WorkflowID),
           Details: state.Result,
       })
   }
   ```

2. **Add to orchestrator**:
   ```go
   // Update total steps
   const totalSteps = 11
   
   // Add step to sequence
   steps = append(steps, NewNotifyStep(notifier))
   ```

3. **Wire dependencies**:
   ```go
   // Add provider if needed
   func provideNotifier(config *Config) Notifier {
       return NewEmailNotifier(config.SMTP)
   }
   ```

### Adding a New MCP Tool

1. **Define tool schema**:
   ```go
   healthCheckTool := mcp.Tool{
       Name:        "health_check",
       Description: "Check application health",
       InputSchema: mcp.ToolInputSchema{
           Type: "object",
           Properties: map[string]interface{}{
               "endpoint": map[string]interface{}{
                   "type": "string",
               },
           },
       },
   }
   ```

2. **Implement handler**:
   ```go
   func handleHealthCheck(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
       endpoint := req.Arguments["endpoint"].(string)
       
       health := checkHealth(endpoint)
       
       return &mcp.CallToolResult{
           Content: []mcp.Content{
               mcp.TextContent{
                   Type: "text",
                   Text: toJSON(health),
               },
           },
       }, nil
   }
   ```

3. **Register with server**:
   ```go
   mcpServer.AddTool(healthCheckTool, handleHealthCheck)
   ```

## Testing Strategy

### Unit Testing

Test individual components in isolation:

```go
func TestAnalyzeRepository(t *testing.T) {
    // Arrange
    args := AnalyzeArgs{
        RepoPath: "testdata/sample-node-app",
    }
    
    // Act
    result, err := steps.AnalyzeRepository(context.Background(), args)
    
    // Assert
    assert.NoError(t, err)
    assert.Equal(t, "javascript", result.Language)
    assert.Contains(t, result.Dependencies, "express")
}
```

### Integration Testing

Test complete workflows:

```go
func TestContainerizeWorkflow(t *testing.T) {
    // Use test mode to avoid external calls
    args := ContainerizeArgs{
        RepoURL:  "https://github.com/test/repo",
        TestMode: true,
    }
    
    orchestrator := NewOrchestrator(logger)
    result, err := orchestrator.Execute(ctx, args)
    
    assert.NoError(t, err)
    assert.True(t, result.Success)
    assert.Len(t, result.Steps, 10)
}
```

### Test Modes

- **Normal Mode**: Full execution with real services
- **Test Mode**: Simulated execution for fast testing
- **AI Stub Mode**: Uses mock LLM responses

### Running Tests

```bash
# Unit tests
make test

# Integration tests
make test-integration

# Specific package
go test -v ./pkg/mcp/domain/workflow/...

# With coverage
go test -cover ./...
```

## Common Tasks

### Debugging a Failed Workflow Step

1. **Enable debug logging**:
   ```bash
   export CONTAINER_KIT_LOG_LEVEL=debug
   ```

2. **Check step logs**:
   ```go
   // Logs show step execution details
   INFO Step 3: Building Docker image repo_url=...
   DEBUG Docker build command args=[build -t test-app:latest .]
   ERROR Build failed error="dockerfile:10 invalid instruction"
   ```

3. **Review AI suggestions**:
   ```go
   // AI error analysis provides fixes
   INFO AI suggestion: "Add 'RUN' before the npm install command"
   ```

### Session Management

```go
// Create session
session, _ := sessionManager.CreateSession(ctx, &SessionRequest{
    Labels: map[string]string{
        "project": "my-app",
        "env":     "dev",
    },
})

// Update session during workflow
sessionManager.UpdateSession(session.ID, func(s *SessionState) error {
    s.Metadata["step_completed"] = "analyze"
    s.Progress = "1/10"
    return nil
})
```

### Working with Wire

```bash
# After adding new provider
go generate ./pkg/mcp/infrastructure/wire

# If circular dependency
# 1. Check layer dependencies
# 2. Use interfaces instead of concrete types
# 3. Move provider to correct layer
```

## Debugging and Troubleshooting

### Common Issues

#### Workflow Failures
- **Check logs**: Step-specific error messages
- **Review AI fixes**: See what corrections were attempted
- **Test mode**: Run with `test_mode: true` to isolate issues

#### Wire Compilation Errors
- **Circular dependencies**: Ensure proper layer separation
- **Missing providers**: Add provider functions for new types
- **Run generate**: `go generate ./pkg/mcp/infrastructure/wire`

#### Session Errors
- **BoltDB locks**: Ensure single process access
- **TTL expiration**: Check session hasn't expired
- **Storage path**: Verify write permissions

### Debug Tools

```bash
# Verbose logging
export CONTAINER_KIT_LOG_LEVEL=debug

# Test specific step
go test -run TestAnalyzeRepository -v

# Profile performance
go test -bench=. -cpuprofile=cpu.prof

# Check wire dependencies
wire check ./pkg/mcp/infrastructure/wire
```

## Reference

### Environment Variables

```bash
# Core Configuration
CONTAINER_KIT_WORKSPACE=/tmp/container-kit
CONTAINER_KIT_TRANSPORT=stdio|http
CONTAINER_KIT_HTTP_PORT=8080
CONTAINER_KIT_LOG_LEVEL=info|debug|error

# Azure OpenAI
AZURE_OPENAI_API_KEY=sk-...
AZURE_OPENAI_ENDPOINT=https://...
AZURE_OPENAI_DEPLOYMENT_NAME=gpt-4
AZURE_OPENAI_MODEL_ID=gpt-4

# Session Management
CONTAINER_KIT_SESSION_TTL=24h
CONTAINER_KIT_MAX_SESSIONS=100
CONTAINER_KIT_STORE_PATH=.container-kit.db

# Feature Flags
CONTAINER_KIT_TEST_MODE=true
CONTAINER_KIT_ENABLE_SCAN=true
CONTAINER_KIT_ENABLE_AI=true
```

### Make Targets

```bash
make build              # Build MCP server
make test               # Run unit tests
make test-integration   # Run integration tests
make lint               # Lint code
make fmt                # Format code
make clean              # Clean artifacts
make version            # Show version
make help               # Show all targets
```

### Key Files and Directories

```
cmd/mcp-server/         # MCP server entry point
pkg/mcp/                # Core MCP implementation
├── api/                # Interfaces and contracts
├── application/        # Use cases and orchestration
├── domain/             # Business logic
└── infrastructure/     # External integrations

docs/                   # Documentation
├── architecture/       # Architecture decisions
└── adr/                # ADR records

test/                   # Integration tests
examples/               # Usage examples
.devcontainer/          # VS Code dev container
```

### Architecture Decision Records (ADRs)

Key architectural decisions:
- [ADR-001: Single Workflow Architecture](../docs/architecture/adr/2025-07-11-single-workflow-architecture.md)
- [ADR-002: Go Embed Template Management](../docs/architecture/adr/2025-07-11-go-embed-template-management.md)
- [ADR-003: Wire-Based Dependency Injection](../docs/architecture/adr/2025-07-11-wire-dependency-injection.md)
- [ADR-004: Unified Rich Error System](../docs/architecture/adr/2025-07-11-unified-rich-error-system.md)
- [ADR-005: AI-Assisted Error Recovery](../docs/architecture/adr/2025-07-11-ai-assisted-error-recovery.md)
- [ADR-006: Four-Layer MCP Architecture](../docs/architecture/adr/2025-07-12-four-layer-mcp-architecture.md)

---

Welcome to Container Kit! This guide provides everything you need to understand and contribute to the project. The clean architecture, comprehensive patterns, and AI-powered features make it a powerful platform for containerization. Take time to explore the codebase, run the examples, and don't hesitate to ask questions in our community channels.