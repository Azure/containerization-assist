# Development Guidelines

This document establishes coding standards and development practices for the Container Kit MCP project to ensure consistent, maintainable, and high-quality code across all workstreams.

## Table of Contents

- [Code Style Standards](#code-style-standards)
- [Testing Requirements](#testing-requirements)
- [Documentation Standards](#documentation-standards)
- [Security Guidelines](#security-guidelines)
- [Error Handling](#error-handling)
- [Performance Standards](#performance-standards)
- [File Organization](#file-organization)
- [CI/CD Integration](#cicd-integration)
- [Code Review Process](#code-review-process)

## Code Style Standards

### Go-Specific Guidelines

#### Package Structure
- Use clear, descriptive package names
- Avoid `util`, `common`, or `misc` package names
- Keep packages focused on a single responsibility
- Maximum 800 lines per file (see [File Organization](#file-organization))

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

// Types: Use CamelCase for exported, camelCase for internal
type AtomicBuildImageTool struct{}
type buildContext struct{}
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

#### Error Handling Standards
- Always handle errors explicitly - never ignore with `_`
- Use `fmt.Errorf` with `%w` verb for error wrapping
- Create custom error types for domain-specific errors
- Use `errors.Is()` and `errors.As()` for error checking

```go
// Good error handling
result, err := someOperation()
if err != nil {
    return fmt.Errorf("failed to execute operation: %w", err)
}

// Custom error types
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed for %s: %s", e.Field, e.Message)
}
```

### Code Quality Standards

#### No Print Statements
- **NEVER** use `fmt.Print*`, `log.Print*`, or `print()` statements
- Use structured logging with `zerolog` consistently
```go
// Bad
fmt.Println("Starting build process")
log.Printf("Build failed: %v", err)

// Good
logger.Info().Str("session_id", sessionID).Msg("Starting build process")
logger.Error().Err(err).Str("session_id", sessionID).Msg("Build failed")
```

#### Dependency Management
- Only import dependencies that are already in use in the codebase
- Check `go.mod` and existing code before adding new dependencies
- Prefer standard library over external dependencies when feasible

## Testing Requirements

### Coverage Standards
- **Minimum 70% test coverage** for `pkg/mcp` packages
- All new code must include comprehensive tests
- Critical path functions require 90%+ coverage

### Test Organization
```go
func TestFunctionName_Scenario(t *testing.T) {
    // Test structure following AAA pattern
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

### Test Helpers
```go
// Test helpers must call t.Helper()
func mustWriteFile(t testing.TB, name string, data []byte, perm os.FileMode) {
    t.Helper()
    if err := os.WriteFile(name, data, perm); err != nil {
        t.Fatalf("Failed to write file %s: %v", name, err)
    }
}

func setupTestSession(t *testing.T, sessionID string) *SessionState {
    t.Helper()
    return &SessionState{
        SessionID: sessionID,
        CreatedAt: time.Now(),
        // ... other fields
    }
}
```

### Benchmarking
- Add benchmarks for critical path functions
- Target: <300μs P95 per request for MCP operations
- Use `testing.B` with proper setup/teardown

## Documentation Standards

### GoDoc Comments
- **100% documentation** for all exported identifiers
- Follow standard Go comment format
```go
// AtomicBuildImageTool provides atomic Docker image building capabilities
// with comprehensive error handling and session state management.
type AtomicBuildImageTool struct {
    adapter    PipelineAdapter
    sessionMgr SessionManager
    logger     zerolog.Logger
}

// Execute performs a Docker image build operation with the given arguments.
// It returns detailed build results including context analysis and recommendations.
//
// The operation is atomic - either the entire build succeeds or fails with
// detailed error information for troubleshooting.
func (t *AtomicBuildImageTool) Execute(ctx context.Context, args AtomicBuildImageArgs) (interface{}, error) {
    // implementation
}
```

### Code Comments
- Explain **why**, not **what**
- Comment complex algorithms and business logic
- Use TODO comments with issue references for future work
```go
// TODO(issue-123): Implement exponential backoff retry logic
// for improved reliability in unstable network conditions

// We use a ring buffer here to efficiently manage build logs
// while preventing memory bloat in long-running builds
buffer := NewRingBuffer(maxLogLines)
```

## Security Guidelines

### Input Validation
- Validate all external inputs (user params, file paths, URLs)
- Use `filepath.Clean()` for all path operations
- Sanitize data before logging to prevent log injection

```go
func validateImageName(name string) error {
    if name == "" {
        return &ValidationError{Field: "image_name", Message: "cannot be empty"}
    }
    // Additional validation logic
    return nil
}

func sanitizePath(path string) string {
    return filepath.Clean(path)
}
```

### Secure Defaults
- Set restrictive file permissions (0600 for sensitive files)
- Never log secrets or credentials
- Use secure random generation for security-sensitive operations

### Security Linting
- Enable `gosec` linter and address all security issues
- Regular security audits using `make lint`
- Use `.golangci.yml` security-focused rules

## Error Handling

### RichError Pattern (In Progress)
```go
// Custom error type with structured context
type RichError struct {
    Code    string
    Message string
    Context map[string]interface{}
    Err     error
}

func (e *RichError) Error() string {
    return e.Message
}

func (e *RichError) Unwrap() error {
    return e.Err
}

// Usage
return &RichError{
    Code:    "BUILD_FAILED",
    Message: "Docker build failed during RUN step",
    Context: map[string]interface{}{
        "dockerfile_line": 15,
        "command":        "RUN npm install",
        "exit_code":      1,
    },
    Err: originalError,
}
```

### Error Context
- Always provide actionable error messages
- Include relevant context (session ID, file paths, etc.)
- Suggest remediation steps where possible

## Performance Standards

### Benchmarking Requirements
- Benchmark critical path functions
- Monitor P95 latency targets
- Use `make bench` for performance validation

### Memory Management
- Avoid memory leaks in long-running operations
- Use context cancellation appropriately
- Implement timeouts for external operations

### Concurrency Guidelines
- Use `-race` flag in all tests
- Implement proper synchronization for shared state
- Prefer channels over shared memory

## File Organization

### File Size Limits
- **Maximum 800 lines per file**
- Split large files using these strategies:
  - Extract helper functions to separate files
  - Split by functional areas (e.g., `tool_validation.go`, `tool_execution.go`)
  - Create focused sub-packages

### Directory Structure
```
pkg/mcp/internal/
├── tools/           # Tool implementations
├── core/           # Core server functionality
├── types/          # Shared type definitions
├── utils/          # Shared utilities
├── transport/      # Transport layer
└── orchestration/  # Workflow orchestration
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
    "github.com/Azure/container-kit/pkg/mcp/internal/types"
    "github.com/Azure/container-kit/pkg/mcp/utils"
)
```

## CI/CD Integration

### Required Checks
All code must pass:
```bash
go build -tags mcp         # Compilation check
go test -race ./...        # All tests with race detection
golangci-lint run          # Linting (with error budget)
make bench                 # Performance benchmarks
```

### Quality Gates
- No `fmt.Print*` or `log.Print*` statements
- No security issues (gosec)
- Error budget compliance (see `docs/LINTING.md`)
- Test coverage ≥70% for new code

### Git Workflow
- Create feature branches from `main`
- Use descriptive commit messages
- Squash commits before merging
- Update documentation with code changes

## Code Review Process

### Review Checklist
- [ ] Code follows style guidelines
- [ ] Tests are comprehensive and pass
- [ ] Documentation is updated
- [ ] Security concerns addressed
- [ ] Performance impact considered
- [ ] Error handling is appropriate
- [ ] No hardcoded values or secrets

### Review Focus Areas
1. **Correctness**: Does the code do what it's supposed to do?
2. **Security**: Are there any security vulnerabilities?
3. **Performance**: Will this impact system performance?
4. **Maintainability**: Is the code easy to understand and modify?
5. **Testing**: Are all scenarios adequately tested?

## Enforcement

### Automated Checks
- Pre-commit hooks for formatting and basic linting
- CI pipeline enforces all quality gates
- Performance regression detection

### Manual Reviews
- All code changes require review by another team member
- Security-sensitive changes require security team review
- Performance-critical changes require performance team review

### Continuous Improvement
- Regular retrospectives on development practices
- Update guidelines based on lessons learned
- Share knowledge through team presentations

---

## Getting Help

- For clarification on guidelines: Open an issue with label `guidelines`
- For tool setup: See `.devcontainer/README.md`
- For linting issues: See `docs/LINTING.md`
- For architecture questions: See `ARCHITECTURE.md`

---

**Version**: 1.0
**Last Updated**: 2025-06-24
**Next Review**: Quarterly (2025-09-24)
