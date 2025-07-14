# Testing Conventions for Container Kit MCP

This document outlines the testing conventions and best practices for the Container Kit MCP codebase, following our 4-layer architecture.

## Table of Contents
- [Architecture Overview](#architecture-overview)
- [Testing Philosophy](#testing-philosophy)
- [Layer-Specific Testing](#layer-specific-testing)
- [Test Organization](#test-organization)
- [Testing Utilities](#testing-utilities)
- [Testing Patterns](#testing-patterns)
- [Examples](#examples)

## Architecture Overview

Our codebase follows a 4-layer clean architecture:
1. **API Layer** - Interface definitions and contracts
2. **Application Layer** - Orchestration and use cases
3. **Domain Layer** - Business logic and rules
4. **Infrastructure Layer** - External dependencies and implementations

## Testing Philosophy

1. **Test at the Right Level**: Each layer should be tested appropriately
2. **Use Test Doubles**: Mock external dependencies, not internal ones
3. **Favor Integration Tests**: Test real interactions when possible
4. **Keep Tests Simple**: Tests should be easy to read and maintain
5. **Follow AAA Pattern**: Arrange, Act, Assert

## Layer-Specific Testing

### API Layer Testing
- Focus on contract validation
- Verify interface implementations
- Use generated mocks for interfaces

### Application Layer Testing
- Test orchestration logic
- Mock infrastructure dependencies
- Verify correct delegation to domain layer

### Domain Layer Testing
- Pure unit tests (no I/O)
- Test business rules and logic
- Use value objects and test data builders

### Infrastructure Layer Testing
- Integration tests with real dependencies when possible
- Use test containers for databases
- Mock external APIs

## Test Organization

### File Naming
```
package_test.go      # Unit tests
integration_test.go  # Integration tests (build tag: integration)
benchmark_test.go    # Performance tests
```

### Test Function Naming
```go
func TestFunctionName_Scenario_ExpectedBehavior(t *testing.T)
func TestWorkflow_Execute_ReturnsErrorOnInvalidRepo(t *testing.T)
```

### Test Structure
```go
func TestExample(t *testing.T) {
    // Arrange
    logger := testutil.NewTestLogger(t)
    fixture := testutil.NewFixtureBuilder().
        WithRepoURL("https://github.com/test/repo").
        Build()
    
    // Act
    result, err := SystemUnderTest(fixture)
    
    // Assert
    testutil.AssertNoError(t, err, "execution")
    assert.Equal(t, expected, result)
}
```

## Testing Utilities

### Location
All shared testing utilities are in `pkg/mcp/infrastructure/testutil/`:
- `logger.go` - Test logger utilities
- `mocks.go` - Common mock implementations
- `fixtures.go` - Test data builders
- `assertions.go` - Custom assertion helpers

### Logger Testing
```go
// Create a test logger that captures output
logger := testutil.NewTestLogger(t)

// Run code that logs
myFunction(logger)

// Assert on log output
testutil.AssertLogged(t, logger, "expected message")
```

### Mock Builders
```go
// Create mock step provider
provider := testutil.NewMockStepProvider()
provider.SetStep("analyze", testutil.MockStepWithError("analyze", errors.New("fail")))

// Create mock progress tracker
tracker := testutil.NewMockProgressTracker()
// ... use tracker
testutil.AssertProgressUpdate(t, tracker, "build", 0.5)
```

### Fixture Builders
```go
// Use fluent builders for test data
ctx := testutil.NewFixtureBuilder().
    WithWorkflowID("test-123").
    WithTimeout(5 * time.Second).
    BuildContext()

args := testutil.NewFixtureBuilder().
    WithRepoURL("https://github.com/test/repo").
    WithBranch("develop").
    BuildArgs()
```

## Testing Patterns

### Table-Driven Tests
```go
func TestValidation(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"valid input", "hello", false},
        {"empty input", "", true},
        {"special chars", "hello@world", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := Validate(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Testing Error Conditions
```go
func TestErrorHandling(t *testing.T) {
    // Use our unified error system
    err := errors.NewError().
        Code(errors.CodeValidationFailed).
        Message("invalid input").
        Build()
    
    // Assert using custom helpers
    testutil.AssertRichError(t, err, errors.CodeValidationFailed, errors.ErrTypeValidation)
}
```

### Testing Async Operations
```go
func TestAsyncOperation(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    done := make(chan struct{})
    go func() {
        // async operation
        close(done)
    }()
    
    select {
    case <-done:
        // success
    case <-ctx.Done():
        t.Fatal("operation timed out")
    }
}
```

### Testing with Contexts
```go
func TestWithContext(t *testing.T) {
    // Create context with test values
    ctx := testutil.MockContext()
    ctx = workflow.WithWorkflowID(ctx, "test-workflow")
    
    // Verify context propagation
    result := FunctionUnderTest(ctx)
    assert.Equal(t, "test-workflow", result.WorkflowID)
}
```

## Examples

### Complete Unit Test Example
```go
package workflow_test

import (
    "testing"
    "github.com/Azure/container-kit/pkg/mcp/domain/workflow"
    "github.com/Azure/container-kit/pkg/mcp/infrastructure/testutil"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestBaseOrchestrator_Execute_Success(t *testing.T) {
    // Arrange
    logger := testutil.NewTestLogger(t)
    
    // Create mocks
    stepProvider := testutil.NewMockStepProvider()
    analyzeStep := testutil.NewMockStep("analyze")
    analyzeStep.ExecuteFunc = func(ctx context.Context, state *workflow.WorkflowState) error {
        state.Results["analyze"] = &workflow.StepResult{
            Success: true,
            Data:    testutil.AnalyzeResultFixture(),
        }
        return nil
    }
    stepProvider.SetStep("analyze", analyzeStep)
    
    // Create orchestrator
    factory := workflow.NewStepFactory(stepProvider, nil, nil, logger)
    orchestrator := workflow.NewBaseOrchestrator(factory, nil, logger)
    
    // Create test data
    ctx := testutil.NewFixtureBuilder().BuildContext()
    args := testutil.NewFixtureBuilder().BuildArgs()
    
    // Act
    result, err := orchestrator.Execute(ctx, nil, args)
    
    // Assert
    testutil.AssertWorkflowSuccess(t, result, err)
    assert.NotEmpty(t, result.ImageRef)
    testutil.AssertLogged(t, logger, "Workflow completed")
}

func TestBaseOrchestrator_Execute_StepFailure(t *testing.T) {
    // Arrange
    logger := testutil.NewTestLogger(t)
    
    // Create failing step
    stepProvider := testutil.NewMockStepProvider()
    stepProvider.SetStep("build", 
        testutil.MockStepWithError("build", errors.New("build failed")))
    
    factory := workflow.NewStepFactory(stepProvider, nil, nil, logger)
    orchestrator := workflow.NewBaseOrchestrator(factory, nil, logger)
    
    ctx := testutil.NewFixtureBuilder().BuildContext()
    args := testutil.NewFixtureBuilder().BuildArgs()
    
    // Act
    result, err := orchestrator.Execute(ctx, nil, args)
    
    // Assert
    testutil.AssertWorkflowError(t, result, err, "build failed")
    testutil.AssertLogged(t, logger, "Step failed")
}
```

### Integration Test Example
```go
//go:build integration

package steps_test

import (
    "testing"
    "github.com/Azure/container-kit/pkg/mcp/infrastructure/steps"
    "github.com/Azure/container-kit/pkg/mcp/infrastructure/testutil"
)

func TestAnalyzeStep_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    // Arrange
    logger := testutil.NewTestLogger(t)
    tempDir := t.TempDir()
    
    // Create test project structure
    testutil.CreateGoProject(t, tempDir)
    
    step := steps.NewAnalyzeStep()
    state := testutil.NewFixtureBuilder().
        WithRepoPath(tempDir).
        BuildWorkflowState()
    
    // Act
    err := step.Execute(context.Background(), state)
    
    // Assert
    testutil.AssertNoError(t, err, "analyze step")
    testutil.AssertStepResult(t, state, "analyze", true)
    
    result := state.Results["analyze"].Data.(*workflow.AnalyzeResult)
    assert.Equal(t, "go", result.Language)
}
```

## Best Practices

1. **Use Test Helpers**: Leverage `testutil` package for common operations
2. **Avoid Test Duplication**: Extract common setup into helper functions
3. **Test One Thing**: Each test should verify a single behavior
4. **Use Descriptive Names**: Test names should explain what is being tested
5. **Clean Up Resources**: Use `t.Cleanup()` or `defer` for cleanup
6. **Parallel Tests**: Use `t.Parallel()` where safe
7. **Skip Expensive Tests**: Use build tags for integration tests
8. **Mock at Boundaries**: Only mock at architectural boundaries

## Running Tests

```bash
# Run all unit tests
make test

# Run integration tests
make test-integration

# Run specific package tests
go test ./pkg/mcp/domain/workflow/...

# Run with coverage
go test -cover ./...

# Run benchmarks
go test -bench=. ./...
```