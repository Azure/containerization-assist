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
- Use the unified RichError system from `pkg/mcp/domain/errors/rich.go`
- Follow ADR-004 patterns for structured error handling
- Use `errors.Is()` and `errors.As()` for error checking

```go
// Use RichError system (ADR-004)
return errors.NewError().
    Code(errors.CodeValidationFailed).
    Type(errors.ErrTypeValidation).
    Severity(errors.SeverityMedium).
    Message("validation failed for field").
    Context("field", fieldName).
    Context("value", fieldValue).
    Suggestion("Check field format and try again").
    WithLocation().
    Build()
```

### Code Quality Standards

#### No Print Statements
- **NEVER** use `fmt.Print*`, `log.Print*`, or `print()` statements
- Use structured logging with `slog` (per ADR-003)
```go
// Bad
fmt.Println("Starting build process")
log.Printf("Build failed: %v", err)

// Good
logger.Info("Starting build process", "session_id", sessionID)
logger.Error("Build failed", "error", err, "session_id", sessionID)
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

### RichError Pattern (ADR-004)
```go
// Use unified RichError system from pkg/mcp/domain/errors/rich.go
return errors.NewError().
    Code(errors.CodeBuildFailed).
    Type(errors.ErrTypeBuild).
    Severity(errors.SeverityHigh).
    Message("Docker build failed during RUN step").
    Context("dockerfile_line", 15).
    Context("command", "RUN npm install").
    Context("exit_code", 1).
    Suggestion("Check package.json and dependency versions").
    WithLocation().
    Cause(originalError).
    Build()
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

### Three-Layer Architecture (ADR-001)
```
pkg/mcp/
├── domain/              # Domain layer - business logic (no dependencies)
│   ├── config/         # Configuration entities and validation
│   ├── containerization/ # Container operations (analyze, build, deploy, scan)
│   ├── errors/         # Rich error handling system
│   ├── security/       # Security policies and validation
│   ├── session/        # Session entities and rules
│   └── internal/       # Shared utilities
├── application/         # Application layer - orchestration (depends on domain)
│   ├── api/            # Canonical interface definitions
│   ├── commands/       # Command implementations
│   ├── core/           # Server lifecycle & registry
│   ├── orchestration/  # Tool coordination & workflows
│   └── services/       # Service interfaces
└── infra/              # Infrastructure layer - external integrations
    ├── adapters/       # Interface adapters
    ├── persistence/    # BoltDB storage
    ├── transport/      # MCP protocol transports
    └── templates/      # YAML templates
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
    "github.com/Azure/container-kit/pkg/mcp/domain/internal/types"
    "github.com/Azure/container-kit/pkg/mcp/domain/internal/utils"
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
- Error budget compliance (see `docs/QUALITY_STANDARDS.md`)
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

## Architecture Patterns

### 1. Template Method Pattern
Use when multiple implementations share common algorithm structure but differ in specific steps.

**Example**: Database detectors with shared detection logic
```go
// Base template
type BaseDetector struct {
    config    DatabaseDetectorConfig
    extractor ConnectionInfoExtractor
}

func (d *BaseDetector) Detect(repoPath string) ([]DetectedDatabase, error) {
    // Template method defining algorithm skeleton
    databases := d.detectFromDocker(repoPath, databases)
    databases = d.detectFromEnvironment(repoPath, databases)
    databases = d.detectFromConfigFiles(repoPath, databases)
    return databases, nil
}
```

### 2. Factory Pattern
Use for complex object creation with multiple variants.

### 3. Pipeline Pattern
Use for sequential processing steps where each step transforms data.

### 4. Service Container Pattern (ADR-006)
Use manual dependency injection for focused services:
```go
type ServiceContainer interface {
    SessionStore() SessionStore
    SessionState() SessionState
    BuildExecutor() BuildExecutor
    ToolRegistry() ToolRegistry
    WorkflowExecutor() WorkflowExecutor
    Scanner() Scanner
    ConfigValidator() ConfigValidator
    ErrorReporter() ErrorReporter
}
```

## Function Design Principles

### 1. Single Responsibility
Each function should have one clear purpose.

### 2. Cyclomatic Complexity < 10
Keep functions simple with minimal branching.

### 3. Parameter Limits
Use options pattern for functions with >5 parameters.

### 4. Safe Type Assertions
Always check type assertions to prevent panics.

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
- For architecture questions: See `docs/THREE_LAYER_ARCHITECTURE.md`

---

**Version**: 1.1
**Last Updated**: 2025-07-07
**Next Review**: Quarterly (2025-10-07)
