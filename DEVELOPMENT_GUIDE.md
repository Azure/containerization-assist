# Development Guide

This comprehensive guide covers everything developers need to know about the Containerization Assist MCP project, including architecture, coding standards, development workflow, and best practices.

## Table of Contents

1. [Quick Start](#quick-start)
2. [Architecture Overview](#architecture-overview)
3. [Development Setup](#development-setup)
4. [Code Style Standards](#code-style-standards)
5. [Error Handling](#error-handling)
6. [Testing Strategy](#testing-strategy)
7. [Key Design Patterns](#key-design-patterns)
8. [Working with MCP Tools](#working-with-mcp-tools)
9. [Common Development Tasks](#common-development-tasks)
10. [Debugging and Troubleshooting](#debugging-and-troubleshooting)
11. [Contributing](#contributing)

## Quick Start

### Prerequisites
- Go 1.24.4+ (configured via `.tool-versions`)
- Docker (for container operations)
- Make (for build commands)
- kubectl (optional, for Kubernetes features)
- Azure OpenAI access (for AI features)

### Initial Setup

```bash
# Clone the repository
git clone https://github.com/Azure/containerization-assist.git
cd containerization-assist

# Build the MCP server
make build

# Run tests
make test                   # Unit tests
make test-integration       # Integration tests

# Start MCP server
./containerization-assist-mcp
```

### Development Environment

The project includes a VS Code Dev Container with all dependencies pre-installed:
1. Open repository in VS Code
2. Choose "Reopen in Container" when prompted
3. All tools and dependencies are pre-configured

## Architecture Overview

Containerization Assist follows a **simplified 3-layer architecture** with Domain-Driven Design:

```
pkg/
├── api/                   # API Layer - Minimal contracts
│   └── interfaces.go      # MCPServer, ProgressEmitter, ProgressUpdate
├── service/               # Service Layer - Application services
│   ├── server.go          # MCP server implementation
│   ├── dependencies.go    # Direct dependency injection
│   ├── bootstrap/         # Application bootstrapping
│   ├── config/            # Configuration management
│   ├── registrar/         # Tool/resource registration
│   ├── session/           # Concurrent-safe session management
│   └── tools/             # MCP tool implementations
│       ├── registry.go    # Table-driven tool registration
│       ├── types.go       # Typed structs for workflow state
│       └── helpers.go     # Atomic operations
├── domain/                # Domain Layer - Business logic
│   ├── workflow/          # Core containerization workflow
│   ├── errors/            # Domain error types
│   ├── events/            # Domain events
│   ├── resources/         # Resource interfaces
│   └── session/           # Session domain objects
└── infrastructure/        # Infrastructure Layer - Technical implementations
    ├── ai_ml/             # AI/ML integrations
    │   ├── sampling/      # LLM integration
    │   └── prompts/       # Prompt templates (YAML)
    ├── core/              # Consolidated utilities
    │   ├── command.go     # CommandRunner interface
    │   ├── providers.go   # Resource store
    │   ├── fs.go          # File operations
    │   ├── repository.go  # Repository analysis
    │   └── masking.go     # Data protection
    ├── container/         # Docker operations
    ├── kubernetes/        # K8s operations
    ├── messaging/         # Progress tracking and events
    ├── orchestration/     # Workflow orchestration
    │   └── steps/         # Step implementations
    ├── persistence/       # BoltDB storage
    └── security/          # Vulnerability types
```

### Layer Responsibilities

#### API Layer
- **Purpose**: Define minimal stable interfaces
- **Contents**: Only essential interfaces (MCPServer, ProgressEmitter, ProgressUpdate)
- **Dependencies**: None (innermost layer)

#### Service Layer
- **Purpose**: Orchestrate use cases and application flow
- **Contents**: Tool registry, session management, direct DI, configuration
- **Dependencies**: Domain layer interfaces

#### Domain Layer
- **Purpose**: Core business logic and rules
- **Contents**: Workflow definitions, domain events, error handling
- **Dependencies**: API layer interfaces only

#### Infrastructure Layer
- **Purpose**: Technical implementations and external integrations
- **Contents**: Docker/K8s clients, AI services, file operations
- **Dependencies**: All other layers (outermost)

### Dependency Flow
```
Infrastructure → Service → Domain → API
```

## Development Setup

### Build Commands

```bash
# Primary build - builds MCP server binary
make build

# Run all tests with race detection
make test

# Run integration tests
make test-integration

# Code quality checks
make fmt                   # Format code
make lint                  # Run linter
make check-all            # Run all checks

# Clean build artifacts
make clean
```

### Environment Variables

Create a `.env` file for local development:

```bash
# AI Configuration
AZURE_OPENAI_API_KEY=your-key
AZURE_OPENAI_ENDPOINT=https://your-endpoint.openai.azure.com

# Non-Interactive Mode (for testing)
NON_INTERACTIVE=true

# Session Configuration
SESSION_STORE_PATH=/tmp/mcp-store
SESSION_TTL=24h
```

## Code Style Standards

### Go-Specific Guidelines

#### Package Structure
- Use clear, descriptive package names
- Keep packages focused on a single responsibility
- Core utilities consolidated in `pkg/infrastructure/core`
- Maximum 800 lines per file

#### Naming Conventions
```go
// Constants: Use CamelCase for exported, camelCase for internal
const MaxRetryAttempts = 3
const defaultTimeout = 30 * time.Second

// Variables: Use camelCase
var sessionManager SessionManager
var requestID string

// Functions: Use CamelCase for exported, camelCase for internal
func BuildDockerImage(args BuildArgs) (*BuildResult, error)
func validateDockerfile(path string) error

// Types: Use CamelCase
type SimpleWorkflowState struct{}
type WorkflowArtifacts struct{}
```

#### Interface Design
- Keep interfaces small and focused (1-3 methods preferred)
- Name interfaces with `-er` suffix when possible
- Place interfaces close to their usage, not implementation

```go
type Builder interface {
    BuildImage(ctx context.Context, args BuildArgs) (*BuildResult, error)
}

type SessionManager interface {
    GetSession(id string) (*Session, error)
    UpdateSession(session *Session) error
}
```

### No Print Statements
- **NEVER** use `fmt.Print*`, `log.Print*`, or `print()` statements
- Use structured logging with `zerolog`

```go
// Bad
fmt.Println("Starting build process")
log.Printf("Build failed: %v", err)

// Good
logger.Info().Str("session_id", sessionID).Msg("Starting build process")
logger.Error().Err(err).Str("session_id", sessionID).Msg("Build failed")
```

### Import Organization
```go
import (
    // Standard library
    "context"
    "fmt"
    "os"

    // Third-party
    "github.com/rs/zerolog"
    "github.com/stretchr/testify/assert"

    // Internal
    "github.com/Azure/containerization-assist/pkg/domain/workflow"
    "github.com/Azure/containerization-assist/pkg/infrastructure/core"
)
```

## Error Handling

### Standard Go Error Wrapping
Use `fmt.Errorf` with the `%w` verb for error wrapping:

```go
// Standard error wrapping (preferred)
if err != nil {
    return fmt.Errorf("failed to build image: %w", err)
}

// With additional context
if err != nil {
    return fmt.Errorf("failed to build image %s at line %d: %w", imageName, lineNum, err)
}
```

### Domain Errors
Use domain errors when structured error handling is needed:

```go
import "github.com/Azure/containerization-assist/pkg/domain/errors"

// For structured errors with codes
return errors.New(
    errors.CodeBuildFailed,
    "build",
    "Docker build failed during RUN step",
    err,
)
```

### Error Context
- Always provide actionable error messages
- Include relevant context (session ID, file paths, etc.)
- Suggest remediation steps where possible

## Testing Strategy

### Test Organization
```go
func TestFunctionName_Scenario(t *testing.T) {
    // Follow AAA pattern
    // Arrange
    ctx := context.Background()
    args := buildTestArgs()

    // Act
    result, err := functionUnderTest(ctx, args)

    // Assert
    require.NoError(t, err)
    assert.Equal(t, expected, result.Field)
}
```

### Test Patterns
- Use table-driven tests for multiple scenarios
- Include helper functions with `t.Helper()` for setup
- Test both success and failure cases
- Use meaningful test names describing the scenario
- Mock external dependencies using interfaces

### Test Types
- **Unit Tests**: `pkg/*/..._test.go`
- **Integration Tests**: `test/integration/`
- **Contract Tests**: `test/contract/` for API stability
- **Benchmarks**: For performance-critical functions

### Running Tests
```bash
# Run all tests
make test

# Run specific package tests
go test -v ./pkg/service/tools

# Run with race detection
go test -race ./...

# Run benchmarks
go test -bench=. ./pkg/...
```

## Key Design Patterns

### 1. Direct Dependency Injection

Simple dependency injection without code generation:

```go
// pkg/service/dependencies.go
type Dependencies struct {
    Logger         *slog.Logger
    Config         workflow.ServerConfig
    SessionManager session.OptimizedSessionManager
    ResourceStore  *core.Store
    
    // Add new dependencies here
}

// Simple validation
func (d *Dependencies) Validate() error {
    if d.Logger == nil {
        return errors.New("logger is required")
    }
    // Add validation for new dependencies
    return nil
}
```

### 2. Table-Driven Tool Registration

All 15 MCP tools are registered via table configuration:

```go
// pkg/service/tools/registry.go
var toolConfigs = []ToolConfig{
    {
        Name:        "analyze_repository",
        Description: "Analyze repository structure",
        Category:    CategoryWorkflow,
        Handler:     handleAnalyzeRepository,
        NextTool:    "resolve_base_images",
        ChainReason: "Generate Dockerfile after analysis",
    },
    // ... more tools
}
```

### 3. Typed Workflow State

Using typed structs instead of `map[string]interface{}`:

```go
type SimpleWorkflowState struct {
    SessionID      string              `json:"session_id"`
    RepoPath       string              `json:"repo_path"`
    Status         string              `json:"status"`
    CurrentStep    string              `json:"current_step"`
    CompletedSteps []string            `json:"completed_steps"`
    Artifacts      *WorkflowArtifacts  `json:"artifacts,omitempty"`
    Metadata       *ToolMetadata       `json:"metadata,omitempty"`
}
```

### 4. Atomic Session Operations

Concurrent-safe session updates:

```go
// Atomic update pattern
err := AtomicUpdateWorkflowState(ctx, sessionManager, sessionID, 
    func(state *SimpleWorkflowState) error {
        state.MarkStepCompleted("analyze_repository")
        state.Artifacts.AnalyzeResult = result
        return nil
    })
```

### 5. Progress Tracking

Built-in progress emitters for real-time feedback:

```go
emitter := progressFactory.CreateEmitter(sessionID, "build_image")
emitter.UpdateProgress(0.5, "Building layer 3 of 6")
emitter.Complete("Build successful")
```

## Working with MCP Tools

### Adding a New Tool

1. **Define the tool configuration** in `pkg/service/tools/registry.go`:
```go
{
    Name:        "your_tool",
    Description: "Tool description",
    Category:    CategoryWorkflow,
    Handler:     handleYourTool,
}
```

2. **Implement the handler**:
```go
func handleYourTool(ctx context.Context, params map[string]interface{}) (interface{}, error) {
    // Load session state
    state, err := LoadWorkflowState(ctx, sessionManager, sessionID)
    
    // Perform tool logic
    result := performOperation()
    
    // Update state
    state.MarkStepCompleted("your_tool")
    
    // Save state
    return result, SaveWorkflowState(ctx, sessionManager, state)
}
```

3. **Add tests** in `pkg/service/tools/your_tool_test.go`

### Tool Categories

- **Workflow Tools** (10): Core containerization steps
- **Orchestration Tools** (2): Workflow management
- **Utility Tools** (3): Helper functions

## Common Development Tasks

### Running the MCP Server

```bash
# Start in stdio mode (default)
./containerization-assist-mcp

# With debug logging
LOG_LEVEL=debug ./containerization-assist-mcp

# Non-interactive mode for testing
NON_INTERACTIVE=true ./containerization-assist-mcp
```

### Updating Dependencies

```bash
# Add a new dependency
go get github.com/some/package

# Update go.mod and go.sum
go mod tidy

# Verify dependencies
go mod verify
```

### Working with Docker

```bash
# Build Docker image
docker build -t containerization-assist:dev .

# Run in container
docker run -it containerization-assist:dev
```

## Debugging and Troubleshooting

### Enable Debug Logging

```bash
# Set log level
export LOG_LEVEL=debug

# Or in code
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))
```

### Common Issues

#### Session State Not Persisting
- Check BoltDB file permissions
- Verify session ID format
- Ensure atomic operations are used

#### Tool Chain Not Working
- Verify tool registration in registry
- Check NextTool configuration
- Review session state between tools

#### AI Features Not Working
- Verify Azure OpenAI credentials
- Check NON_INTERACTIVE mode setting
- Review prompt templates in `pkg/infrastructure/ai_ml/prompts/templates/`

### Performance Profiling

```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof

# Memory profiling
go test -memprofile=mem.prof -bench=.
go tool pprof mem.prof
```

## Contributing

### Pull Request Process

1. Create feature branch from `main`
2. Make changes following coding standards
3. Add/update tests
4. Update documentation if needed
5. Run `make check-all` locally
6. Submit PR with clear description

### Commit Messages

Follow conventional commits:
```
feat: add new tool for deployment verification
fix: resolve session race condition
docs: update architecture diagram
test: add integration tests for workflow
refactor: simplify error handling
```

### Code Review Checklist

- [ ] Code follows style guidelines
- [ ] Tests are comprehensive and pass
- [ ] Documentation is updated
- [ ] No hardcoded values or secrets
- [ ] Error handling is appropriate
- [ ] No `fmt.Print*` statements
- [ ] Imports are properly organized

## Additional Resources

- [MCP Protocol Documentation](https://github.com/mark3labs/mcp-go)
- [Azure OpenAI Documentation](https://learn.microsoft.com/en-us/azure/ai-services/openai/)
- [Go Best Practices](https://go.dev/doc/effective_go)
- [Domain-Driven Design](https://martinfowler.com/bliki/DomainDrivenDesign.html)

---

**Version**: 1.0
**Last Updated**: 2025-08-22
**Maintained By**: Containerization Assist Team