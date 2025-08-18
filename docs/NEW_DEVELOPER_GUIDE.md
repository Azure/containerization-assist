# Containerization Assist MCP Server - New Developer Guide

Welcome to Containerization Assist! This comprehensive guide will help you understand our MCP (Model Context Protocol) server architecture, development workflow, and best practices for contributing to the project.

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
git clone https://github.com/Azure/containerization-assist.git
cd containerization-assist

# Build the MCP server
make build

# Run tests
make test

# Start MCP server (stdio mode)
./containerization-assist-mcp

# Start MCP server (HTTP mode)
./containerization-assist-mcp --transport=http --http-port=8080
```

### Development Environment

The project includes a VS Code Dev Container with all dependencies pre-installed:
1. Open repository in VS Code
2. Choose "Reopen in Container" when prompted
3. Use provided aliases: `run-mcp` (stdio) or `run-mcp-http` (HTTP mode)

## Architecture Overview

Containerization Assist follows a **simplified 4-layer architecture** implementing Domain-Driven Design (DDD) principles:

```
pkg/mcp/
├── api/                    # Interface Layer - Contracts and DTOs
│   └── interfaces.go       # Essential MCP tool interfaces
├── service/                # Service Layer - Application services and orchestration
│   ├── server.go          # MCP server implementation
│   ├── dependencies.go    # Direct dependency injection
│   ├── bootstrap/         # Application bootstrapping
│   ├── commands/          # CQRS command handlers
│   ├── queries/           # CQRS query handlers
│   ├── session/           # Session management
│   ├── tools/             # Tool registry (tools and registrar)
│   ├── registrar/         # MCP registration
│   └── transport/         # HTTP and stdio transports
├── domain/                # Domain Layer - Business logic
│   ├── workflow/          # Core containerization workflow
│   ├── events/            # Domain events system
│   ├── progress/          # Progress tracking
│   └── session/           # Session domain objects
└── infrastructure/        # Infrastructure Layer - External integrations
    ├── orchestration/     # Container orchestration
    │   └── steps/         # Workflow step implementations
    ├── ai_ml/             # AI/ML integrations
    │   ├── sampling/      # LLM client implementation
    │   └── prompts/       # Prompt templates
    ├── messaging/         # Unified event and progress
    ├── observability/     # Unified monitoring and tracing
    └── persistence/       # BoltDB session storage
```

### Layer Responsibilities

#### API Layer
- **Purpose**: Define stable interfaces and contracts
- **Contents**: Interface definitions, DTOs, no implementation
- **Dependencies**: None (innermost layer)
- **Example**: `MCPServer` interface, tool/prompt interfaces

#### Service Layer
- **Purpose**: Orchestrate use cases and application flow
- **Contents**: Tool registry, command/query handlers, session management, direct DI
- **Dependencies**: Domain layer interfaces
- **Example**: `ToolRegistry`, `SessionManager`, `Dependencies struct`

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

### 1. Direct Dependency Injection

Containerization Assist uses simple, direct dependency injection with a Dependencies struct:

```go
// pkg/mcp/service/dependencies.go
type Dependencies struct {
    Logger         *slog.Logger
    Config         workflow.ServerConfig
    SessionManager session.OptimizedSessionManager
    ResourceStore  domainresources.Store
    
    ProgressEmitterFactory workflow.ProgressEmitterFactory
    EventPublisher         domainevents.Publisher
    
    WorkflowOrchestrator   workflow.WorkflowOrchestrator
    EventAwareOrchestrator workflow.EventAwareOrchestrator
    
    SamplingClient domainsampling.UnifiedSampler
    PromptManager  domainprompts.Manager
}

// Simple factory builds dependencies in correct order
func (f *ServerFactory) buildDependencies(ctx context.Context) (*Dependencies, error) {
    // Build infrastructure first
    // Then domain components
    // Finally service components
    return deps, nil
}
```

**Adding new dependencies:**
1. Add field to Dependencies struct
2. Add validation in Validate() method
3. Add initialization in buildDependencies()
4. No code generation needed!

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

### 3. Individual Tools with Chaining

The system provides 15 individual tools that can be chained together:

```go
// Tool configuration in pkg/mcp/service/tools/registry.go
type ToolConfig struct {
    Name           string
    Description    string
    Category       ToolCategory  // workflow, orchestration, utility
    RequiredParams []string
    
    // Chain hint configuration
    NextTool    string   // Suggested next tool
    ChainReason string   // Why to use next tool
    
    // Dependencies
    NeedsStepProvider    bool
    NeedsProgressFactory bool
    NeedsSessionManager  bool
}

// Tools provide chain hints to guide users
type ChainHint struct {
    NextTool string `json:"next_tool"`
    Reason   string `json:"reason"`
}
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

### 5. Simple Workflow Errors

Lightweight error handling with step context:

```go
// pkg/mcp/domain/workflow/workflow_error.go
type WorkflowError struct {
    Step    string // Which workflow step failed
    Attempt int    // Which attempt number
    Err     error  // The underlying error
}

// Usage
return workflow.NewWorkflowError(
    "build",    // Step name
    attempt,    // Current attempt
    fmt.Errorf("Docker build failed: %w", err),
)
```

## MCP Server and Protocol

### What is MCP?

The Model Context Protocol (MCP) is a standardized protocol for AI assistants to interact with external tools. Containerization Assist implements an MCP server that exposes 15 individual containerization tools to AI systems, allowing flexible workflows through tool chaining.

### Server Architecture

