# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Essential Commands

### Build & Test
```bash
# Build MCP server (primary binary)
make mcp
# or: go build -tags mcp -o container-kit-mcp ./cmd/mcp-server

# Run all tests with race detection
make test-all

# Run MCP-specific tests 
make test-mcp

# Run tests with coverage (target: ≥70% for pkg/mcp)
make coverage

# Performance benchmarks (target: <300μs P95)
make bench
```

### Linting & Quality
```bash
# Lint with error budget (threshold: 100 issues)
make lint

# Format code 
make fmt

# Pre-commit checks
make pre-commit

# Coverage baseline and tracking
make coverage-baseline
```

### Development Workflow
```bash
# Test single package
go test -race ./pkg/mcp/internal/[package]/...

# Test specific function
go test -race -run TestFunctionName ./pkg/mcp/...

# Coverage for specific package
go test -cover ./pkg/mcp/internal/[package]/...

# Run server locally
./container-kit-mcp --transport stdio
```

## Architecture Overview

Container Kit is an AI-powered containerization tool with **dual-mode architecture**:

### 1. MCP Server (Primary) - Two Operation Modes
- **Atomic Tools**: Deterministic, composable operations (`analyze`, `build`, `deploy`, `scan`)
- **Conversation Mode**: AI-guided workflow through `chat` tool with stage-based progression

### 2. Legacy CLI (Pipeline-based)
- Three-stage iterative pipeline for direct execution

### Key Architectural Patterns

#### Unified Interface System
All tools implement the `Tool` interface defined in `pkg/mcp/interfaces.go`:
```go
type Tool interface {
    Execute(ctx context.Context, args interface{}) (interface{}, error)
    GetMetadata() ToolMetadata
    Validate(ctx context.Context, args interface{}) error
}
```

#### Import Cycle Prevention
- **Public interfaces**: `pkg/mcp/interfaces.go` (single source of truth)
- **Internal interfaces**: `pkg/mcp/types/interfaces.go` (lightweight, import-cycle safe)

#### Auto-Registration Pattern
Tools are auto-discovered and registered at startup via reflection and build tags.

## Code Structure

### Core Packages
```
pkg/mcp/
├── interfaces.go           # Unified interface definitions (main)
├── internal/
│   ├── core/              # MCP server core (server.go)
│   ├── orchestration/     # Tool coordination & workflow
│   ├── session/           # Session state management
│   ├── transport/         # stdio/HTTP communication
│   ├── analyze/           # Repository analysis tools
│   ├── build/             # Docker image building
│   ├── deploy/            # Kubernetes deployment
│   ├── scan/              # Security scanning
│   └── conversation/      # Chat tool & guided workflows
├── types/                 # Shared types & internal interfaces
└── utils/                 # Common utilities
```

### Key Components
- **Server**: `pkg/mcp/internal/core/server.go` - Main MCP protocol handler
- **Tool Registry**: Auto-registration in `pkg/mcp/internal/runtime/registry.go`
- **Session Manager**: `pkg/mcp/internal/session/session_manager.go`
- **Orchestrator**: `pkg/mcp/internal/orchestration/tool_orchestrator.go`
- **Transport**: `pkg/mcp/internal/transport/` (stdio/HTTP)

## Development Guidelines

### Code Quality Standards
- **No print statements**: Use `zerolog` for all logging
- **Error handling**: Prefer `RichError` pattern, always wrap errors with context
- **File size limit**: Maximum 800 lines per file
- **Test coverage**: Minimum 70% for `pkg/mcp/` packages
- **Performance**: Target <300μs P95 for atomic tools

### Testing Patterns
- Table-driven tests for multiple scenarios
- Use `t.Helper()` in test helper functions
- Mock external dependencies via interfaces
- Include both success and failure test cases

### Import Organization
```go
import (
    // Standard library
    "context"
    "fmt"
    
    // Third-party 
    "github.com/rs/zerolog"
    
    // Internal
    "github.com/Azure/container-kit/pkg/mcp/internal/types"
)
```

## Session & State Management

Sessions persist across tool executions with the following components:
- **Session State**: `pkg/mcp/internal/session/state.go`
- **Workspace Management**: Isolated working directories per session
- **Labels & Indexing**: Session metadata and querying
- **TTL Management**: Automatic cleanup of expired sessions

## Tool Development

### Adding New Tools
1. Implement the `Tool` interface
2. Add to appropriate package under `pkg/mcp/internal/`
3. Include comprehensive tests
4. Add metadata for auto-discovery
5. Follow atomic operation principles

### Atomic Tool Standards
- Single responsibility
- Idempotent operations
- Rich error reporting with context
- Session state updates
- Input validation

## Configuration

### Environment Variables
- `WORKSPACE_DIR`: Default workspace directory
- `SESSION_STORE_PATH`: Session persistence location
- `LOG_LEVEL`: Logging level (debug, info, warn, error)
- `TRANSPORT_TYPE`: Communication protocol (stdio, http)

### Build Tags
- `mcp`: Include MCP-specific functionality
- Build with: `go build -tags mcp`

## Common Issues & Solutions

### Import Cycles
- Use internal interfaces from `pkg/mcp/types/interfaces.go`
- Avoid direct imports between internal packages
- Leverage dependency injection patterns

### Test Coverage Gaps
Current packages below 15% coverage (Phase 1 targets):
- `pkg/mcp/types`: 0.0% → 15%
- `pkg/mcp/internal/analyze`: 6.3% → 15%
- `pkg/mcp/internal/build`: 5.7% → 15%
- `pkg/mcp/internal/deploy`: 7.1% → 15%
- `pkg/mcp/internal/orchestration`: 6.9% → 15%
- `pkg/mcp/internal/server`: 8.1% → 15%

### Linting Issues
The codebase uses a progressive linting approach with error budgets:
- Error threshold: 100 issues
- Warning threshold: 50 issues
- Run `make lint-report` for current status

## Security Considerations

- Input validation for all external parameters
- Path sanitization using `filepath.Clean()`
- No secrets in logs or error messages
- Security scanning via Trivy/Grype integration
- Sandboxed execution capabilities

## Performance Targets

- **MCP Operations**: <300μs P95 latency
- **Memory**: Stable usage over 24h operations
- **Concurrency**: Handle 100 concurrent operations
- **Benchmarking**: Run `make bench` for performance validation