```go
// Example: registering an individual tool (analyze_repository)
func RegisterAnalyzeTool(mcpServer MCPServer, logger *slog.Logger) error {
    tool := mcp.Tool{
        Name:        "analyze_repository",
        Description: "Analyze a repository to detect language and framework",
        InputSchema: mcp.ToolInputSchema{
            Type: "object",
            Properties: map[string]interface{}{
                "repo_path": map[string]interface{}{
                    "type":        "string",
                    "description": "Absolute path to the repository to analyze",
                },
                "session_id": map[string]interface{}{
                    "type":        "string",
                    "description": "Workflow session identifier",
                },
            },
            Required: []string{"repo_path", "session_id"},
        },
    }
    mcpServer.AddTool(tool, handleAnalyzeRepository)
    return nil
}
```

### Transport Modes

1. **STDIO Mode** (Default)
   - Reads JSON-RPC from stdin, writes to stdout
   - Ideal for subprocess integration
   - Usage: `./containerization-assist-mcp`

2. **HTTP Mode**
   - REST API with Server-Sent Events
   - For remote clients and web UIs
   - Usage: `./containerization-assist-mcp --transport=http --port=8080`

### Components Registration

The MCP server registers three types of components:

1. **Tools** across three categories:
    - **Workflow Tools** (10): `analyze_repository`, `generate_dockerfile`, `build_image`, `scan_image`, `tag_image`, `push_image`, `generate_k8s_manifests`, `prepare_cluster`, `deploy_application`, `verify_deployment`
    - **Orchestration Tools** (2): `start_workflow`, `workflow_status`
    - **Utility Tools** (1): `list_tools`
2. **Resources**: Data endpoints (e.g., `progress://workflow-id`)
3. **Prompts**: AI guidance templates (e.g., dockerfile generation)

## Workflow Implementation

### The 10-Step Containerization Process

Containerization Assist provides 10 individual workflow tools that can be chained:

1. analyze_repository – Detect language, framework, dependencies
2. generate_dockerfile – AI-assisted Dockerfile creation
3. build_image – Docker build with error recovery
4. scan_image – Security vulnerability analysis
5. tag_image – Image tagging with version metadata
6. push_image – Push image to a container registry
7. generate_k8s_manifests – Create Kubernetes manifests
8. prepare_cluster – Prepare the Kubernetes cluster
9. deploy_application – Deploy manifests to the cluster
10. verify_deployment – Verify deployment health

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
   git clone https://github.com/Azure/containerization-assist.git
   cd containerization-assist
   
   # Set up environment
   export AZURE_OPENAI_API_KEY="your-key"
   export AZURE_OPENAI_ENDPOINT="https://your-instance.openai.azure.com"
   export CONTAINER_KIT_WORKSPACE="/tmp/containerization-assist"
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

1. **Add tool configuration to registry**:
   ```go
   // In pkg/mcp/service/tools/registry.go
   {
       Name:                 "my_new_tool",
       Description:          "Description of the tool",
       Category:             CategoryWorkflow,
       RequiredParams:       []string{"session_id"},
       NeedsStepProvider:    true,
       NeedsProgressFactory: true,
       NeedsSessionManager:  true,
       StepGetterName:       "GetMyNewStep",
       NextTool:             "next_tool_name",
       ChainReason:          "Tool completed successfully",
   }
   ```

2. **Implement the step**:
   ```go
   // In your StepProvider
   func (p *StepProviderImpl) GetMyNewStep() domainworkflow.Step {
       return &MyNewStep{
           // implementation
       }
   }
   ```

3. **That's it!** The tool is automatically registered with proper schema and handler.

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

### Working with Dependencies

```bash
# No code generation needed!
# Just add to Dependencies struct and buildDependencies()

# If circular dependency:
# 1. Check layer dependencies
# 2. Use interfaces instead of concrete types
# 3. Ensure proper initialization order in buildDependencies()
```

## Debugging and Troubleshooting

### Common Issues

#### Workflow Failures
- **Check logs**: Step-specific error messages
- **Review AI fixes**: See what corrections were attempted
- **Test mode**: Run with `test_mode: true` to isolate issues

#### Dependency Injection Errors
- **Circular dependencies**: Ensure proper layer separation
- **Missing dependencies**: Add to Dependencies struct and Validate()
- **Initialization order**: Check buildDependencies() order

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

# Check dependency initialization
go test ./pkg/mcp/service -run TestDependencies
```

## Reference

### Environment Variables

```bash
# Core Configuration
CONTAINER_KIT_WORKSPACE=/tmp/containerization-assist
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
CONTAINER_KIT_STORE_PATH=.containerization-assist.db

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
├── service/            # Application services
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
- [ADR-001: Tool-Splitting Architecture](../docs/architecture/adr/2025-07-11-tool-splitting-architecture.md)
- [ADR-002: Go Embed Template Management](../docs/architecture/adr/2025-07-11-go-embed-template-management.md)
- [ADR-003: Direct Dependency Injection](../docs/architecture/adr/2025-07-11-direct-dependency-injection.md)
- [ADR-004: Simple Workflow Error Handling](../docs/architecture/adr/2025-07-11-simple-workflow-errors.md)
- [ADR-005: AI-Assisted Error Recovery](../docs/architecture/adr/2025-07-11-ai-assisted-error-recovery.md)
- [ADR-006: Simplified Four-Layer MCP Architecture](../docs/architecture/adr/2025-07-12-simplified-four-layer-architecture.md)
- [ADR-007: Infrastructure Layer Consolidation](../docs/architecture/adr/2025-07-12-infrastructure-consolidation.md)

---

Welcome to Containerization Assist! This guide provides everything you need to understand and contribute to the project. The clean architecture, comprehensive patterns, and AI-powered features make it a powerful platform for containerization. Take time to explore the codebase, run the examples, and don't hesitate to ask questions in our community channels.